package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Indian mobile number series: which operator and telecom circle each four-digit series
// was first given to, from Wikipedia's "Mobile telephone numbering in India" (CC BY-SA).
// Since number portability a number can have moved to another operator, so this is the
// original allocation only.

// MobileSeries is one four-digit series as kept in data/phones/series.json.
type MobileSeries struct {
	Series   string `json:"series"`
	Operator string `json:"operator,omitempty"`
	Circle   string `json:"circle,omitempty"`
}

// Circle is one telecom circle.
type Circle struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	Area     string `json:"area,omitempty"`
}

type phoneData struct {
	Operators map[string]string `json:"operators"`
	Circles   []Circle          `json:"circles"`
	Series    []MobileSeries    `json:"series"`
}

func init() {
	register(&Spec{
		Name:   "phones",
		Schema: 1,
		DDL: `
CREATE TABLE operators (code TEXT PRIMARY KEY, name TEXT NOT NULL) WITHOUT ROWID;
CREATE TABLE circles (code TEXT PRIMARY KEY, name TEXT NOT NULL, category TEXT, area TEXT) WITHOUT ROWID;
CREATE TABLE series (series TEXT PRIMARY KEY, operator TEXT, circle TEXT) WITHOUT ROWID;`,
		Build:  buildPhones,
		Import: importPhones,
		Main:   "series", MaxShrink: 0.05,
	})
}

func buildPhones(b *Builder) error {
	raw, err := os.ReadFile(b.Path("phones", "mobile.json"))
	if err != nil {
		return err
	}
	var data phoneData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("mobile.json: %w", err)
	}
	for code, name := range data.Operators {
		if _, err := b.Tx.Exec(`INSERT INTO operators VALUES (?, ?)`, code, name); err != nil {
			return err
		}
	}
	for _, circle := range data.Circles {
		if _, err := b.Tx.Exec(`INSERT INTO circles VALUES (?,?,?,?)`, circle.Code, circle.Name, nullable(circle.Category), nullable(circle.Area)); err != nil {
			return err
		}
		b.Add("circles", 1)
	}
	insert, err := b.Tx.Prepare(`INSERT OR IGNORE INTO series VALUES (?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, s := range data.Series {
		if _, err := insert.Exec(s.Series, nullable(s.Operator), nullable(s.Circle)); err != nil {
			return err
		}
		if s.Operator != "" || s.Circle != "" {
			b.Add("series", 1)
		}
	}
	return nil
}

const mobilePage = "Mobile_telephone_numbering_in_India"

var (
	fourDigits     = regexp.MustCompile(`^[6-9][0-9]{3}$`)
	operatorHeader = regexp.MustCompile(`^!\s*([A-Z]{2})\s*$`)
)

// parseMobile reads the article's three tables: circles (one cell per line), operators
// (a header cell with the code, then the name) and the series grid (series, operator,
// circle, repeated across the row).
func parseMobile(wikitext string) (phoneData, error) {
	wikitext = wikiComment.ReplaceAllString(wikitext, "")
	data := phoneData{Operators: map[string]string{}}
	sections := regexp.MustCompile(`(?m)^==\s*(.+?)\s*==\s*$`).FindAllStringSubmatchIndex(wikitext, -1)
	section := func(title string) string {
		for i, s := range sections {
			if strings.EqualFold(wikitext[s[2]:s[3]], title) {
				end := len(wikitext)
				if i+1 < len(sections) {
					end = sections[i+1][0]
				}
				return wikitext[s[1]:end]
			}
		}
		return ""
	}
	cells := func(text string) []string {
		var out []string
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "|-") && !strings.HasPrefix(line, "|}") && !strings.HasPrefix(line, "|+") {
				// Some rows put every cell on one line, separated by ||.
				for _, cell := range strings.Split(line[1:], "||") {
					out = append(out, cleanWiki(cell))
				}
			}
		}
		return out
	}

	circleCells := cells(section("Telecom circles"))
	for i := 0; i+3 < len(circleCells); i += 4 {
		code := strings.ToUpper(strings.TrimSpace(circleCells[i+1]))
		if code == "" || len(code) > 3 {
			return data, fmt.Errorf("unexpected circle row %q", circleCells[i:i+4])
		}
		data.Circles = append(data.Circles, Circle{Code: code, Name: circleCells[i], Category: circleCells[i+2], Area: circleCells[i+3]})
	}

	var code string
	for _, line := range strings.Split(section("Network operators"), "\n") {
		if match := operatorHeader.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			code = match[1]
		} else if strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "|-") && !strings.HasPrefix(line, "|}") && !strings.HasPrefix(line, "|+") && code != "" {
			data.Operators[code] = cleanWiki(line[1:])
			code = ""
		}
	}

	grid := cells(section("Call No Series"))
	for i := 0; i < len(grid); i++ {
		if !fourDigits.MatchString(grid[i]) || i+2 >= len(grid) {
			continue
		}
		series := MobileSeries{Series: grid[i], Operator: strings.ToUpper(grid[i+1]), Circle: strings.ToUpper(grid[i+2])}
		if series.Operator != "" && len(series.Operator) != 2 || fourDigits.MatchString(series.Operator) || fourDigits.MatchString(series.Circle) {
			continue // a cell went missing; skip rather than misread the row
		}
		data.Series = append(data.Series, series)
		i += 2
	}
	return data, nil
}

func importPhones(im *Importer) (Source, error) {
	wikitext, revision, err := im.wikipedia(mobilePage)
	if err != nil {
		return Source{}, err
	}
	data, err := parseMobile(string(wikitext))
	if err != nil {
		return Source{}, err
	}
	if len(data.Series) < 2000 || len(data.Circles) < 20 || len(data.Operators) < 3 {
		return Source{}, fmt.Errorf("parsed %d series, %d circles and %d operators: the article has probably changed shape", len(data.Series), len(data.Circles), len(data.Operators))
	}
	encoded, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return Source{}, err
	}
	if err := im.Write("phones/mobile.json", append(encoded, '\n')); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "Mobile telephone numbering in India",
		Publisher:   "Wikipedia contributors",
		URL:         wikipediaURL(mobilePage, revision),
		Licence:     "CC BY-SA 4.0",
		LicenceURL:  "https://creativecommons.org/licenses/by-sa/4.0/",
		Version:     "revision " + revision,
		SHA256:      digest(wikitext),
		Attribution: "Mobile number series adapted from the Wikipedia article \"Mobile telephone numbering in India\", licensed CC BY-SA 4.0. This dataset is shared under the same licence.",
	}, nil
}
