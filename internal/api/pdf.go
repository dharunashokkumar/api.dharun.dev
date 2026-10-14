package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// PDF tools. Reading and rewriting a PDF (information, merge, page selection, rotation,
// compression) is done in the server by pdfcpu (Apache-2.0). Text and page pictures come
// from poppler's pdftotext and pdftoppm (GPL, run as their own programs), which read far
// more real-world PDFs correctly than any Go library.

func init() {
	// pdfcpu would otherwise write a configuration folder under the user's home, which
	// the service may not have.
	model.ConfigPath = "disable"
}

const (
	pdfMaxFiles = 20
	pdfMaxPages = 500 // pages extracted as text or OCR'd in one request
)

func pdfConfig() *model.Configuration {
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	return conf
}

// pdfError answers a PDF that pdfcpu could not handle.
func (s *Server) pdfError(w http.ResponseWriter, r *http.Request, err error) {
	message := "The file is not a PDF this server can read."
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "password") || strings.Contains(lower, "encrypt"):
		message = "The PDF is protected by a password. Remove the protection and send it again."
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		s.fail(w, r, http.StatusGatewayTimeout, "too_slow", "The PDF took too long to process.")
		return
	}
	s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", message)
}

func isPDF(data []byte) bool {
	head := data[:min(len(data), 1024)]
	return bytes.Contains(head, []byte("%PDF-"))
}

func (s *Server) needPDF(w http.ResponseWriter, r *http.Request, in uploadInput) bool {
	for _, file := range in.Files {
		if !isPDF(file.Data) {
			s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The file is not a PDF.")
			return false
		}
	}
	return true
}

// pdfJob runs pdfcpu work under a time limit.
func pdfJob(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 60*time.Second)
}

type pdfPageSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Name   string  `json:"name,omitempty"`
}

type pdfInfo struct {
	Pages       int           `json:"pages"`
	Version     string        `json:"version"`
	Title       string        `json:"title"`
	Author      string        `json:"author"`
	Subject     string        `json:"subject"`
	Creator     string        `json:"creator"`
	Producer    string        `json:"producer"`
	Created     string        `json:"created"`
	Modified    string        `json:"modified"`
	Keywords    []string      `json:"keywords"`
	PageSizes   []pdfPageSize `json:"page_sizes"`
	Encrypted   bool          `json:"encrypted"`
	Form        bool          `json:"form"`
	Signatures  bool          `json:"signatures"`
	Tagged      bool          `json:"tagged"`
	Linearized  bool          `json:"linearized"`
	Bookmarks   bool          `json:"bookmarks"`
	Attachments int           `json:"attachments"`
	Bytes       int           `json:"bytes"`
}

// paperSizes names common page sizes, in points, either way round.
var paperSizes = []struct {
	name          string
	width, height float64
}{
	{"A4", 595.28, 841.89}, {"A3", 841.89, 1190.55}, {"A5", 419.53, 595.28},
	{"Letter", 612, 792}, {"Legal", 612, 1008},
}

func paperName(width, height float64) string {
	near := func(a, b float64) bool { return a-b < 2 && b-a < 2 }
	for _, paper := range paperSizes {
		if near(width, paper.width) && near(height, paper.height) || near(width, paper.height) && near(height, paper.width) {
			return paper.name
		}
	}
	return ""
}

func (s *Server) handlePDFInfo(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	ctx, cancel := pdfJob(r)
	defer cancel()
	data := in.Files[0].Data
	info, err := pdfapi.PDFInfo(ctx, bytes.NewReader(data), "upload.pdf", nil, false, pdfConfig())
	if err != nil {
		s.pdfError(w, r, err)
		return
	}
	view := pdfInfo{
		Pages: info.PageCount, Version: info.Version, Title: info.Title, Author: info.Author, Subject: info.Subject,
		Creator: info.Creator, Producer: info.Producer, Created: info.CreationDate, Modified: info.ModificationDate,
		Keywords: info.Keywords, Encrypted: info.Encrypted, Form: info.Form, Signatures: info.Signatures,
		Tagged: info.Tagged, Linearized: info.Linearized, Bookmarks: info.Outlines, Attachments: len(info.Attachments),
		Bytes: len(data), PageSizes: []pdfPageSize{},
	}
	if view.Keywords == nil {
		view.Keywords = []string{}
	}
	for dim := range info.PageDimensions {
		width, height := round2(dim.Width), round2(dim.Height)
		view.PageSizes = append(view.PageSizes, pdfPageSize{Width: width, Height: height, Name: paperName(width, height)})
	}
	sort.Slice(view.PageSizes, func(a, b int) bool {
		if view.PageSizes[a].Width != view.PageSizes[b].Width {
			return view.PageSizes[a].Width < view.PageSizes[b].Width
		}
		return view.PageSizes[a].Height < view.PageSizes[b].Height
	})
	s.ok(w, r, view, meta{Note: "Page sizes are in points (1/72 inch), each different size listed once."}, cacheNone)
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func (s *Server) handlePDFMerge(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if len(in.Files) < 2 {
		s.badRequest(w, r, "Send two or more PDFs as multipart/form-data, in the order they should appear: -F file=@one.pdf -F file=@two.pdf.")
		return
	}
	if !s.needPDF(w, r, in) {
		return
	}
	ctx, cancel := pdfJob(r)
	defer cancel()
	readers := make([]io.ReadSeeker, len(in.Files))
	for i, file := range in.Files {
		readers[i] = bytes.NewReader(file.Data)
	}
	var out bytes.Buffer
	if err := pdfapi.MergeRaw(ctx, readers, &out, false, pdfConfig()); err != nil {
		s.pdfError(w, r, err)
		return
	}
	sendFile(w, r, out.Bytes(), "application/pdf", "merged.pdf")
}

// pageSelection matches what the pages option takes: numbers and ranges, comma-separated.
var pageSelection = regexp.MustCompile(`^(\d+(-\d*)?|-\d+|odd|even|l(-\d+)?)$`)

// pagesOption reads a page selection such as "1-3,5,8-". Empty means every page.
func pagesOption(raw string) ([]string, bool) {
	raw = strings.ReplaceAll(strings.ToLower(raw), " ", "")
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 100 {
		return nil, false
	}
	for _, part := range parts {
		if !pageSelection.MatchString(part) {
			return nil, false
		}
	}
	return parts, true
}

const pagesHelp = `pages takes page numbers and ranges separated by commas, such as 1-3,5,8- (8 to the end), odd, even or l (the last page).`

func (s *Server) handlePDFPages(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	pages, ok := pagesOption(in.option(r, "pages"))
	if !ok || pages == nil {
		s.badRequest(w, r, pagesHelp+" It is required here.")
		return
	}
	ctx, cancel := pdfJob(r)
	defer cancel()
	var out bytes.Buffer
	if err := pdfapi.Collect(ctx, bytes.NewReader(in.Files[0].Data), &out, pages, pdfConfig()); err != nil {
		s.pdfError(w, r, err)
		return
	}
	sendFile(w, r, out.Bytes(), "application/pdf", "pages.pdf")
}

func (s *Server) handlePDFRotate(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	angle, err := strconv.Atoi(in.option(r, "angle"))
	if in.option(r, "angle") == "" {
		angle, err = 90, nil
	}
	if err != nil || angle%90 != 0 || angle == 0 || angle < -270 || angle > 270 {
		s.badRequest(w, r, "angle must be 90, 180, 270, -90, -180 or -270 (clockwise, in degrees).")
		return
	}
	pages, ok := pagesOption(in.option(r, "pages"))
	if !ok {
		s.badRequest(w, r, pagesHelp)
		return
	}
	ctx, cancel := pdfJob(r)
	defer cancel()
	var out bytes.Buffer
	if err := pdfapi.Rotate(ctx, bytes.NewReader(in.Files[0].Data), &out, angle, pages, pdfConfig()); err != nil {
		s.pdfError(w, r, err)
		return
	}
	sendFile(w, r, out.Bytes(), "application/pdf", "rotated.pdf")
}

func (s *Server) handlePDFCompress(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	ctx, cancel := pdfJob(r)
	defer cancel()
	data := in.Files[0].Data
	var out bytes.Buffer
	if err := pdfapi.Optimize(ctx, bytes.NewReader(data), &out, pdfConfig(), nil); err != nil {
		s.pdfError(w, r, err)
		return
	}
	body := out.Bytes()
	// Optimising removes duplicate fonts and images and packs objects into streams; when
	// that does not help, the original is the better answer.
	if len(body) >= len(data) {
		body = data
	}
	w.Header().Set("X-Original-Bytes", itoa(len(data)))
	sendFile(w, r, body, "application/pdf", "compressed.pdf")
}

// ---- text and page pictures, through poppler ----

// pageRange reads first and last page options for poppler.
func pageRange(in uploadInput, r *http.Request) (first, last int, ok bool) {
	first, last = 1, 0
	if raw := in.option(r, "first"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return 0, 0, false
		}
		first = value
	}
	if raw := in.option(r, "last"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < first {
			return 0, 0, false
		}
		last = value
	}
	if last == 0 || last-first+1 > pdfMaxPages {
		last = first + pdfMaxPages - 1
	}
	return first, last, true
}

type pdfPageText struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

type pdfText struct {
	Pages      []pdfPageText `json:"pages"`
	Characters int           `json:"characters"`
	// NeedsOCR is set when the pages hold almost no text: a scan, which /v1/ocr reads.
	NeedsOCR bool `json:"needs_ocr"`
}

func (s *Server) handlePDFText(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	first, last, ok := pageRange(in, r)
	if !ok {
		s.badRequest(w, r, "first and last are page numbers, first from 1 and last not before first.")
		return
	}
	dir, path, done, err := writeUpload(in.Files[0].Data, "in.pdf")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	args := []string{"-enc", "UTF-8", "-f", itoa(first), "-l", itoa(last)}
	if in.option(r, "layout") == "true" {
		args = append(args, "-layout")
	}
	out, err := s.run(r.Context(), job{program: "pdftotext", args: append(args, path, "-"), dir: dir, timeout: 60 * time.Second})
	if err != nil {
		var failed *jobError
		if errors.As(err, &failed) && strings.Contains(strings.ToLower(failed.stderr), "password") {
			s.pdfError(w, r, errors.New("password"))
			return
		}
		s.toolFailed(w, r, err, "read the PDF's text")
		return
	}
	result := pdfText{Pages: []pdfPageText{}}
	// pdftotext ends every page with a form feed.
	pages := strings.Split(string(out), "\f")
	if len(pages) > 0 && strings.TrimSpace(pages[len(pages)-1]) == "" {
		pages = pages[:len(pages)-1]
	}
	for i, text := range pages {
		text = strings.TrimRight(text, " \n")
		result.Pages = append(result.Pages, pdfPageText{Page: first + i, Text: text})
		result.Characters += len([]rune(strings.TrimSpace(text)))
	}
	result.NeedsOCR = len(result.Pages) > 0 && result.Characters < 20*len(result.Pages)
	note := ""
	if result.NeedsOCR {
		note = "These pages hold little or no text: they are probably scanned pictures. Send the PDF to /v1/ocr to read them."
	}
	total := len(result.Pages)
	s.ok(w, r, result, meta{Total: &total, Note: note}, cacheNone)
}

func (s *Server) handlePDFRender(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	page, err := strconv.Atoi(in.option(r, "page"))
	if in.option(r, "page") == "" {
		page, err = 1, nil
	}
	if err != nil || page < 1 {
		s.badRequest(w, r, "page must be a page number from 1.")
		return
	}
	width, err := strconv.Atoi(in.option(r, "width"))
	if in.option(r, "width") == "" {
		width, err = 1000, nil
	}
	if err != nil || width < 50 || width > 3000 {
		s.badRequest(w, r, "width must be from 50 to 3000 pixels.")
		return
	}
	format := in.option(r, "format")
	if format == "" {
		format = "png"
	}
	if format != "png" && format != "jpeg" {
		s.badRequest(w, r, "format must be png or jpeg.")
		return
	}
	dir, path, done, err := writeUpload(in.Files[0].Data, "in.pdf")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	out := filepath.Join(dir, "page")
	ctx, cancel := pdfJob(r)
	count, err := pdfapi.PageCount(ctx, bytes.NewReader(in.Files[0].Data), pdfConfig())
	cancel()
	if err != nil {
		s.pdfError(w, r, err)
		return
	}
	if page > count {
		s.badRequest(w, r, "The PDF has "+itoa(count)+" page"+plural(count)+".")
		return
	}
	args := []string{"-" + format, "-f", itoa(page), "-l", itoa(page), "-singlefile", "-scale-to-x", itoa(width), "-scale-to-y", "-1", path, out}
	if _, err := s.run(r.Context(), job{program: "pdftoppm", args: args, dir: dir, timeout: 30 * time.Second}); err != nil {
		s.toolFailed(w, r, err, "draw the page")
		return
	}
	extension := ".png"
	if format == "jpeg" {
		extension = ".jpg"
	}
	body, err := os.ReadFile(out + extension)
	if err != nil {
		s.badRequest(w, r, "The PDF has no page "+itoa(page)+".")
		return
	}
	sendFile(w, r, body, "image/"+format, "page-"+itoa(page)+extension)
}
