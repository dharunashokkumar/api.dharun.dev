package dataset

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// Airports from OurAirports, which dedicates its data to the public domain. Kept: every
// airport worldwide with an IATA code, and every Indian airfield and heliport. Closed
// airports are left out.

func init() {
	register(&Spec{
		Name:   "airports",
		Schema: 1,
		DDL: `
CREATE TABLE countries (code TEXT PRIMARY KEY, name TEXT NOT NULL) WITHOUT ROWID;
CREATE TABLE airports (
  ident TEXT PRIMARY KEY, type TEXT NOT NULL, name TEXT NOT NULL, lat REAL, lng REAL, elevation_ft INTEGER,
  country TEXT NOT NULL, region TEXT, state_code TEXT, municipality TEXT, scheduled INTEGER NOT NULL,
  icao TEXT, iata TEXT, wikipedia TEXT, website TEXT, keywords TEXT
) WITHOUT ROWID;
CREATE INDEX airports_iata ON airports(iata) WHERE iata IS NOT NULL;
CREATE INDEX airports_icao ON airports(icao) WHERE icao IS NOT NULL;
CREATE INDEX airports_country ON airports(country, state_code);`,
		Build:  buildAirports,
		Import: importAirports,
		Every:  30 * 24 * time.Hour,
		Main:   "airports", MaxShrink: 0.05,
	})
}

// airportRegions maps OurAirports' Indian region codes, a few of which predate the
// current ISO codes, to this project's state codes.
var airportRegions = map[string]string{"IN-MM": "MH", "IN-OR": "OD", "IN-TG": "TS", "IN-CT": "CG", "IN-UT": "UK", "IN-DH": "DH"}

func airportState(region string) string {
	if code, ok := airportRegions[region]; ok {
		return code
	}
	if state, ok := india.Lookup(strings.TrimPrefix(region, "IN-")); ok {
		return state.Code
	}
	return ""
}

func buildAirports(b *Builder) error {
	countries, _, err := readGzipCSV(b.Path("airports", "countries.csv.gz"))
	if err != nil {
		return err
	}
	col, err := columns(countries[0], "code", "name")
	if err != nil {
		return err
	}
	insertCountry, err := b.Tx.Prepare(`INSERT INTO countries VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer insertCountry.Close()
	for _, row := range countries[1:] {
		if _, err := insertCountry.Exec(row[col["code"]], row[col["name"]]); err != nil {
			return err
		}
	}

	airports, _, err := readGzipCSV(b.Path("airports", "airports.csv.gz"))
	if err != nil {
		return err
	}
	col, err = columns(airports[0], "ident", "type", "name", "latitude_deg", "longitude_deg", "elevation_ft", "iso_country",
		"iso_region", "municipality", "scheduled_service", "icao_code", "iata_code", "wikipedia_link", "home_link", "keywords")
	if err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR IGNORE INTO airports VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	unmapped := map[string]int{}
	for _, row := range airports[1:] {
		field := func(name string) string { return strings.TrimSpace(row[col[name]]) }
		if !keepAirport(field("type"), field("iata_code"), field("iso_country")) {
			continue
		}
		var lat, lng, elevation any
		if value, err := strconv.ParseFloat(field("latitude_deg"), 64); err == nil {
			lat = value
		}
		if value, err := strconv.ParseFloat(field("longitude_deg"), 64); err == nil {
			lng = value
		}
		if value, err := strconv.Atoi(field("elevation_ft")); err == nil {
			elevation = value
		}
		var state any
		if field("iso_country") == "IN" {
			if code := airportState(field("iso_region")); code != "" {
				state = code
			} else {
				unmapped[field("iso_region")]++
			}
		}
		if _, err := insert.Exec(field("ident"), field("type"), field("name"), lat, lng, elevation,
			field("iso_country"), nullable(field("iso_region")), state, nullable(field("municipality")), field("scheduled_service") == "yes",
			nullable(strings.ToUpper(field("icao_code"))), nullable(strings.ToUpper(field("iata_code"))),
			nullable(field("wikipedia_link")), nullable(field("home_link")), nullable(field("keywords"))); err != nil {
			return fmt.Errorf("%s: %w", field("ident"), err)
		}
		b.Add("airports", 1)
		if field("iso_country") == "IN" {
			b.Add("airports_in_india", 1)
		}
	}
	if len(unmapped) > 0 {
		b.Warnf("Indian airports with a region that is not a state: %s", topKeys(unmapped, 8))
	}
	return nil
}

func keepAirport(kind, iata, country string) bool {
	return kind != "closed" && (iata != "" || country == "IN")
}

const ourAirports = "https://davidmegginson.github.io/ourairports-data/"

func importAirports(im *Importer) (Source, error) {
	raw, err := im.Get(ourAirports + "airports.csv")
	if err != nil {
		return Source{}, err
	}
	// Only the kept rows go into the file, so the repository's copy stays small.
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return Source{}, fmt.Errorf("airports.csv: %w", err)
	}
	col, err := columns(header, "type", "iata_code", "iso_country")
	if err != nil {
		return Source{}, err
	}
	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	writer.Write(header)
	kept := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Source{}, fmt.Errorf("airports.csv: %w", err)
		}
		if len(row) == len(header) && keepAirport(strings.TrimSpace(row[col["type"]]), strings.TrimSpace(row[col["iata_code"]]), strings.TrimSpace(row[col["iso_country"]])) {
			writer.Write(row)
			kept++
		}
	}
	writer.Flush()
	if kept < 5000 {
		return Source{}, fmt.Errorf("only %d airports kept, expected about 9,000: the file has probably changed shape", kept)
	}
	countries, err := im.Get(ourAirports + "countries.csv")
	if err != nil {
		return Source{}, err
	}
	if err := im.WriteGzip("airports/airports.csv.gz", out.Bytes()); err != nil {
		return Source{}, err
	}
	if err := im.WriteGzip("airports/countries.csv.gz", countries); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "OurAirports",
		Publisher:   "OurAirports (ourairports.com), data mirrored at github.com/davidmegginson/ourairports-data",
		URL:         "https://ourairports.com/data/",
		Licence:     "Public domain",
		LicenceURL:  "https://ourairports.com/data/",
		Version:     fmt.Sprintf("%d airports kept", kept),
		SHA256:      digest(raw),
		Attribution: "Airport data from OurAirports, released into the public domain.",
	}, nil
}
