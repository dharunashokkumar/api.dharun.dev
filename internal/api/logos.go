package api

import (
	"bytes"
	"container/list"
	"crypto/sha1"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

const (
	pngMinSize     = 16
	pngMaxSize     = dataset.PNGMasterSize
	pngDefaultSize = 128

	typeSVG = "image/svg+xml; charset=utf-8"
	typePNG = "image/png"

	// A logo opened directly in a tab is a document, so it may do nothing but draw itself.
	svgPolicy = "default-src 'none'; style-src 'unsafe-inline'; img-src data:"
)

var (
	imageSuffix    = regexp.MustCompile(`(?i)\.(svg|png)$`)
	monogramColors = []color.RGBA{
		{0x22, 0x48, 0xff, 0xff}, {0x0f, 0x76, 0x6e, 0xff}, {0xb4, 0x53, 0x09, 0xff}, {0xbe, 0x12, 0x3c, 0xff},
		{0x6d, 0x28, 0xd9, 0xff}, {0x15, 0x80, 0x3d, 0xff}, {0x03, 0x69, 0xa1, 0xff}, {0xa2, 0x1c, 0xaf, 0xff},
	}
)

func (s *Server) handleLogo(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	symbol := r.PathValue("symbol")
	format := strings.ToLower(query.Get("format"))

	if suffix := imageSuffix.FindString(symbol); suffix != "" {
		symbol = symbol[:len(symbol)-len(suffix)]
		if format == "" {
			format = strings.ToLower(suffix[1:])
		}
	}
	if format == "" {
		format = "svg"
	}
	if format != "svg" && format != "png" {
		s.fail(w, r, http.StatusBadRequest, "bad_format", "format must be svg or png.")
		return
	}
	size := pngDefaultSize
	if format == "png" {
		var err error
		if size, err = intParam(r, "size", pngDefaultSize, pngMinSize, pngMaxSize); err != nil {
			s.fail(w, r, http.StatusBadRequest, "bad_size", err.Error())
			return
		}
	}
	fallback := strings.ToLower(query.Get("fallback"))
	if fallback != "" && fallback != "monogram" {
		s.fail(w, r, http.StatusBadRequest, "bad_fallback", "fallback must be monogram.")
		return
	}

	info(r).subject = symbol
	c, err := s.companies.Load().resolve(symbol, query.Get("exchange"))
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}

	if c == nil {
		if fallback == "" {
			s.metrics.observeLogo(format, "not_found")
			s.notFound(w, r, `No logo found for "`+symbol+`". Add ?fallback=monogram for a placeholder.`)
			return
		}
		s.metrics.observeLogo(format, "fallback")
		letters, background := monogram(symbol)
		w.Header().Set("X-Logo-Fallback", "monogram")
		if format == "svg" {
			w.Header().Set("Content-Security-Policy", svgPolicy)
			writeBody(w, r, http.StatusOK, monogramSVG(letters, background), typeSVG, cacheShort, false)
			return
		}
		image, err := s.png.get("monogram:"+letters+"|"+strconv.Itoa(size), func() ([]byte, error) {
			return monogramPNG(letters, background, size)
		})
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not draw the placeholder.")
			return
		}
		writeBody(w, r, http.StatusOK, image, typePNG, cacheShort, false)
		return
	}

	logos := s.need(w, r, "logos")
	if logos == nil {
		return
	}
	s.metrics.observeLogo(format, "found")
	if format == "svg" {
		var svg []byte
		if err := logos.row(`SELECT svg FROM logos WHERE id = ?`, c.ID).Scan(&svg); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not read the logo.")
			return
		}
		w.Header().Set("Content-Security-Policy", svgPolicy)
		writeBody(w, r, http.StatusOK, svg, typeSVG, cacheLong, false)
		return
	}

	image, err := s.png.get(logos.fingerprint+"|"+c.ID+"|"+strconv.Itoa(size), func() ([]byte, error) {
		var master []byte
		if err := logos.row(`SELECT png FROM logos WHERE id = ?`, c.ID).Scan(&master); err != nil {
			return nil, err
		}
		if master == nil {
			return nil, errNoPNG
		}
		return resizePNG(master, size)
	})
	switch {
	case errors.Is(err, errNoPNG):
		s.fail(w, r, http.StatusNotImplemented, "png_unavailable", "PNG logos were not built into this server. Use format=svg.")
	case err != nil:
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not render the logo.")
	default:
		writeBody(w, r, http.StatusOK, image, typePNG, cacheLong, false)
	}
}

var errNoPNG = errors.New("no PNG master")

// resizePNG scales the stored master down to the requested edge length.
func resizePNG(master []byte, size int) ([]byte, error) {
	source, err := png.Decode(bytes.NewReader(master))
	if err != nil {
		return nil, err
	}
	if source.Bounds().Dx() == size {
		return master, nil
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), source, source.Bounds(), draw.Src, nil)
	return encodePNG(scaled)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// monogram picks the letters and colour of the stand-in tile for an unknown symbol.
// Only A-Z and 0-9 ever reach the output.
func monogram(symbol string) (string, color.RGBA) {
	letters := looseKey(symbol)
	if len(letters) > 2 {
		letters = letters[:2]
	}
	if letters == "" {
		letters = "?"
	}
	sum := sha1.Sum([]byte(letters))
	return letters, monogramColors[int(sum[0])%len(monogramColors)]
}

func monogramSVG(letters string, background color.RGBA) []byte {
	fontSize := 8
	if len(letters) == 1 {
		fontSize = 10
	}
	return []byte(fmt.Sprintf(
		`<svg viewBox="0 0 18 18" width="18" height="18" xmlns="http://www.w3.org/2000/svg">`+
			`<path fill="#%02x%02x%02x" d="M0 0h18v18H0z"/>`+
			`<text x="9" y="9" fill="#fff" font-family="DejaVu Sans, Arial, Helvetica, sans-serif" font-size="%d" `+
			`font-weight="700" text-anchor="middle" dominant-baseline="central">%s</text></svg>`,
		background.R, background.G, background.B, fontSize, letters))
}

var (
	monogramFont     *opentype.Font
	monogramFontOnce sync.Once
)

func monogramPNG(letters string, background color.RGBA, size int) ([]byte, error) {
	monogramFontOnce.Do(func() { monogramFont, _ = opentype.Parse(gobold.TTF) })
	if monogramFont == nil {
		return nil, errors.New("embedded font failed to load")
	}

	scale := 0.44
	if len(letters) == 1 {
		scale = 0.56
	}
	face, err := opentype.NewFace(monogramFont, &opentype.FaceOptions{Size: float64(size) * scale, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)

	bounds, _ := font.BoundString(face, letters)
	width, height := bounds.Max.X-bounds.Min.X, bounds.Max.Y-bounds.Min.Y
	origin := fixed.Point26_6{
		X: (fixed.I(size)-width)/2 - bounds.Min.X,
		Y: (fixed.I(size)-height)/2 - bounds.Min.Y,
	}
	(&font.Drawer{Dst: img, Src: image.White, Face: face, Dot: origin}).DrawString(letters)
	return encodePNG(img)
}

// pngCache keeps recently rendered PNGs, bounded by total bytes.
type pngCache struct {
	mu      sync.Mutex
	limit   int
	used    int
	order   *list.List
	entries map[string]*list.Element
}

type pngEntry struct {
	key  string
	data []byte
}

func newPNGCache(limit int) *pngCache {
	return &pngCache{limit: limit, order: list.New(), entries: map[string]*list.Element{}}
}

func (c *pngCache) get(key string, render func() ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	if element, ok := c.entries[key]; ok {
		c.order.MoveToFront(element)
		data := element.Value.(*pngEntry).data
		c.mu.Unlock()
		return data, nil
	}
	c.mu.Unlock()

	data, err := render()
	if err != nil {
		return nil, err
	}
	if len(data) > c.limit {
		return data, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; !ok {
		c.entries[key] = c.order.PushFront(&pngEntry{key, data})
		c.used += len(data)
		for c.used > c.limit {
			oldest := c.order.Back()
			entry := c.order.Remove(oldest).(*pngEntry)
			delete(c.entries, entry.key)
			c.used -= len(entry.data)
		}
	}
	return data, nil
}
