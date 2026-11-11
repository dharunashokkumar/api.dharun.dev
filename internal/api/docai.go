package api

// Reading documents without a language model (growth plan Part 7): a PDF's text layer
// with the position of every word (pdftotext -bbox-layout), or Tesseract's words for a
// scan or photo, turned into Markdown, tables and the fields of a GST invoice; the signed
// QR code of a GST e-invoice; and masking an Aadhaar number in a picture.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/docai"
	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
)

func (s *Server) registerDocAI() {
	cpu := uploadRoute{}
	for _, tool := range []struct {
		route, path string
		handler     func(http.ResponseWriter, *http.Request, uploadInput)
	}{
		{"doc_markdown", "/v1/doc/markdown", s.handleDocMarkdown},
		{"doc_tables", "/v1/doc/tables", s.handleDocTables},
		{"doc_invoice", "/v1/doc/invoice", s.handleDocInvoice},
		{"doc_einvoice_qr", "/v1/doc/einvoice-qr", s.handleEInvoiceQR},
		{"doc_mask_aadhaar", "/v1/doc/mask-aadhaar", s.handleMaskAadhaar},
	} {
		s.handleUpload(tool.route, costUpload, "POST "+tool.path, cpu, tool.handler)
	}
}

// docPages is a document read into pages of positioned words.
type docPages struct {
	pages  []docai.Page
	method string // text (the PDF's own text layer) or ocr
	first  int    // the number of the first page read
}

// ocrCellGap: Tesseract's word boxes hug the letters, so a space is a larger share of
// their height than of a PDF's font box.
const ocrCellGap = 0.9

// readDocument reads a PDF's text layer, or OCRs a PDF without one (pages first to last,
// at most ocrMaxPages) or a picture. ocr=true OCRs a PDF even when it has text. It
// answers the request itself when it cannot.
func (s *Server) readDocument(w http.ResponseWriter, r *http.Request, in uploadInput) (docPages, bool) {
	data := in.Files[0].Data
	language, ok := ocrLanguage(in.option(r, "lang"))
	if !ok {
		s.badRequest(w, r, "lang takes up to three of "+strings.Join(ocrLanguageCodes, ", ")+", joined by + or a comma, such as eng+tam.")
		return docPages{}, false
	}
	first, last, ok := pageRange(in, r)
	if !ok {
		s.badRequest(w, r, "first and last are page numbers, first from 1 and last not before first.")
		return docPages{}, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 85*time.Second)
	defer cancel()
	if isPDF(data) && in.option(r, "ocr") != "true" {
		dir, path, done, err := writeUpload(data, "in.pdf")
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return docPages{}, false
		}
		defer done()
		out, err := s.run(ctx, job{program: "pdftotext", args: []string{"-bbox-layout", "-enc", "UTF-8", "-f", itoa(first), "-l", itoa(last), path, "-"}, dir: dir, timeout: 60 * time.Second})
		if err != nil {
			var failed *jobError
			if errors.As(err, &failed) && strings.Contains(strings.ToLower(failed.stderr), "password") {
				s.pdfError(w, r, errors.New("password"))
				return docPages{}, false
			}
			s.toolFailed(w, r, err, "read the PDF's text")
			return docPages{}, false
		}
		pages, err := docai.ParseBBox(out)
		if err != nil {
			s.badRequest(w, r, "The PDF has no pages in that range.")
			return docPages{}, false
		}
		characters := 0
		for _, p := range pages {
			for _, b := range p.Blocks {
				for _, l := range b.Lines {
					for _, word := range l.Words {
						characters += len([]rune(word.Text))
					}
				}
			}
		}
		if characters >= 20*len(pages) || in.option(r, "ocr") == "false" {
			return docPages{pages: pages, method: "text", first: first}, true
		}
		// A scan: read it with OCR instead.
	}
	dir, done, err := workspace()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return docPages{}, false
	}
	defer done()
	pictures, ok := s.ocrPictures(ctx, w, r, in, dir)
	if !ok {
		return docPages{}, false
	}
	result := docPages{method: "ocr", first: 1}
	if isPDF(data) {
		result.first = first
	}
	for i, picture := range pictures {
		page, err := s.tesseract(ctx, dir, picture, i, language, "3")
		if err != nil {
			s.toolFailed(w, r, err, "read text from the picture")
			return docPages{}, false
		}
		result.pages = append(result.pages, ocrToPage(page))
	}
	return result, true
}

// ocrToPage turns Tesseract's words into a page of lines, one block per line.
func ocrToPage(p ocrPage) docai.Page {
	page := docai.Page{Width: float64(p.Width), Height: float64(p.Height), CellGap: ocrCellGap}
	byLine := map[int]*docai.Line{}
	var order []int
	for _, word := range p.Words {
		w := docai.Word{Text: word.Text, X0: float64(word.Box[0]), Y0: float64(word.Box[1]), X1: float64(word.Box[0] + word.Box[2]), Y1: float64(word.Box[1] + word.Box[3])}
		line, ok := byLine[word.Line]
		if !ok {
			line = &docai.Line{X0: w.X0, Y0: w.Y0, X1: w.X1, Y1: w.Y1}
			byLine[word.Line] = line
			order = append(order, word.Line)
		}
		line.Words = append(line.Words, w)
		line.X0, line.Y0, line.X1, line.Y1 = math.Min(line.X0, w.X0), math.Min(line.Y0, w.Y0), math.Max(line.X1, w.X1), math.Max(line.Y1, w.Y1)
	}
	for _, n := range order {
		page.Blocks = append(page.Blocks, docai.Block{Lines: []docai.Line{*byLine[n]}})
	}
	return page
}

func (d docPages) note() string {
	if d.method == "ocr" {
		return "Read with OCR (Tesseract), so check names and numbers against the document. At most " + itoa(ocrMaxPages) + " pages are read per request; use first and last for the rest. Name every script the text uses in lang."
	}
	return "Read from the PDF's own text. A scan without text is read with OCR instead; ocr=true forces it."
}

type docMarkdown struct {
	Markdown string `json:"markdown"`
	Pages    int    `json:"pages"`
	Method   string `json:"method"`
}

func (s *Server) handleDocMarkdown(w http.ResponseWriter, r *http.Request, in uploadInput) {
	doc, ok := s.readDocument(w, r, in)
	if !ok {
		return
	}
	s.ok(w, r, docMarkdown{Markdown: docai.Markdown(doc.pages), Pages: len(doc.pages), Method: doc.method},
		meta{Note: "Headings come from the size of the text, tables from words that line up in columns. " + doc.note()}, cacheNone)
}

func (s *Server) handleDocTables(w http.ResponseWriter, r *http.Request, in uploadInput) {
	format := in.option(r, "format")
	if format != "" && format != "json" && format != "csv" && format != "markdown" {
		s.badRequest(w, r, "format is json, csv or markdown.")
		return
	}
	doc, ok := s.readDocument(w, r, in)
	if !ok {
		return
	}
	tables := docai.Tables(doc.pages)
	for i := range tables {
		tables[i].Page += doc.first - 1
	}
	if format == "csv" {
		number := 1
		if raw := in.option(r, "table"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				s.badRequest(w, r, "table is the table's number, from 1.")
				return
			}
			number = n
		}
		if number > len(tables) {
			s.notFound(w, r, "The document has "+itoa(len(tables))+" tables.")
			return
		}
		var b bytes.Buffer
		out := csv.NewWriter(&b)
		_ = out.WriteAll(tables[number-1].Rows)
		sendFile(w, r, b.Bytes(), "text/csv; charset=utf-8", "table-"+itoa(number)+".csv")
		return
	}
	type tableView struct {
		docai.Table
		Markdown string `json:"markdown,omitempty"`
	}
	views := make([]tableView, len(tables))
	for i, t := range tables {
		views[i] = tableView{Table: t}
		if format == "markdown" {
			views[i].Markdown = docai.MarkdownTable(t)
		}
	}
	total := len(views)
	note := "The first row of each table is usually its header. format=csv&table=N gives one table as CSV. " + doc.note()
	if total == 0 {
		note = "No table found: a table here is three or more rows whose words line up in two or more columns. " + doc.note()
	}
	s.ok(w, r, views, meta{Total: &total, Note: note}, cacheNone)
}

func (s *Server) handleDocInvoice(w http.ResponseWriter, r *http.Request, in uploadInput) {
	doc, ok := s.readDocument(w, r, in)
	if !ok {
		return
	}
	invoice := docai.ReadInvoice(doc.pages)
	type answer struct {
		docai.Invoice
		Method string `json:"method"`
		Passed bool   `json:"all_checks_passed"`
	}
	result := answer{Invoice: invoice, Method: doc.method, Passed: true}
	for _, c := range invoice.Checks {
		result.Passed = result.Passed && c.Passed
	}
	s.ok(w, r, result, meta{Note: "Fields are read by their labels, so an unusual layout can leave some empty (null). The checks test the invoice against itself and the GST rules; they do not ask the GST portal whether the invoice was filed. " + doc.note()}, cacheNone)
}

// ---- GST e-invoice signed QR code ----

// eInvoice is the payload of the signed QR code the Invoice Registration Portal puts on
// an e-invoice, with names spelt out.
type eInvoice struct {
	IRN          string  `json:"irn"`
	IRNDate      string  `json:"irn_date"`
	SellerGSTIN  string  `json:"seller_gstin"`
	BuyerGSTIN   string  `json:"buyer_gstin"`
	DocumentNo   string  `json:"document_number"`
	DocumentType string  `json:"document_type"`
	DocumentDate string  `json:"document_date"`
	TotalValue   float64 `json:"total_value"`
	ItemCount    int     `json:"item_count"`
	MainHSN      string  `json:"main_hsn"`
	Issuer       string  `json:"issuer,omitempty"`
	Signature    struct {
		Algorithm string `json:"algorithm"`
		KeyID     string `json:"key_id,omitempty"`
		Verified  bool   `json:"verified"`
	} `json:"signature"`
	Checks []docai.Check `json:"checks"`
}

var eInvoiceDocTypes = map[string]string{"INV": "Tax invoice", "CRN": "Credit note", "DBN": "Debit note"}

// readEInvoiceQR reads the signed QR's text: a JSON Web Token whose payload holds the
// invoice summary as a JSON string in "data".
func readEInvoiceQR(text string) (eInvoice, bool) {
	parts := strings.Split(strings.TrimSpace(text), ".")
	if len(parts) != 3 {
		return eInvoice{}, false
	}
	decode := func(s string) []byte {
		b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
		if err != nil {
			return nil
		}
		return b
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		X5t string `json:"x5t"`
	}
	var payload struct {
		Data string `json:"data"`
		Iss  string `json:"iss"`
	}
	if json.Unmarshal(decode(parts[0]), &header) != nil || json.Unmarshal(decode(parts[1]), &payload) != nil || payload.Data == "" {
		return eInvoice{}, false
	}
	var data struct {
		SellerGstin, BuyerGstin, DocNo, DocTyp, DocDt, Irn, IrnDt, MainHsnCode string
		TotInvVal                                                              json.Number
		ItemCnt                                                                json.Number
	}
	decoder := json.NewDecoder(strings.NewReader(payload.Data))
	decoder.UseNumber()
	if decoder.Decode(&data) != nil || data.Irn == "" {
		return eInvoice{}, false
	}
	out := eInvoice{IRN: data.Irn, IRNDate: data.IrnDt, SellerGSTIN: data.SellerGstin, BuyerGSTIN: data.BuyerGstin, DocumentNo: data.DocNo,
		DocumentType: data.DocTyp, MainHSN: data.MainHsnCode, Issuer: payload.Iss, Checks: []docai.Check{}}
	if name, ok := eInvoiceDocTypes[data.DocTyp]; ok {
		out.DocumentType = name
	}
	if d, ok := docai.ParseDate(data.DocDt); ok {
		out.DocumentDate = d
	} else {
		out.DocumentDate = data.DocDt
	}
	out.TotalValue, _ = data.TotInvVal.Float64()
	count, _ := data.ItemCnt.Int64()
	out.ItemCount = int(count)
	out.Signature.Algorithm, out.Signature.KeyID = header.Alg, header.Kid
	if out.Signature.KeyID == "" {
		out.Signature.KeyID = header.X5t
	}
	add := func(name string, passed bool, detail string) {
		out.Checks = append(out.Checks, docai.Check{Name: name, Passed: passed, Detail: detail})
	}
	for _, party := range []struct{ role, gstin string }{{"seller", out.SellerGSTIN}, {"buyer", out.BuyerGSTIN}} {
		if party.gstin == "" || strings.EqualFold(party.gstin, "URP") {
			continue // URP: an unregistered buyer, for exports
		}
		check := validate.GSTIN(party.gstin)
		detail := party.gstin + " is a valid GSTIN."
		if !check.Valid {
			detail = party.gstin + ": " + check.Reason
		}
		add(party.role+"_gstin", check.Valid, detail)
	}
	irnOK := len(out.IRN) == 64 && strings.Trim(strings.ToLower(out.IRN), "0123456789abcdef") == ""
	add("irn_format", irnOK, "The IRN is a SHA-256 hash: 64 hexadecimal characters. This one has "+itoa(len(out.IRN))+".")
	return out, true
}

func (s *Server) handleEInvoiceQR(w http.ResponseWriter, r *http.Request, in uploadInput) {
	data := in.Files[0].Data
	var pictures []image.Image
	if isPDF(data) {
		dir, path, done, err := writeUpload(data, "in.pdf")
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		defer done()
		if _, err := s.run(r.Context(), job{program: "pdftoppm", args: []string{"-r", "200", "-gray", "-png", "-f", "1", "-l", "3", path, filepath.Join(dir, "page")}, dir: dir, timeout: 60 * time.Second}); err != nil {
			s.toolFailed(w, r, err, "read the PDF")
			return
		}
		names, _ := filepath.Glob(filepath.Join(dir, "page-*.png"))
		sort.Slice(names, func(a, b int) bool { return pageNumber(names[a]) < pageNumber(names[b]) })
		for _, name := range names {
			raw, err := os.ReadFile(name)
			if err != nil {
				continue
			}
			if picture, err := png.Decode(bytes.NewReader(raw)); err == nil {
				pictures = append(pictures, picture)
			}
		}
	} else {
		picture, err := s.loadPicture(r.Context(), data)
		if err != nil {
			s.notPicture(w, r)
			return
		}
		defer freeLarge(picture)
		pictures = append(pictures, picture)
	}
	var codes []foundCode
	for _, picture := range pictures {
		s.extras.render.work(func() { codes = append(codes, readCodes(picture)...) })
	}
	invoices := []eInvoice{}
	for _, code := range codes {
		if code.Format != "qr" {
			continue
		}
		if invoice, ok := readEInvoiceQR(code.Text); ok {
			invoices = append(invoices, invoice)
		}
	}
	if len(invoices) == 0 {
		message := "No e-invoice QR code found."
		if len(codes) > 0 {
			message = "The codes found are not the signed QR code of a GST e-invoice."
		}
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", message+" Send a sharp, straight picture or the PDF (the first three pages are read).")
		return
	}
	total := len(invoices)
	s.ok(w, r, invoices, meta{Total: &total, Note: "The signature is not checked here (verified is false): the QR is signed by the Invoice Registration Portal, and the GST e-invoice portal's \"Verify signed QR code\" checks it. A valid-looking QR proves nothing until its signature is checked."}, cacheNone)
}

// ---- masking an Aadhaar number ----

func (s *Server) handleMaskAadhaar(w http.ResponseWriter, r *http.Request, in uploadInput) {
	data := in.Files[0].Data
	if isPDF(data) {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "Send a picture of the card (JPEG, PNG, WebP, HEIC), not a PDF; /v1/pdf/render draws a PDF's page as one.")
		return
	}
	all := in.option(r, "all") == "true"
	keepQR := in.option(r, "qr") == "false"
	picture, err := s.loadPicture(r.Context(), data)
	if err != nil {
		s.notPicture(w, r)
		return
	}
	defer freeLarge(picture)
	dir, done, err := workspace()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	// Tesseract reads the very picture that is masked, so the boxes match.
	path := filepath.Join(dir, "card.png")
	var encoded bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&encoded, picture); err != nil || os.WriteFile(path, encoded.Bytes(), 0o600) != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 85*time.Second)
	defer cancel()
	var numbers []docai.IDNumber
	// Sparse text suits a card; a full page of a letter reads better automatically.
	for i, psm := range []string{"11", "3"} {
		page, err := s.tesseract(ctx, dir, path, i, "eng", psm)
		if err != nil {
			s.toolFailed(w, r, err, "read the picture")
			return
		}
		lines := map[int][]docai.Word{}
		var order []int
		for _, word := range page.Words {
			if _, ok := lines[word.Line]; !ok {
				order = append(order, word.Line)
			}
			lines[word.Line] = append(lines[word.Line], docai.Word{Text: word.Text, X0: float64(word.Box[0]), Y0: float64(word.Box[1]), X1: float64(word.Box[0] + word.Box[2]), Y1: float64(word.Box[1] + word.Box[3])})
		}
		var grouped [][]docai.Word
		for _, n := range order {
			grouped = append(grouped, lines[n])
		}
		numbers = append(numbers, docai.FindIDNumbers(grouped)...)
		if len(numbers) > 0 {
			break
		}
	}
	var codes []foundCode
	if !keepQR {
		s.extras.render.work(func() { codes = readCodes(picture) })
	}
	qrs := 0
	for _, c := range codes {
		if c.Format == "qr" && len(c.Points) >= 3 {
			qrs++
		}
	}
	if len(numbers) == 0 && qrs == 0 {
		s.fail(w, r, http.StatusUnprocessableEntity, "nothing_found", "No Aadhaar number or Virtual ID was found, so nothing was masked. Send a sharp, straight, well-lit picture of the card or letter.")
		return
	}
	bounds := picture.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, picture, bounds.Min, draw.Src)
	black := image.NewUniform(color.Black)
	for _, n := range numbers {
		// One box over the digits before the last four, or every digit with all=true.
		covered := image.Rectangle{}
		for i, word := range n.Words {
			if !all && i == len(n.Words)-1 {
				if len(n.Digits[i]) <= 4 {
					continue
				}
				// One word of 12 or 16 digits: cover all but its last four.
				word.X1 = word.X0 + (word.X1-word.X0)*float64(len(n.Digits[i])-4)/float64(len(n.Digits[i]))
			}
			pad := (word.Y1 - word.Y0) * 0.2
			covered = covered.Union(image.Rect(int(word.X0-pad), int(word.Y0-pad), int(math.Ceil(word.X1+pad)), int(math.Ceil(word.Y1+pad))))
		}
		draw.Draw(canvas, covered.Add(bounds.Min).Intersect(bounds), black, image.Point{}, draw.Src)
	}
	// The QR code on a card or letter holds the whole number and more: cover it too.
	for _, c := range codes {
		if c.Format != "qr" || len(c.Points) < 3 {
			continue
		}
		x0, y0, x1, y1 := math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64
		for _, p := range c.Points {
			x0, y0, x1, y1 = math.Min(x0, float64(p[0])), math.Min(y0, float64(p[1])), math.Max(x1, float64(p[0])), math.Max(y1, float64(p[1]))
		}
		// The points are the finder patterns' centres: grow the box past the edges.
		pad := math.Max(x1-x0, y1-y0) * 0.25
		box := image.Rect(int(x0-pad), int(y0-pad), int(x1+pad), int(y1+pad)).Add(bounds.Min)
		draw.Draw(canvas, box.Intersect(bounds), black, image.Point{}, draw.Src)
	}
	var out bytes.Buffer
	mediaType, name := "image/png", "masked.png"
	if pictureKind(data) == "jpg" {
		mediaType, name = "image/jpeg", "masked.jpg"
		err = jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 90})
	} else {
		err = (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(&out, canvas)
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	w.Header().Set("X-Masked-Numbers", itoa(len(numbers)))
	w.Header().Set("X-Masked-QR-Codes", itoa(qrs))
	sendFile(w, r, out.Bytes(), mediaType, name)
}
