package dataset

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The constituents of the NSE indices, from the lists NSE Indices Ltd publishes at
// niftyindices.com (one CSV per index: company, industry, symbol, series, ISIN). Agreed
// by Dharun on 2026-11-11 under the plan's data rule. Weights are not in these lists and
// are not published here.

// niftyIndices maps each list's file name to the index's name.
var niftyIndices = [][2]string{
	{"nifty50", "Nifty 50"}, {"niftynext50", "Nifty Next 50"}, {"nifty100", "Nifty 100"}, {"nifty200", "Nifty 200"}, {"nifty500", "Nifty 500"},
	{"niftymidcap50", "Nifty Midcap 50"}, {"niftymidcap100", "Nifty Midcap 100"}, {"niftymidcap150", "Nifty Midcap 150"},
	{"niftysmallcap100", "Nifty Smallcap 100"}, {"niftysmallcap250", "Nifty Smallcap 250"}, {"niftylargemidcap250", "Nifty LargeMidcap 250"},
	{"niftybank", "Nifty Bank"}, {"niftyfinance", "Nifty Financial Services"}, {"niftypsubank", "Nifty PSU Bank"}, {"niftyit", "Nifty IT"},
	{"niftypharma", "Nifty Pharma"}, {"niftyhealthcare", "Nifty Healthcare"}, {"niftyauto", "Nifty Auto"}, {"niftyfmcg", "Nifty FMCG"},
	{"niftymetal", "Nifty Metal"}, {"niftyrealty", "Nifty Realty"}, {"niftyenergy", "Nifty Energy"}, {"niftyoilgas", "Nifty Oil & Gas"},
	{"niftymedia", "Nifty Media"}, {"niftyconsumerdurables", "Nifty Consumer Durables"}, {"niftypse", "Nifty PSE"}, {"niftycpse", "Nifty CPSE"},
	{"niftyinfra", "Nifty Infrastructure"}, {"niftycommodities", "Nifty Commodities"}, {"niftyconsumption", "Nifty India Consumption"}, {"niftymnc", "Nifty MNC"},
}

func init() {
	register(&Spec{
		Name:   "indices",
		Schema: 1,
		DDL: `
CREATE TABLE indices (id TEXT PRIMARY KEY, name TEXT NOT NULL, members INTEGER NOT NULL);
CREATE TABLE members (
  index_id TEXT NOT NULL REFERENCES indices(id), symbol TEXT NOT NULL, company TEXT NOT NULL,
  industry TEXT, series TEXT, isin TEXT,
  PRIMARY KEY (index_id, symbol)
);
CREATE INDEX members_symbol ON members(symbol);
CREATE INDEX members_isin ON members(isin);`,
		Build:  buildIndices,
		Import: importIndices,
		// Indices are reviewed twice a year, with changes in between for mergers and
		// suspensions; a weekly look is plenty.
		Every: 7 * 24 * time.Hour,
		Main:  "members", MaxShrink: 0.1,
	})
}

// indexID turns a list's file name into the address form: nifty50 -> nifty-50.
func indexID(file string) string {
	for _, pair := range niftyIndices {
		if pair[0] == file {
			return strings.ToLower(strings.NewReplacer(" ", "-", "&", "and").Replace(pair[1]))
		}
	}
	return file
}

func buildIndices(b *Builder) error {
	insertIndex, err := b.Tx.Prepare(`INSERT INTO indices VALUES (?,?,?)`)
	if err != nil {
		return err
	}
	defer insertIndex.Close()
	insertMember, err := b.Tx.Prepare(`INSERT OR IGNORE INTO members VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insertMember.Close()
	built := 0
	for _, pair := range niftyIndices {
		raw, err := os.ReadFile(b.Path("indices", pair[0]+".csv"))
		if errors.Is(err, os.ErrNotExist) {
			b.Warnf("indices: no list for %s", pair[1])
			continue
		}
		if err != nil {
			return err
		}
		rows, err := readIndexList(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", pair[1], err)
		}
		id := indexID(pair[0])
		if _, err := insertIndex.Exec(id, pair[1], len(rows)); err != nil {
			return err
		}
		for _, row := range rows {
			if _, err := insertMember.Exec(id, row[2], row[0], nullable(row[1]), nullable(row[3]), nullable(row[4])); err != nil {
				return err
			}
			b.Add("members", 1)
		}
		built++
	}
	if built == 0 {
		return fmt.Errorf("no index lists were found: %w", os.ErrNotExist)
	}
	b.Count("indices", built)
	return nil
}

// readIndexList reads one list: Company Name, Industry, Symbol, Series, ISIN Code.
func readIndexList(raw []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 || len(records[0]) < 5 || !strings.EqualFold(strings.TrimSpace(records[0][2]), "Symbol") {
		return nil, errors.New("not an index list (no Symbol column)")
	}
	var rows [][]string
	for _, record := range records[1:] {
		if len(record) < 5 || strings.TrimSpace(record[2]) == "" {
			continue
		}
		row := make([]string, 5)
		for i := range row {
			row[i] = strings.TrimSpace(record[i])
		}
		rows = append(rows, row)
	}
	return rows, nil
}

const niftyLists = "https://www.niftyindices.com/IndexConstituent/ind_%slist.csv"

func importIndices(im *Importer) (Source, error) {
	var all bytes.Buffer
	found := 0
	for _, pair := range niftyIndices {
		raw, err := im.Browse(fmt.Sprintf(niftyLists, pair[0]))
		if err != nil {
			im.Logf("indices: %s: %v", pair[1], err)
			continue
		}
		rows, err := readIndexList(raw)
		if err != nil || len(rows) == 0 {
			im.Logf("indices: %s is not a list (the site may have moved it)", pair[1])
			continue
		}
		if err := im.Write(filepath.Join("indices", pair[0]+".csv"), raw); err != nil {
			return Source{}, err
		}
		all.Write(raw)
		found++
	}
	if found < len(niftyIndices)-3 {
		return Source{}, fmt.Errorf("only %d of %d index lists could be read", found, len(niftyIndices))
	}
	version := digest(all.Bytes())[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	return Source{
		Name:        "NSE index constituents",
		Publisher:   "NSE Indices Limited",
		URL:         "https://www.niftyindices.com/",
		Licence:     "Published by NSE Indices Limited; reused under this project's data rule, credited",
		Version:     version,
		SHA256:      digest(all.Bytes()),
		Attribution: "Index constituents as published by NSE Indices Limited (niftyindices.com). Nifty is a trademark of NSE Indices Limited.",
	}, nil
}

// IndexNames lists the indices in a fixed order, for documentation.
func IndexNames() []string {
	names := make([]string, len(niftyIndices))
	for i, pair := range niftyIndices {
		names[i] = indexID(pair[0])
	}
	sort.Strings(names)
	return names
}
