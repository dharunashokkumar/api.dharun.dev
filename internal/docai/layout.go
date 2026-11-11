// Package docai reads documents without a language model: the text layer of a PDF
// with the position of every word (from poppler's pdftotext -bbox-layout, or OCR), turned
// into Markdown with headings and lists, tables read from aligned columns, and the
// fields of a GST invoice checked against each other.
package docai

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Word is one word with its box, in points from the page's top left.
type Word struct {
	Text           string
	X0, Y0, X1, Y1 float64
}

// Line is a line of words.
type Line struct {
	Words          []Word
	X0, Y0, X1, Y1 float64
}

// Text is the line's words joined by spaces.
func (l Line) Text() string {
	parts := make([]string, len(l.Words))
	for i, w := range l.Words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

// Height is the line's height, which stands for its font size.
func (l Line) Height() float64 { return l.Y1 - l.Y0 }

// Block is a run of lines that pdftotext found belong together.
type Block struct{ Lines []Line }

// Page is one page.
type Page struct {
	Width, Height float64
	Blocks        []Block
}

// ParseBBox reads pdftotext -bbox-layout output.
func ParseBBox(data []byte) ([]Page, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	var pages []Page
	var page *Page
	var block *Block
	var line *Line
	var word *Word
	attr := func(e xml.StartElement, name string) float64 {
		for _, a := range e.Attr {
			if a.Name.Local == name {
				v, _ := strconv.ParseFloat(a.Value, 64)
				return v
			}
		}
		return 0
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "page":
				pages = append(pages, Page{Width: attr(t, "width"), Height: attr(t, "height")})
				page = &pages[len(pages)-1]
			case "block":
				if page != nil {
					page.Blocks = append(page.Blocks, Block{})
					block = &page.Blocks[len(page.Blocks)-1]
				}
			case "line":
				if block != nil {
					block.Lines = append(block.Lines, Line{X0: attr(t, "xMin"), Y0: attr(t, "yMin"), X1: attr(t, "xMax"), Y1: attr(t, "yMax")})
					line = &block.Lines[len(block.Lines)-1]
				}
			case "word":
				if line != nil {
					word = &Word{X0: attr(t, "xMin"), Y0: attr(t, "yMin"), X1: attr(t, "xMax"), Y1: attr(t, "yMax")}
				}
			}
		case xml.CharData:
			if word != nil {
				word.Text += string(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "word":
				if word != nil && line != nil && strings.TrimSpace(word.Text) != "" {
					word.Text = strings.TrimSpace(word.Text)
					line.Words = append(line.Words, *word)
				}
				word = nil
			case "line":
				line = nil
			case "block":
				block = nil
			case "page":
				page = nil
			}
		}
	}
	if len(pages) == 0 {
		return nil, errors.New("no pages")
	}
	return pages, nil
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}

var bullets = []string{"•", "◦", "▪", "‣", "–", "-", "*", "●", "○", "■", "□", "➢", "✓"}

func listMarker(text string) (string, bool, bool) {
	for _, b := range bullets {
		if strings.HasPrefix(text, b+" ") {
			return strings.TrimSpace(text[len(b):]), true, false
		}
	}
	// 1. 1) a) (i)
	if i := strings.IndexAny(text, ".)"); i > 0 && i <= 4 && len(text) > i+1 && text[i+1] == ' ' {
		head := strings.Trim(text[:i], "(")
		if _, err := strconv.Atoi(head); err == nil || (len(head) == 1 && head[0] >= 'a' && head[0] <= 'z') || isRoman(head) {
			return strings.TrimSpace(text[i+1:]), true, true
		}
	}
	return text, false, false
}

func isRoman(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}
	return strings.Trim(strings.ToLower(s), "ivx") == ""
}

// Markdown turns pages into Markdown: a line taller than the body text by a quarter or
// more becomes a heading (the tallest #, then ## and ###), bulleted and numbered lines
// become lists, and the rest of each block one paragraph. Pages are separated by a rule.
func Markdown(pages []Page) string {
	var heights []float64
	for _, p := range pages {
		for _, b := range p.Blocks {
			for _, l := range b.Lines {
				for range l.Words {
					heights = append(heights, l.Height()) // weighted by words: body text dominates
				}
			}
		}
	}
	body := median(heights)
	level := func(h float64) int {
		if body <= 0 {
			return 0
		}
		ratio := h / body
		switch {
		case ratio >= 1.8:
			return 1
		case ratio >= 1.4:
			return 2
		case ratio >= 1.18:
			return 3
		}
		return 0
	}
	var out []string
	for pi, p := range pages {
		if pi > 0 {
			out = append(out, "---")
		}
		// Running headers and footers (page numbers) are left out: a line alone in a
		// block in the top or bottom twentieth of the page that is only a number.
		for _, b := range p.Blocks {
			var paragraph []string
			flush := func() {
				if len(paragraph) > 0 {
					out = append(out, joinHyphenated(paragraph))
					paragraph = nil
				}
			}
			for _, l := range b.Lines {
				text := l.Text()
				if len(b.Lines) == 1 && (l.Y1 < p.Height*0.05 || l.Y0 > p.Height*0.95) {
					if _, err := strconv.Atoi(strings.Trim(text, " -–")); err == nil {
						continue
					}
				}
				if n := level(l.Height()); n > 0 && len(l.Words) <= 20 {
					flush()
					out = append(out, strings.Repeat("#", n)+" "+text)
					continue
				}
				if item, ok, numbered := listMarker(text); ok {
					flush()
					if numbered {
						out = append(out, "1. "+item)
					} else {
						out = append(out, "- "+item)
					}
					continue
				}
				paragraph = append(paragraph, text)
			}
			flush()
		}
	}
	// Join list items of one list without blank lines between them.
	var b strings.Builder
	for i, part := range out {
		if i > 0 {
			isItem := strings.HasPrefix(part, "- ") || strings.HasPrefix(part, "1. ")
			wasItem := strings.HasPrefix(out[i-1], "- ") || strings.HasPrefix(out[i-1], "1. ")
			if isItem && wasItem {
				b.WriteString("\n")
			} else {
				b.WriteString("\n\n")
			}
		}
		b.WriteString(part)
	}
	return b.String()
}

// joinHyphenated joins a paragraph's lines, mending words broken with a hyphen.
func joinHyphenated(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			prev := lines[i-1]
			if strings.HasSuffix(prev, "-") && len(prev) > 1 && isLower(l) {
				s := b.String()
				b.Reset()
				b.WriteString(strings.TrimSuffix(s, "-"))
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString(l)
	}
	return b.String()
}

func isLower(s string) bool { return s != "" && s[0] >= 'a' && s[0] <= 'z' }

// Table is a table found in a page's text.
type Table struct {
	Page    int        `json:"page"`
	Columns int        `json:"columns"`
	Rows    [][]string `json:"rows"`
}

// Tables finds tables: three or more lines in a row whose words fall into the same two
// or more columns, the columns being the gaps that run down every one of those lines.
func Tables(pages []Page) []Table {
	var tables []Table
	for pi, p := range pages {
		var lines []Line
		for _, b := range p.Blocks {
			lines = append(lines, b.Lines...)
		}
		// pdftotext can split one visual row into several blocks: rebuild rows by y.
		rows := mergeRows(lines)
		var run []row
		flush := func() {
			if t, ok := tableFrom(run); ok {
				t.Page = pi + 1
				tables = append(tables, t)
			}
			run = nil
		}
		for _, r := range rows {
			if len(r.cells()) >= 2 {
				if len(run) > 0 && r.y0-run[len(run)-1].y1 > 3*math.Max(r.height(), 1) {
					flush()
				}
				run = append(run, r)
			} else {
				flush()
			}
		}
		flush()
	}
	return tables
}

type row struct {
	words  []Word
	y0, y1 float64
}

func (r row) height() float64 { return r.y1 - r.y0 }

// cells splits a row where the gap between words is wider than one and a half times the
// row's height (an ordinary space is about a quarter of it).
func (r row) cells() [][]Word {
	var out [][]Word
	var current []Word
	for i, w := range r.words {
		if i > 0 && w.X0-r.words[i-1].X1 > 1.5*math.Max(r.height(), 4) {
			out = append(out, current)
			current = nil
		}
		current = append(current, w)
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

func mergeRows(lines []Line) []row {
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Y0 < lines[j].Y0 })
	var rows []row
	for _, l := range lines {
		mid := (l.Y0 + l.Y1) / 2
		if n := len(rows); n > 0 && mid >= rows[n-1].y0 && mid <= rows[n-1].y1 {
			rows[n-1].words = append(rows[n-1].words, l.Words...)
			rows[n-1].y0, rows[n-1].y1 = math.Min(rows[n-1].y0, l.Y0), math.Max(rows[n-1].y1, l.Y1)
			continue
		}
		rows = append(rows, row{words: append([]Word(nil), l.Words...), y0: l.Y0, y1: l.Y1})
	}
	for i := range rows {
		sort.Slice(rows[i].words, func(a, b int) bool { return rows[i].words[a].X0 < rows[i].words[b].X0 })
	}
	return rows
}

// tableFrom aligns a run of rows on column starts that most rows share.
func tableFrom(run []row) (Table, bool) {
	if len(run) < 3 {
		return Table{}, false
	}
	// Column edges: the left x of every cell, clustered within a few points.
	var starts []float64
	for _, r := range run {
		for _, c := range r.cells() {
			starts = append(starts, c[0].X0)
		}
	}
	sort.Float64s(starts)
	var edges []float64
	var counts []int
	for _, x := range starts {
		if n := len(edges); n > 0 && x-edges[n-1] < 12 {
			counts[n-1]++
			continue
		}
		edges, counts = append(edges, x), append(counts, 1)
	}
	var columns []float64
	for i, x := range edges {
		if counts[i]*2 >= len(run) {
			columns = append(columns, x)
		}
	}
	if len(columns) < 2 {
		return Table{}, false
	}
	t := Table{Columns: len(columns)}
	for _, r := range run {
		cells := make([]string, len(columns))
		for _, c := range r.cells() {
			col := 0
			for i, x := range columns {
				if c[0].X0 >= x-12 {
					col = i
				}
			}
			var parts []string
			for _, w := range c {
				parts = append(parts, w.Text)
			}
			text := strings.Join(parts, " ")
			if cells[col] != "" {
				cells[col] += " " + text
			} else {
				cells[col] = text
			}
		}
		t.Rows = append(t.Rows, cells)
	}
	return t, true
}

// MarkdownTable writes a table in Markdown.
func MarkdownTable(t Table) string {
	var b strings.Builder
	for i, r := range t.Rows {
		escaped := make([]string, len(r))
		for j, c := range r {
			escaped[j] = strings.ReplaceAll(c, "|", `\|`)
		}
		b.WriteString("| " + strings.Join(escaped, " | ") + " |\n")
		if i == 0 {
			b.WriteString("|" + strings.Repeat("---|", len(r)) + "\n")
		}
	}
	return b.String()
}
