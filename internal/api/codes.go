package api

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/aztec"
	"github.com/boombuler/barcode/codabar"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/code39"
	"github.com/boombuler/barcode/code93"
	"github.com/boombuler/barcode/datamatrix"
	"github.com/boombuler/barcode/ean"
	"github.com/boombuler/barcode/pdf417"
	"github.com/boombuler/barcode/qr"
	"github.com/boombuler/barcode/twooffive"
)

// QR codes and barcodes, generated only. What goes into a code can be anything from a
// URL with a token to a Wi-Fi password, so every request here is private: not logged
// and not stored in shared caches.

// modules is an encoded code as a grid of dark (true) and light cells.
type modules struct {
	cells         [][]bool
	width, height int
	linear        bool // a 1D barcode: one row, drawn as bars of any height
}

func readModules(code barcode.Barcode) modules {
	bounds := code.Bounds()
	grid := modules{width: bounds.Dx(), height: bounds.Dy(), linear: code.Metadata().Dimensions == 1}
	if grid.linear {
		grid.height = 1
	}
	grid.cells = make([][]bool, grid.height)
	for y := 0; y < grid.height; y++ {
		grid.cells[y] = make([]bool, grid.width)
		for x := 0; x < grid.width; x++ {
			r, _, _, _ := code.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			grid.cells[y][x] = r < 0x8000
		}
	}
	return grid
}

// drawing says how a grid becomes an image.
type drawing struct {
	format     string // svg or png
	margin     int    // light cells around the code (the quiet zone)
	scale      int    // pixels per cell
	barHeight  int    // pixels, for 1D codes
	foreground color.RGBA
	background color.RGBA // alpha 0 is transparent
}

var hexColor = regexp.MustCompile(`^#?([0-9a-fA-F]{6}|[0-9a-fA-F]{3})$`)

func parseColor(value string, fallback color.RGBA) (color.RGBA, error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return fallback, nil
	case strings.EqualFold(value, "transparent"):
		return color.RGBA{}, nil
	case !hexColor.MatchString(value):
		return fallback, fmt.Errorf("colours are hex, such as 000000 or 1f6feb, or transparent")
	}
	hex := strings.TrimPrefix(value, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	n, _ := strconv.ParseUint(hex, 16, 32)
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 0xff}, nil
}

func cssColor(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// readDrawing reads the options every code endpoint shares. size is the pixel width a 2D
// code should come close to; 1D codes are sized by scale and height instead.
func readDrawing(get func(string) string, linear bool, defaultMargin int) (drawing, error) {
	d := drawing{format: strings.ToLower(get("format")), margin: defaultMargin, barHeight: 80}
	if d.format == "" {
		d.format = "svg"
	}
	if d.format != "svg" && d.format != "png" {
		return d, errors.New("format must be svg or png")
	}
	number := func(name string, fallback, min, max int) (int, error) {
		raw := strings.TrimSpace(get(name))
		if raw == "" {
			return fallback, nil
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < min || value > max {
			return 0, fmt.Errorf("%s must be an integer from %d to %d", name, min, max)
		}
		return value, nil
	}
	var err error
	if d.margin, err = number("margin", defaultMargin, 0, 16); err != nil {
		return d, err
	}
	if linear {
		if d.scale, err = number("scale", 2, 1, 10); err != nil {
			return d, err
		}
		if d.barHeight, err = number("height", 80, 10, 600); err != nil {
			return d, err
		}
	} else if d.scale, err = number("size", 256, 32, 2048); err != nil {
		// For 2D codes scale holds the wanted size until the grid is known.
		return d, err
	}
	if d.foreground, err = parseColor(get("fg"), color.RGBA{A: 0xff}); err != nil {
		return d, err
	}
	if d.foreground.A == 0 {
		return d, errors.New("fg cannot be transparent")
	}
	if d.background, err = parseColor(get("bg"), color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}); err != nil {
		return d, err
	}
	return d, nil
}

// The largest code image drawn. A barcode's width grows with its text, and the PNG is
// held in memory as one byte per pixel: unchecked, a long text at a large scale asks
// for gigabytes. Nothing scannable comes near these limits.
const (
	maxCodeSide   = 10000
	maxCodePixels = 16 << 20
)

// render draws a grid. For a 2D code the cell size is the largest whole number of
// pixels that keeps the image within the asked size, at least one. It refuses an image
// larger than the limits above, before drawing anything.
func (d drawing) render(grid modules) ([]byte, string, error) {
	cellsWide := grid.width + 2*d.margin
	cellsHigh := grid.height + 2*d.margin
	scaleX, scaleY := d.scale, d.scale
	if grid.linear {
		cellsHigh = 1
		scaleY = d.barHeight
	} else {
		scaleX = max(1, d.scale/cellsWide)
		scaleY = scaleX
	}
	wide, high := cellsWide*scaleX, cellsHigh*scaleY
	if wide > maxCodeSide || high > maxCodeSide || wide*high > maxCodePixels {
		return nil, "", fmt.Errorf("the image would be %d by %d pixels, larger than this API draws (at most %d pixels a side); use shorter text or a smaller scale", wide, high, maxCodeSide)
	}
	if d.format == "png" {
		return d.png(grid, cellsWide, cellsHigh, scaleX, scaleY), typePNG, nil
	}
	return d.svg(grid, cellsWide, cellsHigh, scaleX, scaleY), typeSVG, nil
}

func (d drawing) svg(grid modules, cellsWide, cellsHigh, scaleX, scaleY int) []byte {
	var out bytes.Buffer
	// In SVG units a cell is 1 wide; a 1D code's single row is as tall as asked, which
	// need not be a whole number of cells.
	unitHigh := strconv.Itoa(cellsHigh)
	if grid.linear {
		unitHigh = strconv.FormatFloat(float64(scaleY)/float64(scaleX), 'f', -1, 64)
	}
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %s" width="%d" height="%d" shape-rendering="crispEdges">`,
		cellsWide, unitHigh, cellsWide*scaleX, cellsHigh*scaleY)
	if d.background.A != 0 {
		fmt.Fprintf(&out, `<rect width="%d" height="%s" fill="%s"/>`, cellsWide, unitHigh, cssColor(d.background))
	}
	fmt.Fprintf(&out, `<path fill="%s" d="`, cssColor(d.foreground))
	for y, row := range grid.cells {
		for x := 0; x < len(row); {
			if !row[x] {
				x++
				continue
			}
			run := 0
			for x+run < len(row) && row[x+run] {
				run++
			}
			if grid.linear {
				fmt.Fprintf(&out, "M%d 0h%dv%sh-%dz", x+d.margin, run, unitHigh, run)
			} else {
				fmt.Fprintf(&out, "M%d %dh%dv1h-%dz", x+d.margin, y+d.margin, run, run)
			}
			x += run
		}
	}
	out.WriteString(`"/></svg>`)
	return out.Bytes()
}

func (d drawing) png(grid modules, cellsWide, cellsHigh, scaleX, scaleY int) []byte {
	palette := color.Palette{d.background, d.foreground}
	img := image.NewPaletted(image.Rect(0, 0, cellsWide*scaleX, cellsHigh*scaleY), palette)
	for y := 0; y < img.Bounds().Dy(); y++ {
		row := grid.cells[0]
		if !grid.linear {
			cellY := y/scaleY - d.margin
			if cellY < 0 || cellY >= grid.height {
				continue
			}
			row = grid.cells[cellY]
		}
		for x := 0; x < img.Bounds().Dx(); x++ {
			cellX := x/scaleX - d.margin
			if cellX >= 0 && cellX < grid.width && row[cellX] {
				img.SetColorIndex(x, y, 1)
			}
		}
	}
	var out bytes.Buffer
	(&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&out, img)
	return out.Bytes()
}

// sendCode writes a generated image. Nothing generated here may be cached by a shared
// cache: the content is the caller's.
func (s *Server) sendCode(w http.ResponseWriter, r *http.Request, body []byte, contentType string) {
	if contentType == typeSVG {
		w.Header().Set("Content-Security-Policy", svgPolicy)
	}
	writeBody(w, r, http.StatusOK, body, contentType, "private, no-store", contentType == typeSVG)
}

// ---- QR ----

var qrLevels = map[string]qr.ErrorCorrectionLevel{"l": qr.L, "m": qr.M, "q": qr.Q, "h": qr.H}

func encodeQR(content, level string) (barcode.Barcode, error) {
	if level == "" {
		level = "m"
	}
	ecc, ok := qrLevels[strings.ToLower(level)]
	if !ok {
		return nil, errors.New("ecc must be L, M, Q or H")
	}
	code, err := qr.Encode(content, ecc, qr.Auto)
	if err != nil {
		return nil, fmt.Errorf("the text does not fit in a QR code at this error correction level (at most about 2,900 bytes at L): %v", err)
	}
	return code, nil
}

func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	input, ok := s.readText(w, r)
	if !ok {
		return
	}
	get := func(name string) string { return input.option(r, name) }
	d, err := readDrawing(get, false, 4)
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	code, err := encodeQR(input.Text, get("ecc"))
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	body, contentType, err := d.render(readModules(code))
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	s.sendCode(w, r, body, contentType)
}

// ---- UPI ----

var (
	upiAddress = regexp.MustCompile(`^[a-zA-Z0-9._-]{2,256}@[a-zA-Z0-9]{2,64}$`)
	upiAmount  = regexp.MustCompile(`^[0-9]{1,8}(\.[0-9]{1,2})?$`)
)

// upiEscape percent-encodes a value for a upi:// link. Apps differ in what they decode,
// so only letters, digits and - . _ ~ @ stay as they are (NPCI's own examples write the
// @ of a UPI ID plainly), and a space is always %20, never +.
func upiEscape(value string) string {
	var out strings.Builder
	for _, b := range []byte(value) {
		switch {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', strings.IndexByte("-._~@", b) >= 0:
			out.WriteByte(b)
		default:
			fmt.Fprintf(&out, "%%%02X", b)
		}
	}
	return out.String()
}

type upiLink struct {
	Link   string `json:"link"`
	Payee  string `json:"payeeAddress"`
	Name   string `json:"payeeName,omitempty"`
	Amount string `json:"amount,omitempty"`
	// QR is the address of a QR code image for the same link.
	QR string `json:"qr,omitempty"`
}

// buildUPILink makes a upi://pay link in the form NPCI's linking specification gives.
func buildUPILink(get func(string) string) (upiLink, error) {
	link := upiLink{Payee: strings.TrimSpace(get("pa")), Name: strings.TrimSpace(get("pn")), Amount: strings.TrimSpace(get("am"))}
	if !upiAddress.MatchString(link.Payee) {
		return link, errors.New("pa must be a UPI ID, such as shop@okaxis")
	}
	if link.Amount != "" {
		if !upiAmount.MatchString(link.Amount) {
			return link, errors.New("am is the amount in rupees with at most two decimals, such as 499 or 499.50")
		}
		if value, _ := strconv.ParseFloat(link.Amount, 64); value <= 0 {
			return link, errors.New("am must be more than zero")
		}
		value, _ := strconv.ParseFloat(link.Amount, 64)
		link.Amount = strconv.FormatFloat(value, 'f', 2, 64)
	}
	parts := []string{"pa=" + upiEscape(link.Payee)}
	optional := []struct{ name, value string }{
		{"pn", link.Name}, {"am", link.Amount}, {"tn", strings.TrimSpace(get("tn"))},
		{"tr", strings.TrimSpace(get("tr"))}, {"mc", strings.TrimSpace(get("mc"))},
	}
	for _, field := range optional {
		if len(field.value) > 80 {
			return link, fmt.Errorf("%s is longer than 80 characters", field.name)
		}
		if field.value != "" {
			parts = append(parts, field.name+"="+upiEscape(field.value))
		}
		if field.name == "am" && field.value != "" {
			parts = append(parts, "cu=INR")
		}
	}
	if mc := strings.TrimSpace(get("mc")); mc != "" && (len(mc) != 4 || strings.Trim(mc, "0123456789") != "") {
		return link, errors.New("mc is a four-digit merchant category code")
	}
	link.Link = "upi://pay?" + strings.Join(parts, "&")
	return link, nil
}

func (s *Server) handleUPIQR(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	query := r.URL.Query()
	link, err := buildUPILink(query.Get)
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	if strings.EqualFold(query.Get("format"), "json") {
		image := r.URL.Query()
		image.Del("format")
		link.QR = s.origin(r) + "/v1/upi-qr?" + image.Encode()
		s.ok(w, r, link, meta{Note: "A link built to NPCI's UPI linking format. It is not checked against any bank: whether the UPI ID exists is only known when someone pays."}, "private, no-store")
		return
	}
	d, err := readDrawing(query.Get, false, 4)
	if err != nil {
		s.badRequest(w, r, err.Error()+", or json for the link itself.")
		return
	}
	code, err := encodeQR(link.Link, query.Get("ecc"))
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	body, contentType, err := d.render(readModules(code))
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	s.sendCode(w, r, body, contentType)
}

// ---- barcodes ----

type barcodeKind struct {
	ID, Name string
	encode   func(text string) (barcode.Barcode, error)
	margin   int
}

var barcodeKinds = []barcodeKind{
	{"code128", "Code 128: any ASCII text, used on shipping labels", func(t string) (barcode.Barcode, error) { return code128.Encode(t) }, 10},
	{"code39", "Code 39: A-Z, 0-9 and - . $ / + % space", func(t string) (barcode.Barcode, error) { return code39.Encode(t, false, false) }, 10},
	{"code93", "Code 93: like Code 39, denser", func(t string) (barcode.Barcode, error) { return code93.Encode(t, true, false) }, 10},
	{"ean13", "EAN-13: retail products; give 12 digits and the check digit is added, or 13 to have it checked", eanOf(12, 13), 10},
	{"ean8", "EAN-8: small retail products; 7 or 8 digits", eanOf(7, 8), 10},
	{"itf", "Interleaved 2 of 5: an even number of digits, used on cartons", func(t string) (barcode.Barcode, error) { return twooffive.Encode(t, true) }, 10},
	{"codabar", "Codabar: digits and - $ : / . +, starting and ending with A, B, C or D", func(t string) (barcode.Barcode, error) { return codabar.Encode(t) }, 10},
	{"datamatrix", "Data Matrix: a small square 2D code", func(t string) (barcode.Barcode, error) { return datamatrix.Encode(t) }, 1},
	{"pdf417", "PDF417: a stacked 2D code, used on ID cards and boarding passes", func(t string) (barcode.Barcode, error) { return pdf417.Encode(t, 4) }, 2},
	{"aztec", "Aztec: a 2D code with a bullseye, used on tickets", func(t string) (barcode.Barcode, error) { return aztec.Encode([]byte(t), 23, 0) }, 1},
}

func eanOf(short, full int) func(string) (barcode.Barcode, error) {
	return func(text string) (barcode.Barcode, error) {
		if (len(text) != short && len(text) != full) || strings.Trim(text, "0123456789") != "" {
			return nil, fmt.Errorf("give %d digits, or %d with the check digit", short, full)
		}
		return ean.Encode(text)
	}
}

func barcodeKindIDs() []string {
	ids := make([]string, len(barcodeKinds))
	for i, kind := range barcodeKinds {
		ids[i] = kind.ID
	}
	return ids
}

func (s *Server) handleBarcode(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	var kind *barcodeKind
	for i := range barcodeKinds {
		if barcodeKinds[i].ID == strings.ToLower(r.PathValue("type")) {
			kind = &barcodeKinds[i]
		}
	}
	if kind == nil {
		s.notFound(w, r, "Unknown barcode type. Use one of: "+strings.Join(barcodeKindIDs(), ", ")+". For QR codes use /v1/qr.")
		return
	}
	input, ok := s.readText(w, r)
	if !ok {
		return
	}
	code, err := kind.encode(input.Text)
	if err != nil {
		s.badRequest(w, r, "That text cannot be encoded as "+kind.ID+": "+err.Error()+".")
		return
	}
	grid := readModules(code)
	d, err := readDrawing(func(name string) string { return input.option(r, name) }, grid.linear, kind.margin)
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	body, contentType, err := d.render(grid)
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	s.sendCode(w, r, body, contentType)
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "make_upi_payment_link",
		Description: "Build a UPI payment link (upi://pay) for an Indian UPI ID, with optional payee name, amount and note, plus the URL of a QR code image for it. Generate only; nothing is checked with a bank.",
		InputSchema: schema([]string{"upi_id"}, map[string]any{
			"upi_id": text("Payee UPI ID, such as shop@okaxis."),
			"name":   text("Payee name."),
			"amount": text("Amount in rupees, such as 499.50."),
			"note":   text("Transaction note."),
		}),
		path: func(args map[string]any) (string, error) {
			_, err := required(args, "upi_id")
			if args["format"] == nil {
				args["format"] = "json"
			}
			return "/v1/upi-qr" + query(args, map[string]string{"upi_id": "pa", "name": "pn", "amount": "am", "note": "tn", "format": "format"}), err
		},
	})
}
