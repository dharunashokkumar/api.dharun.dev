package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// The RTO list is read from the wikitext of Wikipedia's "List of Regional Transport
// Office districts in India": one section per state ("==TN—Tamil Nadu=="), each with
// a table whose columns differ a little from state to state.

var (
	sectionHeading = regexp.MustCompile(`(?m)^==\s*([A-Z]{2})\s*[—–-]\s*(.+?)\s*==\s*$`)
	rtoCode        = regexp.MustCompile(`^([A-Z]{2})[- ]?(\d{1,3})\s*([A-Z]{0,2})$`)

	wikiRef      = regexp.MustCompile(`(?s)<ref[^>/]*/>|<ref[^>]*>.*?</ref>`)
	wikiComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	wikiTag      = regexp.MustCompile(`<[^>]+>`)
	wikiLink     = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]]*)\]\]`)
	wikiExtLink  = regexp.MustCompile(`\[https?://\S+\s*([^\]]*)\]`)
	wikiTemplate = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	cellAttrs    = regexp.MustCompile(`^\s*(?:[a-z-]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s|]+)\s*)+\|`)
	rowspanAttr  = regexp.MustCompile(`rowspan\s*=\s*"?(\d+)`)
)

func parseRTO(wikitext string) ([]dataset.RTO, error) {
	wikitext = wikiComment.ReplaceAllString(wikitext, "")
	headings := sectionHeading.FindAllStringSubmatchIndex(wikitext, -1)
	if len(headings) < 30 {
		return nil, fmt.Errorf("found only %d state sections; the page layout has changed", len(headings))
	}

	seen := map[string]bool{}
	var offices []dataset.RTO
	for i, heading := range headings {
		prefix := wikitext[heading[2]:heading[3]]
		end := len(wikitext)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		for _, table := range splitTables(wikitext[heading[1]:end]) {
			for _, office := range parseRTOTable(table, prefix) {
				if seen[office.Code] {
					continue
				}
				seen[office.Code] = true
				offices = append(offices, office)
			}
		}
	}

	sort.SliceStable(offices, func(a, b int) bool { return offices[a].Code < offices[b].Code })
	return offices, nil
}

func splitTables(section string) []string {
	var tables []string
	for {
		start := strings.Index(section, "{|")
		if start < 0 {
			return tables
		}
		end := strings.Index(section[start:], "\n|}")
		if end < 0 {
			return append(tables, section[start:])
		}
		tables = append(tables, section[start:start+end])
		section = section[start+end+3:]
	}
}

func parseRTOTable(table, prefix string) []dataset.RTO {
	rows := tableRows(table)
	if len(rows) == 0 {
		return nil
	}

	// Find which column holds what from the header row.
	codeCol, officeCol, areaCol, notesCol := -1, -1, -1, -1
	headerRow := -1
	for r, row := range rows {
		for c, cell := range row {
			if strings.EqualFold(cell, "code") {
				codeCol, headerRow = c, r
			}
		}
		if headerRow >= 0 {
			break
		}
	}
	if headerRow < 0 {
		return nil
	}
	for c, cell := range rows[headerRow] {
		name := strings.ToLower(cell)
		switch {
		case c == codeCol:
		case strings.Contains(name, "address"), strings.Contains(name, "zone"), strings.Contains(name, "division"):
		case strings.Contains(name, "annotation"):
			notesCol = c
		case strings.Contains(name, "jurisdiction"), strings.Contains(name, "district"):
			if areaCol < 0 {
				areaCol = c
			}
		case strings.Contains(name, "location"), strings.Contains(name, "office"):
			if officeCol < 0 {
				officeCol = c
			}
		}
	}

	cell := func(row []string, col int) string {
		if col < 0 || col >= len(row) {
			return ""
		}
		return row[col]
	}

	var offices []dataset.RTO
	position := map[string]int{}
	for _, row := range rows[headerRow+1:] {
		// One cell can name several codes that share an office: "CH-01; CH-02" or "AP-39, AP-40".
		for _, candidate := range codeSeparators.Split(strings.ToUpper(cell(row, codeCol)), -1) {
			match := rtoCode.FindStringSubmatch(strings.TrimSpace(candidate))
			if match == nil || match[1] != prefix {
				continue
			}
			number, _ := strconv.Atoi(match[2])
			office := dataset.RTO{
				Code:         fmt.Sprintf("%s-%02d%s", match[1], number, match[3]),
				Prefix:       match[1],
				Office:       cell(row, officeCol),
				Jurisdiction: cell(row, areaCol),
				Notes:        cell(row, notesCol),
			}
			// A code spread over several rows (one per district) is one office with a longer jurisdiction.
			if at, repeated := position[office.Code]; repeated {
				if office.Jurisdiction != "" && !strings.Contains(offices[at].Jurisdiction, office.Jurisdiction) {
					offices[at].Jurisdiction = strings.Trim(offices[at].Jurisdiction+", "+office.Jurisdiction, ", ")
				}
				continue
			}
			// Some states list only a jurisdiction; it then doubles as the office name.
			if office.Office == "" {
				office.Office = office.Jurisdiction
			}
			if office.Office == "" {
				continue
			}
			position[office.Code] = len(offices)
			offices = append(offices, office)
		}
	}
	return offices
}

var codeSeparators = regexp.MustCompile("[,;/]| AND ")

// tableRows turns a wikitable into rows of cleaned cell text, repeating cells that
// span several rows.
func tableRows(table string) [][]string {
	type span struct {
		text string
		left int
	}
	carry := map[int]*span{}
	var rows [][]string

	flush := func(raw []string) {
		if len(raw) == 0 {
			return
		}
		var row []string
		next := 0
		place := func() {
			for {
				s, ok := carry[len(row)]
				if !ok || s.left == 0 {
					return
				}
				row = append(row, s.text)
				s.left--
			}
		}
		for next < len(raw) {
			place()
			text := raw[next]
			next++
			spanRows := 1
			if attrs := cellAttrs.FindString(text); attrs != "" {
				if m := rowspanAttr.FindStringSubmatch(attrs); m != nil {
					spanRows, _ = strconv.Atoi(m[1])
				}
				text = text[len(attrs):]
			}
			// "|| text" at the start of a line is a cell with an empty attribute part.
			text = strings.TrimPrefix(strings.TrimSpace(text), "|")
			clean := cleanWiki(text)
			if spanRows > 1 {
				carry[len(row)] = &span{text: clean, left: spanRows - 1}
			}
			row = append(row, clean)
		}
		place()
		rows = append(rows, row)
	}

	var current []string
	for _, line := range strings.Split(table, "\n") {
		switch {
		case strings.HasPrefix(line, "{|"), strings.HasPrefix(line, "|+"):
		case strings.HasPrefix(line, "|-"):
			flush(current)
			current = nil
		case strings.HasPrefix(line, "!"):
			for _, part := range strings.Split(strings.ReplaceAll(line[1:], "!!", "||"), "||") {
				current = append(current, part)
			}
		case strings.HasPrefix(line, "|"):
			for _, part := range strings.Split(line[1:], "||") {
				current = append(current, part)
			}
		default:
			// A continuation line belongs to the cell above it.
			if len(current) > 0 {
				current[len(current)-1] += " " + line
			}
		}
	}
	flush(current)
	return rows
}

func cleanWiki(text string) string {
	text = wikiRef.ReplaceAllString(text, "")
	for i := 0; i < 3 && strings.Contains(text, "{{"); i++ {
		text = wikiTemplate.ReplaceAllString(text, "")
	}
	text = strings.ReplaceAll(text, "<br>", "; ")
	text = strings.ReplaceAll(text, "<br/>", "; ")
	text = strings.ReplaceAll(text, "<br />", "; ")
	text = wikiTag.ReplaceAllString(text, "")
	text = wikiLink.ReplaceAllString(text, "$1")
	text = wikiExtLink.ReplaceAllString(text, "$1")
	text = strings.NewReplacer("'''", "", "''", "", "&nbsp;", " ", "&amp;", "&").Replace(text)
	return strings.Trim(strings.Join(strings.Fields(text), " "), " ;,")
}
