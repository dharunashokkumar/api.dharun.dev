package dataset

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Card number prefixes (BINs, the first six digits of a card): network, debit or credit,
// product, issuing bank and country, from the open BIN list of github.com/venelinkochev/
// bin-list-data (CC BY 4.0), pinned to one commit and its checksum. 374,000 prefixes and
// 27 MB of CSV, so a server's own copy only, like the IP database. Quality varies: the
// list is what BIN databases publish, not the card networks' own tables.

const (
	binCommit = "023a4f68ab1b5d7edd45f03acb132f88a49db96a"
	binSHA256 = "0583860988c7b15d2921025ee8afe89eaf4947fe31acde8ffccfc82691d13e35"
	binURL    = "https://raw.githubusercontent.com/venelinkochev/bin-list-data/" + binCommit + "/bin-list-data.csv"
)

func init() {
	register(&Spec{
		Name:   "bins",
		Schema: 1,
		DDL: `
CREATE TABLE bins (bin TEXT PRIMARY KEY, brand TEXT NOT NULL, type TEXT NOT NULL, category TEXT NOT NULL,
  issuer TEXT NOT NULL, issuer_phone TEXT NOT NULL, issuer_url TEXT NOT NULL, country TEXT NOT NULL, country_name TEXT NOT NULL) WITHOUT ROWID;
CREATE INDEX bins_country ON bins(country);`,
		Build:  buildBins,
		Import: importBins,
		Every:  30 * 24 * time.Hour,
		Main:   "bins", MaxShrink: 0.05,
		NoSeed: true,
	})
}

func importBins(im *Importer) (Source, error) {
	version := binCommit[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	path, err := im.Download(binURL, "bins/bins.csv")
	if err != nil {
		return Source{}, err
	}
	if err := checkDigest(path, binSHA256); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "BIN List Data",
		Publisher:   "Venelin Kochev (github.com/venelinkochev/bin-list-data), a lite version of binlookup.io",
		URL:         "https://github.com/venelinkochev/bin-list-data",
		Licence:     "CC BY 4.0",
		LicenceURL:  "https://creativecommons.org/licenses/by/4.0/",
		Version:     version,
		SHA256:      binSHA256,
		Attribution: "Card prefixes from BIN List Data by Venelin Kochev (binlookup.io), CC BY 4.0.",
	}, nil
}

func buildBins(b *Builder) error {
	f, err := os.Open(b.Path("bins", "bins.csv"))
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		return err
	}
	col, err := columns(head, "bin", "brand", "type", "category", "issuer", "issuerphone", "issuerurl", "isocode2", "countryname")
	if err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR IGNORE INTO bins VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(row) < len(head) {
			continue
		}
		bin := strings.TrimSpace(row[col["bin"]])
		if len(bin) < 6 || len(bin) > 8 || strings.Trim(bin, "0123456789") != "" {
			continue
		}
		get := func(name string) string { return strings.TrimSpace(row[col[name]]) }
		if _, err := insert.Exec(bin, get("brand"), get("type"), get("category"), get("issuer"), get("issuerphone"),
			get("issuerurl"), get("isocode2"), get("countryname")); err != nil {
			return fmt.Errorf("%s: %w", bin, err)
		}
		b.Add("bins", 1)
	}
	return nil
}
