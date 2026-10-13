package dataset

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"
)

// HSN codes for goods and SAC codes for services. The HSN codes and their descriptions
// are the First Schedule to the Customs Tariff Act, 1975; the SAC codes are the scheme
// of classification of services annexed to GST notification 11/2017-Central Tax (Rate).
// Reproducing an Act or a gazette notification is allowed by section 52(1)(q) of the
// Copyright Act, 1957. The copy used is the HSN_SAC.xlsx the GST portal publishes.
// Rates are deliberately not included (Dharun, 2026-10-13).

func init() {
	register(&Spec{
		Name:   "hsn",
		Schema: 1,
		DDL: `
CREATE TABLE codes (
  code TEXT PRIMARY KEY, kind TEXT NOT NULL, level TEXT NOT NULL, description TEXT NOT NULL,
  parent TEXT, search TEXT NOT NULL
) WITHOUT ROWID;
CREATE INDEX codes_parent ON codes(parent);`,
		Build:  buildHSN,
		Import: importHSN,
		Every:  30 * 24 * time.Hour,
		Main:   "codes", MaxShrink: 0.05,
	})
}

// hsnLevel names a code by its length: chapters have 2 digits, headings 4, subheadings
// 6 and tariff items 8. Service codes start with 99.
func hsnLevel(code, kind string) string {
	if kind == "sac" {
		switch len(code) {
		case 2:
			return "section"
		case 4:
			return "heading"
		case 5:
			return "group"
		}
		return "service"
	}
	switch len(code) {
	case 2:
		return "chapter"
	case 4:
		return "heading"
	case 6:
		return "subheading"
	}
	return "tariff_item"
}

func buildHSN(b *Builder) error {
	records, _, err := readGzipCSV(b.Path("hsn", "hsn_sac.csv.gz"))
	if err != nil {
		return err
	}
	col, err := columns(records[0], "kind", "code", "description")
	if err != nil {
		return err
	}
	type entry struct{ kind, description string }
	codes := map[string]entry{}
	for _, row := range records[1:] {
		// A couple of codes are written with a space inside ("2307 00").
		code := strings.Join(strings.Fields(row[col["code"]]), "")
		kind := row[col["kind"]]
		if code == "" || strings.Trim(code, "0123456789") != "" || (kind != "hsn" && kind != "sac") {
			b.Warnf("skipped %q (%s)", code, kind)
			continue
		}
		description := strings.Join(strings.Fields(row[col["description"]]), " ")
		if existing, ok := codes[code]; ok && existing.description != description {
			b.Warnf("code %s appears twice with different descriptions; the first is kept", code)
			continue
		}
		codes[code] = entry{kind, description}
	}
	insert, err := b.Tx.Prepare(`INSERT INTO codes VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for code, e := range codes {
		// The parent is the longest shorter code of the same kind that exists.
		var parent any
		for length := len(code) - 1; length >= 2; length-- {
			if candidate, ok := codes[code[:length]]; ok && candidate.kind == e.kind {
				parent = code[:length]
				break
			}
		}
		if _, err := insert.Exec(code, e.kind, hsnLevel(code, e.kind), e.description, parent, strings.ToLower(e.description)); err != nil {
			return fmt.Errorf("%s: %w", code, err)
		}
		b.Add("codes", 1)
		b.Add(e.kind+"_codes", 1)
	}
	return nil
}

const hsnWorkbook = "https://tutorial.gst.gov.in/downloads/HSN_SAC.xlsx"

func importHSN(im *Importer) (Source, error) {
	raw, err := im.Browse(hsnWorkbook)
	if err != nil {
		return Source{}, err
	}
	version := digest(raw)[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	sheets, err := readXLSX(raw)
	if err != nil {
		return Source{}, err
	}
	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	writer.Write([]string{"kind", "code", "description"})
	counts := map[string]int{}
	for kind, sheet := range map[string]string{"hsn": "HSN_MSTR", "sac": "SAC_MSTR"} {
		rows, ok := sheets[sheet]
		if !ok || len(rows) < 2 {
			return Source{}, fmt.Errorf("the workbook has no %s sheet", sheet)
		}
		for _, row := range rows[1:] {
			if len(row) < 2 || strings.TrimSpace(row[0]) == "" {
				continue
			}
			writer.Write([]string{kind, strings.TrimSpace(row[0]), strings.TrimSpace(row[1])})
			counts[kind]++
		}
	}
	writer.Flush()
	if counts["hsn"] < 10000 || counts["sac"] < 300 {
		return Source{}, fmt.Errorf("only %d HSN and %d SAC codes: the workbook has changed shape", counts["hsn"], counts["sac"])
	}
	if err := im.WriteGzip("hsn/hsn_sac.csv.gz", out.Bytes()); err != nil {
		return Source{}, err
	}
	return Source{
		Name:      "HSN and SAC codes",
		Publisher: "Customs Tariff Act, 1975 (First Schedule) and GST notification 11/2017-Central Tax (Rate), as published on the GST portal",
		URL:       hsnWorkbook,
		Licence:   "Statute and gazette notification; reproduction permitted by section 52(1)(q) of the Copyright Act, 1957",
		Version:   version,
		SHA256:    digest(raw),
		Attribution: "HSN codes from the First Schedule to the Customs Tariff Act, 1975, and SAC codes from the annexure to notification " +
			"11/2017-Central Tax (Rate), in the copy published on the GST portal.",
	}, nil
}
