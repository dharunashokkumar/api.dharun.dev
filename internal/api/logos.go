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
	"unicode"

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

	// The tile for an unknown symbol is kept a day, so a logo added later replaces it soon.
	cacheUnknownLogo = "public, max-age=86400"
	// Longer than any real ticker, ISIN or Yahoo symbol, short enough to never be a text.
	maxLogoSymbol = 64

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

// logoRequest is what every logo address reads besides the name: format, size and fallback.
type logoRequest struct {
	symbol, format, fallback string
	size                     int
}

func (s *Server) readLogoRequest(w http.ResponseWriter, r *http.Request, name string) (logoRequest, bool) {
	query := r.URL.Query()
	req := logoRequest{symbol: r.PathValue(name), format: strings.ToLower(query.Get("format")), size: pngDefaultSize}
	if suffix := imageSuffix.FindString(req.symbol); suffix != "" {
		req.symbol = req.symbol[:len(req.symbol)-len(suffix)]
		if req.format == "" {
			req.format = strings.ToLower(suffix[1:])
		}
	}
	if req.format == "" {
		req.format = "svg"
	}
	if req.format != "svg" && req.format != "png" {
		s.fail(w, r, http.StatusBadRequest, "bad_format", "format must be svg or png.")
		return req, false
	}
	if req.format == "png" {
		var err error
		if req.size, err = intParam(r, "size", pngDefaultSize, pngMinSize, pngMaxSize); err != nil {
			s.fail(w, r, http.StatusBadRequest, "bad_size", err.Error())
			return req, false
		}
	}
	req.fallback = strings.ToLower(query.Get("fallback"))
	if req.fallback != "" && req.fallback != "monogram" && req.fallback != "none" {
		s.fail(w, r, http.StatusBadRequest, "bad_fallback", "fallback must be monogram or none.")
		return req, false
	}
	if len(req.symbol) > maxLogoSymbol || strings.ContainsFunc(req.symbol, unicode.IsControl) {
		s.fail(w, r, http.StatusBadRequest, "bad_symbol", "A symbol is at most "+strconv.Itoa(maxLogoSymbol)+" characters, without control characters.")
		return req, false
	}
	info(r).subject = req.symbol
	return req, true
}

// serveUnknownLogo answers a name that matched nothing: the tile, so an <img> tag never
// breaks, or 404 with fallback=none. The name is written down so its logo can be added,
// unless the server has none of the sets that were searched (then every name would miss).
func (s *Server) serveUnknownLogo(w http.ResponseWriter, r *http.Request, req logoRequest, searched bool) {
	if searched {
		s.metrics.missedLogos.add(req.symbol)
	}
	if req.fallback == "none" {
		s.metrics.observeLogo(req.format, "not_found")
		s.notFound(w, r, `No logo found for "`+req.symbol+`". Leave out fallback=none for an initials tile.`)
		return
	}
	w.Header().Set("X-Logo-Known", "false")
	s.serveMonogram(w, r, req.symbol, req.format, req.size, "unknown", cacheUnknownLogo)
}

func (s *Server) handleLogo(w http.ResponseWriter, r *http.Request) {
	req, ok := s.readLogoRequest(w, r, "symbol")
	if !ok {
		return
	}
	// exchange=nse or bse limits the Indian listing; a global exchange skips it and looks
	// only at the global stocks.
	hint := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("exchange")))
	if hint == "AUTO" || hint == "ALL" {
		hint = ""
	}
	global := hint != "" && hint != "NSE" && hint != "BSE"
	if global && !stockExchanges[hint] {
		s.badRequest(w, r, "exchange is nse, bse or one of "+strings.Join(stockExchangeList, ", ")+".")
		return
	}

	index := s.companies.Load()
	var c *company
	if !global {
		var err error
		if c, err = index.resolve(req.symbol, hint); err != nil {
			s.badRequest(w, r, err.Error())
			return
		}
	}
	if c == nil {
		// Not an Indian listing: the brands, global stocks and assets, in that order.
		sets := worldLogoSets
		if global {
			sets = []worldLogoSet{stockLogoSet}
		} else if hint != "" {
			sets = nil // exchange=nse or bse asked for an Indian listing only
		}
		found, searched := s.findWorldLogo(req.symbol, sets, hint)
		if found != nil {
			s.serveWorldLogo(w, r, req, found)
			return
		}
		s.serveUnknownLogo(w, r, req, searched || len(index.all) > 0)
		return
	}
	w.Header().Set("X-Logo-Set", "india")
	if !c.HasLogo {
		// A listed company the source has no logo for: always the tile, so its logo URL never breaks.
		s.serveMonogram(w, r, c.Ticker, req.format, req.size, "fallback", cacheShort)
		return
	}

	logos := s.need(w, r, "logos")
	if logos == nil {
		return
	}
	s.metrics.observeLogo(req.format, "found")
	if req.format == "svg" {
		var svg []byte
		if err := logos.row(`SELECT svg FROM logos WHERE id = ?`, c.ID).Scan(&svg); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not read the logo.")
			return
		}
		w.Header().Set("Content-Security-Policy", svgPolicy)
		writeBody(w, r, http.StatusOK, svg, typeSVG, cacheLong, false)
		return
	}
	s.servePNGMaster(w, r, logos, c.ID, `SELECT png FROM logos WHERE id = ?`, req.size)
}

// servePNGMaster answers the stored PNG master of id, scaled to size and kept in the cache.
func (s *Server) servePNGMaster(w http.ResponseWriter, r *http.Request, data *handle, id, query string, size int) {
	key := data.fingerprint + "|" + id + "|" + strconv.Itoa(size)
	if _, hit := s.png.peek(key); hit {
		info(r).cached = true
	}
	image, err := s.png.get(key, func() ([]byte, error) {
		var master []byte
		if err := data.row(query, id).Scan(&master); err != nil {
			return nil, err
		}
		if master == nil {
			return nil, errNoPNG
		}
		var resized []byte
		var err error
		s.extras.render.work(func() { resized, err = resizePNG(master, size) })
		return resized, err
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

// serveMonogram answers with the initials tile for symbol, marked with X-Logo-Fallback.
// result is the metric outcome; cache the Cache-Control the tile is sent with.
func (s *Server) serveMonogram(w http.ResponseWriter, r *http.Request, symbol, format string, size int, result, cache string) {
	s.metrics.observeLogo(format, result)
	letters, background := monogram(symbol)
	w.Header().Set("X-Logo-Fallback", "monogram")
	if format == "svg" {
		w.Header().Set("Content-Security-Policy", svgPolicy)
		writeBody(w, r, http.StatusOK, monogramSVG(letters, background), typeSVG, cache, false)
		return
	}
	key := "monogram:" + letters + "|" + strconv.Itoa(size)
	if _, hit := s.png.peek(key); hit {
		info(r).cached = true
	}
	image, err := s.png.get(key, func() ([]byte, error) {
		return monogramPNG(letters, background, size)
	})
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not draw the placeholder.")
		return
	}
	writeBody(w, r, http.StatusOK, image, typePNG, cache, false)
}

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

// peek returns a cached entry without making one.
func (c *pngCache) peek(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		c.order.MoveToFront(element)
		return element.Value.(*pngEntry).data, true
	}
	return nil, false
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
