package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"math/bits"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/aztec"
	"github.com/makiuchi-d/gozxing/datamatrix"
	multiqr "github.com/makiuchi-d/gozxing/multi/qrcode"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/qrcode"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// Uploaded pictures read here, in the server: barcode reading and perceptual hashes.
// JPEG, PNG, GIF, WebP, BMP and TIFF are decoded in Go. Anything else (HEIC from a
// phone, AVIF, JPEG XL) and pictures too large to hold whole are first turned into a
// smaller PNG by libvips, when the server has it.

const (
	// goPixels is the largest picture decoded whole in Go: 24 megapixels is about 100 MB
	// as RGBA, and a phone photo is 12.
	goPixels = 24_000_000
	// shrinkTo is the longest side libvips shrinks a larger picture to.
	shrinkTo = 4000
)

var errNotPicture = errors.New("not a picture")

// loadPicture decodes an uploaded picture.
// Callers that may hold a large picture call debug.FreeOSMemory when done (freeLarge).
func (s *Server) loadPicture(ctx context.Context, data []byte) (image.Image, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && config.Width*config.Height <= goPixels && config.Width > 0 && config.Height > 0 {
		var picture image.Image
		s.extras.render.work(func() { picture, _, err = image.Decode(bytes.NewReader(data)) })
		if err == nil {
			return picture, nil
		}
	}
	// libvips reads more kinds and shrinks while it reads, so a huge picture never
	// has to be held whole.
	dir, done, err := workspace()
	if err != nil {
		return nil, err
	}
	defer done()
	in := filepath.Join(dir, "in")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, "out.png")
	if _, err := s.run(ctx, job{program: "vips", args: []string{"thumbnail", in, out, itoa(shrinkTo), "--size", "down"}, dir: dir, timeout: 30 * time.Second}); err != nil {
		return nil, errNotPicture
	}
	shrunk, err := os.ReadFile(out)
	if err != nil {
		return nil, errNotPicture
	}
	picture, _, err := image.Decode(bytes.NewReader(shrunk))
	if err != nil {
		return nil, errNotPicture
	}
	return picture, nil
}

func (s *Server) notPicture(w http.ResponseWriter, r *http.Request) {
	s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The file is not a picture this server can read. Send JPEG, PNG, WebP, GIF, BMP, TIFF, HEIC or AVIF.")
}

// ---- perceptual hashes ----

// pictureHashes are three 64-bit fingerprints of how a picture looks. Pictures that look
// alike have hashes a few bits apart, whatever their size, format or compression.
type pictureHashes struct {
	Average    string `json:"average"`
	Difference string `json:"difference"`
	Perceptual string `json:"perceptual"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// grey scales a picture to w by h in grey levels.
func grey(picture image.Image, w, h int) []float64 {
	small := image.NewGray(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(small, small.Bounds(), picture, picture.Bounds(), draw.Src, nil)
	values := make([]float64, w*h)
	for i, v := range small.Pix {
		values[i] = float64(v)
	}
	return values
}

func hashOf(picture image.Image) (hashes pictureHashes, a, d, p uint64) {
	// Average: 8x8, each pixel against the mean.
	pixels := grey(onWhite(picture), 8, 8)
	mean := 0.0
	for _, v := range pixels {
		mean += v
	}
	mean /= 64
	for i, v := range pixels {
		if v > mean {
			a |= 1 << (63 - i)
		}
	}
	// Difference: 9x8, each pixel against its right neighbour.
	pixels = grey(onWhite(picture), 9, 8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if pixels[y*9+x] < pixels[y*9+x+1] {
				d |= 1 << (63 - (y*8 + x))
			}
		}
	}
	// Perceptual: the lowest 8x8 frequencies of a 32x32 cosine transform, each against
	// their median, leaving out the constant term.
	pixels = grey(onWhite(picture), 32, 32)
	frequencies := dct32(pixels)
	low := make([]float64, 0, 64)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			low = append(low, frequencies[y*32+x])
		}
	}
	sorted := append([]float64(nil), low[1:]...)
	sort.Float64s(sorted)
	median := (sorted[31] + sorted[32]) / 2
	for i, v := range low {
		if v > median {
			p |= 1 << (63 - i)
		}
	}
	bounds := picture.Bounds()
	return pictureHashes{
		Average: fmt.Sprintf("%016x", a), Difference: fmt.Sprintf("%016x", d), Perceptual: fmt.Sprintf("%016x", p),
		Width: bounds.Dx(), Height: bounds.Dy(),
	}, a, d, p
}

// onWhite lays a picture with transparency on white, so transparent areas hash the same
// whatever colour their hidden pixels have.
func onWhite(picture image.Image) image.Image {
	if opaque, ok := picture.(interface{ Opaque() bool }); ok && opaque.Opaque() {
		return picture
	}
	canvas := image.NewRGBA(picture.Bounds())
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), picture, picture.Bounds().Min, draw.Over)
	return canvas
}

// dct32 is the two-dimensional DCT-II of a 32x32 block.
func dct32(values []float64) []float64 {
	const n = 32
	var cosines [n][n]float64
	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			cosines[k][i] = math.Cos(math.Pi / n * (float64(i) + 0.5) * float64(k))
		}
	}
	rows := make([]float64, n*n)
	for y := 0; y < n; y++ {
		for k := 0; k < n; k++ {
			sum := 0.0
			for x := 0; x < n; x++ {
				sum += values[y*n+x] * cosines[k][x]
			}
			rows[y*n+k] = sum
		}
	}
	out := make([]float64, n*n)
	for x := 0; x < n; x++ {
		for k := 0; k < n; k++ {
			sum := 0.0
			for y := 0; y < n; y++ {
				sum += rows[y*n+x] * cosines[k][y]
			}
			out[k*n+x] = sum
		}
	}
	return out
}

// freeLarge hands the memory of a large decoded picture back to the system.
func freeLarge(picture image.Image) {
	if bounds := picture.Bounds(); bounds.Dx()*bounds.Dy() > 4_000_000 {
		debug.FreeOSMemory()
	}
}

func (s *Server) handlePictureHash(w http.ResponseWriter, r *http.Request, in uploadInput) {
	picture, err := s.loadPicture(r.Context(), in.Files[0].Data)
	if err != nil {
		s.notPicture(w, r)
		return
	}
	defer freeLarge(picture)
	var hashes pictureHashes
	s.extras.render.work(func() { hashes, _, _, _ = hashOf(picture) })
	s.ok(w, r, hashes, meta{Note: "Hex 64-bit hashes. Compare two pictures by counting the bits that differ: up to about 10 of 64 means they look alike."}, cacheNone)
}

// similarBits is how many of the 64 perceptual-hash bits may differ for two pictures to
// count as the same picture.
const similarBits = 10

type comparison struct {
	Similar  bool             `json:"similar"`
	Distance map[string]int   `json:"distance"`
	Score    float64          `json:"score"`
	Pictures [2]pictureHashes `json:"pictures"`
}

func (s *Server) handlePictureCompare(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if len(in.Files) != 2 {
		s.badRequest(w, r, "Send two pictures as multipart/form-data, for example -F a=@one.jpg -F b=@two.jpg.")
		return
	}
	var result comparison
	var sums [2][3]uint64
	for i, file := range in.Files {
		picture, err := s.loadPicture(r.Context(), file.Data)
		if err != nil {
			s.notPicture(w, r)
			return
		}
		s.extras.render.work(func() {
			result.Pictures[i], sums[i][0], sums[i][1], sums[i][2] = hashOf(picture)
		})
	}
	result.Distance = map[string]int{
		"average":    bits.OnesCount64(sums[0][0] ^ sums[1][0]),
		"difference": bits.OnesCount64(sums[0][1] ^ sums[1][1]),
		"perceptual": bits.OnesCount64(sums[0][2] ^ sums[1][2]),
	}
	result.Similar = result.Distance["perceptual"] <= similarBits
	result.Score = math.Round((1-float64(result.Distance["perceptual"])/64)*1000) / 1000
	s.ok(w, r, result, meta{Note: fmt.Sprintf("Distances are bits that differ, of 64. similar is true when the perceptual hashes differ by %d bits or fewer. score is 1 for the same picture.", similarBits)}, cacheNone)
}

// ---- barcode reading ----

type foundCode struct {
	Format string   `json:"format"`
	Text   string   `json:"text"`
	Points [][2]int `json:"points"`
}

var codeFormats = map[gozxing.BarcodeFormat]string{
	gozxing.BarcodeFormat_QR_CODE: "qr", gozxing.BarcodeFormat_DATA_MATRIX: "datamatrix", gozxing.BarcodeFormat_AZTEC: "aztec",
	gozxing.BarcodeFormat_CODE_128: "code128", gozxing.BarcodeFormat_CODE_39: "code39", gozxing.BarcodeFormat_CODE_93: "code93",
	gozxing.BarcodeFormat_EAN_13: "ean13", gozxing.BarcodeFormat_EAN_8: "ean8", gozxing.BarcodeFormat_UPC_A: "upca",
	gozxing.BarcodeFormat_UPC_E: "upce", gozxing.BarcodeFormat_ITF: "itf", gozxing.BarcodeFormat_CODABAR: "codabar",
}

// readCodes finds every code it can in a picture: all QR codes, then one of each other
// kind. A picture is tried as it is, then with light and dark swapped, for codes printed
// light on dark.
func readCodes(picture image.Image) []foundCode {
	var found []foundCode
	seen := map[string]bool{}
	add := func(result *gozxing.Result) {
		format, ok := codeFormats[result.GetBarcodeFormat()]
		if !ok || seen[format+"\x00"+result.GetText()] {
			return
		}
		seen[format+"\x00"+result.GetText()] = true
		code := foundCode{Format: format, Text: result.GetText(), Points: [][2]int{}}
		for _, point := range result.GetResultPoints() {
			code.Points = append(code.Points, [2]int{int(math.Round(point.GetX())), int(math.Round(point.GetY()))})
		}
		found = append(found, code)
	}
	source := gozxing.NewLuminanceSourceFromImage(picture)
	hints := map[gozxing.DecodeHintType]any{gozxing.DecodeHintType_TRY_HARDER: true}
	for _, luminance := range []gozxing.LuminanceSource{source, source.Invert()} {
		for _, binarizer := range []gozxing.Binarizer{gozxing.NewHybridBinarizer(luminance), gozxing.NewGlobalHistgramBinarizer(luminance)} {
			bitmap, err := gozxing.NewBinaryBitmap(binarizer)
			if err != nil {
				continue
			}
			if results, err := multiqr.NewQRCodeMultiReader().DecodeMultiple(bitmap, hints); err == nil {
				for _, result := range results {
					add(result)
				}
			}
			for _, reader := range []gozxing.Reader{
				qrcode.NewQRCodeReader(), datamatrix.NewDataMatrixReader(), aztec.NewAztecReader(),
				oned.NewMultiFormatUPCEANReader(hints), oned.NewCode128Reader(), oned.NewCode39Reader(),
				oned.NewCode93Reader(), oned.NewITFReader(), oned.NewCodaBarReader(),
			} {
				if result, err := reader.Decode(bitmap, hints); err == nil {
					add(result)
				}
			}
		}
		if len(found) > 0 {
			break
		}
	}
	return found
}

func (s *Server) handleBarcodeRead(w http.ResponseWriter, r *http.Request, in uploadInput) {
	picture, err := s.loadPicture(r.Context(), in.Files[0].Data)
	if err != nil {
		s.notPicture(w, r)
		return
	}
	defer freeLarge(picture)
	var codes []foundCode
	s.extras.render.work(func() { codes = readCodes(picture) })
	if codes == nil {
		codes = []foundCode{}
	}
	note := "Points are pixel positions in the picture: the corners or finder patterns of each code."
	if len(codes) == 0 {
		note = "No code found. Reads QR, Data Matrix, Aztec, Code 128, Code 39, Code 93, EAN, UPC, ITF and Codabar; not PDF417. A sharp, straight, well-lit photo works best."
	}
	total := len(codes)
	s.ok(w, r, codes, meta{Total: &total, Note: strings.TrimSpace(note)}, cacheNone)
}
