package dataset

import (
	"encoding/json"
	"errors"
	"html"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The "Current Rates" box of the Reserve Bank of India's home page: policy rates,
// reserve ratios, FBIL reference exchange rates, lending and deposit rate ranges, and
// treasury bill and government bond yields. Public statements of a public body (low
// risk; agreed 2026-11-11). A server's own copy: each refresh keeps the earlier days'
// exchange rates and yields, so history builds up.

func init() {
	register(&Spec{
		Name:   "rbi-rates",
		Schema: 1,
		DDL: `
CREATE TABLE rates (date TEXT NOT NULL, kind TEXT NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, number REAL, PRIMARY KEY (date, kind, name)) WITHOUT ROWID;`,
		Build:  buildRBIRates,
		Import: importRBIRates,
		Every:  3 * time.Hour,
		Main:   "rates", MaxShrink: 0.05,
		NoSeed: true,
	})
}

// RBIRate is one figure of the box. Kind is policy, ratio, fx, lending, money or bond.
type RBIRate struct {
	Date   string   `json:"date"`
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	Value  string   `json:"value"`
	Number *float64 `json:"number,omitempty"`
}

func buildRBIRates(b *Builder) error {
	raw, err := os.ReadFile(b.Path("rbi-rates", "rates.json"))
	if err != nil {
		return err
	}
	var rates []RBIRate
	if err := json.Unmarshal(raw, &rates); err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR REPLACE INTO rates VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, r := range rates {
		if _, err := insert.Exec(r.Date, r.Kind, r.Name, r.Value, r.Number); err != nil {
			return err
		}
		b.Add("rates", 1)
	}
	return nil
}

var (
	rbiTags     = regexp.MustCompile(`(?s)<[^>]+>`)
	rbiPair     = regexp.MustCompile(`^(.+?)\s*:\s*(.+)$`)
	rbiAsAt     = regexp.MustCompile(`(?i)as at[^A-Za-z]*\d+\.\d+\s*[ap]m of\s*([A-Z][a-z]+ \d{1,2}, \d{4})`)
	rbiNumber   = regexp.MustCompile(`-?\d+(?:\.\d+)?`)
	rbiSections = []struct{ heading, kind string }{
		{"Policy Rates", "policy"}, {"Reserve Ratios", "ratio"}, {"Exchange Rates", "fx"}, {"Lending / Deposit Rates", "lending"},
		{"Money Market", "money"}, {"Government Securities Market", "bond"}, {"Capital Market", "stop"},
	}
)

// ParseRBIRates reads the box from the home page. today is the date given to figures
// whose own date the page does not state.
func ParseRBIRates(page []byte, today string) ([]RBIRate, error) {
	text := string(page)
	start := strings.Index(text, "Policy Repo Rate")
	if start < 0 {
		return nil, errors.New("the page has no Policy Repo Rate")
	}
	text = text[max(0, start-2000):min(len(text), start+20000)]
	text = rbiTags.ReplaceAllString(text, "|")
	text = strings.ReplaceAll(html.UnescapeString(text), " ", " ")
	var tokens []string
	for _, t := range strings.Split(text, "|") {
		t = strings.Join(strings.Fields(t), " ")
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	// Join "Label" "|:" "value" into "Label : value".
	var lines []string
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if i+1 < len(tokens) && strings.HasPrefix(tokens[i+1], ":") {
			t += " " + tokens[i+1]
			i++
		}
		lines = append(lines, t)
	}
	kind := ""
	var out []RBIRate
	// Figures carry a mark (none, * or #) saying which "as at" note dates them.
	pending := map[string][]int{}
	assign := func(mark, date string) {
		if d, err := time.Parse("January 2, 2006", date); err == nil {
			for _, i := range pending[mark] {
				out[i].Date = d.Format(time.DateOnly)
			}
		}
		delete(pending, mark)
	}
	for i, line := range lines {
		for _, s := range rbiSections {
			if strings.EqualFold(strings.Join(strings.Fields(line), " "), s.heading) {
				kind = s.kind
			}
		}
		if kind == "stop" {
			break
		}
		if m := rbiAsAt.FindStringSubmatch(line); m != nil { // "(As at 1.00pm of November 11, 2026)"
			mark := ""
			if strings.HasPrefix(line, "*") {
				mark = "*"
			}
			assign(mark, m[1])
			continue
		}
		if strings.EqualFold(line, "as on") && i+1 < len(lines) { // "#", "as on", "November 09, 2026"
			mark := ""
			if i > 0 && (lines[i-1] == "*" || lines[i-1] == "#") {
				mark = lines[i-1]
			}
			assign(mark, lines[i+1])
			continue
		}
		m := rbiPair.FindStringSubmatch(line)
		if m == nil || kind == "" {
			continue
		}
		name := strings.TrimSpace(strings.TrimSuffix(m[1], "*"))
		rawValue := strings.TrimSpace(m[2])
		value := strings.TrimSpace(strings.TrimRight(rawValue, " *#"))
		if value == "" || value == "-%" || value == "-" {
			continue
		}
		r := RBIRate{Date: today, Kind: kind, Name: name, Value: value}
		if strings.Contains(name, "T-bill") {
			r.Kind = "tbill" // the cut-off yield at the last auction: undated on the page
		}
		if n := rbiNumber.FindString(value); n != "" && !strings.Contains(value, " - ") {
			if f, err := strconv.ParseFloat(n, 64); err == nil {
				r.Number = &f
			}
		}
		out = append(out, r)
		if r.Kind == "fx" || r.Kind == "money" || r.Kind == "bond" {
			mark := ""
			switch {
			case strings.HasSuffix(rawValue, "#") || strings.HasSuffix(m[1], "#"):
				mark = "#"
			case strings.HasSuffix(rawValue, "*") || strings.HasSuffix(m[1], "*"):
				mark = "*"
			}
			pending[mark] = append(pending[mark], len(out)-1)
		}
	}
	if len(out) < 8 {
		return nil, errors.New("the rates box has fewer figures than usual: the page may have changed")
	}
	return out, nil
}

func importRBIRates(im *Importer) (Source, error) {
	now := im.Now
	if now.IsZero() {
		now = time.Now()
	}
	today := now.In(time.FixedZone("IST", 5*3600+1800)).Format(time.DateOnly)
	page, err := im.Browse("https://www.rbi.org.in/")
	if err != nil {
		return Source{}, err
	}
	fresh, err := ParseRBIRates(page, today)
	if err != nil {
		return Source{}, err
	}
	all := map[string]RBIRate{}
	key := func(r RBIRate) string { return r.Date + "|" + r.Kind + "|" + r.Name }
	if im.Current != nil {
		if rows, err := im.Current.Query(`SELECT date, kind, name, value, number FROM rates`); err == nil {
			for rows.Next() {
				var r RBIRate
				if rows.Scan(&r.Date, &r.Kind, &r.Name, &r.Value, &r.Number) == nil {
					all[key(r)] = r
				}
			}
			rows.Close()
		}
	}
	for _, r := range fresh {
		all[key(r)] = r
	}
	list := make([]RBIRate, 0, len(all))
	for _, r := range all {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool { return key(list[i]) < key(list[j]) })
	raw, _ := json.Marshal(list)
	version := digest(raw)[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	if err := im.Write("rbi-rates/rates.json", raw); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "RBI current rates",
		Publisher:   "Reserve Bank of India; reference exchange rates by Financial Benchmarks India (FBIL)",
		URL:         "https://www.rbi.org.in/",
		Licence:     "Public statements of the Reserve Bank of India, credited",
		Version:     version,
		Attribution: "Rates as shown by the Reserve Bank of India; exchange rates are FBIL reference rates.",
	}, nil
}
