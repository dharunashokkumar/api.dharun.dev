package api

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/typeset"
)

// Typst to PDF. Typst (Apache-2.0) ships in the release as its own program. What it is
// given is a stranger's program, so it runs with nothing to reach: an empty project
// folder, no packages (a proxy that goes nowhere stops it downloading any), no system
// fonts, one thread, a time limit and a memory cap. It has the Noto fonts the server
// already carries, so Tamil, Hindi and the other Indian scripts set correctly.

const (
	typstTime   = 20 * time.Second
	typstMemory = 2 << 30 // address space; resident use stays far below this
)

// typstFonts writes the server's fonts to a folder once, for Typst to read.
var typstFonts = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "dharun-api-fonts-")
	if err != nil {
		return "", err
	}
	fonts := typeset.Fonts()
	err = fs.WalkDir(fonts, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".ttf") {
			return err
		}
		data, err := fs.ReadFile(fonts, path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, filepath.Base(path)), data, 0o644)
	})
	return dir, err
})

// typstOutput says what to make of a document.
type typstOutput struct {
	format string // pdf, png or svg
	page   int    // for png and svg: which page
	ppi    int    // for png
}

// typstError is a mistake in the document, with Typst's own message.
type typstError struct{ message string }

func (e *typstError) Error() string { return e.message }

// compileTypst compiles main.typ in dir and returns the output file's contents.
func (s *Server) compileTypst(ctx context.Context, dir string, out typstOutput) ([]byte, error) {
	fonts, err := typstFonts()
	if err != nil {
		return nil, err
	}
	empty := filepath.Join(dir, ".packages")
	if err := os.MkdirAll(empty, 0o700); err != nil {
		return nil, err
	}
	target := filepath.Join(dir, "out."+out.format)
	args := []string{"compile", "--root", dir, "--font-path", fonts, "--ignore-system-fonts",
		"--package-path", empty, "--package-cache-path", empty, "--jobs", "1",
		"--diagnostic-format", "short", "--format", out.format}
	if out.format != "pdf" {
		args = append(args, "--pages", strconv.Itoa(out.page))
	}
	if out.format == "png" {
		args = append(args, "--ppi", strconv.Itoa(out.ppi))
	}
	args = append(args, filepath.Join(dir, "main.typ"), target)
	_, err = s.run(ctx, job{
		program: "typst", args: args, dir: dir, timeout: typstTime, memory: typstMemory,
		// Typst would download a package it is asked to import; these send it nowhere.
		env: []string{"HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9", "ALL_PROXY=http://127.0.0.1:9", "NO_PROXY="},
	})
	var failed *jobError
	if errors.As(err, &failed) && !failed.timeout {
		return nil, &typstError{message: typstMessage(failed.stderr, dir)}
	}
	if err != nil {
		return nil, err
	}
	return os.ReadFile(target)
}

// typstMessage tidies Typst's diagnostics for the caller: no server paths.
func typstMessage(stderr, dir string) string {
	message := strings.ReplaceAll(stderr, dir+string(filepath.Separator), "")
	message = strings.ReplaceAll(message, dir, "")
	var lines []string
	for _, line := range strings.Split(message, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "warning:") {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return "Typst could not compile the document."
	}
	if strings.Contains(message, "@preview") || strings.Contains(message, "package") {
		lines = append(lines, "Packages cannot be imported here: only Typst's built-in functions are available.")
	}
	return clip(strings.Join(lines, "\n"), 2000)
}

var typstFormats = map[string]string{"pdf": "application/pdf", "png": "image/png", "svg": "image/svg+xml"}

func readTypstOutput(get func(string) string) (typstOutput, error) {
	out := typstOutput{format: strings.ToLower(get("format")), page: 1, ppi: 144}
	if out.format == "" {
		out.format = "pdf"
	}
	if _, ok := typstFormats[out.format]; !ok {
		return out, errors.New("format must be pdf, png or svg.")
	}
	if raw := get("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 || page > 1000 {
			return out, errors.New("page must be a page number from 1.")
		}
		out.page = page
	}
	if raw := get("ppi"); raw != "" {
		ppi, err := strconv.Atoi(raw)
		if err != nil || ppi < 36 || ppi > 300 {
			return out, errors.New("ppi must be from 36 to 300.")
		}
		out.ppi = ppi
	}
	return out, nil
}

func (s *Server) handleTypst(w http.ResponseWriter, r *http.Request) {
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	out, err := readTypstOutput(func(name string) string { return in.option(r, name) })
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	dir, done, err := workspace()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	if err := os.WriteFile(filepath.Join(dir, "main.typ"), []byte(in.Text), 0o600); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	body, err := s.compileTypst(r.Context(), dir, out)
	if err != nil {
		s.typstFailed(w, r, err)
		return
	}
	sendFile(w, r, body, typstFormats[out.format], "document."+out.format)
}

func (s *Server) typstFailed(w http.ResponseWriter, r *http.Request, err error) {
	var mistake *typstError
	if errors.As(err, &mistake) {
		s.fail(w, r, http.StatusUnprocessableEntity, "typst_error", mistake.message)
		return
	}
	if os.IsNotExist(err) {
		s.badRequest(w, r, "The document has no such page.")
		return
	}
	s.toolFailed(w, r, err, "compile the document")
}
