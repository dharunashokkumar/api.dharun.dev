package api

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func init() {
	mcpTestCalls["domain_icon"] = `{"domain":"icons.site.in","size":32}`
}

func solidPNG(size int, c color.Color) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, c)
		}
	}
	var out bytes.Buffer
	png.Encode(&out, img)
	return out.Bytes()
}

// icoWithBitmap builds an icon file holding one 32-bit bitmap of a single colour,
// whose top-left pixel is transparent.
func icoWithBitmap(size int, c color.NRGBA) []byte {
	var dib bytes.Buffer
	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header[0:], 40)
	binary.LittleEndian.PutUint32(header[4:], uint32(size))
	binary.LittleEndian.PutUint32(header[8:], uint32(2*size))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 32)
	dib.Write(header)
	for y := size - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			alpha := c.A
			if x == 0 && y == 0 {
				alpha = 0
			}
			dib.Write([]byte{c.B, c.G, c.R, alpha})
		}
	}
	dib.Write(make([]byte, ((size+31)/32)*4*size)) // the mask, unused with alpha
	return icoFile(size, 32, dib.Bytes())
}

func icoFile(size, depth int, body []byte) []byte {
	var out bytes.Buffer
	out.Write([]byte{0, 0, 1, 0, 1, 0})
	entry := make([]byte, 16)
	entry[0], entry[1] = byte(size), byte(size)
	binary.LittleEndian.PutUint16(entry[4:], 1)
	binary.LittleEndian.PutUint16(entry[6:], uint16(depth))
	binary.LittleEndian.PutUint32(entry[8:], uint32(len(body)))
	binary.LittleEndian.PutUint32(entry[12:], 22)
	out.Write(entry)
	out.Write(body)
	return out.Bytes()
}

func TestDecodeICO(t *testing.T) {
	green := color.NRGBA{G: 200, A: 255}
	img, err := decodeICO(icoWithBitmap(16, green))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 16 || color.NRGBAModel.Convert(img.At(5, 5)) != green || color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA).A != 0 {
		t.Errorf("bitmap icon: %v, %v at 5,5, %v at 0,0", img.Bounds(), img.At(5, 5), img.At(0, 0))
	}
	// Modern icons hold a PNG.
	img, err = decodeICO(icoFile(0, 32, solidPNG(256, color.NRGBA{B: 255, A: 255})))
	if err != nil || img.Bounds().Dx() != 256 {
		t.Errorf("PNG icon: %v %v", img, err)
	}
	for _, broken := range [][]byte{nil, []byte("not an icon"), icoFile(16, 32, []byte("short")), icoFile(16, 32, make([]byte, 40))} {
		if _, err := decodeICO(broken); err == nil {
			t.Errorf("a broken icon decoded: %q", broken)
		}
	}
}

func TestDomainIcon(t *testing.T) {
	sites := useTestSites(t)
	red, blue := color.NRGBA{R: 220, A: 255}, color.NRGBA{B: 220, A: 255}
	sites.serve("icons.site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<!doctype html><html><head><link rel="icon" href="/tiny.png" sizes="16x16"><link rel="icon" type="image/svg+xml" href="/icon.svg">` +
				`<link rel="apple-touch-icon" href="/touch.png"><link rel="mask-icon" href="/mask.svg"></head><body><link rel="icon" href="/late.png"></body></html>`))
		case "/touch.png":
			w.Write(solidPNG(180, red))
		case "/tiny.png":
			w.Write(solidPNG(16, blue))
		default:
			http.NotFound(w, r)
		}
	}))
	sites.serve("manifest.site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><head><link rel="manifest" href="/app.webmanifest"></head></html>`))
		case "/app.webmanifest":
			w.Write([]byte(`{"icons":[{"src":"/mono.png","sizes":"512x512","purpose":"monochrome"},{"src":"icons/512.png","sizes":"512x512","type":"image/png"}]}`))
		case "/icons/512.png":
			w.Write(solidPNG(512, blue))
		default:
			http.NotFound(w, r)
		}
	}))
	sites.serve("plain.site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.Write(icoWithBitmap(32, color.NRGBA{G: 180, A: 255}))
			return
		}
		w.Write([]byte("<html><head><title>no links</title></head></html>"))
	}))
	// A home page longer than iconPageLimit (GitHub's is 570 KB): the head is still read.
	sites.serve("long.site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apple.png" {
			w.Write(solidPNG(180, blue))
			return
		}
		w.Write([]byte(`<html><head><link rel="apple-touch-icon" href="/apple.png"></head><body>`))
		w.Write(bytes.Repeat([]byte("<p>text</p>"), iconPageLimit/5))
	}))
	// A home page that refuses the check, with a favicon all the same.
	sites.serve("closed.site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.Write(icoWithBitmap(32, color.NRGBA{G: 180, A: 255}))
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	var asked atomic.Int32
	sites.serve("noicon.site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		if r.URL.Path == "/" {
			w.Write([]byte("<html></html>"))
			return
		}
		http.NotFound(w, r)
	}))

	picture := func(path string) (image.Image, result) {
		t.Helper()
		got := expectStatus(t, path, http.StatusOK)
		if got.header.Get("Content-Type") != typePNG {
			t.Fatalf("%s: %s", path, got.header.Get("Content-Type"))
		}
		img, err := png.Decode(bytes.NewReader(got.body))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return img, got
	}

	img, got := picture("/v1/icon/icons.site.in")
	if img.Bounds().Dx() != iconDefaultSize || color.NRGBAModel.Convert(img.At(32, 32)) != red || !strings.HasSuffix(got.header.Get("X-Icon-Source"), "/touch.png") ||
		got.header.Get("X-Logo-Fallback") != "" || !strings.Contains(got.header.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("icon from the page: %v, %v, %v", img.Bounds(), img.At(32, 32), got.header)
	}
	// Another size comes from the picture already fetched.
	if img, got := picture("/v1/icon/icons.site.in.png?size=16"); img.Bounds().Dx() != 16 || got.header.Get("X-Icon-Source") != "" {
		t.Errorf("second size: %v %v", img.Bounds(), got.header)
	}
	if img, got := picture("/v1/icon/manifest.site.in?size=256"); img.Bounds().Dx() != 256 || color.NRGBAModel.Convert(img.At(100, 100)) != blue ||
		!strings.HasSuffix(got.header.Get("X-Icon-Source"), "/icons/512.png") {
		t.Errorf("icon from the manifest: %v %v", img.At(100, 100), got.header.Get("X-Icon-Source"))
	}
	if img, got := picture("/v1/icon/plain.site.in?size=32"); color.NRGBAModel.Convert(img.At(16, 16)).(color.NRGBA).G != 180 || !strings.HasSuffix(got.header.Get("X-Icon-Source"), "/favicon.ico") {
		t.Errorf("favicon.ico: %v %v", img.At(16, 16), got.header.Get("X-Icon-Source"))
	}
	if img, got := picture("/v1/icon/long.site.in"); color.NRGBAModel.Convert(img.At(32, 32)) != blue || !strings.HasSuffix(got.header.Get("X-Icon-Source"), "/apple.png") {
		t.Errorf("long page: %v %v", img.At(32, 32), got.header)
	}
	if img, got := picture("/v1/icon/closed.site.in"); color.NRGBAModel.Convert(img.At(32, 32)).(color.NRGBA).G != 180 || !strings.HasSuffix(got.header.Get("X-Icon-Source"), "/favicon.ico") {
		t.Errorf("refused home page: %v %v", img.At(32, 32), got.header)
	}

	// No icon: the initials tile, and the site is not asked again.
	_, got = picture("/v1/icon/noicon.site.in")
	before := asked.Load()
	picture("/v1/icon/noicon.site.in")
	if got.header.Get("X-Logo-Fallback") != "monogram" || asked.Load() != before {
		t.Errorf("no icon: %v, asked %d then %d", got.header, before, asked.Load())
	}
	// A private address is never fetched; the tile stands in.
	if _, got := picture("/v1/icon/inside.site.in"); got.header.Get("X-Logo-Fallback") != "monogram" || got.header.Get("X-Icon-Error") == "" {
		t.Errorf("private address: %v", got.header)
	}
	for _, bad := range []string{"/v1/icon/1.1.1.1", "/v1/icon/site.in?size=300", "/v1/icon/site.in?format=svg", "/v1/icon/not%20a%20name"} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
}

func TestIconLinks(t *testing.T) {
	base := mustParseURL(t, "https://site.in/blog/")
	links := iconLinks([]byte(`<link rel="shortcut icon" href="fav.png" sizes="32x32 64x64"><link rel=icon href="javascript:alert(1)"><link rel="apple-touch-icon-precomposed" href="//cdn.site.in/t.png">`), base)
	if len(links) != 2 || links[0].URL != "https://site.in/blog/fav.png" || links[0].Size != 64 || links[1].URL != "https://cdn.site.in/t.png" || links[1].Size != 180 {
		t.Errorf("links: %+v", links)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
