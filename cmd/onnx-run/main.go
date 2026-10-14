//go:build cgo

// Command onnx-run runs one ONNX model once: it reads a float32 tensor (little-endian,
// in the shape given) on standard input and writes the model's first output, also as
// little-endian float32, on standard output. The API starts it for each job that needs a
// neural network, so the memory a model takes is given back as soon as the job ends.
//
//	onnx-run --model isnet.onnx --shape 1,3,1024,1024 --threads 2 < in.f32 > out.f32
//
// It links ONNX Runtime (MIT) at run time from libonnxruntime.so, found next to this
// program or named by ONNXRUNTIME_LIB.
package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "onnx-run:", err)
		os.Exit(1)
	}
}

func run() error {
	model := flag.String("model", "", "the .onnx file")
	shapeText := flag.String("shape", "", "the input's shape, such as 1,3,1024,1024")
	threads := flag.Int("threads", 1, "threads used inside one operation")
	flag.Parse()
	if *model == "" || *shapeText == "" {
		return errors.New("--model and --shape are required")
	}
	var dims []int64
	size := int64(1)
	for _, part := range strings.Split(*shapeText, ",") {
		n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || n < 1 || n > 1<<16 {
			return fmt.Errorf("bad shape %q", *shapeText)
		}
		dims = append(dims, n)
		size *= n
	}
	if size > 64<<20 {
		return errors.New("input too large")
	}

	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 4*size+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) != 4*size {
		return fmt.Errorf("expected %d bytes of input, got %d", 4*size, len(raw))
	}
	input := make([]float32, size)
	for i := range input {
		input[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
	}
	raw = nil

	library := os.Getenv("ONNXRUNTIME_LIB")
	if library == "" {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		matches, _ := filepath.Glob(filepath.Join(filepath.Dir(self), "libonnxruntime.so*"))
		if len(matches) == 0 {
			return errors.New("libonnxruntime.so not found next to onnx-run; set ONNXRUNTIME_LIB")
		}
		library = matches[0]
	}
	ort.SetSharedLibraryPath(library)
	if err := ort.InitializeEnvironment(); err != nil {
		return err
	}
	defer ort.DestroyEnvironment()

	inputs, outputs, err := ort.GetInputOutputInfo(*model)
	if err != nil {
		return err
	}
	if len(inputs) != 1 || len(outputs) == 0 {
		return fmt.Errorf("the model has %d inputs and %d outputs; one input is supported", len(inputs), len(outputs))
	}
	options, err := ort.NewSessionOptions()
	if err != nil {
		return err
	}
	defer options.Destroy()
	options.SetIntraOpNumThreads(max(1, *threads))
	options.SetInterOpNumThreads(1)
	// One run and out: the arena and memory planning only keep memory for runs that never
	// come, and without them the peak is lower.
	if os.Getenv("ONNX_ARENA") == "" {
		options.SetCpuMemArena(false)
		options.SetMemPattern(false)
	}
	session, err := ort.NewDynamicAdvancedSession(*model, []string{inputs[0].Name}, []string{outputs[0].Name}, options)
	if err != nil {
		return err
	}
	defer session.Destroy()

	tensor, err := ort.NewTensor(ort.NewShape(dims...), input)
	if err != nil {
		return err
	}
	defer tensor.Destroy()
	results := []ort.Value{nil}
	if err := session.Run([]ort.Value{tensor}, results); err != nil {
		return err
	}
	defer results[0].Destroy()
	output, ok := results[0].(*ort.Tensor[float32])
	if !ok {
		return errors.New("the model's first output is not float32")
	}
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	var word [4]byte
	for _, v := range output.GetData() {
		binary.LittleEndian.PutUint32(word[:], math.Float32bits(v))
		out.Write(word[:])
	}
	return out.Flush()
}
