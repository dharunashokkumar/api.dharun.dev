package api

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"
)

func init() {
	invoice, _ := sampleFile("invoice.pdf")
	qr, _ := sampleFile("einvoice-qr.png")
	card, _ := sampleFile("idcard.png")
	pdf := `{"file":"` + base64.StdEncoding.EncodeToString(invoice) + `"}`
	mcpTestCalls["read_document_as_markdown"] = pdf
	mcpTestCalls["read_document_tables"] = pdf
	mcpTestCalls["read_gst_invoice"] = pdf
	mcpTestCalls["read_einvoice_qr"] = `{"file":"` + base64.StdEncoding.EncodeToString(qr) + `"}`
	mcpTestCalls["mask_aadhaar_number"] = `{"file":"` + base64.StdEncoding.EncodeToString(card) + `"}`
}

func TestDocMarkdownAndTables(t *testing.T) {
	requireProgram(t, "pdftotext")
	invoice := sampleOf(t, "invoice.pdf")
	got := postBody(t, "/v1/doc/markdown", "application/pdf", invoice)
	if got.status != http.StatusOK {
		t.Fatalf("doc/markdown = %d: %s", got.status, got.body)
	}
	data := got.json(t)["data"].(map[string]any)
	md := data["markdown"].(string)
	if data["method"] != "text" || !strings.Contains(md, "| 1 | Basmati rice, 25 kg bag | 1006 |") || !strings.HasPrefix(md, "# Sri Murugan Stores") {
		t.Errorf("markdown: %v\n%s", data["method"], md)
	}
	got = postBody(t, "/v1/doc/tables?format=markdown", "application/pdf", invoice)
	if got.status != http.StatusOK {
		t.Fatalf("doc/tables = %d: %s", got.status, got.body)
	}
	tables := got.json(t)["data"].([]any)
	if len(tables) < 3 {
		t.Fatalf("tables: %s", got.body)
	}
	got = postBody(t, "/v1/doc/tables?format=csv&table=2", "application/pdf", invoice)
	if got.status != http.StatusOK || !strings.HasPrefix(got.header.Get("Content-Type"), "text/csv") {
		t.Fatalf("csv = %d %s: %s", got.status, got.header.Get("Content-Type"), got.body)
	}
	rows, err := csv.NewReader(bytes.NewReader(got.body)).ReadAll()
	if err != nil || len(rows) != 3 || rows[1][1] != "Basmati rice, 25 kg bag" {
		t.Errorf("csv rows %v, %v", rows, err)
	}
	if got := postBody(t, "/v1/doc/tables?format=csv&table=99", "application/pdf", invoice); got.status != http.StatusNotFound {
		t.Errorf("table 99 = %d", got.status)
	}
	if got := postBody(t, "/v1/doc/tables?format=xml", "application/pdf", invoice); got.status != http.StatusBadRequest {
		t.Errorf("format=xml = %d", got.status)
	}
	if got := postBody(t, "/v1/doc/markdown?lang=xyz", "application/pdf", invoice); got.status != http.StatusBadRequest {
		t.Errorf("lang=xyz = %d", got.status)
	}
}

func TestDocInvoice(t *testing.T) {
	requireProgram(t, "pdftotext")
	got := postBody(t, "/v1/doc/invoice", "application/pdf", sampleOf(t, "invoice.pdf"))
	if got.status != http.StatusOK {
		t.Fatalf("doc/invoice = %d: %s", got.status, got.body)
	}
	data := got.json(t)["data"].(map[string]any)
	if data["number"] != "INV-2026-0042" || data["all_checks_passed"] != true || data["supply"] != "inter_state" {
		t.Errorf("invoice: %s", got.body)
	}
	amounts := data["amounts"].(map[string]any)
	if amounts["total"] != 13013.0 || amounts["igst"] != 1037.5 || amounts["cgst"] != nil {
		t.Errorf("amounts: %v", amounts)
	}
	if seller := data["seller"].(map[string]any); seller["state"] != "Tamil Nadu" {
		t.Errorf("seller: %v", seller)
	}
}

func TestEInvoiceQR(t *testing.T) {
	got := postBody(t, "/v1/doc/einvoice-qr", "image/png", sampleOf(t, "einvoice-qr.png"))
	if got.status != http.StatusOK {
		t.Fatalf("einvoice-qr = %d: %s", got.status, got.body)
	}
	list := got.json(t)["data"].([]any)
	if len(list) != 1 {
		t.Fatalf("found %d", len(list))
	}
	e := list[0].(map[string]any)
	if e["seller_gstin"] != "33AABCS1429B1Z1" || e["document_number"] != "INV-2026-0042" || e["document_date"] != "2026-10-14" ||
		e["total_value"] != 13013.0 || e["document_type"] != "Tax invoice" || e["signature"].(map[string]any)["verified"] != false {
		t.Errorf("e-invoice: %s", got.body)
	}
	for _, c := range e["checks"].([]any) {
		if c.(map[string]any)["passed"] != true {
			t.Errorf("check failed: %v", c)
		}
	}
	// An ordinary QR code is not an e-invoice.
	if got := postBody(t, "/v1/doc/einvoice-qr", "image/png", sampleOf(t, "codes.png")); got.status != http.StatusUnprocessableEntity {
		t.Errorf("codes.png = %d", got.status)
	}
	if _, ok := readEInvoiceQR("a.b"); ok {
		t.Error("two parts read as a token")
	}
}

func TestMaskAadhaar(t *testing.T) {
	requireProgram(t, "tesseract")
	card := sampleOf(t, "idcard.png")
	got := postBody(t, "/v1/doc/mask-aadhaar", "image/png", card)
	if got.status != http.StatusOK {
		t.Fatalf("mask = %d: %s", got.status, got.body)
	}
	if got.header.Get("X-Masked-Numbers") != "1" || got.header.Get("X-Masked-QR-Codes") != "1" {
		t.Errorf("masked %s numbers, %s QR codes", got.header.Get("X-Masked-Numbers"), got.header.Get("X-Masked-QR-Codes"))
	}
	masked, err := png.Decode(bytes.NewReader(got.body))
	if err != nil {
		t.Fatal(err)
	}
	original, _ := png.Decode(bytes.NewReader(card))
	if masked.Bounds() != original.Bounds() {
		t.Errorf("size changed: %v", masked.Bounds())
	}
	// The QR code is gone, so the card's code cannot be read any more.
	if codes := readCodes(masked); len(codes) != 0 {
		t.Errorf("codes still readable: %v", codes)
	}
	// The masked picture no longer shows the first eight digits to OCR.
	again := postBody(t, "/v1/doc/mask-aadhaar?qr=false", "image/png", got.body)
	if again.status != http.StatusUnprocessableEntity {
		t.Errorf("masked card masked again = %d", again.status)
	}
	if got := postBody(t, "/v1/doc/mask-aadhaar", "image/png", blankPNG(t)); got.status != http.StatusUnprocessableEntity {
		t.Errorf("blank = %d: %s", got.status, got.body)
	}
}

func blankPNG(t *testing.T) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, 200, 100))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
