package api

import (
	"bufio"
	"bytes"
	"context"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Text from pictures and scanned PDFs with Tesseract 5 (Apache-2.0, installed on the
// server with its trained models for English and ten Indian languages). A PDF is first
// drawn page by page at 300 dpi by pdftoppm. Each Tesseract run is told to use one thread,
// so the CPU slots the server counts are the CPUs it uses.

const (
	ocrMaxPages = 10
	ocrDPI      = "300"
)

// ocrLanguages maps what a caller may write to Tesseract's model names.
var ocrLanguages = map[string]string{
	"eng": "eng", "en": "eng", "english": "eng",
	"hin": "hin", "hi": "hin", "hindi": "hin",
	"tam": "tam", "ta": "tam", "tamil": "tam",
	"tel": "tel", "te": "tel", "telugu": "tel",
	"kan": "kan", "kn": "kan", "kannada": "kan",
	"mal": "mal", "ml": "mal", "malayalam": "mal",
	"ben": "ben", "bn": "ben", "bengali": "ben", "bangla": "ben",
	"guj": "guj", "gu": "guj", "gujarati": "guj",
	"pan": "pan", "pa": "pan", "punjabi": "pan",
	"ori": "ori", "or": "ori", "odia": "ori", "oriya": "ori",
	"mar": "mar", "mr": "mar", "marathi": "mar",
}

var ocrLanguageCodes = []string{"eng", "hin", "tam", "tel", "kan", "mal", "ben", "guj", "pan", "ori", "mar"}

// ocrLanguage turns "en,ta" or "eng+tam" into Tesseract's "eng+tam". At most three.
func ocrLanguage(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "eng", true
	}
	var names []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(strings.ToLower(raw), func(r rune) bool { return r == '+' || r == ',' || r == ' ' }) {
		name, ok := ocrLanguages[part]
		if !ok {
			return "", false
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if len(names) == 0 || len(names) > 3 {
		return "", false
	}
	return strings.Join(names, "+"), true
}

type ocrWord struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Box        [4]int  `json:"box"` // left, top, width, height in pixels of the page picture
	Line       int     `json:"line"`
}

type ocrPage struct {
	Page       int       `json:"page"`
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Words      []ocrWord `json:"words,omitempty"`
}

type ocrResult struct {
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
	Language   string    `json:"language"`
	Pages      []ocrPage `json:"pages"`
}

// pictureKind recognises the formats Tesseract reads directly.
func pictureKind(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		return "jpg"
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		return "png"
	case bytes.HasPrefix(data, []byte("II*\x00")), bytes.HasPrefix(data, []byte("MM\x00*")):
		return "tif"
	case bytes.HasPrefix(data, []byte("BM")):
		return "bmp"
	case bytes.HasPrefix(data, []byte("GIF8")):
		return "gif"
	case len(data) > 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "webp"
	}
	return ""
}

func (s *Server) handleOCR(w http.ResponseWriter, r *http.Request, in uploadInput) {
	language, ok := ocrLanguage(in.option(r, "lang"))
	if !ok {
		s.badRequest(w, r, "lang takes up to three of "+strings.Join(ocrLanguageCodes, ", ")+", joined by + or a comma, such as eng+tam.")
		return
	}
	withWords := in.option(r, "words") == "true"
	data := in.Files[0].Data
	dir, done, err := workspace()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	ctx, cancel := context.WithTimeout(r.Context(), 85*time.Second)
	defer cancel()

	var pictures []string
	switch kind := pictureKind(data); {
	case isPDF(data):
		first, last, ok := pageRange(in, r)
		if !ok {
			s.badRequest(w, r, "first and last are page numbers, first from 1 and last not before first.")
			return
		}
		last = min(last, first+ocrMaxPages-1)
		path := filepath.Join(dir, "in.pdf")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		if _, err := s.run(ctx, job{program: "pdftoppm", args: []string{"-r", ocrDPI, "-gray", "-png", "-f", itoa(first), "-l", itoa(last), path, filepath.Join(dir, "page")}, dir: dir, timeout: 60 * time.Second}); err != nil {
			s.toolFailed(w, r, err, "read the PDF")
			return
		}
		pictures, _ = filepath.Glob(filepath.Join(dir, "page-*.png"))
		sort.Slice(pictures, func(a, b int) bool { return pageNumber(pictures[a]) < pageNumber(pictures[b]) })
		if len(pictures) == 0 {
			s.badRequest(w, r, "The PDF has no pages in that range.")
			return
		}
	case kind != "":
		path := filepath.Join(dir, "in."+kind)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		pictures = []string{path}
	default:
		// HEIC, AVIF and the rest: libvips turns them into a PNG Tesseract can read.
		path := filepath.Join(dir, "in")
		out := filepath.Join(dir, "in.png")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		if _, err := s.run(ctx, job{program: "vips", args: []string{"thumbnail", path, out, "10000", "--size", "down"}, dir: dir, timeout: 30 * time.Second}); err != nil {
			s.notPicture(w, r)
			return
		}
		pictures = []string{out}
	}

	result := ocrResult{Language: language, Pages: []ocrPage{}}
	var texts []string
	var confidence, words float64
	for i, picture := range pictures {
		base := filepath.Join(dir, "out-"+itoa(i))
		_, err := s.run(ctx, job{
			program: "tesseract",
			args:    []string{picture, base, "-l", language, "--oem", "1", "--psm", "3", "txt", "tsv"},
			dir:     dir, env: []string{"OMP_THREAD_LIMIT=1"}, timeout: 60 * time.Second,
		})
		if err != nil {
			s.toolFailed(w, r, err, "read text from the picture")
			return
		}
		text, _ := os.ReadFile(base + ".txt")
		table, _ := os.ReadFile(base + ".tsv")
		page := readTesseractTSV(table)
		page.Page = i + 1
		if isPDF(data) {
			page.Page = pageNumber(picture)
		}
		page.Text = strings.TrimSpace(strings.ReplaceAll(string(text), "\f", ""))
		confidence += page.Confidence * float64(len(page.Words))
		words += float64(len(page.Words))
		if !withWords {
			page.Words = nil
		}
		texts = append(texts, page.Text)
		result.Pages = append(result.Pages, page)
	}
	result.Text = strings.Join(texts, "\n\n")
	if words > 0 {
		result.Confidence = math.Round(confidence/words*10) / 10
	}
	note := "confidence is Tesseract's own, 0 to 100, averaged over words. Name every script the text uses in lang."
	if isPDF(data) {
		note += " At most " + itoa(ocrMaxPages) + " pages per request; use first and last for the rest. A PDF with real text is read faster and better by /v1/pdf/text."
	}
	s.ok(w, r, result, meta{Note: note}, cacheNone)
}

// pageNumber reads the page number pdftoppm writes into a file name: page-07.png.
func pageNumber(path string) int {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	_, number, _ := strings.Cut(name, "-")
	n, _ := strconv.Atoi(number)
	return n
}

// readTesseractTSV reads Tesseract's table: one row per page, block, paragraph, line and
// word. Words come with their confidence and box; the page row gives its size.
func readTesseractTSV(table []byte) ocrPage {
	page := ocrPage{Words: []ocrWord{}}
	scanner := bufio.NewScanner(bytes.NewReader(table))
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	lines := map[string]int{}
	var sum float64
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 12 || fields[0] == "level" {
			continue
		}
		number := func(i int) int { n, _ := strconv.Atoi(fields[i]); return n }
		switch fields[0] {
		case "1":
			page.Width, page.Height = number(8), number(9)
		case "5":
			text := strings.TrimSpace(fields[11])
			conf, err := strconv.ParseFloat(fields[10], 64)
			if text == "" || err != nil || conf < 0 {
				continue
			}
			key := fields[2] + "." + fields[3] + "." + fields[4]
			line, ok := lines[key]
			if !ok {
				line = len(lines) + 1
				lines[key] = line
			}
			page.Words = append(page.Words, ocrWord{
				Text: text, Confidence: math.Round(conf*10) / 10, Line: line,
				Box: [4]int{number(6), number(7), number(8), number(9)},
			})
			sum += conf
		}
	}
	if len(page.Words) > 0 {
		page.Confidence = math.Round(sum/float64(len(page.Words))*10) / 10
	}
	return page
}
