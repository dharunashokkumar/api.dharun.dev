package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
	"github.com/dharunashokkumar/api.dharun.dev/internal/subword"
)

// textModelState holds what the text models need on the API's side: tokenizers (a 17 MB
// file takes a second or two to read, so one is kept while it is used) and other small
// tables, loaded on first use and dropped when their dataset changes or nobody has used
// them for as long as a model helper is kept.
type textModelState struct {
	mu     sync.Mutex
	loaded map[string]*loadedFile // by file path
	// byDigest finds a file already loaded under another path: several models share a
	// tokenizer file (XLM-RoBERTa's, for one), and it is held once.
	byDigest map[string]*loadedFile
}

type loadedFile struct {
	dataset string
	value   any
	used    time.Time
	digest  string
}

// load returns what read made of a file, reading it once.
func (t *textModelState) load(dataset, path string, read func(string) (any, error)) (any, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.loaded == nil {
		t.loaded = map[string]*loadedFile{}
	}
	if file, ok := t.loaded[path]; ok {
		file.used = time.Now()
		return file.value, nil
	}
	digest := fileDigest(path)
	if file, ok := t.byDigest[digest]; ok && digest != "" {
		file.used = time.Now()
		t.loaded[path] = &loadedFile{dataset: dataset, value: file.value, used: time.Now(), digest: digest}
		return file.value, nil
	}
	value, err := read(path)
	if err != nil {
		return nil, err
	}
	file := &loadedFile{dataset: dataset, value: value, used: time.Now(), digest: digest}
	t.loaded[path] = file
	if digest != "" {
		if t.byDigest == nil {
			t.byDigest = map[string]*loadedFile{}
		}
		t.byDigest[digest] = file
	}
	return value, nil
}

// fileDigest is a file's SHA-256, or "" when it cannot be read.
func fileDigest(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (t *textModelState) tokenizer(dataset, path string) (*subword.Tokenizer, error) {
	if path == "" {
		return nil, errors.New("tokenizer file missing")
	}
	value, err := t.load(dataset, path, func(path string) (any, error) { return subword.LoadFile(path) })
	if err != nil {
		return nil, err
	}
	return value.(*subword.Tokenizer), nil
}

func (t *textModelState) forget(dataset string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for path, file := range t.loaded {
		if file.dataset == dataset {
			delete(t.loaded, path)
		}
	}
	t.prune()
}

// prune forgets shared files no path uses any more.
func (t *textModelState) prune() {
	inUse := map[string]bool{}
	for _, file := range t.loaded {
		inUse[file.digest] = true
	}
	for digest := range t.byDigest {
		if !inUse[digest] {
			delete(t.byDigest, digest)
		}
	}
}

func (t *textModelState) dropIdle(after time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for path, file := range t.loaded {
		if time.Since(file.used) > after {
			delete(t.loaded, path)
		}
	}
	t.prune()
}

// The kinds of text model helper and the memory each takes with all its models loaded,
// measured on Linux with ONNX Runtime 1.29 (2026-10-15), rounded up: embeddings 171 MB,
// names 355 MB, moderation 863 MB, translation both ways 1,069 MB, transliteration
// 123 MB; the reranker is estimated from its 570 MB of weights.
var (
	embeddingsKind  = modelKind{name: "embeddings", memory: 250 << 20}
	rerankerKind    = modelKind{name: "reranker", memory: 900 << 20}
	namesKind       = modelKind{name: "names", memory: 400 << 20}
	moderationKind  = modelKind{name: "moderation", memory: 900 << 20}
	translationKind = modelKind{name: "translation", memory: 1100 << 20}
	xlitKind        = modelKind{name: "xlit", memory: 150 << 20}
)

// batch is token IDs padded into one rectangle, with the mask and segment IDs BERT-like
// models take.
type batch struct {
	ids, mask, types []int64
	rows, width      int64
}

func newBatch(sequences [][]int64, pad int64) batch {
	b := batch{rows: int64(len(sequences))}
	for _, seq := range sequences {
		b.width = max(b.width, int64(len(seq)))
	}
	for _, seq := range sequences {
		for i := int64(0); i < b.width; i++ {
			if i < int64(len(seq)) {
				b.ids = append(b.ids, seq[i])
				b.mask = append(b.mask, 1)
			} else {
				b.ids = append(b.ids, pad)
				b.mask = append(b.mask, 0)
			}
			b.types = append(b.types, 0)
		}
	}
	return b
}

func (b batch) inputs() []onnxwire.Tensor {
	shape := []int64{b.rows, b.width}
	return []onnxwire.Tensor{
		{Name: "input_ids", Type: onnxwire.Int64, Shape: shape, Ints: b.ids},
		{Name: "attention_mask", Type: onnxwire.Int64, Shape: shape, Ints: b.mask},
		{Name: "token_type_ids", Type: onnxwire.Int64, Shape: shape, Ints: b.types},
	}
}

// readJSON reads a JSON request body into v, answering the error itself. The body is
// read as JSON whatever Content-Type says, as clients often send text/plain.
func (s *Server) readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(s.cfg.MaxBodyBytes)))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The body is larger than "+byteSize(s.cfg.MaxBodyBytes)+".")
			return false
		}
		s.badRequest(w, r, "The request body could not be read.")
		return false
	}
	if kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); kind != "application/json" && !json.Valid(raw) {
		// Not JSON: the body is one text.
		if one, ok := v.(interface{ setText(string) }); ok {
			one.setText(string(raw))
			return true
		}
	}
	if err := json.Unmarshal(raw, v); err != nil {
		s.badRequest(w, r, "The body is not valid JSON of the expected shape: "+jsonProblem(err))
		return false
	}
	return true
}

func jsonProblem(err error) string {
	var typed *json.UnmarshalTypeError
	if errors.As(err, &typed) && typed.Field != "" {
		return typed.Field + " has the wrong type."
	}
	return "it could not be parsed."
}
