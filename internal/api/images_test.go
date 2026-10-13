package api

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/typeset"
)

// svgSize checks that an SVG is well-formed XML, which is what makes a renderer refuse
// it, and returns the size it declares.
func svgSize(t *testing.T, svg []byte) (width, height int) {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	for {
		if _, err := decoder.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("SVG is not well-formed: %v\n%s", err, clip(string(svg), 400))
		}
	}
	var root struct {
		Width  int `xml:"width,attr"`
		Height int `xml:"height,attr"`
	}
	if err := xml.Unmarshal(svg, &root); err != nil {
		t.Fatal(err)
	}
	return root.Width, root.Height
}

// needRenderer skips a test of PNG output where resvg is not installed. CI sets RESVG,
// so the PNG path is tested there.
func needRenderer(t *testing.T) {
	t.Helper()
	if !testServer.extras.render.available() {
		t.Skip("no resvg here; set RESVG (see tools/fetch-resvg.sh)")
	}
}

func TestOGImageSVG(t *testing.T) {
	got := expectStatus(t, "/v1/og-image?format=svg&title="+url.QueryEscape("Free APIs for <Indian> developers")+"&subtitle=Pincodes+%26+IFSC&site=api.dharun.dev&tag=docs", http.StatusOK)
	if contentType := got.header.Get("Content-Type"); contentType != typeSVG {
		t.Errorf("content type %q", contentType)
	}
	if got.header.Get("Content-Security-Policy") == "" {
		t.Error("an SVG answer carries a content security policy")
	}
	if width, height := svgSize(t, got.body); width != 1200 || height != 630 {
		t.Errorf("size %dx%d", width, height)
	}
	svg := string(got.body)
	// The words are drawn as outlines, so the picture needs no fonts where it is shown;
	// the title is there as text for screen readers.
	if strings.Contains(svg, "<text") || strings.Contains(svg, "font-family") {
		t.Error("the SVG relies on fonts")
	}
	if !strings.Contains(svg, "<title>Free APIs for &lt;Indian&gt; developers</title>") {
		t.Error("the SVG lacks its title")
	}
	if glyphs := strings.Count(svg, "<use "); glyphs < 50 {
		t.Errorf("only %d glyphs drawn", glyphs)
	}
	// Each distinct glyph is defined once however often it is used.
	if outlines := strings.Count(svg, "<path id="); outlines < 20 || outlines > 60 {
		t.Errorf("%d outlines defined", outlines)
	}
	if !strings.Contains(got.header.Get("Cache-Control"), "public") {
		t.Errorf("a preview image can be cached: %q", got.header.Get("Cache-Control"))
	}
}

func TestOGImageLayout(t *testing.T) {
	lay := func(title, subtitle string) ogLayout {
		return ogImage{Title: title, Subtitle: subtitle}.lay(newCanvas())
	}
	canvas := newCanvas()
	fits := func(l ogLayout) {
		t.Helper()
		for _, line := range l.title {
			if canvas.Width(line, typeset.Style{Bold: true})*l.size/typeset.Em > ogUsable {
				t.Errorf("title line %q is wider than the card", line)
			}
		}
		for _, line := range l.subtitle {
			if canvas.Width(line, typeset.Style{})*ogSubtitleSize/typeset.Em > ogUsable {
				t.Errorf("subtitle line %q is wider than the card", line)
			}
		}
		if len(l.title) > 3 || len(l.subtitle) > 2 || l.height > ogBlockHeight {
			t.Errorf("does not fit: %d title lines, %d subtitle lines, %.0f high", len(l.title), len(l.subtitle), l.height)
		}
	}

	short := lay("Hello", "")
	if short.size != 84 || len(short.title) != 1 {
		t.Errorf("a short title is set large on one line: %+v", short)
	}
	fits(short)

	long := lay(strings.TrimSpace(strings.Repeat("A fairly long title that keeps going ", 5)), strings.Repeat("and a long subtitle ", 15))
	if long.size != 48 || len(long.title) != 3 || !strings.HasSuffix(long.title[2], "…") {
		t.Errorf("a long title is set small and cut: %d %q", long.size, long.title)
	}
	if len(long.subtitle) != 2 || !strings.HasSuffix(long.subtitle[1], "…") {
		t.Errorf("a long subtitle is cut at two lines: %q", long.subtitle)
	}
	fits(long)

	// Indian scripts get more room between lines, and still fit.
	tamil := lay("இந்திய டெவலப்பர்களுக்கான இலவச API", "नमस्ते भारत · తెలుగు · ಕನ್ನಡ · മലയാളം · বাংলা · ગુજરાતી · ਪੰਜਾਬੀ · ଓଡ଼ିଆ")
	if tamil.titleLeading <= short.titleLeading {
		t.Error("a Tamil title has the line height of a Latin one")
	}
	fits(tamil)

	// One word wider than the card is cut, not left to run off the edge.
	word := lay(strings.Repeat("W", 120), "")
	if len(word.title) < 2 {
		t.Errorf("an unbroken word is wrapped: %q", word.title)
	}
	fits(word)
}

func TestOGImageDrawsEveryScript(t *testing.T) {
	for _, title := range []string{"नमस्ते भारत", "বাংলা", "ਪੰਜਾਬੀ", "ગુજરાતી", "ଓଡ଼ିଆ", "தமிழ்", "తెలుగు", "ಕನ್ನಡ", "മലയാളം"} {
		got := expectStatus(t, "/v1/og-image?format=svg&title="+url.QueryEscape(title), http.StatusOK)
		svgSize(t, got.body)
		if strings.Count(string(got.body), "<use ") < 2 {
			t.Errorf("%s: nothing drawn", title)
		}
		// Glyph 0 of a font is the empty box drawn for a missing character.
		for font := 0; font < 22; font++ {
			if strings.Contains(string(got.body), fmt.Sprintf(`id="g%x"`, font<<16)) {
				t.Errorf("%s: a character is missing from the fonts", title)
			}
		}
	}
}

func TestOGImageColours(t *testing.T) {
	got := expectStatus(t, "/v1/og-image?format=svg&title=Hi&theme=light&bg=102030&accent=ff0000", http.StatusOK)
	for _, want := range []string{"#102030", "#ff0000"} {
		if !strings.Contains(string(got.body), want) {
			t.Errorf("SVG lacks colour %s", want)
		}
	}
	for _, bad := range []string{
		"/v1/og-image", "/v1/og-image?title=+", "/v1/og-image?title=Hi&theme=neon", "/v1/og-image?title=Hi&bg=nope",
		"/v1/og-image?title=Hi&format=gif", "/v1/og-image?title=" + strings.Repeat("a", 201), "/v1/og-image?title=Hi&tag=" + strings.Repeat("a", 41),
	} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
	// Control characters are dropped rather than breaking the picture.
	svgSize(t, expectStatus(t, "/v1/og-image?format=svg&title="+url.QueryEscape("a\x00b\x1bc\nd"), http.StatusOK).body)
}

func TestCodeImageSVG(t *testing.T) {
	code := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tif 1 < 2 {\n\t\tfmt.Println(\"வணக்கம்\")\n\t}\n}\n"
	got := expectStatus(t, "/v1/code-image?format=svg&language=go&title=main.go&lineNumbers=true&code="+url.QueryEscape(code), http.StatusOK)
	_, height := svgSize(t, got.body)
	svg := string(got.body)
	if language := got.header.Get("X-Code-Language"); language != "Go" {
		t.Errorf("language %q", language)
	}
	// Nine lines: the trailing line break does not make a tenth.
	if want := codeHeader + 9*codeLineHeight + 2*codePadding - 8; height != want {
		t.Errorf("height %d, want %d", height, want)
	}
	if !strings.Contains(svg, "<title>main.go</title>") {
		t.Error("the SVG lacks its title")
	}
	// Keywords, strings and names get different colours.
	colours := map[string]bool{}
	for _, part := range strings.Split(svg, `<g fill="`)[1:] {
		colours[part[:7]] = true
	}
	if len(colours) < 4 {
		t.Errorf("only %d colours: %v", len(colours), colours)
	}
	if cache := got.header.Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Errorf("code is not cached: %q", cache)
	}

	plain := expectStatus(t, "/v1/code-image?format=svg&language=go&theme=light&code="+url.QueryEscape(code), http.StatusOK)
	if !strings.Contains(string(plain.body), `rx="12" fill="#f7f7f7"`) {
		t.Error("light theme has a light frame")
	}
	// Without line numbers the picture is narrower by the gutter and draws fewer glyphs.
	numbered, _ := svgSize(t, got.body)
	bare, _ := svgSize(t, plain.body)
	if bare >= numbered || strings.Count(string(plain.body), "<use ") >= strings.Count(svg, "<use ") {
		t.Errorf("line numbers are off unless asked for: widths %d and %d", bare, numbered)
	}
}

func TestCodeImageLayout(t *testing.T) {
	// Code falls on the grid of the monospaced font: the picture is as wide as its
	// longest line.
	wide := codeImage{Code: strings.Repeat("x", 100) + "\nshort", Theme: "dark"}.draw()
	if want := int(100*codeCharWidth) + 2*codePadding; wide.width != want || wide.lines != 2 {
		t.Errorf("width %d, want %d; %d lines", wide.width, want, wide.lines)
	}
	// A short snippet still gets a window of a sensible width.
	small := codeImage{Code: "x", Theme: "dark"}.draw()
	if small.width != int(codeMinColumns*codeCharWidth)+2*codePadding || small.zoom() != 2 {
		t.Errorf("small: %dx%d at zoom %d", small.width, small.height, small.zoom())
	}
	// Text in another script takes the room it needs, and the picture grows with it.
	tamil := codeImage{Code: `s := "` + strings.Repeat("வணக்கம் ", 12) + `"`, Language: "go", Theme: "dark"}.draw()
	latin := codeImage{Code: `s := "` + strings.Repeat("abcdefg ", 12) + `"`, Language: "go", Theme: "dark"}.draw()
	if tamil.width <= latin.width {
		t.Errorf("Tamil text is drawn at its own width: %d and %d", tamil.width, latin.width)
	}
	// The largest picture allowed is rendered at single size, to bound the renderer's memory.
	big := codeImage{Code: strings.TrimSpace(strings.Repeat(strings.Repeat("x", codeMaxColumns)+"\n", codeMaxLines)), Theme: "dark"}.draw()
	if big.zoom() != 1 || !big.large() || small.large() || big.lines != codeMaxLines {
		t.Errorf("big: %dx%d at zoom %d, %d lines", big.width, big.height, big.zoom(), big.lines)
	}
	if len(big.svg) > 1<<20 {
		t.Errorf("the largest SVG is %d KB", len(big.svg)>>10)
	}
}

func TestCodeImageGuessesTheLanguage(t *testing.T) {
	byTitle := expectStatus(t, "/v1/code-image?format=svg&title=query.sql&code="+url.QueryEscape("select 1"), http.StatusOK)
	if language := byTitle.header.Get("X-Code-Language"); !strings.Contains(language, "SQL") {
		t.Errorf("from the title: %q", language)
	}
	plain := expectStatus(t, "/v1/code-image?format=svg&code="+url.QueryEscape("just some words"), http.StatusOK)
	svgSize(t, plain.body)
	if strings.Count(string(plain.body), "<use ") != len("justsomewords") {
		t.Error("plain text is drawn as it is")
	}
}

func TestCodeImagePost(t *testing.T) {
	text := request(t, http.MethodPost, "/v1/code-image?format=svg&language=python", "print('hi')\n", "Content-Type", "text/plain")
	if text.status != http.StatusOK || text.header.Get("X-Code-Language") != "Python" {
		t.Fatalf("text/plain: %d %q %s", text.status, text.header.Get("X-Code-Language"), clip(string(text.body), 200))
	}
	svgSize(t, text.body)

	body := `{"text": "const a = 1;", "language": "js", "title": "a.js", "format": "svg", "lineNumbers": true}`
	posted := request(t, http.MethodPost, "/v1/code-image", body, "Content-Type", "application/json")
	if posted.status != http.StatusOK || posted.header.Get("X-Code-Language") != "JavaScript" {
		t.Fatalf("JSON: %d %q %s", posted.status, posted.header.Get("X-Code-Language"), clip(string(posted.body), 200))
	}
	if !strings.Contains(string(posted.body), "<title>a.js</title>") {
		t.Error("options in the JSON body are used")
	}
}

func TestCodeImageRefusals(t *testing.T) {
	for _, bad := range []string{
		"/v1/code-image", "/v1/code-image?code=+",
		"/v1/code-image?code=x&language=klingon", "/v1/code-image?code=x&theme=neon", "/v1/code-image?code=x&format=gif",
		"/v1/code-image?code=" + strings.Repeat("x", codeMaxColumns+1),
		"/v1/code-image?code=" + url.QueryEscape(strings.Repeat("x\n", codeMaxLines+1)),
		"/v1/code-image?code=x&title=" + strings.Repeat("t", 81),
	} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
	// As many lines as allowed, with a line break after the last, is fine.
	expectStatus(t, "/v1/code-image?format=svg&code="+url.QueryEscape(strings.Repeat("x\n", codeMaxLines)), http.StatusOK)
	// Characters XML cannot carry are dropped, not passed to the renderer.
	got := expectStatus(t, "/v1/code-image?format=svg&title=a%00b&code="+url.QueryEscape("a\x00b\x1bc"), http.StatusOK)
	svgSize(t, got.body)
}

func TestImagesWithoutARenderer(t *testing.T) {
	server := newTestServer(Config{Resvg: "/nowhere/resvg"})
	defer server.Close()
	for _, path := range []string{"/v1/og-image?title=Hi", "/v1/code-image?code=x"} {
		recorder := serve(server, http.MethodGet, path)
		if recorder.Code != http.StatusNotImplemented || !strings.Contains(recorder.Body.String(), "png_unavailable") {
			t.Errorf("%s = %d %s", path, recorder.Code, clip(recorder.Body.String(), 200))
		}
		if recorder := serve(server, http.MethodGet, path+"&format=svg"); recorder.Code != http.StatusOK {
			t.Errorf("%s as SVG = %d", path, recorder.Code)
		}
	}
}

func TestOGImagePNG(t *testing.T) {
	needRenderer(t)
	got := expectStatus(t, "/v1/og-image?title="+url.QueryEscape("இந்திய டெவலப்பர்களுக்கான இலவச API")+"&subtitle="+url.QueryEscape("नमस्ते भारत")+"&site=api.dharun.dev", http.StatusOK)
	if contentType := got.header.Get("Content-Type"); contentType != typePNG {
		t.Fatalf("content type %q", contentType)
	}
	picture, err := png.Decode(bytes.NewReader(got.body))
	if err != nil {
		t.Fatal(err)
	}
	if size := picture.Bounds().Size(); size.X != ogWidth || size.Y != ogHeight {
		t.Errorf("size %v", size)
	}
	// Text was drawn in the title's colour: a good share of the pixels in the middle of
	// the card are light on the dark background.
	light := 0
	for x := ogMargin; x < ogWidth-ogMargin; x += 3 {
		for y := 150; y < 450; y += 3 {
			if r, g, b, _ := picture.At(x, y).RGBA(); r>>8 > 0xc0 && g>>8 > 0xc0 && b>>8 > 0xc0 {
				light++
			}
		}
	}
	if light < 1000 {
		t.Errorf("only %d light pixels: the text is missing", light)
	}
	if got.header.Get("ETag") == "" {
		t.Error("no ETag")
	}
}

func TestCodeImagePNG(t *testing.T) {
	needRenderer(t)
	code := "func main() {\n\tfmt.Println(\"hi\")\n}"
	vector := expectStatus(t, "/v1/code-image?format=svg&language=go&code="+url.QueryEscape(code), http.StatusOK)
	width, height := svgSize(t, vector.body)
	got := expectStatus(t, "/v1/code-image?language=go&code="+url.QueryEscape(code), http.StatusOK)
	picture, err := png.Decode(bytes.NewReader(got.body))
	if err != nil {
		t.Fatal(err)
	}
	if size := picture.Bounds().Size(); size.X != 2*width || size.Y != 2*height {
		t.Errorf("PNG is %v, want twice %dx%d", size, width, height)
	}
}

// A large picture waits for every slot and holds them all; pictures queue behind it.
func TestRenderSlots(t *testing.T) {
	r := newRenderer(Config{RenderConcurrency: 2})
	one, err := r.take(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	waiting, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.take(waiting, 2); err == nil {
		t.Fatal("a large picture started while another was being rendered")
	}
	if len(r.slots) != 1 {
		t.Fatalf("a wait that gave up left %d slots taken", len(r.slots))
	}
	one()
	all, err := r.take(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	blocked, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.take(blocked, 1); err == nil {
		t.Fatal("a picture started beside a large one")
	}
	all()
	if len(r.slots) != 0 || len(r.turn) != 0 {
		t.Fatalf("slots left taken: %d, turn %d", len(r.slots), len(r.turn))
	}
}
