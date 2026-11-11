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
	"regexp"
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
	// CellGap is the gap between words, in line heights, that starts a new table cell;
	// zero means 0.4, right for a PDF's text layer.
	CellGap float64
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

// Markdown turns pages into Markdown. Lines are rebuilt by their height on the page, so
// text laid out in columns reads across; tables (see Tables) are written as Markdown
// tables where they stand; a line taller than the body text by a fifth or more becomes a
// heading (the tallest size #, the next ##, the rest ###); bulleted and numbered lines become lists; and
// lines close together of one size are joined into paragraphs. A page number alone at
// the top or bottom of a page is left out, and pages are separated by a rule.
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
	// Heading sizes, tallest first: sizes within a twentieth of each other are one level.
	var sizes []float64
	for _, p := range pages {
		for _, r := range pageRows(p) {
			if body > 0 && r.height() >= 1.18*body && len(r.words) <= 20 {
				sizes = append(sizes, r.height())
			}
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(sizes)))
	var levels []float64
	for _, h := range sizes {
		if n := len(levels); n == 0 || h < levels[n-1]*0.95 {
			levels = append(levels, h)
		}
	}
	level := func(h float64) int {
		if body <= 0 || h < 1.18*body {
			return 0
		}
		for i, top := range levels {
			if h >= top*0.95 {
				return min(i+1, 3)
			}
		}
		return 3
	}
	var out []string
	for pi, p := range pages {
		if pi > 0 {
			out = append(out, "---")
		}
		rows := pageRows(p)
		tables := findTables(rows)
		var paragraph []string
		var last *row
		flush := func() {
			if len(paragraph) > 0 {
				out = append(out, joinHyphenated(paragraph))
				paragraph = nil
			}
			last = nil
		}
		for i := 0; i < len(rows); i++ {
			if t, ok := tables[i]; ok {
				flush()
				out = append(out, strings.TrimRight(MarkdownTable(t.table), "\n"))
				i = t.end - 1
				continue
			}
			r := rows[i]
			text := r.text()
			// Things side by side (a name on the left, a title on the right) stay apart.
			spread := false
			if cells := r.cells(); len(cells) > 1 {
				parts := make([]string, 0, len(cells))
				for j, c := range cells {
					if j > 0 && c[0].X0-cells[j-1][len(cells[j-1])-1].X1 > 4*r.height() {
						spread = true
					}
					parts = append(parts, row{words: c}.text())
				}
				if spread {
					text = strings.Join(parts, " · ")
				}
			}
			if r.y1 < p.Height*0.06 || r.y0 > p.Height*0.94 {
				if pageNumber.MatchString(text) {
					continue
				}
			}
			if n := level(r.height()); n > 0 && len(r.words) <= 20 {
				flush()
				out = append(out, strings.Repeat("#", n)+" "+text)
				continue
			}
			if item, ok, numbered := listMarker(text); ok {
				flush()
				if numbered {
					out = append(out, "1. "+escapeLead(item))
				} else {
					out = append(out, "- "+escapeLead(item))
				}
				continue
			}
			// A new paragraph after a gap, a change of size, or a line that starts
			// elsewhere than the one before.
			if spread {
				flush()
				out = append(out, escapeLead(text))
				continue
			}
			if last != nil && (r.y0-last.y1 > 0.6*r.height() || math.Abs(r.height()-last.height()) > 0.15*r.height() ||
				math.Abs(r.words[0].X0-last.words[0].X0) > 2*r.height()) {
				flush()
			}
			paragraph = append(paragraph, escapeLead(text))
			last = &rows[i]
		}
		flush()
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

var pageNumber = regexp.MustCompile(`(?i)^(page\s+)?[-–]?\s*\d{1,4}\s*[-–]?(\s*(of|/)\s*\d{1,4})?$`)

// escapeLead keeps a line that starts with a Markdown mark from being read as one.
func escapeLead(s string) string {
	if s != "" && strings.ContainsRune("#>|=+", rune(s[0])) {
		return "\\" + s
	}
	return s
}

func pageRows(p Page) []row {
	var lines []Line
	for _, b := range p.Blocks {
		lines = append(lines, b.Lines...)
	}
	return mergeRows(lines, p.CellGap)
}

func (r row) text() string {
	parts := make([]string, len(r.words))
	for i, w := range r.words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
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

// Tables finds tables: three or more rows in a row whose cells line up in the same two or
// more columns. A column is the stretch across the page that cells of those rows cover;
// a row whose cells would join two columns, or mostly fall outside them, ends the table.
func Tables(pages []Page) []Table {
	tables := []Table{}
	for pi, p := range pages {
		rows := pageRows(p)
		found := findTables(rows)
		for i := range rows {
			if t, ok := found[i]; ok {
				t.table.Page = pi + 1
				tables = append(tables, t.table)
			}
		}
	}
	return tables
}

type foundTable struct {
	table Table
	end   int // the row after the table
}

// findTables returns the tables of a page by the row each starts at.
func findTables(rows []row) map[int]foundTable {
	found := map[int]foundTable{}
	start := 0
	var spans []span
	flush := func(end int) {
		if t, ok := tableFrom(rows[start:end], spans); ok {
			found[start] = foundTable{t, end}
		}
		start, spans = end, nil
	}
	for i, r := range rows {
		cells := r.cells()
		if len(cells) < 2 {
			flush(i)
			start = i + 1
			continue
		}
		if i > start {
			last := rows[i-1]
			if r.y0-last.y1 > 2.5*math.Max(r.height(), 1) || !fits(spans, cells) {
				flush(i)
			}
		}
		spans = addSpans(spans, cells)
	}
	flush(len(rows))
	return found
}

type row struct {
	words  []Word
	y0, y1 float64
	gap    float64 // a gap wider than this many heights starts a new cell
}

func (r row) height() float64 { return r.y1 - r.y0 }

// cells splits a row where the gap between words is wider than an ordinary space: in a
// PDF's text layer a space is about a fifth of the line's height, so a gap of two fifths
// starts a new cell. OCR boxes are tighter than font boxes, and their pages set CellGap.
func (r row) cells() [][]Word {
	gap := r.gap
	if gap <= 0 {
		gap = 0.4
	}
	var out [][]Word
	var current []Word
	for i, w := range r.words {
		if i > 0 && w.X0-r.words[i-1].X1 > math.Max(gap*r.height(), 3) {
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

func mergeRows(lines []Line, gap float64) []row {
	lines = append([]Line(nil), lines...)
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Y0 < lines[j].Y0 })
	var rows []row
	for _, l := range lines {
		if len(l.Words) == 0 {
			continue
		}
		mid := (l.Y0 + l.Y1) / 2
		if n := len(rows); n > 0 && mid >= rows[n-1].y0 && mid <= rows[n-1].y1 {
			rows[n-1].words = append(rows[n-1].words, l.Words...)
			rows[n-1].y0, rows[n-1].y1 = math.Min(rows[n-1].y0, l.Y0), math.Max(rows[n-1].y1, l.Y1)
			continue
		}
		rows = append(rows, row{words: append([]Word(nil), l.Words...), y0: l.Y0, y1: l.Y1, gap: gap})
	}
	for i := range rows {
		sort.Slice(rows[i].words, func(a, b int) bool { return rows[i].words[a].X0 < rows[i].words[b].X0 })
	}
	return rows
}

type span struct{ x0, x1 float64 }

func cellSpan(c []Word) span { return span{c[0].X0, c[len(c)-1].X1} }

func (a span) overlaps(b span) bool { return a.x0 < b.x1 && b.x0 < a.x1 }

// fits says whether a row's cells line up with a table's columns: none joins two columns,
// and at least half fall in one.
func fits(spans []span, cells [][]Word) bool {
	inside := 0
	for _, c := range cells {
		n := 0
		for _, s := range spans {
			if cellSpan(c).overlaps(s) {
				n++
			}
		}
		if n > 1 {
			return false
		}
		inside += n
	}
	return inside*2 >= len(cells)
}

// addSpans widens the columns by a row's cells, joining any that come to overlap.
func addSpans(spans []span, cells [][]Word) []span {
	for _, c := range cells {
		spans = append(spans, cellSpan(c))
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].x0 < spans[j].x0 })
	var out []span
	for _, s := range spans {
		if n := len(out); n > 0 && s.x0 < out[n-1].x1 {
			out[n-1].x1 = math.Max(out[n-1].x1, s.x1)
			continue
		}
		out = append(out, s)
	}
	return out
}

// tableFrom puts each cell of a run of rows in the column it falls in.
func tableFrom(run []row, spans []span) (Table, bool) {
	if len(run) < 3 || len(spans) < 2 {
		return Table{}, false
	}
	t := Table{Columns: len(spans)}
	for _, r := range run {
		cells := make([]string, len(spans))
		for _, c := range r.cells() {
			col := 0
			for i, s := range spans {
				if cellSpan(c).overlaps(s) {
					col = i
				}
			}
			parts := make([]string, len(c))
			for i, w := range c {
				parts[i] = w.Text
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
