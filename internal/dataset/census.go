package dataset

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Census of India 2011, Primary Census Abstract: households, population, children,
// Scheduled Castes and Tribes, literacy and workers for India, each state, district,
// sub-district and town, each as a total and split into rural and urban. Published by
// the Office of the Registrar General and Census Commissioner in the Census Digital
// Library (table "Basic Population Figures of India/State/District/Sub-District/Town",
// 2011-IndiaStateDistSbDistTwn-0000.xlsx). The workbook is 16 MB; `apidata import census`
// keeps the columns the API serves in data/census/pca.csv.gz. Fixed: the 2011 count does
// not change. Villages are left out (600,000 rows); the LGD dataset names them.

func init() {
	register(&Spec{
		Name:   "census",
		Schema: 1,
		DDL: `
CREATE TABLE places (level TEXT NOT NULL, code TEXT NOT NULL, state TEXT NOT NULL, district TEXT NOT NULL,
  subdistrict TEXT NOT NULL, name TEXT NOT NULL, search TEXT NOT NULL, PRIMARY KEY (level, code)) WITHOUT ROWID;
CREATE INDEX places_state ON places(state, level);
CREATE INDEX places_district ON places(district, level);
CREATE INDEX places_subdistrict ON places(subdistrict, level);
CREATE TABLE figures (level TEXT NOT NULL, code TEXT NOT NULL, area TEXT NOT NULL,
  households INTEGER, persons INTEGER, males INTEGER, females INTEGER, children INTEGER, boys INTEGER, girls INTEGER,
  sc INTEGER, st INTEGER, literate INTEGER, literate_males INTEGER, literate_females INTEGER,
  workers INTEGER, male_workers INTEGER, female_workers INTEGER, main_workers INTEGER, marginal_workers INTEGER,
  cultivators INTEGER, agricultural_labourers INTEGER, household_industry INTEGER, other_workers INTEGER, non_workers INTEGER,
  PRIMARY KEY (level, code, area)) WITHOUT ROWID;`,
		Build:  buildCensus,
		Import: importCensus,
		Main:   "places", MaxShrink: 0.01,
		Fixed: true,
	})
}

// CensusFigures names the figure columns, in table order after level, code and area.
var CensusFigures = []string{"households", "persons", "males", "females", "children", "boys", "girls", "sc", "st",
	"literate", "literate_males", "literate_females", "workers", "male_workers", "female_workers", "main_workers",
	"marginal_workers", "cultivators", "agricultural_labourers", "household_industry", "other_workers", "non_workers"}

// censusColumns maps each figure to the workbook's columns, added together.
var censusColumns = [][]string{{"No_HH"}, {"TOT_P"}, {"TOT_M"}, {"TOT_F"}, {"P_06"}, {"M_06"}, {"F_06"}, {"P_SC"}, {"P_ST"},
	{"P_LIT"}, {"M_LIT"}, {"F_LIT"}, {"TOT_WORK_P"}, {"TOT_WORK_M"}, {"TOT_WORK_F"}, {"MAINWORK_P"}, {"MARGWORK_P"},
	{"MAIN_CL_P", "MARG_CL_P"}, {"MAIN_AL_P", "MARG_AL_P"}, {"MAIN_HH_P", "MARG_HH_P"}, {"MAIN_OT_P", "MARG_OT_P"}, {"NON_WORK_P"}}

const censusWorkbook = "https://censusindia.gov.in/nada/index.php/catalog/42559/download/46185/2011-IndiaStateDistSbDistTwn-0000.xlsx"

var censusLevels = map[string]string{"India": "india", "STATE": "state", "DISTRICT": "district", "SUB-DISTRICT": "subdistrict", "TOWN": "town"}

func importCensus(im *Importer) (Source, error) {
	var raw []byte
	var err error
	if im.File != "" {
		raw, err = os.ReadFile(im.File)
	} else {
		raw, err = im.Browse(censusWorkbook)
	}
	if err != nil {
		return Source{}, err
	}
	sheets, err := readXLSX(raw)
	if err != nil {
		return Source{}, err
	}
	rows := sheets["Data"]
	if len(rows) < 1000 {
		return Source{}, fmt.Errorf("the workbook has no Data sheet of the expected size")
	}
	head := map[string]int{}
	for i, name := range rows[0] {
		head[strings.TrimSpace(name)] = i
	}
	for _, need := range append([]string{"State", "District", "Subdistt", "Town/Village", "Level", "Name", "TRU"}, flatten(censusColumns)...) {
		if _, ok := head[need]; !ok {
			return Source{}, fmt.Errorf("the workbook has no %s column", need)
		}
	}
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	w.Write(append([]string{"state", "district", "subdistrict", "town", "level", "name", "area"}, CensusFigures...))
	count := 0
	for _, row := range rows[1:] {
		cell := func(name string) string {
			if i := head[name]; i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		level, ok := censusLevels[cell("Level")]
		if !ok {
			continue
		}
		record := []string{cell("State"), cell("District"), cell("Subdistt"), cell("Town/Village"), level, cell("Name"), strings.ToLower(cell("TRU"))}
		for _, parts := range censusColumns {
			total := 0
			for _, p := range parts {
				n, err := strconv.Atoi(cell(p))
				if err != nil {
					return Source{}, fmt.Errorf("%s of %s is not a number", p, cell("Name"))
				}
				total += n
			}
			record = append(record, strconv.Itoa(total))
		}
		w.Write(record)
		count++
	}
	w.Flush()
	if count < 20000 {
		return Source{}, fmt.Errorf("only %d rows: the workbook has changed shape", count)
	}
	if err := im.WriteGzip("census/pca.csv.gz", out.Bytes()); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "Census of India 2011, Primary Census Abstract (India, states, districts, sub-districts and towns)",
		Publisher:   "Office of the Registrar General and Census Commissioner, India",
		URL:         "https://censusindia.gov.in/nada/index.php/catalog/42559",
		Licence:     "Government of India publication, published openly in the Census Digital Library; credited",
		Version:     "2011",
		SHA256:      digest(raw),
		Attribution: "Census of India 2011, Primary Census Abstract, Office of the Registrar General and Census Commissioner, India.",
	}, nil
}

func flatten(groups [][]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func buildCensus(b *Builder) error {
	records, _, err := readGzipCSV(b.Path("census", "pca.csv.gz"))
	if err != nil {
		return err
	}
	col, err := columns(records[0], "state", "district", "subdistrict", "town", "level", "name", "area")
	if err != nil {
		return err
	}
	place, err := b.Tx.Prepare(`INSERT INTO places VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer place.Close()
	figures, err := b.Tx.Prepare(`INSERT INTO figures VALUES (?,?,?` + strings.Repeat(",?", len(CensusFigures)) + `)`)
	if err != nil {
		return err
	}
	defer figures.Close()
	seen := map[string]bool{}
	skip := map[string]bool{}
	for _, row := range records[1:] {
		level := row[col["level"]]
		code := map[string]string{"india": "0", "state": row[col["state"]], "district": row[col["district"]],
			"subdistrict": row[col["subdistrict"]], "town": row[col["town"]]}[level]
		key := level + "/" + code
		area := row[col["area"]]
		// A town's code is shared by its parts ("Samalkha (MC + OG)" and "Samalkha (MC)"):
		// the first, the whole town, is kept.
		if skip[key+"/"+row[col["name"]]] {
			continue
		}
		if !seen[key] {
			seen[key] = true
			name := row[col["name"]]
			if level == "state" || level == "india" {
				name = censusTitle(name)
			}
			if _, err := place.Exec(level, code, row[col["state"]], row[col["district"]], row[col["subdistrict"]], name, strings.ToLower(name)); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			b.Add("places", 1)
			b.Add(level+"s", 1)
		} else if area == "total" || (level == "town" && seenArea(seen, key, area)) {
			skip[key+"/"+row[col["name"]]] = true
			continue
		}
		seen[key+"/"+area] = true
		args := []any{level, code, area}
		for i := range CensusFigures {
			n, _ := strconv.Atoi(row[len(col)+i])
			args = append(args, n)
		}
		if _, err := figures.Exec(args...); err != nil {
			return fmt.Errorf("%s %s: %w", key, area, err)
		}
	}
	return nil
}

func seenArea(seen map[string]bool, key, area string) bool { return seen[key+"/"+area] }

// censusTitle turns the workbook's capitals ("JAMMU & KASHMIR") into a name.
func censusTitle(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if w == "and" || w == "&" || (w == "of" && i > 0) {
			continue
		}
		if w == "nct" {
			words[i] = "NCT"
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
