package api

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/aztec"
	"github.com/makiuchi-d/gozxing/datamatrix"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func init() {
	mcpTestCalls["make_upi_payment_link"] = `{"upi_id":"shop@okaxis","name":"Sri Murugan Stores","amount":"250"}`
}

// scan decodes a PNG the way a phone camera would, so a test proves the code works and
// not only that an image came back.
func scan(t *testing.T, body []byte, reader gozxing.Reader) string {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	result, err := reader.Decode(bitmap, map[gozxing.DecodeHintType]any{gozxing.DecodeHintType_PURE_BARCODE: true})
	if err != nil {
		t.Fatalf("the code does not scan: %v", err)
	}
	return result.GetText()
}

func pngSize(t *testing.T, body []byte) image.Point {
	t.Helper()
	config, err := png.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	return image.Pt(config.Width, config.Height)
}

func wellFormedSVG(t *testing.T, body []byte) {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		_, err := decoder.Token()
		if err != nil {
			if err.Error() != "EOF" {
				t.Fatalf("SVG is not well-formed XML: %v\n%s", err, clip(string(body), 300))
			}
			return
		}
	}
}

func TestQR(t *testing.T) {
	text := "https://api.dharun.dev/v1/pincode/637001?உங்கள்=நன்றி"
	got := expectStatus(t, "/v1/qr?format=png&size=300&text="+url.QueryEscape(text), http.StatusOK)
	if decoded := scan(t, got.body, qrcode.NewQRCodeReader()); decoded != text {
		t.Errorf("scanned %q, want %q", decoded, text)
	}
	if size := pngSize(t, got.body); size.X != size.Y || size.X > 300 || size.X < 150 {
		t.Errorf("PNG is %v, want a square close to 300", size)
	}
	if got.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("a generated code must not be shared-cached: %q", got.header.Get("Cache-Control"))
	}

	// The same text posted as the body, with options in the query, gives the same image.
	posted := request(t, http.MethodPost, "/v1/qr?format=png&size=300", text, "Content-Type", "text/plain; charset=utf-8")
	if posted.status != http.StatusOK || !bytes.Equal(posted.body, got.body) {
		t.Errorf("POST text/plain: %d, same image %v", posted.status, bytes.Equal(posted.body, got.body))
	}
	asJSON := request(t, http.MethodPost, "/v1/qr", `{"text":"hello","format":"png","ecc":"H","size":200}`, "Content-Type", "application/json")
	if asJSON.status != http.StatusOK || scan(t, asJSON.body, qrcode.NewQRCodeReader()) != "hello" {
		t.Errorf("POST JSON: %d", asJSON.status)
	}

	svg := expectStatus(t, "/v1/qr?text=hello&fg=1f6feb&bg=transparent", http.StatusOK)
	wellFormedSVG(t, svg.body)
	if !strings.Contains(string(svg.body), `fill="#1f6feb"`) || strings.Contains(string(svg.body), "<rect") || !strings.HasPrefix(svg.header.Get("Content-Type"), "image/svg+xml") {
		t.Errorf("SVG colours or type wrong: %s", clip(string(svg.body), 200))
	}

	for _, bad := range []string{
		"/v1/qr", "/v1/qr?text=", "/v1/qr?text=a&ecc=X", "/v1/qr?text=a&format=gif", "/v1/qr?text=a&size=5000",
		"/v1/qr?text=a&fg=red", "/v1/qr?text=a&fg=transparent", "/v1/qr?text=a&margin=40",
		"/v1/qr?ecc=H&text=" + strings.Repeat("x", 1400),
	} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
	if bad := request(t, http.MethodPost, "/v1/qr", `{"text":`, "Content-Type", "application/json"); bad.status != http.StatusBadRequest {
		t.Errorf("broken JSON: %d", bad.status)
	}
	if put := request(t, http.MethodPut, "/v1/qr", "x"); put.status != http.StatusMethodNotAllowed {
		t.Errorf("PUT: %d", put.status)
	}
}

func TestPostBodyLimit(t *testing.T) {
	server := newTestServer(Config{Version: "t", MaxBodyBytes: 1024})
	defer server.Close()
	recorder := httptestPost(server, "/v1/qr", strings.Repeat("x", 2000))
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("an oversized body: %d %s", recorder.Code, recorder.Body)
	}
}

func TestUPIQR(t *testing.T) {
	link := expectStatus(t, "/v1/upi-qr?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250&tn=Order%2042&format=json", http.StatusOK)
	data := link.data(t)
	want := "upi://pay?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250.00&cu=INR&tn=Order%2042"
	if data["link"] != want {
		t.Errorf("link = %v\nwant   %s", data["link"], want)
	}
	if qr, _ := data["qr"].(string); !strings.HasPrefix(qr, testOrigin+"/v1/upi-qr?") || strings.Contains(qr, "format=json") {
		t.Errorf("qr link: %v", data["qr"])
	}
	if link.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("a UPI ID must not be shared-cached: %q", link.header.Get("Cache-Control"))
	}

	image := expectStatus(t, "/v1/upi-qr?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250&tn=Order%2042&format=png", http.StatusOK)
	if decoded := scan(t, image.body, qrcode.NewQRCodeReader()); decoded != want {
		t.Errorf("the QR code holds %q", decoded)
	}
	// Without an amount the payer types it, and no currency is given either.
	if open := expectStatus(t, "/v1/upi-qr?pa=a.b-c_d@ybl&format=json", http.StatusOK).data(t); open["link"] != "upi://pay?pa=a.b-c_d@ybl" {
		t.Errorf("open amount: %v", open["link"])
	}
	for _, bad := range []string{
		"/v1/upi-qr", "/v1/upi-qr?pa=nobody", "/v1/upi-qr?pa=a@b", "/v1/upi-qr?pa=shop@okaxis&am=-5",
		"/v1/upi-qr?pa=shop@okaxis&am=0", "/v1/upi-qr?pa=shop@okaxis&am=1.234", "/v1/upi-qr?pa=shop@okaxis&mc=12",
		"/v1/upi-qr?pa=shop@okaxis&tn=" + strings.Repeat("x", 81),
	} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
}

func TestBarcodes(t *testing.T) {
	cases := []struct {
		kind, text, want string
		reader           gozxing.Reader
	}{
		{"code128", "AWB-1234 5678", "AWB-1234 5678", oned.NewCode128Reader()},
		{"code39", "TN28AB1234", "TN28AB1234", oned.NewCode39Reader()},
		{"code93", "HELLO-93", "HELLO-93", oned.NewCode93Reader()},
		{"ean13", "890123456789", "8901234567890", oned.NewEAN13Reader()}, // check digit 0: weighted sum 130
		{"ean8", "1234567", "12345670", oned.NewEAN8Reader()},
		{"itf", "12345678", "12345678", oned.NewITFReader()},
		{"codabar", "A12345B", "A12345B", oned.NewCodaBarReader()},
		{"datamatrix", "Namakkal 637001", "Namakkal 637001", datamatrix.NewDataMatrixReader()},
		{"aztec", "PNR 4512369870", "PNR 4512369870", aztec.NewAztecReader()},
	}
	for _, c := range cases {
		got := expectStatus(t, "/v1/barcode/"+c.kind+"?format=png&text="+url.QueryEscape(c.text), http.StatusOK)
		if got.status != http.StatusOK {
			continue
		}
		decoded := scan(t, got.body, c.reader)
		// Codabar readers drop the start and stop letters.
		if decoded != c.want && !(c.kind == "codabar" && decoded == strings.Trim(c.want, "AB")) {
			t.Errorf("%s: scanned %q, want %q", c.kind, decoded, c.want)
		}
		wellFormedSVG(t, expectStatus(t, "/v1/barcode/"+c.kind+"?text="+url.QueryEscape(c.text), http.StatusOK).body)
	}
	// PDF417 has no reader here; check it draws a wide, stacked image.
	if size := pngSize(t, expectStatus(t, "/v1/barcode/pdf417?format=png&text=boarding%20pass", http.StatusOK).body); size.X <= size.Y {
		t.Errorf("PDF417 is %v", size)
	}

	linear := pngSize(t, expectStatus(t, "/v1/barcode/code128?format=png&text=abc&scale=3&height=120", http.StatusOK).body)
	if linear.Y != 120 || linear.X%3 != 0 {
		t.Errorf("a linear code with scale 3 and height 120 is %v", linear)
	}
	posted := request(t, http.MethodPost, "/v1/barcode/code128?format=png", "AWB-1234 5678", "Content-Type", "text/plain")
	if posted.status != http.StatusOK || scan(t, posted.body, oned.NewCode128Reader()) != "AWB-1234 5678" {
		t.Errorf("POST barcode: %d", posted.status)
	}

	// The SVG is as tall as asked even when the height is not a multiple of the scale.
	svg := string(expectStatus(t, "/v1/barcode/code128?text=abc&scale=10&height=81", http.StatusOK).body)
	if !strings.Contains(svg, `height="81"`) || !strings.Contains(svg, " 8.1\"") {
		t.Errorf("an SVG barcode 81 pixels high at scale 10 starts %.200s", svg)
	}

	// A long text at a large scale would need gigabytes as a PNG: refused before drawing.
	// Code 39 has no length limit of its own.
	long := strings.Repeat("A", 20000)
	tooBig := request(t, http.MethodPost, "/v1/barcode/code39?format=png&scale=10&height=600", long, "Content-Type", "text/plain")
	if tooBig.status != http.StatusBadRequest || !strings.Contains(string(tooBig.body), "pixels") {
		t.Errorf("an oversized barcode: %d %s", tooBig.status, tooBig.body)
	}
	if fits := request(t, http.MethodPost, "/v1/barcode/code39?format=png&scale=1&height=10", strings.Repeat("A", 300), "Content-Type", "text/plain"); fits.status != http.StatusOK {
		t.Errorf("a long barcode within the limits: %d %s", fits.status, fits.body)
	}

	expectStatus(t, "/v1/barcode/upc?text=1", http.StatusNotFound)
	for _, bad := range []string{
		"/v1/barcode/ean13?text=12345", "/v1/barcode/ean13?text=89012345678X", "/v1/barcode/itf?text=123",
		"/v1/barcode/code39?text=lowercase%7E", "/v1/barcode/code128?text=a&height=5000", "/v1/barcode/code128",
	} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
}

func httptestPost(server *Server, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "text/plain")
	server.ServeHTTP(recorder, request)
	return recorder
}
