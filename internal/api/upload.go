package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Endpoints that take a file. A file comes as the request body itself (Content-Type
// image/png, application/pdf, ...) or as multipart/form-data, where other form fields
// carry the options. The server never fetches a file from a URL: it would download
// whatever a stranger names. Uploads are held in memory or in a temporary directory that
// is deleted when the request ends; nothing is stored, and every upload endpoint is
// private, so no part of it reaches the log.

const (
	uploadLimit = 10 << 20 // one file
	mergeLimit  = 25 << 20 // all files of a request that takes several
	// An upload over a slow connection takes longer than the server's usual 15 s, and a
	// job over a large file longer than its usual 45 s. Cloudflare gives up after 100 s.
	uploadReadTime  = 90 * time.Second
	uploadWriteTime = 95 * time.Second
	// How long a request waits for a CPU or model slot once its file is in.
	uploadWait = 30 * time.Second
	modelWait  = 60 * time.Second
)

// uploadFile is one file as it was sent.
type uploadFile struct {
	Name string // the file name in a form, "" for a raw body
	Type string // the media type the client gave, which is only a hint
	Data []byte
}

// uploadInput is what an upload endpoint receives: the files, and options from form
// fields or the query string.
type uploadInput struct {
	Files   []uploadFile
	options map[string]string
}

// option returns a form field, or else the query parameter of that name.
func (in uploadInput) option(r *http.Request, name string) string {
	if value, ok := in.options[name]; ok {
		return value
	}
	return r.URL.Query().Get(name)
}

// uploadRoute says what an endpoint accepts and which pool its work waits in.
type uploadRoute struct {
	maxFiles int
	limit    int64         // bytes, all files together
	pool     chan struct{} // the slots its work takes: CPU or model
	wait     time.Duration
}

// handleUpload registers an endpoint that takes files. The files are read in full before
// the request takes a slot, so a slow upload holds no slot while it trickles in.
func (s *Server) handleUpload(route string, cost int, pattern string, spec uploadRoute, handler func(http.ResponseWriter, *http.Request, uploadInput)) {
	if spec.maxFiles == 0 {
		spec.maxFiles = 1
	}
	if spec.limit == 0 {
		spec.limit = uploadLimit
	}
	if spec.pool == nil {
		spec.pool, spec.wait = s.heavy, uploadWait
	}
	s.handle(route, cost, pattern, func(w http.ResponseWriter, r *http.Request) {
		allowTime(w, uploadReadTime, uploadWriteTime)
		in, ok := s.readUpload(w, r, spec.maxFiles, spec.limit)
		if !ok {
			return
		}
		release, ok := s.takeSlot(w, r, spec.pool, spec.wait)
		if !ok {
			return
		}
		defer release()
		handler(w, r, in)
	})
}

// allowTime moves the connection's deadlines out for a request that sends or makes a
// large file. A response kept in memory (an MCP call, a sample) has no deadlines.
func allowTime(w http.ResponseWriter, read, write time.Duration) {
	controller := http.NewResponseController(w)
	controller.SetReadDeadline(time.Now().Add(read))
	controller.SetWriteDeadline(time.Now().Add(write))
}

func (s *Server) readUpload(w http.ResponseWriter, r *http.Request, maxFiles int, limit int64) (uploadInput, bool) {
	in := uploadInput{options: map[string]string{}}
	if r.Method != http.MethodPost {
		s.badRequest(w, r, "POST the file as the request body, or as multipart/form-data.")
		return in, false
	}
	// Room for the form's own boundaries and fields on top of the files.
	body := http.MaxBytesReader(w, r.Body, limit+64<<10)
	tooLarge := func() (uploadInput, bool) {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The upload is larger than "+byteSize(int(limit))+".")
		return in, false
	}
	kind, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if kind == "multipart/form-data" {
		reader, err := r.MultipartReader()
		if err != nil || params["boundary"] == "" {
			s.badRequest(w, r, "The multipart form could not be read.")
			return in, false
		}
		r.Body = body
		var total int64
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				if isTooLarge(err) {
					return tooLarge()
				}
				s.badRequest(w, r, "The multipart form could not be read.")
				return in, false
			}
			if part.FileName() == "" {
				// An ordinary field: an option.
				value, err := io.ReadAll(io.LimitReader(part, 4<<10))
				if err != nil {
					s.badRequest(w, r, "The multipart form could not be read.")
					return in, false
				}
				in.options[part.FormName()] = strings.TrimSpace(string(value))
				continue
			}
			if len(in.Files) == maxFiles {
				s.badRequest(w, r, fmt.Sprintf("Send at most %d file%s.", maxFiles, plural(maxFiles)))
				return in, false
			}
			data, err := io.ReadAll(io.LimitReader(part, limit-total+1))
			if err != nil {
				if isTooLarge(err) {
					return tooLarge()
				}
				s.badRequest(w, r, "The upload could not be read.")
				return in, false
			}
			total += int64(len(data))
			if total > limit || int64(len(data)) > uploadLimit {
				return tooLarge()
			}
			in.Files = append(in.Files, uploadFile{Name: part.FileName(), Type: part.Header.Get("Content-Type"), Data: data})
		}
	} else {
		data, err := io.ReadAll(body)
		if err != nil {
			if isTooLarge(err) {
				return tooLarge()
			}
			s.badRequest(w, r, "The upload could not be read.")
			return in, false
		}
		if int64(len(data)) > min(limit, uploadLimit) {
			return tooLarge()
		}
		if len(data) > 0 {
			in.Files = append(in.Files, uploadFile{Type: kind, Data: data})
		}
	}
	if len(in.Files) == 0 {
		s.badRequest(w, r, "No file was sent. POST the file as the request body, or as multipart/form-data with a file field.")
		return in, false
	}
	return in, true
}

func isTooLarge(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ---- outside programs ----

// programs finds the outside programs the upload tools run. A release carries some of
// them next to the server binary (typst, piper, deep-filter, matte); the system provides
// the rest (tesseract, vips, ffmpeg, pdftotext, pdftoppm). A program found is
// remembered; one missing is looked for again on the next request, so installing it
// needs no restart.
type programs struct {
	override map[string]string
	mu       sync.Mutex
	found    map[string]string
}

// programFolders names the folder of the release a program lives in, when that is not
// the program's own name.
var programFolders = map[string]string{"whisper-cli": "whisper"}

func newPrograms(override map[string]string) *programs {
	return &programs{override: override, found: map[string]string{}}
}

// path returns where a program is, or "" when this server does not have it.
func (p *programs) path(name string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if path, ok := p.found[name]; ok {
		return path
	}
	var path string
	if configured := p.override[name]; configured != "" {
		if isFile(configured) {
			path = configured
		}
	} else {
		if self, err := os.Executable(); err == nil {
			dir := filepath.Dir(self)
			// piper and whisper come in their own folders, with their libraries beside them.
			candidates := []string{filepath.Join(dir, name), filepath.Join(dir, name, name)}
			if folder, ok := programFolders[name]; ok {
				candidates = append(candidates, filepath.Join(dir, folder, name))
			}
			for _, candidate := range candidates {
				if isFile(candidate) {
					path = candidate
					break
				}
			}
		}
		if path == "" {
			path, _ = exec.LookPath(name)
		}
	}
	if path != "" {
		p.found[name] = path
	}
	return path
}

var errNoProgram = errors.New("program not installed")

// job is one run of an outside program.
type job struct {
	program string
	args    []string
	stdin   []byte
	dir     string
	env     []string
	timeout time.Duration
	// memory caps the program's address space in bytes, through prlimit, for programs
	// that run what a stranger wrote (Typst). Zero leaves it to the service's limit.
	memory int64
}

// jobError carries what a program printed when it failed, for messages and the log.
type jobError struct {
	program string
	err     error
	stderr  string
	timeout bool
}

func (e *jobError) Error() string {
	if e.timeout {
		return e.program + ": took too long"
	}
	return fmt.Sprintf("%s: %v: %s", e.program, e.err, clip(e.stderr, 300))
}

// run starts a program, feeds it stdin and returns what it wrote to stdout.
func (s *Server) run(ctx context.Context, j job) ([]byte, error) {
	path := s.extras.programs.path(j.program)
	if path == "" {
		return nil, errNoProgram
	}
	if j.timeout == 0 {
		j.timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()
	name, args := path, j.args
	if j.memory > 0 && runtime.GOOS == "linux" {
		if prlimit := s.extras.programs.path("prlimit"); prlimit != "" {
			name, args = prlimit, append([]string{fmt.Sprintf("--as=%d", j.memory), "--", path}, args...)
		}
	}
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = j.dir
	command.WaitDelay = 2 * time.Second
	if j.env != nil {
		command.Env = append(minimalEnv(), j.env...)
	} else {
		command.Env = minimalEnv()
	}
	if j.stdin != nil {
		command.Stdin = bytes.NewReader(j.stdin)
	}
	var out bytes.Buffer
	problems := &limitedBuffer{max: 8 << 10}
	command.Stdout, command.Stderr = &out, problems
	if err := command.Run(); err != nil {
		return nil, &jobError{program: j.program, err: err, stderr: strings.TrimSpace(problems.String()), timeout: ctx.Err() == context.DeadlineExceeded}
	}
	return out.Bytes(), nil
}

// minimalEnv is what an outside program inherits: enough to run, and nothing of the
// server's own settings (the Cloudflare token lives in its environment file).
func minimalEnv() []string {
	env := []string{"PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "HOME=" + os.TempDir()}
	if runtime.GOOS == "windows" {
		for _, name := range []string{"SYSTEMROOT", "PATH", "TEMP", "TMP"} {
			env = append(env, name+"="+os.Getenv(name))
		}
	}
	return env
}

// limitedBuffer keeps the first bytes written to it, so a chatty program cannot use up
// memory through its error output.
type limitedBuffer struct {
	bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.Len(); room > 0 {
		b.Buffer.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

// workspace is a temporary directory for one request, removed when it ends.
func workspace() (string, func(), error) {
	dir, err := os.MkdirTemp("", "dharun-api-")
	if err != nil {
		return "", nil, err
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}

// toolFailed answers a failed job: 503 when the program is missing on this server, 504
// when it ran too long, and 422 with the reason otherwise, as a program usually fails
// on what it was given.
func (s *Server) toolFailed(w http.ResponseWriter, r *http.Request, err error, what string) {
	var failed *jobError
	switch {
	case errors.Is(err, errNoProgram):
		w.Header().Set("Retry-After", "3600")
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "This server cannot "+what+" yet: a program it needs is not installed.")
	case errors.As(err, &failed) && failed.timeout:
		s.fail(w, r, http.StatusGatewayTimeout, "too_slow", "It took too long to "+what+". Try a smaller file.")
	default:
		// Only the program and how it ended: what it printed may quote the upload.
		if errors.As(err, &failed) {
			logError(what, fmt.Errorf("%s: %v", failed.program, failed.err))
		} else {
			logError(what, err)
		}
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "Could not "+what+". Check that the file is complete and of a supported kind.")
	}
}
