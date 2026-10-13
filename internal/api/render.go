package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/typeset"
)

// Generated images are drawn as SVG here, with their text already shaped and set as
// glyph outlines (internal/typeset), and turned into PNG by the resvg program, the same
// one that renders logos at build time. resvg only fills paths: it needs no fonts. It
// runs once per image and exits, so it holds no memory between requests; a small fixed
// number may run at once.

var errNoRenderer = errors.New("no renderer")

// renderer finds resvg once.
type renderer struct {
	resvg string
	slots chan struct{} // one per picture being rendered
	turn  chan struct{} // held while taking slots, so two large pictures cannot each wait for the other's
}

func newRenderer(cfg Config) *renderer {
	r := &renderer{resvg: cfg.Resvg, slots: make(chan struct{}, max(1, cfg.RenderConcurrency)), turn: make(chan struct{}, 1)}
	if r.resvg == "" {
		// Next to the server binary in a release, or on PATH while developing.
		if self, err := os.Executable(); err == nil {
			if candidate := filepath.Join(filepath.Dir(self), "resvg"); isFile(candidate) {
				r.resvg = candidate
			}
		}
		if r.resvg == "" {
			r.resvg, _ = exec.LookPath("resvg")
		}
	} else if !isFile(r.resvg) {
		r.resvg = ""
	}
	return r
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (r *renderer) available() bool { return r.resvg != "" }

// take waits for n rendering slots and returns the function that gives them back.
func (r *renderer) take(ctx context.Context, n int) (func(), error) {
	taken := 0
	release := func() {
		for ; taken > 0; taken-- {
			<-r.slots
		}
	}
	select {
	case r.turn <- struct{}{}:
		defer func() { <-r.turn }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	for taken < n {
		select {
		case r.slots <- struct{}{}:
			taken++
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		}
	}
	return release, nil
}

// png renders an SVG. zoom scales the image for sharper output. A large picture, which
// takes resvg 40 to 75 MB, is rendered on its own, so that its memory is never added to
// another's.
func (r *renderer) png(ctx context.Context, svg []byte, zoom int, large bool) ([]byte, error) {
	if !r.available() {
		return nil, errNoRenderer
	}
	slots := 1
	if large {
		slots = cap(r.slots)
	}
	release, err := r.take(ctx, slots)
	if err != nil {
		return nil, err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	args := []string{"-", "-c", "--skip-system-fonts", "--quiet"}
	if zoom > 1 {
		args = append(args, "--zoom", strconv.Itoa(zoom))
	}
	command := exec.CommandContext(ctx, r.resvg, args...)
	command.Stdin = bytes.NewReader(svg)
	var out, problems bytes.Buffer
	command.Stdout, command.Stderr = &out, &problems
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("resvg: %v: %s", err, clip(strings.TrimSpace(problems.String()), 200))
	}
	return out.Bytes(), nil
}

// ---- drawing ----

// canvas builds an SVG picture. Shapes are written to it directly; text goes through
// text, which draws each glyph as a reference to its outline, defined once.
type canvas struct {
	*typeset.Setter
	body bytes.Buffer
	defs bytes.Buffer
	// outlines already looked up: true when the glyph has a shape to draw
	glyphs map[uint32]bool
}

func newCanvas() *canvas {
	return &canvas{Setter: typeset.New(), glyphs: map[uint32]bool{}}
}

func (d *canvas) printf(format string, args ...any) { fmt.Fprintf(&d.body, format, args...) }

// text draws a shaped line with the start of its baseline at x, y. slant leans it, for
// italics, which the fonts do not have.
func (d *canvas) text(line typeset.Line, x, y, size float64, fill string, slant bool) {
	lean := ""
	if slant {
		lean = " skewX(-11)"
	}
	opened := false
	for _, glyph := range line.Glyphs {
		key := uint32(glyph.Font)<<16 | uint32(glyph.ID)
		visible, seen := d.glyphs[key]
		if !seen {
			outline := d.Outline(glyph.Font, glyph.ID)
			visible = outline != ""
			d.glyphs[key] = visible
			if visible {
				fmt.Fprintf(&d.defs, `<path id="g%x" d="%s"/>`, key, outline)
			}
		}
		if !visible {
			continue // a space
		}
		if !opened {
			opened = true
			d.printf(`<g fill="%s" transform="translate(%s %s)%s scale(%s)">`, fill, coordinate(x), coordinate(y), lean, coordinate(size/typeset.Em))
		}
		d.printf(`<use xlink:href="#g%x" x="%d"`, key, glyph.X)
		if glyph.Y != 0 {
			d.printf(` y="%d"`, glyph.Y)
		}
		d.body.WriteString(`/>`)
	}
	if opened {
		d.body.WriteString(`</g>`)
	}
}

// svg finishes the picture. label says what it shows, for screen readers.
func (d *canvas) svg(width, height int, label string) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d" viewBox="0 0 %d %d" role="img">`,
		width, height, width, height)
	if label != "" {
		fmt.Fprintf(&out, `<title>%s</title>`, html.EscapeString(label))
	}
	if d.defs.Len() > 0 {
		out.WriteString(`<defs>`)
		out.Write(d.defs.Bytes())
		out.WriteString(`</defs>`)
	}
	out.Write(d.body.Bytes())
	out.WriteString(`</svg>`)
	return out.Bytes()
}

// coordinate writes a number without needless digits.
func coordinate(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(strconv.FormatFloat(v, 'f', 4, 64), "0"), ".")
}

// drawable drops characters that have no place in an image: control characters and
// broken UTF-8. Tabs and line breaks are kept for the caller to deal with.
func drawable(text string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t', r == '\n':
			return r
		case r < 0x20, r == 0x7f, r == 0xfffe, r == 0xffff, r == utf8.RuneError:
			return -1
		}
		return r
	}, text)
}
