package api

import (
	"errors"
	"fmt"
	"image/color"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/typeset"
)

// Social preview ("Open Graph") images: the 1200x630 card that link previews show.
// Text and colours only; no images are fetched from anywhere.

const (
	ogWidth, ogHeight = 1200, 630
	ogMargin          = 80
	ogUsable          = ogWidth - 2*ogMargin
	ogSubtitleSize    = 34
	ogFooterSize      = 28
	// The title and subtitle share the space between the accent bar and the footer.
	ogBlockTop    = ogMargin + 10
	ogBlockHeight = ogHeight - 2*ogMargin - 10 - 56
)

type ogTheme struct{ background, text, muted, accent color.RGBA }

var ogThemes = map[string]ogTheme{
	"dark":  {color.RGBA{0x17, 0x15, 0x12, 0xff}, color.RGBA{0xf5, 0xf0, 0xe6, 0xff}, color.RGBA{0xa8, 0xa2, 0x96, 0xff}, color.RGBA{0xc8, 0xff, 0x3d, 0xff}},
	"light": {color.RGBA{0xf3, 0xef, 0xe7, 0xff}, color.RGBA{0x17, 0x15, 0x12, 0xff}, color.RGBA{0x5c, 0x55, 0x4a, 0xff}, color.RGBA{0x22, 0x48, 0xff, 0xff}},
}

type ogImage struct {
	Title, Subtitle, Site, Tag string
	Theme                      ogTheme
}

// ogLayout is where the words of a card go.
type ogLayout struct {
	size                          int // of the title, in pixels
	title, subtitle               []string
	titleLeading, subtitleLeading float64 // line height, in ems
	height                        float64 // of title and subtitle together
}

// leading is the line height for a text: Indian scripts stack marks above and below the
// letters and need more room between lines than Latin does.
func leading(d *canvas, text string, style typeset.Style, latin, indic float64) float64 {
	if d.Shape(text, style).Indic {
		return indic
	}
	return latin
}

const ogGap = 18 // between title and subtitle

// lay picks the largest title size at which the title takes at most three lines and the
// whole block fits; a title too long even at the smallest size is cut with an ellipsis.
func (o ogImage) lay(d *canvas) ogLayout {
	bold, plain := typeset.Style{Bold: true}, typeset.Style{}
	l := ogLayout{
		titleLeading:    leading(d, o.Title, bold, 1.16, 1.36),
		subtitleLeading: leading(d, o.Subtitle, plain, 1.35, 1.5),
	}
	subtitleMax := ogUsable * typeset.Em / ogSubtitleSize
	l.subtitle = d.Clamp(d.Wrap(o.Subtitle, plain, subtitleMax), 2, plain, subtitleMax)
	below := 0.0
	if len(l.subtitle) > 0 {
		below = ogGap + float64(len(l.subtitle))*ogSubtitleSize*l.subtitleLeading
	}
	sizes := []int{84, 74, 64, 56, 48}
	for i, size := range sizes {
		room := ogUsable * typeset.Em / size
		l.size = size
		l.title = d.Wrap(o.Title, bold, room)
		if i == len(sizes)-1 {
			l.title = d.Clamp(l.title, 3, bold, room)
		}
		l.height = float64(len(l.title))*float64(size)*l.titleLeading + below
		if len(l.title) <= 3 && l.height <= ogBlockHeight {
			break
		}
	}
	return l
}

// svg draws the card: an accent bar, the title, the subtitle under it, and the site name
// and tag along the bottom.
func (o ogImage) svg() []byte {
	d := newCanvas()
	bold, plain := typeset.Style{Bold: true}, typeset.Style{}
	l := o.lay(d)

	d.printf(`<rect width="%d" height="%d" fill="%s"/>`, ogWidth, ogHeight, cssColor(o.Theme.background))
	d.printf(`<rect x="%d" y="%d" width="96" height="10" fill="%s"/>`, ogMargin, ogMargin, cssColor(o.Theme.accent))

	// Within its line, a text's baseline sits below half the extra leading and the ascent.
	baseline := func(size, leading float64) float64 { return size * ((leading-1)/2 + 0.8) }
	y := ogBlockTop + (ogBlockHeight-l.height)/2
	for _, line := range l.title {
		d.text(d.Shape(line, bold), ogMargin, y+baseline(float64(l.size), l.titleLeading), float64(l.size), cssColor(o.Theme.text), false)
		y += float64(l.size) * l.titleLeading
	}
	y += ogGap
	for _, line := range l.subtitle {
		d.text(d.Shape(line, plain), ogMargin, y+baseline(ogSubtitleSize, l.subtitleLeading), ogSubtitleSize, cssColor(o.Theme.muted), false)
		y += ogSubtitleSize * l.subtitleLeading
	}

	// The footer: the site keeps to the left three fifths, the tag to the right third.
	footer := float64(ogHeight - ogMargin + 6)
	toEms := func(pixels float64) int { return int(pixels * typeset.Em / ogFooterSize) }
	if o.Site != "" {
		d.text(d.Shape(d.Fit(o.Site, bold, toEms(ogUsable*0.6)), bold), ogMargin, footer, ogFooterSize, cssColor(o.Theme.text), false)
	}
	if o.Tag != "" {
		tag := d.Shape(d.Fit(o.Tag, plain, toEms(ogUsable*0.33)), plain)
		d.text(tag, ogWidth-ogMargin-float64(tag.Width)*ogFooterSize/typeset.Em, footer, ogFooterSize, cssColor(o.Theme.muted), false)
	}
	return d.svg(ogWidth, ogHeight, o.Title)
}

// imageFormat reads format= for a generated image: png by default.
func imageFormat(raw string) (string, error) {
	switch format := strings.ToLower(strings.TrimSpace(raw)); format {
	case "", "png":
		return "png", nil
	case "svg":
		return "svg", nil
	}
	return "", errors.New("format must be png or svg")
}

// sendImage answers with the SVG, or its PNG rendering. cache is the Cache-Control value.
func (s *Server) sendImage(w http.ResponseWriter, r *http.Request, svg []byte, format string, zoom int, large bool, cache string) {
	if format == "svg" {
		w.Header().Set("Content-Security-Policy", svgPolicy)
		writeBody(w, r, http.StatusOK, svg, typeSVG, cache, true)
		return
	}
	image, err := s.extras.render.png(r.Context(), svg, zoom, large)
	switch {
	case errors.Is(err, errNoRenderer):
		s.fail(w, r, http.StatusNotImplemented, "png_unavailable", "This server has no image renderer. Use format=svg.")
	case err != nil:
		s.fail(w, r, http.StatusInternalServerError, "render_failed", "The image could not be rendered.")
	default:
		writeBody(w, r, http.StatusOK, image, typePNG, cache, false)
	}
}

func (s *Server) handleOGImage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	field := func(name string) string { return strings.Join(strings.Fields(drawable(query.Get(name))), " ") }
	card := ogImage{Title: field("title"), Subtitle: field("subtitle"), Site: field("site"), Tag: field("tag")}
	info(r).subject = card.Title
	limits := []struct {
		name, value string
		max         int
	}{{"title", card.Title, 200}, {"subtitle", card.Subtitle, 300}, {"site", card.Site, 60}, {"tag", card.Tag, 40}}
	if card.Title == "" {
		s.badRequest(w, r, "title is required.")
		return
	}
	for _, limit := range limits {
		if utf8.RuneCountInString(limit.value) > limit.max {
			s.badRequest(w, r, fmt.Sprintf("%s can be at most %d characters.", limit.name, limit.max))
			return
		}
	}
	themeName := strings.ToLower(strings.TrimSpace(query.Get("theme")))
	if themeName == "" {
		themeName = "dark"
	}
	theme, ok := ogThemes[themeName]
	if !ok {
		s.badRequest(w, r, "theme must be dark or light.")
		return
	}
	var err error
	for _, override := range []struct {
		name   string
		target *color.RGBA
	}{{"bg", &theme.background}, {"fg", &theme.text}, {"accent", &theme.accent}} {
		if *override.target, err = parseColor(query.Get(override.name), *override.target); err != nil || override.target.A == 0 {
			s.badRequest(w, r, override.name+" must be a hex colour, such as 1f6feb.")
			return
		}
	}
	card.Theme = theme
	format, err := imageFormat(query.Get("format"))
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	s.sendImage(w, r, card.svg(), format, 1, false, cacheLong)
}
