package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
	"golang.org/x/image/draw"
)

// Tests of the endpoints that take files. Those that run an outside program skip
// themselves where it is not installed, unless REQUIRE_TOOLS is set (CI and the Linux
// test container install them all); those that need the voices or the background model
// run only where MODELS_DIR offers a refreshed copy, or must when REQUIRE_MODELS is set.

func init() {
	mcpTestCalls["make_gst_invoice"] = `{"invoice":` + invoiceExample + `,"format":"json"}`
	mcpTestCalls["compile_typst"] = `{"source":"= Hello"}`
	mcpTestCalls["text_to_speech"] = `{"text":"Hello","voice":"en"}`
	receipt, _ := sampleFile("receipt.png")
	codes, _ := sampleFile("codes.png")
	mcpTestCalls["read_text_in_image"] = `{"file":"` + base64.StdEncoding.EncodeToString(receipt) + `","lang":"eng+tam"}`
	mcpTestCalls["read_barcodes"] = `{"file":"data:image/png;base64,` + base64.StdEncoding.EncodeToString(codes) + `"}`
}

// missingTool reports an answer that only says this machine lacks a program or model.
func missingTool(got result) bool {
	if got.status != http.StatusServiceUnavailable {
		return false
	}
	switch {
	case bytes.Contains(got.body, []byte(`"not_available"`)):
		return os.Getenv("REQUIRE_TOOLS") == ""
	case bytes.Contains(got.body, []byte(`"dataset_unavailable"`)):
		return os.Getenv("REQUIRE_MODELS") == ""
	}
	return false
}

// requireProgram skips a test that needs an outside program this machine lacks.
func requireProgram(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if testServer.extras.programs.path(name) == "" {
			if os.Getenv("REQUIRE_TOOLS") != "" {
				t.Fatalf("%s is not installed, and REQUIRE_TOOLS is set", name)
			}
			t.Skipf("%s is not installed here", name)
		}
	}
}

func requireDataset(t *testing.T, name string) {
	t.Helper()
	if testServer.data(name) == nil {
		if os.Getenv("REQUIRE_MODELS") != "" {
			t.Fatalf("no %s dataset, and REQUIRE_MODELS is set", name)
		}
		t.Skipf("no %s dataset here; set MODELS_DIR to a live directory holding one", name)
	}
}

// copyModelDatasets copies the voices and matting datasets from a refreshed live
// directory into the tests' own.
func copyModelDatasets(t interface{ Fatal(...any) }, from, to string) {
	if from == "" {
		return
	}
	for _, name := range []string{"voices", "matting"} {
		files, _ := filepath.Glob(filepath.Join(from, name+".*"))
		for _, file := range files {
			if err := os.MkdirAll(to, 0o755); err != nil {
				t.Fatal(err)
			}
			// A hard link where possible: the files are large.
			target := filepath.Join(to, filepath.Base(file))
			if os.Link(file, target) != nil {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func sampleOf(t *testing.T, name string) []byte {
	t.Helper()
	data, ok := sampleFile(name)
	if !ok {
		t.Fatalf("no sample file %s", name)
	}
	return data
}

func do(t *testing.T, req *http.Request) result {
	t.Helper()
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return result{status: response.StatusCode, header: response.Header, body: body}
}

// postFiles sends files as multipart/form-data, with fields beside them.
func postFiles(t *testing.T, path string, fields map[string]string, files ...[]byte) result {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for name, value := range fields {
		form.WriteField(name, value)
	}
	for i, data := range files {
		part, _ := form.CreateFormFile("file", "upload-"+itoa(i))
		part.Write(data)
	}
	form.Close()
	req, _ := http.NewRequest(http.MethodPost, testHTTP.URL+path, &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return do(t, req)
}

func postBody(t *testing.T, path, contentType string, data []byte) result {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, testHTTP.URL+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", contentType)
	return do(t, req)
}

// testUploadExample posts an endpoint's documented sample and checks the answer's kind.
func testUploadExample(t *testing.T, e endpoint) {
	t.Helper()
	req, err := exampleRequest(context.Background(), e)
	if err != nil {
		t.Errorf("%s: %v", e.Path, err)
		return
	}
	req.URL, _ = req.URL.Parse(testHTTP.URL + e.Example)
	req.RequestURI = ""
	got := do(t, req)
	if missingTool(got) {
		return
	}
	if got.status != http.StatusOK {
		t.Errorf("POST %s = %d: %s", e.Example, got.status, clip(string(got.body), 300))
		return
	}
	kind := got.header.Get("Content-Type")
	switch {
	case e.Output == "pdf":
		if kind != "application/pdf" || !bytes.HasPrefix(got.body, []byte("%PDF-")) {
			t.Errorf("%s: %q is not a PDF", e.Example, kind)
		}
	case e.Output == "audio":
		if !strings.HasPrefix(kind, "audio/") {
			t.Errorf("%s: content type %q", e.Example, kind)
		}
	case e.Image:
		if !strings.HasPrefix(kind, "image/") {
			t.Errorf("%s: content type %q", e.Example, kind)
		}
	default:
		envelope := got.json(t)
		if _, ok := envelope["data"]; !ok {
			t.Errorf("%s: no data in envelope", e.Example)
		}
	}
	if !e.Private {
		t.Errorf("%s takes files but is not marked Private", e.Path)
	}
}

func TestUploadInput(t *testing.T) {
	if got := postBody(t, "/v1/image/hash", "image/png", nil); got.status != http.StatusBadRequest {
		t.Errorf("empty body = %d", got.status)
	}
	big := make([]byte, uploadLimit+1)
	if got := postBody(t, "/v1/image/hash", "image/png", big); got.status != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body = %d", got.status)
	}
	if got := postFiles(t, "/v1/image/hash", nil, big); got.status != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized form = %d", got.status)
	}
	photo := sampleOf(t, "photo.jpg")
	if got := postFiles(t, "/v1/image/compare", nil, photo, photo, photo); got.status != http.StatusBadRequest {
		t.Errorf("three files to compare = %d", got.status)
	}
	if got := get(t, "/v1/ocr"); got.status != http.StatusMethodNotAllowed {
		t.Errorf("GET of an upload endpoint = %d", got.status)
	}
	if got := postBody(t, "/v1/image/hash", "text/plain", []byte("not a picture")); got.status != http.StatusUnprocessableEntity {
		t.Errorf("text as a picture = %d", got.status)
	}
	// Options may come as form fields.
	got := postFiles(t, "/v1/pdf/pages", map[string]string{"pages": "2"}, sampleOf(t, "document.pdf"))
	if got.status != http.StatusOK {
		t.Fatalf("pages as a form field = %d: %s", got.status, got.body)
	}
	if pages := pdfPages(t, got.body); pages != 1 {
		t.Errorf("pages=2 gave %d pages", pages)
	}
	if got.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("a file made from an upload is cacheable: %q", got.header.Get("Cache-Control"))
	}
}

func pdfPages(t *testing.T, pdf []byte) int {
	t.Helper()
	got := postBody(t, "/v1/pdf/info", "application/pdf", pdf)
	if got.status != http.StatusOK {
		t.Fatalf("pdf/info = %d: %s", got.status, got.body)
	}
	return int(got.json(t)["data"].(map[string]any)["pages"].(float64))
}

func TestPDFTools(t *testing.T) {
	document := sampleOf(t, "document.pdf")
	info := postBody(t, "/v1/pdf/info", "application/pdf", document).json(t)["data"].(map[string]any)
	if info["pages"].(float64) != 2 {
		t.Errorf("sample has %v pages", info["pages"])
	}
	if sizes := info["page_sizes"].([]any); len(sizes) == 0 || sizes[0].(map[string]any)["name"] != "A4" {
		t.Errorf("page sizes %v", info["page_sizes"])
	}
	merged := postFiles(t, "/v1/pdf/merge", nil, document, document, document)
	if merged.status != http.StatusOK || pdfPages(t, merged.body) != 6 {
		t.Errorf("merge = %d", merged.status)
	}
	if got := postFiles(t, "/v1/pdf/merge", nil, document); got.status != http.StatusBadRequest {
		t.Errorf("merging one file = %d", got.status)
	}
	picked := postBody(t, "/v1/pdf/pages?pages=2,1,2", "application/pdf", document)
	if picked.status != http.StatusOK || pdfPages(t, picked.body) != 3 {
		t.Errorf("pages=2,1,2 = %d", picked.status)
	}
	for _, bad := range []string{"", "0-x", "1;2"} {
		if got := postBody(t, "/v1/pdf/pages?pages="+bad, "application/pdf", document); got.status != http.StatusBadRequest {
			t.Errorf("pages=%q = %d", bad, got.status)
		}
	}
	if got := postBody(t, "/v1/pdf/rotate?angle=90&pages=1", "application/pdf", document); got.status != http.StatusOK || pdfPages(t, got.body) != 2 {
		t.Errorf("rotate = %d", got.status)
	}
	if got := postBody(t, "/v1/pdf/rotate?angle=45", "application/pdf", document); got.status != http.StatusBadRequest {
		t.Errorf("rotate by 45 = %d", got.status)
	}
	compressed := postBody(t, "/v1/pdf/compress", "application/pdf", document)
	if compressed.status != http.StatusOK || len(compressed.body) > len(document) || compressed.header.Get("X-Original-Bytes") != itoa(len(document)) {
		t.Errorf("compress = %d, %d bytes from %d", compressed.status, len(compressed.body), len(document))
	}
	if got := postBody(t, "/v1/pdf/info", "application/pdf", []byte("%PDF-1.7 not really")); got.status != http.StatusUnprocessableEntity {
		t.Errorf("broken PDF = %d", got.status)
	}
	if got := postBody(t, "/v1/pdf/info", "application/pdf", sampleOf(t, "photo.jpg")); got.status != http.StatusUnprocessableEntity {
		t.Errorf("a picture as a PDF = %d", got.status)
	}
}

func TestPDFTextAndPictures(t *testing.T) {
	requireProgram(t, "pdftotext", "pdftoppm")
	document := sampleOf(t, "document.pdf")
	got := postBody(t, "/v1/pdf/text", "application/pdf", document)
	if got.status != http.StatusOK {
		t.Fatalf("pdf/text = %d: %s", got.status, got.body)
	}
	data := got.json(t)["data"].(map[string]any)
	pages := data["pages"].([]any)
	if len(pages) != 2 || data["needs_ocr"] != false {
		t.Fatalf("pdf/text: %v", data)
	}
	if text := pages[1].(map[string]any)["text"].(string); !strings.Contains(text, "வணக்கம்") {
		t.Errorf("page 2 text lost its Tamil: %q", text)
	}
	picture := postBody(t, "/v1/pdf/render?page=2&width=300", "application/pdf", document)
	if picture.status != http.StatusOK {
		t.Fatalf("render = %d: %s", picture.status, picture.body)
	}
	if config, err := png.DecodeConfig(bytes.NewReader(picture.body)); err != nil || config.Width != 300 {
		t.Errorf("render: %v, %+v", err, config)
	}
	if got := postBody(t, "/v1/pdf/render?page=9", "application/pdf", document); got.status != http.StatusBadRequest {
		t.Errorf("render of a missing page = %d", got.status)
	}
}

func TestPictureHashes(t *testing.T) {
	photo := sampleOf(t, "photo.jpg")
	original, _, err := image.Decode(bytes.NewReader(photo))
	if err != nil {
		t.Fatal(err)
	}
	// The same picture, half the size and as PNG, must still look alike.
	bounds := original.Bounds()
	half := image.NewRGBA(image.Rect(0, 0, bounds.Dx()/2, bounds.Dy()/2))
	draw.ApproxBiLinear.Scale(half, half.Bounds(), original, bounds, draw.Src, nil)
	var smaller bytes.Buffer
	png.Encode(&smaller, half)

	got := postFiles(t, "/v1/image/compare", nil, photo, smaller.Bytes())
	if got.status != http.StatusOK {
		t.Fatalf("compare = %d: %s", got.status, got.body)
	}
	if data := got.json(t)["data"].(map[string]any); data["similar"] != true {
		t.Errorf("a picture and its half-size copy are not similar: %v", data["distance"])
	}
	other := postFiles(t, "/v1/image/compare", nil, photo, sampleOf(t, "codes.png")).json(t)["data"].(map[string]any)
	if other["similar"] != false {
		t.Errorf("a photo and a QR code look alike: %v", other["distance"])
	}
	hashes := postBody(t, "/v1/image/hash", "image/jpeg", photo).json(t)["data"].(map[string]any)
	if len(hashes["perceptual"].(string)) != 16 || hashes["width"].(float64) != float64(bounds.Dx()) {
		t.Errorf("hash: %v", hashes)
	}
}

func TestBarcodeReading(t *testing.T) {
	got := postBody(t, "/v1/barcode/read", "image/png", sampleOf(t, "codes.png"))
	if got.status != http.StatusOK {
		t.Fatalf("read = %d: %s", got.status, got.body)
	}
	found := map[string]string{}
	for _, item := range got.json(t)["data"].([]any) {
		code := item.(map[string]any)
		found[code["format"].(string)] = code["text"].(string)
	}
	if found["qr"] != "https://api.dharun.dev" || found["ean13"] != "8901234567890" {
		t.Errorf("codes found: %v", found)
	}
	// A code this API drew reads back.
	drawn := get(t, "/v1/qr?text=வணக்கம்%201234&format=png&size=300")
	read := postBody(t, "/v1/barcode/read", "image/png", drawn.body).json(t)["data"].([]any)
	if len(read) != 1 || read[0].(map[string]any)["text"] != "வணக்கம் 1234" {
		t.Errorf("read back %v", read)
	}
	// A picture without a code is not an error.
	empty := postBody(t, "/v1/barcode/read", "image/jpeg", sampleOf(t, "photo.jpg"))
	if empty.status != http.StatusOK || len(empty.json(t)["data"].([]any)) != 0 {
		t.Errorf("photo without codes = %d: %s", empty.status, clip(string(empty.body), 200))
	}
}

func TestImageConvertAndInfo(t *testing.T) {
	requireProgram(t, "vips", "vipsheader")
	photo := sampleOf(t, "photo.jpg")
	info := postBody(t, "/v1/image/info", "image/jpeg", photo)
	if info.status != http.StatusOK {
		t.Fatalf("info = %d: %s", info.status, info.body)
	}
	data := info.json(t)["data"].(map[string]any)
	if data["format"] != "jpeg" || data["width"].(float64) < 100 {
		t.Errorf("info: %v", data)
	}
	for format, magic := range map[string]string{"webp": "RIFF", "png": "\x89PNG", "jpeg": "\xff\xd8\xff", "avif": "", "gif": "GIF8"} {
		got := postBody(t, "/v1/image/convert?format="+format+"&width=200", "image/jpeg", photo)
		if got.status != http.StatusOK {
			t.Errorf("convert to %s = %d: %s", format, got.status, got.body)
			continue
		}
		if !bytes.HasPrefix(got.body, []byte(magic)) || got.header.Get("Content-Type") != imageFormats[format].mediaType {
			t.Errorf("convert to %s gave %q, %q", format, got.header.Get("Content-Type"), clip(string(got.body), 12))
		}
		if format == "avif" && !bytes.Contains(got.body[:32], []byte("avif")) {
			t.Errorf("not an AVIF file")
		}
	}
	cover := postBody(t, "/v1/image/convert?format=png&width=120&height=120&fit=cover", "image/jpeg", photo)
	if config, err := png.DecodeConfig(bytes.NewReader(cover.body)); err != nil || config.Width != 120 || config.Height != 120 {
		t.Errorf("fit=cover: %v %+v", err, config)
	}
	// Never enlarged by default.
	big := postBody(t, "/v1/image/convert?format=png&width=8000", "image/jpeg", photo)
	if config, err := png.DecodeConfig(bytes.NewReader(big.body)); err != nil || config.Width != int(data["width"].(float64)) {
		t.Errorf("enlarged: %v %+v", err, config)
	}
	for _, bad := range []string{"format=bmp", "width=0", "quality=101", "fit=cover&width=10"} {
		if got := postBody(t, "/v1/image/convert?"+bad, "image/jpeg", photo); got.status != http.StatusBadRequest {
			t.Errorf("%s = %d", bad, got.status)
		}
	}
}

func TestOCR(t *testing.T) {
	requireProgram(t, "tesseract", "vips", "pdftoppm")
	got := postBody(t, "/v1/ocr?lang=eng+tam&words=true", "image/png", sampleOf(t, "receipt.png"))
	if got.status != http.StatusOK {
		t.Fatalf("ocr = %d: %s", got.status, got.body)
	}
	data := got.json(t)["data"].(map[string]any)
	text := data["text"].(string)
	for _, want := range []string{"MURUGAN", "Total", "நன்றி"} {
		if !strings.Contains(text, want) {
			t.Errorf("OCR text lacks %q: %q", want, text)
		}
	}
	words := data["pages"].([]any)[0].(map[string]any)["words"].([]any)
	if len(words) < 5 || data["confidence"].(float64) < 50 {
		t.Errorf("%d words, confidence %v", len(words), data["confidence"])
	}
	// A PDF is drawn first; Tamil comes through.
	pdf := postBody(t, "/v1/ocr?lang=ta,en&first=2&last=2", "application/pdf", sampleOf(t, "document.pdf"))
	if pdf.status != http.StatusOK {
		t.Fatalf("ocr of a PDF = %d: %s", pdf.status, pdf.body)
	}
	pages := pdf.json(t)["data"].(map[string]any)["pages"].([]any)
	if len(pages) != 1 || pages[0].(map[string]any)["page"].(float64) != 2 || !strings.Contains(pages[0].(map[string]any)["text"].(string), "வணக்கம்") {
		t.Errorf("ocr of page 2: %v", pages)
	}
	if got := postBody(t, "/v1/ocr?lang=klingon", "image/png", sampleOf(t, "receipt.png")); got.status != http.StatusBadRequest {
		t.Errorf("unknown language = %d", got.status)
	}
}

func TestOCRLanguages(t *testing.T) {
	for in, want := range map[string]string{"": "eng", "ta": "tam", "en+ta": "eng+tam", "Tamil, English": "tam+eng", "hin+hi": "hin"} {
		if got, ok := ocrLanguage(in); !ok || got != want {
			t.Errorf("ocrLanguage(%q) = %q, %v", in, got, ok)
		}
	}
	for _, bad := range []string{"xx", "eng+tam+hin+tel"} {
		if _, ok := ocrLanguage(bad); ok {
			t.Errorf("ocrLanguage(%q) accepted", bad)
		}
	}
}

func TestTypst(t *testing.T) {
	requireProgram(t, "typst")
	got := postBody(t, "/v1/typst", "text/plain", []byte(typstExample))
	if got.status != http.StatusOK || !bytes.HasPrefix(got.body, []byte("%PDF-")) {
		t.Fatalf("typst = %d: %s", got.status, clip(string(got.body), 300))
	}
	picture := postBody(t, "/v1/typst?format=png&ppi=72", "text/plain", []byte(typstExample))
	if picture.status != http.StatusOK || !bytes.HasPrefix(picture.body, []byte("\x89PNG")) {
		t.Errorf("typst png = %d: %s", picture.status, clip(string(picture.body), 300))
	}
	mistake := postBody(t, "/v1/typst", "text/plain", []byte("= Title\n#undefined-thing"))
	if mistake.status != http.StatusUnprocessableEntity || !strings.Contains(string(mistake.body), "main.typ:2") || strings.Contains(string(mistake.body), os.TempDir()) {
		t.Errorf("a mistake = %d: %s", mistake.status, mistake.body)
	}
	// Nothing outside the document can be reached: no package, no file.
	for _, document := range []string{`#import "@preview/cetz:0.3.4": *`, `#read("/etc/passwd")`, `#image("../../../etc/hostname")`} {
		started := time.Now()
		got := postBody(t, "/v1/typst", "text/plain", []byte(document))
		if got.status != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d: %s", document, got.status, clip(string(got.body), 200))
		}
		if time.Since(started) > 10*time.Second {
			t.Errorf("%s took %v: it may have tried the network", document, time.Since(started))
		}
	}
}

func TestInvoiceMaths(t *testing.T) {
	var request invoiceRequest
	if err := json.Unmarshal([]byte(invoiceExample), &request); err != nil {
		t.Fatal(err)
	}
	today := time.Date(2026, 10, 14, 0, 0, 0, 0, indiaTime)
	result, err := workInvoice(request, today)
	if err != nil {
		t.Fatal(err)
	}
	// Tamil Nadu to Karnataka: IGST. Rice 4 x 2150 at 5%; tiffins 10 x 349.50 less 120 at 18%.
	totals := result.Totals
	if result.Supply != "inter_state" || result.PlaceOfSupply.Code != "KA" || totals.Taxable != 11975 || totals.IGST != 1037.5 || totals.CGST != 0 ||
		totals.Total != 13013 || totals.RoundOff != 0.5 || totals.Words != "Thirteen thousand thirteen rupees only" {
		t.Errorf("inter-state: %+v %+v", result.PlaceOfSupply, totals)
	}
	if len(result.Summary) != 2 || result.Summary[0].HSN != "1006" || result.Summary[0].IGST != 430 {
		t.Errorf("summary %+v", result.Summary)
	}

	// The same sale within Tamil Nadu: CGST and SGST, half each.
	request.Buyer.GSTIN, request.Buyer.State = "", "Tamil Nadu"
	result, err = workInvoice(request, today)
	if err != nil {
		t.Fatal(err)
	}
	if result.Supply != "intra_state" || result.StateTax != "SGST" || result.Totals.CGST != 518.75 || result.Totals.SGST != 518.75 || result.Totals.IGST != 0 {
		t.Errorf("intra-state: %+v", result.Totals)
	}
	// Place of supply wins over the buyer's state.
	request.PlaceOfSupply = "07"
	if result, _ = workInvoice(request, today); result.Supply != "inter_state" || result.PlaceOfSupply.Name != "Delhi" {
		t.Errorf("place of supply: %+v", result.PlaceOfSupply)
	}
	// A union territory without a legislature charges UTGST.
	chandigarh := request
	chandigarh.PlaceOfSupply = ""
	chandigarh.Seller.GSTIN = "04AABCS1429B1Z" + string(validate.GSTINCheckChar("04AABCS1429B1Z"))
	chandigarh.Buyer.State = "CH"
	if result, err = workInvoice(chandigarh, today); err != nil || result.StateTax != "UTGST" {
		t.Errorf("Chandigarh: %v %+v", err, result)
	}

	for name, change := range map[string]func(*invoiceRequest){
		"bad seller GSTIN": func(r *invoiceRequest) { r.Seller.GSTIN = "33AABCS1429B1Z2" },
		"no number":        func(r *invoiceRequest) { r.Number = "" },
		"long number":      func(r *invoiceRequest) { r.Number = "INV-2026-000000042" },
		"no items":         func(r *invoiceRequest) { r.Items = nil },
		"no gst rate":      func(r *invoiceRequest) { r.Items = []invoiceItem{{Description: "x", Rate: 1}} },
		"rate over 40":     func(r *invoiceRequest) { rate := 41.0; r.Items[0].GSTRate = &rate },
		"bad state":        func(r *invoiceRequest) { r.Buyer.GSTIN, r.Buyer.State = "", "Atlantis" },
		"bad date":         func(r *invoiceRequest) { r.Date = "14/10/2026" },
		"bad HSN":          func(r *invoiceRequest) { r.Items[0].HSN = "10a6" },
		"discount too big": func(r *invoiceRequest) { r.Items[0].Discount = 1e6 },
	} {
		var broken invoiceRequest
		json.Unmarshal([]byte(invoiceExample), &broken)
		change(&broken)
		if _, err := workInvoice(broken, today); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestInvoiceEndpoint(t *testing.T) {
	got := postBody(t, "/v1/invoice?format=json", "application/json", []byte(invoiceExample))
	if got.status != http.StatusOK {
		t.Fatalf("invoice json = %d: %s", got.status, got.body)
	}
	if total := got.json(t)["data"].(map[string]any)["totals"].(map[string]any)["total"]; total != 13013.0 {
		t.Errorf("total %v", total)
	}
	if got := postBody(t, "/v1/invoice?format=json", "application/json", []byte(`{"numbr": "1"}`)); got.status != http.StatusBadRequest || !strings.Contains(string(got.body), "numbr") {
		t.Errorf("unknown field = %d: %s", got.status, got.body)
	}
	requireProgram(t, "typst")
	pdf := postBody(t, "/v1/invoice", "application/json", []byte(invoiceExample))
	if pdf.status != http.StatusOK || !bytes.HasPrefix(pdf.body, []byte("%PDF-")) {
		t.Fatalf("invoice pdf = %d: %s", pdf.status, clip(string(pdf.body), 300))
	}
	page := postBody(t, "/v1/invoice?format=png", "application/json", []byte(invoiceExample))
	if page.status != http.StatusOK || !bytes.HasPrefix(page.body, []byte("\x89PNG")) {
		t.Errorf("invoice png = %d", page.status)
	}
	// Names in Indian scripts and an intra-state sale set as well.
	var request map[string]any
	json.Unmarshal([]byte(invoiceExample), &request)
	request["buyer"] = map[string]any{"name": "முருகன் ஸ்டோர்ஸ்", "state": "TN"}
	request["ship_to"] = map[string]any{"name": "गोदाम", "address": "Salem"}
	request["upi"] = ""
	body, _ := json.Marshal(request)
	if got := postBody(t, "/v1/invoice", "application/json", body); got.status != http.StatusOK {
		t.Errorf("intra-state invoice = %d: %s", got.status, clip(string(got.body), 300))
	}
}

func TestIndianNumbers(t *testing.T) {
	for value, want := range map[float64]string{0: "0.00", 999: "999.00", 1000: "1,000.00", 100000: "1,00,000.00", 1234567.8: "12,34,567.80", 123456789012: "1,23,45,67,89,012.00", -1500.5: "-1,500.50"} {
		if got := inr(value); got != want {
			t.Errorf("inr(%v) = %q, want %q", value, got, want)
		}
	}
}

func TestSpeech(t *testing.T) {
	voices := get(t, "/v1/tts/voices").json(t)
	if len(voices["data"].([]any)) != 10 {
		t.Errorf("%d voices", len(voices["data"].([]any)))
	}
	for value, want := range map[string]string{"": "en_US-ljspeech-medium", "hi": "hi_IN-rohan-medium", "en-GB": "en_GB-cori-medium", "te_IN": "te_IN-padmavathi-medium", "hi_IN-pratham-medium": "hi_IN-pratham-medium"} {
		if voice, ok := findVoice(value); !ok || voice.ID != want {
			t.Errorf("findVoice(%q) = %s", value, voice.ID)
		}
	}
	if _, ok := findVoice("ta"); ok {
		t.Error("a Tamil voice was found")
	}
	if got := get(t, "/v1/tts?text=hello&voice=xx"); got.status != http.StatusBadRequest {
		t.Errorf("unknown voice = %d", got.status)
	}
	if got := get(t, "/v1/tts?text=hello&speed=5"); got.status != http.StatusBadRequest {
		t.Errorf("speed 5 = %d", got.status)
	}
	wav := wavFile(make([]byte, 100), 22050)
	if len(wav) != 144 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		t.Errorf("wav header: %q", wav[:12])
	}

	requireProgram(t, "piper", "ffmpeg")
	requireDataset(t, "voices")
	for _, test := range []struct{ query, magic string }{
		{"text=Hello%20from%20India%20API&format=wav", "RIFF"},
		{"text=" + "%E0%A4%A8%E0%A4%AE%E0%A4%B8%E0%A5%8D%E0%A4%A4%E0%A5%87&voice=hi&format=mp3", ""},
		{"text=%E0%B0%A8%E0%B0%AE%E0%B0%B8%E0%B1%8D%E0%B0%95%E0%B0%BE%E0%B0%B0%E0%B0%82&voice=te&format=ogg", "OggS"},
		{"text=%E0%B4%A8%E0%B4%AE%E0%B4%B8%E0%B5%8D%E0%B4%95%E0%B4%BE%E0%B4%B0%E0%B4%82&voice=ml_IN-meera-medium&format=mp3", ""},
	} {
		got := get(t, "/v1/tts?"+test.query)
		if got.status != http.StatusOK || !bytes.HasPrefix(got.body, []byte(test.magic)) || len(got.body) < 2000 {
			t.Errorf("tts %s = %d, %d bytes: %s", test.query, got.status, len(got.body), clip(string(got.body), 200))
		}
		if got.header.Get("X-Voice-Licence") == "" {
			t.Errorf("tts %s: no licence header", test.query)
		}
	}
}

func TestDenoise(t *testing.T) {
	requireProgram(t, "deep-filter", "ffmpeg")
	got := postBody(t, "/v1/audio/denoise?format=wav", "audio/mpeg", sampleOf(t, "noisy.mp3"))
	if got.status != http.StatusOK || string(got.body[:4]) != "RIFF" {
		t.Fatalf("denoise = %d: %s", got.status, clip(string(got.body), 200))
	}
	if got := postBody(t, "/v1/audio/denoise", "audio/mpeg", []byte("not audio at all")); got.status != http.StatusUnprocessableEntity {
		t.Errorf("not audio = %d", got.status)
	}
}

func TestRemoveBackground(t *testing.T) {
	requireProgram(t, "onnx-run")
	requireDataset(t, "matting")
	photo := sampleOf(t, "photo.jpg")
	got := postBody(t, "/v1/image/remove-background", "image/jpeg", photo)
	if got.status != http.StatusOK {
		t.Fatalf("remove-background = %d: %s", got.status, clip(string(got.body), 300))
	}
	cut, err := png.Decode(bytes.NewReader(got.body))
	if err != nil {
		t.Fatal(err)
	}
	// The corner is background; the middle is the subject.
	bounds := cut.Bounds()
	_, _, _, corner := cut.At(2, 2).RGBA()
	_, _, _, middle := cut.At(bounds.Dx()/2, bounds.Dy()/2).RGBA()
	if corner > 0x2000 || middle < 0xe000 {
		t.Errorf("alpha at the corner %x, in the middle %x", corner, middle)
	}
	if mask := postBody(t, "/v1/image/remove-background?format=mask", "image/jpeg", photo); mask.status != http.StatusOK {
		t.Errorf("mask = %d", mask.status)
	}
}
