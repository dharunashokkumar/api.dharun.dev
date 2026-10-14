package dataset

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	userAgent = "api.dharun.dev data importer (https://github.com/dharunashokkumar/api.dharun.dev)"
	// Some official sites answer only what looks like a browser.
	browserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

	// No source file is anywhere near this size; a response that is means something is wrong.
	maxDownload = 512 << 20
	// downloadStall is how long a download may go without receiving a byte.
	downloadStall = 2 * time.Minute
)

// progressReader calls progress whenever bytes arrive.
type progressReader struct {
	io.ReadCloser
	progress func()
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.progress()
	}
	return n, err
}

// Importer is handed to a dataset's Import function. The same function serves a
// maintainer refreshing the repository's copy and a server refreshing its own.
type Importer struct {
	// Dir is where the source files are written, laid out as in the repository's data/.
	Dir string
	// File reads the source from a local file instead of downloading it.
	File string
	// Version pins a source version (a release tag, a revision id). Empty means latest.
	Version string
	// Year limits a holiday import to one year. Zero lets the importer choose.
	Year int
	// Current is the copy of the dataset in use, when there is one. An importer whose
	// source only covers part of the data carries the rest forward from it.
	Current *sql.DB
	Now     time.Time
	Log     func(format string, args ...any)

	client *http.Client
}

func (im *Importer) http() *http.Client {
	if im.client == nil {
		jar, _ := cookiejar.New(nil)
		im.client = &http.Client{Jar: jar, Timeout: 10 * time.Minute}
	}
	return im.client
}

// Logf reports progress.
func (im *Importer) Logf(format string, args ...any) {
	if im.Log != nil {
		im.Log(format, args...)
	}
}

// Path joins a name onto the directory being filled.
func (im *Importer) Path(parts ...string) string {
	return filepath.Join(append([]string{im.Dir}, parts...)...)
}

// Get downloads a URL as this project's importer.
func (im *Importer) Get(address string) ([]byte, error) {
	return im.fetch(http.MethodGet, address, nil, userAgent)
}

// Browse downloads a URL the way a browser would, for sites that refuse anything else.
func (im *Importer) Browse(address string) ([]byte, error) {
	return im.fetch(http.MethodGet, address, nil, browserAgent)
}

// Post submits a form the way a browser would.
func (im *Importer) Post(address string, form url.Values) ([]byte, error) {
	return im.fetch(http.MethodPost, address, form, browserAgent)
}

func (im *Importer) fetch(method, address string, form url.Values, agent string) ([]byte, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := http.NewRequest(method, address, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", agent)
	if agent == browserAgent {
		request.Header.Set("Accept-Language", "en-US,en;q=0.9")
		request.Header.Set("Referer", address)
	}
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := im.http().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s: %s", method, address, response.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxDownload+1))
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, address, err)
	}
	if len(raw) > maxDownload {
		return nil, fmt.Errorf("%s %s: response is larger than %d MB", method, address, maxDownload>>20)
	}
	return raw, nil
}

// ErrNotChanged is returned by an importer that found the source still at the version
// the copy in use was built from. Nothing is downloaded or rebuilt, and the dataset is
// not checked again until it is next due.
var ErrNotChanged = errors.New("the source has not changed")

// Download streams a URL into a file under the import directory, for sources too large
// to hold in memory. It returns the file's path.
//
// A large file over a slow connection can take longer than the client's overall time
// limit, so a download has none: it is given up only when no byte arrives for
// downloadStall.
func (im *Importer) Download(address, name string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", userAgent)
	client := *im.http()
	client.Timeout = 0
	stall := time.AfterFunc(downloadStall, cancel)
	defer stall.Stop()
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	response.Body = &progressReader{ReadCloser: response.Body, progress: func() { stall.Reset(downloadStall) }}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", address, response.Status)
	}
	path := im.Path(filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	written, err := io.Copy(file, io.LimitReader(response.Body, maxDownload+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", address, err)
	}
	if written > maxDownload {
		return "", fmt.Errorf("GET %s: larger than %d MB", address, maxDownload>>20)
	}
	return path, nil
}

// CurrentVersion is the source version of the copy in use, or "" when there is none.
func (im *Importer) CurrentVersion() string {
	if im.Current == nil {
		return ""
	}
	meta, err := ReadMeta(im.Current)
	if err != nil {
		return ""
	}
	return meta.Source.Version
}

// Write saves a source file under the import directory, creating folders as needed.
func (im *Importer) Write(name string, raw []byte) error {
	path := im.Path(filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// WriteGzip saves a source file compressed. No name or timestamp goes in the header,
// so the same input always gives the same file.
func (im *Importer) WriteGzip(name string, raw []byte) error {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := writer.Write(raw); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return im.Write(name, buffer.Bytes())
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
