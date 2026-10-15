package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
)

// Text models (round 3): embeddings, reranking, name detection, moderation, translation
// and transliteration. They answer many small questions, so unlike background removal
// they are not started for each request: each kind gets an `onnx-run serve` helper that
// keeps its models loaded while it is used and is stopped once it has been idle for a
// while, which gives its memory back. The helpers together stay under a memory budget:
// starting one that does not fit stops the one used longest ago, if it is idle.
//
// A helper answers one request at a time; ONNX Runtime uses several threads inside each.
// Requests for the same kind wait in line for it, and give up after textModelWait.

const (
	textModelWait     = 30 * time.Second
	textModelCallTime = 80 * time.Second
)

// modelKind is one helper: which dataset it serves and roughly how much memory its
// models take once loaded.
type modelKind struct {
	name    string // also the dataset's name
	memory  int64  // bytes, as measured with every model of the kind loaded
	threads int
}

type modelWorker struct {
	kind    modelKind
	command *exec.Cmd
	input   io.WriteCloser
	conn    *onnxwire.Conn
	slot    chan struct{} // held while a request is answered
	used    time.Time
	stderr  *limitedBuffer
	broken  bool
}

type textModels struct {
	mu      sync.Mutex
	workers map[string]*modelWorker
	budget  int64
	idle    time.Duration
	threads int
	start   func(kind modelKind) (*modelWorker, error)
}

func newTextModels(cfg Config, programs *programs) *textModels {
	m := &textModels{workers: map[string]*modelWorker{}, budget: cfg.TextModelMemory, idle: cfg.TextModelIdle, threads: cfg.TextModelThreads}
	if m.budget <= 0 {
		m.budget = 2 << 30
	}
	if m.idle <= 0 {
		m.idle = 10 * time.Minute
	}
	if m.threads <= 0 {
		m.threads = 4
	}
	m.start = func(kind modelKind) (*modelWorker, error) {
		path := programs.path("onnx-run")
		if path == "" {
			return nil, errNoProgram
		}
		threads := kind.threads
		if threads == 0 {
			threads = m.threads
		}
		command := exec.Command(path, "serve", "--threads", itoa(threads))
		command.Env = minimalEnv()
		stdin, err := command.StdinPipe()
		if err != nil {
			return nil, err
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			return nil, err
		}
		problems := &limitedBuffer{max: 8 << 10}
		command.Stderr = problems
		if err := command.Start(); err != nil {
			return nil, err
		}
		return &modelWorker{kind: kind, command: command, input: stdin, conn: onnxwire.NewConn(stdout, stdin), stderr: problems}, nil
	}
	return m
}

func (w *modelWorker) stop() {
	w.input.Close()
	done := make(chan struct{})
	go func() {
		w.command.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		w.command.Process.Kill()
		<-done
	}
}

// worker returns the running helper of a kind, starting it if need be.
func (m *textModels) worker(kind modelKind) (*modelWorker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.workers[kind.name]; ok && !w.broken {
		return w, nil
	}
	// Make room: stop idle helpers, the one used longest ago first.
	var total int64
	var idle []*modelWorker
	for _, w := range m.workers {
		total += w.kind.memory
		if len(w.slot) == 0 {
			idle = append(idle, w)
		}
	}
	sort.Slice(idle, func(a, b int) bool { return idle[a].used.Before(idle[b].used) })
	for _, w := range idle {
		if total+kind.memory <= m.budget {
			break
		}
		delete(m.workers, w.kind.name)
		total -= w.kind.memory
		go w.stop()
	}
	w, err := m.start(kind)
	if err != nil {
		return nil, err
	}
	w.slot = make(chan struct{}, 1)
	w.used = time.Now()
	m.workers[kind.name] = w
	return w, nil
}

// use runs fn with a kind's helper, waiting for it if another request has it.
func (m *textModels) use(ctx context.Context, kind modelKind, fn func(*onnxwire.Conn) error) error {
	w, err := m.worker(kind)
	if err != nil {
		return err
	}
	wait, cancel := context.WithTimeout(ctx, textModelWait)
	defer cancel()
	select {
	case w.slot <- struct{}{}:
	case <-wait.Done():
		return errModelBusy
	}
	defer func() { <-w.slot }()
	// A request the client gave up on still finishes, as the helper cannot be stopped
	// halfway; one that runs far too long is ended by stopping the helper.
	done := make(chan error, 1)
	go func() { done <- fn(w.conn) }()
	select {
	case err = <-done:
	case <-time.After(textModelCallTime):
		w.command.Process.Kill()
		err = <-done
		if err == nil {
			err = errors.New("took too long")
		}
	}
	var failed *onnxwire.ModelError
	if err != nil && !errors.As(err, &failed) {
		// The helper died or the stream broke: start a new one next time.
		m.mu.Lock()
		w.broken = true
		if m.workers[kind.name] == w {
			delete(m.workers, kind.name)
		}
		m.mu.Unlock()
		logError("text model "+kind.name, errors.New(clip(w.stderr.String(), 300)))
		go w.stop()
	}
	m.mu.Lock()
	w.used = time.Now()
	m.mu.Unlock()
	return err
}

// forget stops the helper of a kind, after its dataset was replaced, so the new files
// are loaded next time.
func (m *textModels) forget(name string) {
	m.mu.Lock()
	w, ok := m.workers[name]
	if ok {
		delete(m.workers, name)
	}
	m.mu.Unlock()
	if ok {
		go func() {
			// Let a request in progress finish.
			w.slot <- struct{}{}
			w.stop()
		}()
	}
}

// sweep stops helpers that have been idle too long.
func (m *textModels) sweep() {
	m.mu.Lock()
	var old []*modelWorker
	for name, w := range m.workers {
		if len(w.slot) == 0 && time.Since(w.used) > m.idle {
			delete(m.workers, name)
			old = append(old, w)
		}
	}
	m.mu.Unlock()
	for _, w := range old {
		w.stop()
	}
}

func (m *textModels) close() {
	m.mu.Lock()
	workers := m.workers
	m.workers = map[string]*modelWorker{}
	m.mu.Unlock()
	for _, w := range workers {
		w.stop()
	}
}

// running lists the helpers, for /healthz and the metrics.
func (m *textModels) running() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for name := range m.workers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var errModelBusy = errors.New("model busy")

// modelFailed answers for a text model that could not be used.
func (s *Server) modelFailed(w http.ResponseWriter, r *http.Request, err error, what string) {
	var failed *onnxwire.ModelError
	switch {
	case errors.Is(err, errNoProgram):
		w.Header().Set("Retry-After", "3600")
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "This server cannot "+what+" yet: a program it needs is not installed.")
	case errors.Is(err, errModelBusy):
		w.Header().Set("Retry-After", "10")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The server is busy; try again in a few seconds.")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The request ended before the model answered.")
	default:
		if errors.As(err, &failed) {
			// The model's own message names tensors, never the text.
			logError(what, failed)
		}
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "The model could not "+what+".")
	}
}
