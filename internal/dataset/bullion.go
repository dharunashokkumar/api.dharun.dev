package dataset

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// Gold, silver and platinum rates from the India Bullion and Jewellers Association
// (ibjarates.com), published twice each working day (AM and PM). IBJA is a private
// association; Dharun accepted the risk on 2026-11-11 (high: a takedown is possible).
// A server's own copy only: each refresh adds the day's rates and carries the earlier
// days forward from the copy in use.

func init() {
	register(&Spec{
		Name:   "bullion",
		Schema: 1,
		DDL: `
CREATE TABLE rates (date TEXT NOT NULL, session TEXT NOT NULL, metal TEXT NOT NULL, purity TEXT NOT NULL, rupees REAL NOT NULL, unit TEXT NOT NULL,
  PRIMARY KEY (date, session, metal, purity)) WITHOUT ROWID;`,
		Build:  buildBullion,
		Import: importBullion,
		Every:  2 * time.Hour,
		Main:   "rates", MaxShrink: 0.05,
		NoSeed: true,
	})
}

type bullionRate struct {
	Date    string  `json:"date"`
	Session string  `json:"session"`
	Metal   string  `json:"metal"`
	Purity  string  `json:"purity"`
	Rupees  float64 `json:"rupees"`
	Unit    string  `json:"unit"`
}

func buildBullion(b *Builder) error {
	raw, err := os.ReadFile(b.Path("bullion", "rates.json"))
	if err != nil {
		return err
	}
	var rates []bullionRate
	if err := json.Unmarshal(raw, &rates); err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR REPLACE INTO rates VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, r := range rates {
		if _, err := insert.Exec(r.Date, r.Session, r.Metal, r.Purity, r.Rupees, r.Unit); err != nil {
			return err
		}
		b.Add("rates", 1)
	}
	return nil
}

var ibjaLabel = regexp.MustCompile(`id="lbl(Gold|Silver|Platinum)(\d{3})_(AM|PM)"[^>]*>\s*([\d,.]*)\s*<`)

// ParseIBJA reads the rates from IBJA's home page.
func ParseIBJA(page []byte, date string) []bullionRate {
	var out []bullionRate
	for _, m := range ibjaLabel.FindAllSubmatch(page, -1) {
		value, err := strconv.ParseFloat(string(bytes.ReplaceAll(m[4], []byte(","), nil)), 64)
		if err != nil || value <= 0 {
			continue
		}
		metal := map[string]string{"Gold": "gold", "Silver": "silver", "Platinum": "platinum"}[string(m[1])]
		unit := "10 grams"
		if metal == "silver" {
			unit = "1 kg"
		}
		out = append(out, bullionRate{Date: date, Session: string(m[3]), Metal: metal, Purity: string(m[2]), Rupees: value, Unit: unit})
	}
	return out
}

func importBullion(im *Importer) (Source, error) {
	now := im.Now
	if now.IsZero() {
		now = time.Now()
	}
	india := time.FixedZone("IST", 5*3600+1800)
	today := now.In(india).Format(time.DateOnly)
	page, err := im.Browse("https://ibjarates.com/")
	if err != nil {
		return Source{}, err
	}
	fresh := ParseIBJA(page, today)
	if len(fresh) < 5 {
		return Source{}, errors.New("IBJA's page shows fewer than five rates: it may have changed")
	}
	all := map[string]bullionRate{}
	keyOf := func(r bullionRate) string { return r.Date + "|" + r.Session + "|" + r.Metal + "|" + r.Purity }
	if im.Current != nil {
		if rows, err := im.Current.Query(`SELECT date, session, metal, purity, rupees, unit FROM rates`); err == nil {
			for rows.Next() {
				var r bullionRate
				if rows.Scan(&r.Date, &r.Session, &r.Metal, &r.Purity, &r.Rupees, &r.Unit) == nil {
					all[keyOf(r)] = r
				}
			}
			rows.Close()
		}
	}
	// IBJA shows the last published rates on holidays too: a session whose numbers are
	// exactly the previous day's is not a new rate.
	previous := map[string]bullionRate{} // session|metal|purity -> the latest earlier rate
	for _, r := range all {
		k := r.Session + "|" + r.Metal + "|" + r.Purity
		if r.Date < today && r.Date > previous[k].Date {
			previous[k] = r
		}
	}
	for _, session := range []string{"AM", "PM"} {
		same, seen := true, false
		for _, r := range fresh {
			if r.Session == session {
				seen = true
				if p, ok := previous[r.Session+"|"+r.Metal+"|"+r.Purity]; !ok || p.Rupees != r.Rupees {
					same = false
				}
			}
		}
		if !seen || same {
			continue
		}
		for _, r := range fresh {
			if r.Session == session {
				all[keyOf(r)] = r
			}
		}
	}
	if len(all) == 0 {
		return Source{}, errors.New("no rates")
	}
	list := make([]bullionRate, 0, len(all))
	for _, r := range all {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool { return keyOf(list[i]) < keyOf(list[j]) })
	raw, _ := json.Marshal(list)
	version := digest(raw)[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	if err := im.Write("bullion/rates.json", raw); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "Gold, silver and platinum rates",
		Publisher:   "India Bullion and Jewellers Association (IBJA)",
		URL:         "https://ibjarates.com/",
		Licence:     "Published by IBJA; reused under this project's data rule, credited",
		Version:     version,
		Attribution: "Rates as published by the India Bullion and Jewellers Association (ibjarates.com), without GST and making charges.",
	}, nil
}
