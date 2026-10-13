// Package typeset lays text out for generated images: it picks a font for each stretch
// of a script, shapes it (conjuncts, vowel signs and reordering included) and hands back
// glyph outlines with their exact positions.
//
// The images draw those outlines as paths, so a picture looks the same wherever it is
// rendered, and the renderer needs no fonts and does no text layout of its own. The
// fonts are Noto Sans for Latin and nine Indian scripts and Noto Sans Mono, embedded in
// the program (SIL Open Font License 1.1, see fonts/LICENSE).
//
// All distances are thousandths of an em, so one layout serves any font size.
package typeset

import (
	"bytes"
	"embed"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/*.ttf
var files embed.FS

// Em is the number of layout units in one em.
const Em = 1000

// Style picks the fonts for a text.
type Style struct {
	Mono bool // code: Noto Sans Mono for everything it covers
	Bold bool
}

// family is one font family and the script it is used for.
type family struct {
	file     string
	script   language.Script
	language language.Language
	from, to rune // the Unicode block; zero for the Latin families
}

const (
	sans = 0
	mono = 10
)

var families = [...]family{
	sans: {file: "NotoSans", script: language.Latin, language: "en"},
	1:    {"NotoSansDevanagari", language.Devanagari, "hi", 0x0900, 0x097F},
	2:    {"NotoSansBengali", language.Bengali, "bn", 0x0980, 0x09FF},
	3:    {"NotoSansGurmukhi", language.Gurmukhi, "pa", 0x0A00, 0x0A7F},
	4:    {"NotoSansGujarati", language.Gujarati, "gu", 0x0A80, 0x0AFF},
	5:    {"NotoSansOriya", language.Oriya, "or", 0x0B00, 0x0B7F},
	6:    {"NotoSansTamil", language.Tamil, "ta", 0x0B80, 0x0BFF},
	7:    {"NotoSansTelugu", language.Telugu, "te", 0x0C00, 0x0C7F},
	8:    {"NotoSansKannada", language.Kannada, "kn", 0x0C80, 0x0CFF},
	9:    {"NotoSansMalayalam", language.Malayalam, "ml", 0x0D00, 0x0D7F},
	mono: {file: "NotoSansMono", script: language.Latin, language: "en"},
}

// Scripts names the scripts that have a font, for documentation.
func Scripts() []string {
	return []string{"Latin", "Devanagari", "Bengali", "Gurmukhi", "Gujarati", "Odia", "Tamil", "Telugu", "Kannada", "Malayalam"}
}

// Font numbers a font: two per family, regular then bold.
type Font uint8

func fontOf(familyIndex int, bold bool) Font {
	if bold {
		return Font(familyIndex*2 + 1)
	}
	return Font(familyIndex * 2)
}

func (f Font) family() int { return int(f) / 2 }

func (f Font) fileName() string {
	weight := "Regular"
	if f%2 == 1 {
		weight = "Bold"
	}
	return "fonts/" + families[f.family()].file + "-" + weight + ".ttf"
}

// load parses a font. Nothing is kept between images: parsing takes about four
// milliseconds a font and an image uses one to three of them, while all twenty-two kept
// parsed would hold 16 MB for as long as the server runs.
func load(f Font) (*font.Face, error) {
	raw, err := files.ReadFile(f.fileName())
	if err != nil {
		return nil, err
	}
	face, err := font.ParseTTF(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.fileName(), err)
	}
	return face, nil
}

// Glyph is one glyph of a line: which outline, and where its origin sits relative to the
// start of the line's baseline. Y grows downwards, as in an image.
type Glyph struct {
	Font Font
	ID   uint16
	X, Y int
	// rune index this glyph's cluster starts at, and the pen position before it
	cluster, pen int
}

// Line is a shaped line of text.
type Line struct {
	Glyphs []Glyph
	Width  int
	// Indic is true when the line holds an Indian script, whose letters reach further
	// above and below the baseline than Latin ones.
	Indic bool
}

// Setter shapes text. It holds the fonts it has needed so far, so make one per image and
// let it go afterwards. It is not safe for concurrent use.
type Setter struct {
	shaper shaping.HarfbuzzShaper
	faces  [len(families) * 2]*font.Face
}

// New returns a Setter.
func New() *Setter { return &Setter{} }

func (s *Setter) face(f Font) *font.Face {
	if s.faces[f] == nil {
		face, err := load(f)
		if err != nil {
			// The fonts are compiled in and checked by the tests.
			panic("typeset: " + err.Error())
		}
		s.faces[f] = face
	}
	return s.faces[f]
}

// neutral reports characters that belong to no one script: spaces, digits, punctuation.
// They stay in the font of the letters before them when that font has them.
func neutral(r rune) bool {
	switch {
	case r == 0x0964 || r == 0x0965: // danda and double danda, shared by the Indian scripts
		return true
	case r < 0x80:
		return !unicode.IsLetter(r)
	case r >= 0x2000 && r <= 0x206F: // general punctuation: dashes, quotes, ellipsis
		return true
	case r == 0x00A0 || r == 0x20B9: // no-break space, rupee sign
		return true
	}
	return false
}

// familyOf finds the family for a letter of an Indian script.
func familyOf(r rune) (int, bool) {
	if r < 0x0900 || r > 0x0D7F {
		return 0, false
	}
	for i := 1; i < mono; i++ {
		if r >= families[i].from && r <= families[i].to {
			return i, true
		}
	}
	return 0, false
}

type run struct {
	start, end int
	family     int
}

// itemise cuts the text into stretches that each use one font.
func (s *Setter) itemise(text []rune, style Style) []run {
	base := sans
	if style.Mono {
		base = mono
	}
	var runs []run
	current := -1
	for i, r := range text {
		var want int
		switch {
		case r == 0x200C || r == 0x200D: // joiners act inside whatever surrounds them
			want = current
		case neutral(r):
			want = base
			if current > 0 && current != mono {
				if _, ok := s.face(fontOf(current, style.Bold)).NominalGlyph(r); ok {
					want = current
				} else if r == 0x0964 || r == 0x0965 {
					want = 1
				}
			} else if r == 0x0964 || r == 0x0965 {
				want = 1
			}
		default:
			if indic, ok := familyOf(r); ok {
				want = indic
			} else {
				want = base
			}
		}
		if want < 0 {
			want = base
		}
		if want != current {
			runs = append(runs, run{start: i, end: i + 1, family: want})
			current = want
		} else {
			runs[len(runs)-1].end = i + 1
		}
	}
	return runs
}

// Shape lays one line of text out. Line breaks in the text are not interpreted.
func (s *Setter) Shape(text string, style Style) Line {
	return s.shape([]rune(text), style)
}

func (s *Setter) shape(text []rune, style Style) Line {
	var line Line
	pen := fixed.Int26_6(0)
	for _, piece := range s.itemise(text, style) {
		used := fontOf(piece.family, style.Bold)
		family := families[piece.family]
		if family.from != 0 {
			line.Indic = true
		}
		out := s.shaper.Shape(shaping.Input{
			Text: text, RunStart: piece.start, RunEnd: piece.end,
			Direction: di.DirectionLTR, Face: s.face(used), Size: fixed.I(Em),
			Script: family.script, Language: family.language,
		})
		for _, glyph := range out.Glyphs {
			line.Glyphs = append(line.Glyphs, Glyph{
				Font: used, ID: uint16(glyph.GlyphID),
				X: (pen + glyph.XOffset).Round(), Y: -glyph.YOffset.Round(),
				cluster: glyph.ClusterIndex, pen: pen.Round(),
			})
			pen += glyph.XAdvance
		}
	}
	line.Width = pen.Round()
	return line
}

// Width measures a text.
func (s *Setter) Width(text string, style Style) int { return s.Shape(text, style).Width }

// Wrap breaks text into lines no wider than max, at spaces where it can. A word wider
// than a line is cut between letters (never inside a conjunct or between a letter and
// its vowel sign).
func (s *Setter) Wrap(text string, style Style, max int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		for s.Width(word, style) > max {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			head, tail := s.cut(word, style, max)
			if head == "" {
				break
			}
			lines = append(lines, head)
			word = tail
		}
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if line != "" && s.Width(candidate, style) > max {
			lines = append(lines, line)
			candidate = word
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// cut splits a text at the last cluster boundary that keeps the head within max.
func (s *Setter) cut(text string, style Style, max int) (head, tail string) {
	runes := []rune(text)
	at := 0
	for _, glyph := range s.shape(runes, style).Glyphs {
		if glyph.pen > max {
			break
		}
		at = max2(at, glyph.cluster)
	}
	return string(runes[:at]), string(runes[at:])
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Clamp keeps at most n lines. When text is dropped, the last line kept ends with an
// ellipsis and still fits in max.
func (s *Setter) Clamp(lines []string, n int, style Style, max int) []string {
	if len(lines) <= n {
		return lines
	}
	kept := append([]string{}, lines[:n]...)
	kept[n-1] = s.shorten(kept[n-1], style, max)
	return kept
}

// Fit returns the text if it is no wider than max, and otherwise as much of it as fits
// with an ellipsis after it.
func (s *Setter) Fit(text string, style Style, max int) string {
	if s.Width(text, style) <= max {
		return text
	}
	return s.shorten(text, style, max)
}

// shorten ends a text with an ellipsis, dropping letters until that fits in max.
func (s *Setter) shorten(text string, style Style, max int) string {
	text = strings.TrimRight(text, " ,.;:")
	for text != "" && s.Width(text+"…", style) > max {
		runes := []rune(text)
		// Drop the last cluster: the glyph with the highest cluster start begins it.
		start := 0
		for _, glyph := range s.shape(runes, style).Glyphs {
			start = max2(start, glyph.cluster)
		}
		text = strings.TrimRight(string(runes[:start]), " ,.;:")
	}
	return text + "…"
}

// Outline returns a glyph's shape as SVG path data, in layout units with the origin on
// the baseline and Y growing downwards. A glyph with no shape (a space) gives "".
func (s *Setter) Outline(f Font, id uint16) string {
	face := s.face(f)
	outline, ok := face.GlyphDataOutline(font.GID(id))
	if !ok || len(outline.Segments) == 0 {
		return ""
	}
	scale := float64(Em) / float64(face.Upem())
	var path strings.Builder
	point := func(p ot.SegmentPoint) {
		path.WriteString(strconv.Itoa(round(float64(p.X) * scale)))
		path.WriteByte(' ')
		path.WriteString(strconv.Itoa(round(-float64(p.Y) * scale)))
	}
	open := false
	for _, segment := range outline.Segments {
		switch segment.Op {
		case ot.SegmentOpMoveTo:
			if open {
				path.WriteByte('Z')
			}
			open = true
			path.WriteByte('M')
			point(segment.Args[0])
		case ot.SegmentOpLineTo:
			path.WriteByte('L')
			point(segment.Args[0])
		case ot.SegmentOpQuadTo:
			path.WriteByte('Q')
			point(segment.Args[0])
			path.WriteByte(' ')
			point(segment.Args[1])
		case ot.SegmentOpCubeTo:
			path.WriteByte('C')
			point(segment.Args[0])
			path.WriteByte(' ')
			point(segment.Args[1])
			path.WriteByte(' ')
			point(segment.Args[2])
		}
	}
	if open {
		path.WriteByte('Z')
	}
	return path.String()
}

func round(v float64) int {
	if v < 0 {
		return int(v - 0.5)
	}
	return int(v + 0.5)
}

// Covered reports whether every character of the text has a glyph in the fonts, which is
// false for scripts outside Scripts (those are drawn as empty boxes).
func (s *Setter) Covered(text string, style Style) bool {
	for _, glyph := range s.Shape(text, style).Glyphs {
		if glyph.ID == 0 {
			return false
		}
	}
	return true
}

// check is used by tests: every font file parses.
func check() error {
	for f := Font(0); int(f) < len(families)*2; f++ {
		if _, err := load(f); err != nil {
			return err
		}
	}
	return nil
}
