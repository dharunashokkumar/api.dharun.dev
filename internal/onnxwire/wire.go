// Package onnxwire is how the API talks to `onnx-run serve`, the long-lived helper that
// keeps ONNX models loaded while they are in use. Requests and answers go over the
// helper's standard input and output, one after the other, little-endian:
//
//	request  'R' model name, inputs (name, tensor), wanted output names
//	         'G' a sequence-to-sequence generation, run entirely inside the helper
//	answer   1 and the result, or 0 and a message
//
// The package has no cgo, so the API can use it; the helper links ONNX Runtime.
package onnxwire

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// Element types of a tensor.
const (
	Float32 byte = 1
	Int64   byte = 2
)

// Operations.
const (
	OpRun      byte = 'R'
	OpGenerate byte = 'G'
)

// Tensor is a named tensor of float32 or int64 values.
type Tensor struct {
	Name   string
	Type   byte
	Shape  []int64
	Floats []float32
	Ints   []int64
}

// Size is the number of values the shape holds.
func (t Tensor) Size() int64 {
	n := int64(1)
	for _, d := range t.Shape {
		n *= d
	}
	return n
}

// RunRequest runs one model once.
type RunRequest struct {
	Model   string
	Inputs  []Tensor
	Outputs []string
}

// GenerateRequest translates one sequence with an encoder-decoder model. With DecoderPast
// set it decodes greedily and keeps the attention cache between steps; without it, it
// runs a beam search that feeds the decoder everything decoded so far.
type GenerateRequest struct {
	Encoder, Decoder, DecoderPast string
	Input                         []int64 // the source token IDs, ending in the end token
	Start, End                    int64   // the decoder's first token and the end token
	MaxLength                     int
	Beams                         int
	Best                          int   // how many finished sequences to return, at most Beams
	VocabLimit                    int64 // IDs at or above it are never chosen; 0 for none
	Banned                        []int64
}

// Sequence is one generated sequence with its mean log probability per token.
type Sequence struct {
	Score float32
	IDs   []int64
}

const maxValues = 64 << 20 // values in one tensor
const maxDims = 8

// Writer and Reader helpers -------------------------------------------------------

type writer struct {
	w   *bufio.Writer
	err error
}

func (w *writer) bytes(b []byte) {
	if w.err == nil {
		_, w.err = w.w.Write(b)
	}
}
func (w *writer) u8(v byte) { w.bytes([]byte{v}) }
func (w *writer) u32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	w.bytes(b[:])
}
func (w *writer) i64(v int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	w.bytes(b[:])
}
func (w *writer) str(s string) { w.u32(uint32(len(s))); w.bytes([]byte(s)) }
func (w *writer) ints(v []int64) {
	w.u32(uint32(len(v)))
	for _, x := range v {
		w.i64(x)
	}
}

func (w *writer) tensor(t Tensor) {
	w.str(t.Name)
	w.u8(t.Type)
	w.u8(byte(len(t.Shape)))
	for _, d := range t.Shape {
		w.i64(d)
	}
	switch t.Type {
	case Float32:
		buf := make([]byte, 4*len(t.Floats))
		for i, v := range t.Floats {
			binary.LittleEndian.PutUint32(buf[4*i:], math.Float32bits(v))
		}
		w.bytes(buf)
	case Int64:
		buf := make([]byte, 8*len(t.Ints))
		for i, v := range t.Ints {
			binary.LittleEndian.PutUint64(buf[8*i:], uint64(v))
		}
		w.bytes(buf)
	default:
		if w.err == nil {
			w.err = fmt.Errorf("tensor %q has an unknown type", t.Name)
		}
	}
}

type reader struct {
	r   *bufio.Reader
	err error
}

func (r *reader) full(b []byte) {
	if r.err == nil {
		_, r.err = io.ReadFull(r.r, b)
	}
}
func (r *reader) u8() byte {
	var b [1]byte
	r.full(b[:])
	return b[0]
}
func (r *reader) u32() uint32 {
	var b [4]byte
	r.full(b[:])
	return binary.LittleEndian.Uint32(b[:])
}
func (r *reader) i64() int64 {
	var b [8]byte
	r.full(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}
func (r *reader) str() string {
	n := r.u32()
	if n > 1<<16 {
		r.fail("string too long")
		return ""
	}
	b := make([]byte, n)
	r.full(b)
	return string(b)
}
func (r *reader) ints() []int64 {
	n := r.u32()
	if n > maxValues {
		r.fail("too many values")
		return nil
	}
	v := make([]int64, n)
	for i := range v {
		v[i] = r.i64()
	}
	return v
}
func (r *reader) fail(message string) {
	if r.err == nil {
		r.err = errors.New(message)
	}
}

func (r *reader) tensor() Tensor {
	t := Tensor{Name: r.str(), Type: r.u8()}
	dims := int(r.u8())
	if dims > maxDims {
		r.fail("too many dimensions")
		return t
	}
	size := int64(1)
	for range dims {
		d := r.i64()
		if d < 0 || d > maxValues {
			r.fail("bad dimension")
			return t
		}
		t.Shape = append(t.Shape, d)
		size *= d
	}
	if size > maxValues {
		r.fail("tensor too large")
		return t
	}
	switch t.Type {
	case Float32:
		buf := make([]byte, 4*size)
		r.full(buf)
		t.Floats = make([]float32, size)
		for i := range t.Floats {
			t.Floats[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[4*i:]))
		}
	case Int64:
		buf := make([]byte, 8*size)
		r.full(buf)
		t.Ints = make([]int64, size)
		for i := range t.Ints {
			t.Ints[i] = int64(binary.LittleEndian.Uint64(buf[8*i:]))
		}
	default:
		r.fail("unknown tensor type")
	}
	return t
}

// Conn is one side of the conversation.
type Conn struct {
	r reader
	w writer
}

// NewConn wraps a pair of streams.
func NewConn(in io.Reader, out io.Writer) *Conn {
	return &Conn{r: reader{r: bufio.NewReaderSize(in, 1<<20)}, w: writer{w: bufio.NewWriterSize(out, 1<<20)}}
}

func (c *Conn) flush() error {
	if c.w.err == nil {
		c.w.err = c.w.w.Flush()
	}
	err := c.w.err
	c.w.err = nil
	return err
}

// The client's side ---------------------------------------------------------------

// Run sends a run request and reads its outputs.
func (c *Conn) Run(req RunRequest) ([]Tensor, error) {
	c.w.u8(OpRun)
	c.w.str(req.Model)
	c.w.u32(uint32(len(req.Inputs)))
	for _, t := range req.Inputs {
		c.w.tensor(t)
	}
	c.w.u32(uint32(len(req.Outputs)))
	for _, name := range req.Outputs {
		c.w.str(name)
	}
	if err := c.flush(); err != nil {
		return nil, err
	}
	if err := c.answer(); err != nil {
		return nil, err
	}
	n := c.r.u32()
	if n > 256 {
		return nil, errors.New("onnxwire: too many outputs")
	}
	out := make([]Tensor, n)
	for i := range out {
		out[i] = c.r.tensor()
	}
	return out, c.takeErr()
}

// Generate sends a generation request and reads the sequences.
func (c *Conn) Generate(req GenerateRequest) ([]Sequence, error) {
	c.w.u8(OpGenerate)
	c.w.str(req.Encoder)
	c.w.str(req.Decoder)
	c.w.str(req.DecoderPast)
	c.w.ints(req.Input)
	c.w.i64(req.Start)
	c.w.i64(req.End)
	c.w.u32(uint32(req.MaxLength))
	c.w.u32(uint32(req.Beams))
	c.w.u32(uint32(req.Best))
	c.w.i64(req.VocabLimit)
	c.w.ints(req.Banned)
	if err := c.flush(); err != nil {
		return nil, err
	}
	if err := c.answer(); err != nil {
		return nil, err
	}
	n := c.r.u32()
	if n > 64 {
		return nil, errors.New("onnxwire: too many sequences")
	}
	out := make([]Sequence, n)
	for i := range out {
		out[i].Score = math.Float32frombits(c.r.u32())
		out[i].IDs = c.r.ints()
	}
	return out, c.takeErr()
}

// ModelError is a failure the helper reported; the helper itself is still fine.
type ModelError struct{ Message string }

func (e *ModelError) Error() string { return "onnx-run: " + e.Message }

func (c *Conn) answer() error {
	status := c.r.u8()
	if c.r.err != nil {
		return c.takeErr()
	}
	if status == 0 {
		message := c.r.str()
		if err := c.takeErr(); err != nil {
			return err
		}
		return &ModelError{Message: message}
	}
	return nil
}

func (c *Conn) takeErr() error {
	err := c.r.err
	c.r.err = nil
	return err
}

// The helper's side ---------------------------------------------------------------

// Handler answers requests inside the helper.
type Handler interface {
	Run(RunRequest) ([]Tensor, error)
	Generate(GenerateRequest) ([]Sequence, error)
}

// Serve reads requests until its input ends. A request that cannot be read ends it; a
// request the handler fails is answered with the message and the helper goes on.
func (c *Conn) Serve(h Handler) error {
	for {
		op := c.r.u8()
		if c.r.err != nil {
			if errors.Is(c.r.err, io.EOF) {
				return nil
			}
			return c.r.err
		}
		switch op {
		case OpRun:
			req := RunRequest{Model: c.r.str()}
			n := c.r.u32()
			if n > 64 {
				return errors.New("too many inputs")
			}
			for range n {
				req.Inputs = append(req.Inputs, c.r.tensor())
			}
			n = c.r.u32()
			if n > 256 {
				return errors.New("too many outputs")
			}
			for range n {
				req.Outputs = append(req.Outputs, c.r.str())
			}
			if err := c.takeErr(); err != nil {
				return err
			}
			out, err := h.Run(req)
			if err != nil {
				c.w.u8(0)
				c.w.str(err.Error())
			} else {
				c.w.u8(1)
				c.w.u32(uint32(len(out)))
				for _, t := range out {
					c.w.tensor(t)
				}
			}
		case OpGenerate:
			req := GenerateRequest{Encoder: c.r.str(), Decoder: c.r.str(), DecoderPast: c.r.str(), Input: c.r.ints()}
			req.Start, req.End = c.r.i64(), c.r.i64()
			req.MaxLength, req.Beams, req.Best = int(c.r.u32()), int(c.r.u32()), int(c.r.u32())
			req.VocabLimit = c.r.i64()
			req.Banned = c.r.ints()
			if err := c.takeErr(); err != nil {
				return err
			}
			out, err := h.Generate(req)
			if err != nil {
				c.w.u8(0)
				c.w.str(err.Error())
			} else {
				c.w.u8(1)
				c.w.u32(uint32(len(out)))
				for _, s := range out {
					c.w.u32(math.Float32bits(s.Score))
					c.w.ints(s.IDs)
				}
			}
		default:
			return fmt.Errorf("unknown operation %q", op)
		}
		if err := c.flush(); err != nil {
			return err
		}
	}
}
