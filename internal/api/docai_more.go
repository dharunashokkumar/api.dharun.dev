package api

// The rest of Part 7: receipts, cheques, bank statements, and redacting a PDF or picture.

import (
	"bytes"
	"context"
	"encoding/csv"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"

	"github.com/dharunashokkumar/api.dharun.dev/internal/docai"
	"github.com/dharunashokkumar/api.dharun.dev/internal/textkit"
)

func (s *Server) registerDocAIMore() {
	cpu := uploadRoute{}
	s.handleUpload("doc_receipt", costUpload, "POST /v1/doc/receipt", cpu, s.handleDocReceipt)
	s.handleUpload("doc_cheque", costUpload, "POST /v1/doc/cheque", cpu, s.handleDocCheque)
	s.handleUpload("doc_statement", costUpload, "POST /v1/doc/statement", cpu, s.handleDocStatement)
	s.handleUpload("doc_redact", costUpload, "POST /v1/doc/redact", cpu, s.handleDocRedact)
}

func (s *Server) handleDocReceipt(w http.ResponseWriter, r *http.Request, in uploadInput) {
	doc, ok := s.readDocument(w, r, in)
	if !ok {
		return
	}
	s.ok(w, r, docai.ReadReceipt(doc.pages), meta{Note: "Items are the lines above the total that end in an amount. A crumpled or faded bill reads worse: photograph it flat and straight. " + doc.note()}, cacheNone)
}

func (s *Server) handleDocCheque(w http.ResponseWriter, r *http.Request, in uploadInput) {
	doc, ok := s.readDocument(w, r, in)
	if !ok {
		return
	}
	cheque := docai.ReadCheque(doc.pages)
	type answer struct {
		docai.Cheque
		Branch     map[string]any `json:"branch"`
		MICRBranch map[string]any `json:"micr_branch"`
		Checks     []docai.Check  `json:"checks"`
	}
	out := answer{Cheque: cheque, Checks: []docai.Check{}}
	if cheque.IFSC != "" {
		var branch map[string]any
		if s.inner(r, "/v1/ifsc/"+cheque.IFSC, &branch) {
			out.Branch = branch
		}
		out.Checks = append(out.Checks, docai.Check{Name: "ifsc_known", Passed: out.Branch != nil, Detail: "The IFSC " + cheque.IFSC + map[bool]string{true: " is a branch in RBI's list.", false: " is not in RBI's list of branches."}[out.Branch != nil]})
	}
	if cheque.MICR != "" {
		var branch map[string]any
		if s.inner(r, "/v1/micr/"+cheque.MICR, &branch) {
			out.MICRBranch = branch
		}
		if out.Branch != nil && out.MICRBranch != nil {
			same := out.Branch["ifsc"] == out.MICRBranch["ifsc"] || out.Branch["micr"] == cheque.MICR
			out.Checks = append(out.Checks, docai.Check{Name: "micr_matches_ifsc", Passed: same, Detail: "The MICR code " + cheque.MICR + " and the IFSC " + cheque.IFSC + map[bool]string{true: " are the same branch.", false: " are different branches."}[same]})
		}
	}
	s.ok(w, r, out, meta{Note: "The MICR line is printed in a magnetic font that ordinary OCR reads imperfectly; the MICR code is checked against RBI's branch list. This reads a cheque; it does not say whether it will be honoured. " + doc.note()}, cacheNone)
}

func (s *Server) handleDocStatement(w http.ResponseWriter, r *http.Request, in uploadInput) {
	format := in.option(r, "format")
	if format != "" && format != "json" && format != "csv" {
		s.badRequest(w, r, "format is json or csv.")
		return
	}
	doc, ok := s.readDocument(w, r, in)
	if !ok {
		return
	}
	list := docai.ReadStatement(doc.pages)
	if format == "csv" {
		var b bytes.Buffer
		out := csv.NewWriter(&b)
		_ = out.Write([]string{"date", "description", "reference", "debit", "credit", "balance"})
		number := func(p *float64) string {
			if p == nil {
				return ""
			}
			return strconv.FormatFloat(*p, 'f', 2, 64)
		}
		for _, t := range list {
			_ = out.Write([]string{t.Date, t.Description, t.Reference, number(t.Debit), number(t.Credit), number(t.Balance)})
		}
		out.Flush()
		sendFile(w, r, b.Bytes(), "text/csv; charset=utf-8", "statement.csv")
		return
	}
	total := len(list)
	mismatches := 0
	for _, t := range list {
		if t.BalanceOK != nil && !*t.BalanceOK {
			mismatches++
		}
	}
	note := "Columns are found by the header row (date, narration, reference, withdrawals or debits, deposits or credits, balance). balance_ok checks each balance against the one before; false marks a line misread or missed. " + doc.note()
	if total == 0 {
		note = "No transactions found: the statement needs a header row naming date, withdrawals or deposits, and balance. " + doc.note()
	} else if mismatches > 0 {
		note = itoa(mismatches) + " balances do not follow from the line before. " + note
	}
	s.ok(w, r, list, meta{Total: &total, Note: note}, cacheNone)
}

// redactDPI is how finely a redacted PDF's pages are drawn: the result is pictures, so
// nothing under a box survives in the file.
const redactDPI = 150

func (s *Server) handleDocRedact(w http.ResponseWriter, r *http.Request, in uploadInput) {
	var kinds []string
	if raw := strings.TrimSpace(in.option(r, "types")); raw != "" {
		for _, kind := range strings.Split(strings.ToLower(raw), ",") {
			kind = strings.TrimSpace(kind)
			if !contains(textkit.KindIDs(), kind) {
				s.badRequest(w, r, "types is a comma-separated list of: "+strings.Join(textkit.KindIDs(), ", ")+".")
				return
			}
			kinds = append(kinds, kind)
		}
	}
	data := in.Files[0].Data
	doc, ok := s.readDocument(w, r, in)
	if !ok {
		return
	}
	// The boxes to draw on each page, in the page's own units.
	boxes := make([][]docai.Word, len(doc.pages))
	counts := map[string]int{}
	for i, p := range doc.pages {
		text := docai.TextOf(p)
		for _, f := range textkit.Redact(text.Text, kinds, false).Findings {
			boxes[i] = append(boxes[i], text.Covering(f.Start, f.End)...)
			counts[f.Type]++
		}
	}
	found := 0
	for _, n := range counts {
		found += n
	}
	if found == 0 {
		s.fail(w, r, http.StatusUnprocessableEntity, "nothing_found", "Nothing to redact was found, so no file is returned. "+strings.TrimSpace(doc.note()))
		return
	}
	var kindsFound []string
	for k, n := range counts {
		kindsFound = append(kindsFound, k+"="+itoa(n))
	}
	sort.Strings(kindsFound)
	w.Header().Set("X-Redacted", strings.Join(kindsFound, ","))
	ctx, cancel := context.WithTimeout(r.Context(), 85*time.Second)
	defer cancel()
	if !isPDF(data) {
		picture, err := s.loadPicture(ctx, data)
		if err != nil {
			s.notPicture(w, r)
			return
		}
		defer freeLarge(picture)
		canvas := coverBoxes(picture, boxes[0], 1)
		var out bytes.Buffer
		if pictureKind(data) == "jpg" {
			err = jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 90})
			if err == nil {
				sendFile(w, r, out.Bytes(), "image/jpeg", "redacted.jpg")
			}
		} else if err = png.Encode(&out, canvas); err == nil {
			sendFile(w, r, out.Bytes(), "image/png", "redacted.png")
		}
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		}
		return
	}
	// A PDF: draw each page as a picture, cover the boxes, and make a new PDF of them.
	dir, path, done, err := writeUpload(data, "in.pdf")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	last := doc.first + len(doc.pages) - 1
	dpi := redactDPI
	if doc.method == "ocr" {
		dpi, _ = strconv.Atoi(ocrDPI) // OCR boxes are in pixels of a page drawn at this resolution
	}
	if _, err := s.run(ctx, job{program: "pdftoppm", args: []string{"-r", itoa(dpi), "-png", "-f", itoa(doc.first), "-l", itoa(last), path, filepath.Join(dir, "page")}, dir: dir, timeout: 60 * time.Second}); err != nil {
		s.toolFailed(w, r, err, "draw the PDF's pages")
		return
	}
	names, _ := filepath.Glob(filepath.Join(dir, "page-*.png"))
	sort.Slice(names, func(a, b int) bool { return pageNumber(names[a]) < pageNumber(names[b]) })
	scale := float64(dpi) / 72
	if doc.method == "ocr" {
		scale = 1
	}
	var pictures []io.Reader
	for i, name := range names {
		raw, err := os.ReadFile(name)
		if err != nil || i >= len(boxes) {
			continue
		}
		picture, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			continue
		}
		var out bytes.Buffer
		if err := png.Encode(&out, coverBoxes(picture, boxes[i], scale)); err != nil {
			continue
		}
		pictures = append(pictures, bytes.NewReader(out.Bytes()))
	}
	if len(pictures) == 0 {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	var out bytes.Buffer
	imp := pdfapi.DefaultImportConfig()
	imp.DPI = dpi
	if err := pdfapi.ImportImages(ctx, nil, &out, pictures, imp, pdfConfig()); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "The redacted pages could not be put in a PDF.")
		return
	}
	sendFile(w, r, out.Bytes(), "application/pdf", "redacted.pdf")
}

// coverBoxes draws black boxes, given in the page's units times scale, on a copy of a
// picture.
func coverBoxes(picture image.Image, words []docai.Word, scale float64) *image.RGBA {
	bounds := picture.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, picture, bounds.Min, draw.Src)
	black := image.NewUniform(color.Black)
	for _, word := range words {
		pad := (word.Y1 - word.Y0) * 0.15
		box := image.Rect(int((word.X0-pad)*scale), int((word.Y0-pad)*scale), int(math.Ceil((word.X1+pad)*scale)), int(math.Ceil((word.Y1+pad)*scale)))
		draw.Draw(canvas, box.Add(bounds.Min).Intersect(bounds), black, image.Point{}, draw.Src)
	}
	return canvas
}
