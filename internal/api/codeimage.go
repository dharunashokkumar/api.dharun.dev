package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/dharunashokkumar/api.dharun.dev/internal/typeset"
)

// Code images: a snippet drawn as a tidy window with syntax colouring, for slides, posts
// and READMEs. Highlighting is by chroma; the picture is SVG, rendered to PNG by resvg.

const (
	codeMaxLines   = 150
	codeMaxColumns = 140
	codeFontSize   = 16
	codeLineHeight = 24
	codeCharWidth  = 9.6 // Noto Sans Mono is 0.6 em wide
	codeMinColumns = 40
	codePadding    = 28
	codeHeader     = 46
	codeTitleSize  = 14
	// PNGs are drawn at twice the size for sharp text, unless that would be a very large
	// picture: the renderer needs several bytes for every pixel, and for every glyph.
	codeMaxPixels   = 8 << 20
	codeLargePixels = 3 << 20
	codeLargeSVG    = 200 << 10
)

var codeThemes = map[string]string{"dark": "github-dark", "light": "github"}

type codeImage struct {
	Code, Language, Title, Theme string
	LineNumbers                  bool
}

// codePicture is a drawn snippet.
type codePicture struct {
	svg           []byte
	language      string
	width, height int
	lines         int
}

// zoom is the scale a PNG of the picture is rendered at.
func (p codePicture) zoom() int {
	if p.width*p.height*4 > codeMaxPixels {
		return 1
	}
	return 2
}

// large reports a picture that is rendered on its own (see renderer.png).
func (p codePicture) large() bool {
	return p.width*p.height*p.zoom()*p.zoom() > codeLargePixels || len(p.svg) > codeLargeSVG
}

// draw highlights the snippet and draws it.
func (c codeImage) draw() codePicture {
	source := strings.ReplaceAll(strings.ReplaceAll(drawable(c.Code), "\r\n", "\n"), "\t", "    ")
	source = strings.TrimRight(source, "\n ")

	lexer := lexers.Get(c.Language)
	if lexer == nil && c.Title != "" {
		lexer = lexers.Match(c.Title)
	}
	if lexer == nil {
		lexer = lexers.Analyse(source)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	style := styles.Get(codeThemes[c.Theme])
	background := style.Get(chroma.Background)
	frame, muted := "#0d1117", "#8b949e"
	if c.Theme == "light" {
		frame, muted = "#ffffff", "#6e7781"
	}
	if background.Background.IsSet() {
		frame = background.Background.String()
	}

	// One row per source line, each a list of coloured pieces already shaped. Code in
	// Noto Sans Mono falls on a grid; a piece in another script takes the room it needs
	// and what follows moves along.
	d := newCanvas()
	type piece struct {
		line   typeset.Line
		at     int // where it starts, in thousandths of an em
		colour string
		italic bool
	}
	rows := [][]piece{nil}
	pen, widest := 0, 0
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		iterator = chroma.Literator(chroma.Token{Type: chroma.Text, Value: source})
	}
	for token := iterator(); token != chroma.EOF; token = iterator() {
		entry := style.Get(token.Type)
		colour := entry.Colour
		if !colour.IsSet() {
			colour = background.Colour
		}
		for i, part := range strings.Split(token.Value, "\n") {
			if i > 0 {
				rows = append(rows, nil)
				pen = 0
			}
			if part == "" {
				continue
			}
			line := d.Shape(part, typeset.Style{Mono: true, Bold: entry.Bold == chroma.Yes})
			rows[len(rows)-1] = append(rows[len(rows)-1], piece{line, pen, colour.String(), entry.Italic == chroma.Yes})
			pen += line.Width
			widest = max(widest, pen)
		}
	}

	pixels := func(ems int) float64 { return float64(ems) * codeFontSize / typeset.Em }
	gutter := 0.0
	if c.LineNumbers {
		gutter = float64(len(strconv.Itoa(len(rows)))+2) * codeCharWidth
	}
	picture := codePicture{language: lexer.Config().Name, lines: len(rows)}
	picture.width = int(max(pixels(widest), codeMinColumns*codeCharWidth)+gutter+0.999) + 2*codePadding
	picture.height = codeHeader + len(rows)*codeLineHeight + 2*codePadding - 8

	d.printf(`<rect width="%d" height="%d" rx="12" fill="%s"/>`, picture.width, picture.height, frame)
	for i, dot := range []string{"#ff5f56", "#ffbd2e", "#27c93f"} {
		d.printf(`<circle cx="%d" cy="24" r="6.5" fill="%s"/>`, codePadding+4+i*22, dot)
	}
	if c.Title != "" {
		// Centred, and kept clear of the three dots.
		room := int(float64(picture.width-2*(codePadding+70)) * typeset.Em / codeTitleSize)
		title := d.Shape(d.Fit(c.Title, typeset.Style{}, room), typeset.Style{})
		d.text(title, float64(picture.width)/2-float64(title.Width)*codeTitleSize/typeset.Em/2, 29, codeTitleSize, muted, false)
	}
	for i, row := range rows {
		y := float64(codeHeader + codePadding/2 + (i+1)*codeLineHeight - 7)
		if c.LineNumbers {
			label := d.Shape(strconv.Itoa(i+1), typeset.Style{Mono: true})
			d.text(label, codePadding+gutter-2*codeCharWidth-pixels(label.Width), y, codeFontSize, muted, false)
		}
		for _, p := range row {
			d.text(p.line, codePadding+gutter+pixels(p.at), y, codeFontSize, p.colour, p.italic)
		}
	}
	label := c.Title
	if label == "" {
		label = picture.language + " code"
	}
	picture.svg = d.svg(picture.width, picture.height, label)
	return picture
}

func (s *Server) handleCodeImage(w http.ResponseWriter, r *http.Request) {
	// Code is often unreleased work: not logged, not shared-cached.
	info(r).private = true
	query := r.URL.Query()
	var in textInput
	if r.Method == http.MethodPost || query.Get("text") != "" {
		var ok bool
		if in, ok = s.readText(w, r); !ok {
			return
		}
	} else {
		in = textInput{Text: query.Get("code"), options: map[string]string{}}
		if strings.TrimSpace(in.Text) == "" {
			s.badRequest(w, r, "No code given. Use ?code=..., or POST it as text/plain or as JSON {\"text\": \"...\"}.")
			return
		}
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(in.Text, "\r\n", "\n"), "\n "), "\n")
	if len(lines) > codeMaxLines {
		s.badRequest(w, r, fmt.Sprintf("At most %d lines fit in one image.", codeMaxLines))
		return
	}
	for _, line := range lines {
		if utf8.RuneCountInString(strings.ReplaceAll(line, "\t", "    ")) > codeMaxColumns {
			s.badRequest(w, r, fmt.Sprintf("Lines can be at most %d characters wide.", codeMaxColumns))
			return
		}
	}
	image := codeImage{Code: in.Text, Language: strings.TrimSpace(in.option(r, "language")),
		Title: strings.Join(strings.Fields(drawable(in.option(r, "title"))), " "),
		Theme: strings.ToLower(strings.TrimSpace(in.option(r, "theme")))}
	if image.Theme == "" {
		image.Theme = "dark"
	}
	if _, ok := codeThemes[image.Theme]; !ok {
		s.badRequest(w, r, "theme must be dark or light.")
		return
	}
	if image.Language != "" && lexers.Get(image.Language) == nil {
		s.badRequest(w, r, "Unknown language. Use a common name or file extension, such as go, python, js, sql or bash; leave it out to guess.")
		return
	}
	if utf8.RuneCountInString(image.Title) > 80 {
		s.badRequest(w, r, "title can be at most 80 characters.")
		return
	}
	switch strings.ToLower(in.option(r, "lineNumbers")) {
	case "1", "true", "yes":
		image.LineNumbers = true
	}
	format, err := imageFormat(in.option(r, "format"))
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	var picture codePicture
	s.extras.render.work(func() { picture = image.draw() })
	w.Header().Set("X-Code-Language", picture.language)
	s.sendImage(w, r, picture.svg, format, picture.zoom(), picture.large(), "private, no-store")
}
