package dataset

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

func init() {
	register(&Spec{
		Name:   "pincode",
		Schema: 2,
		DDL: `
CREATE TABLE pincodes (
  pincode TEXT NOT NULL, office TEXT NOT NULL, office_type TEXT, delivery INTEGER,
  division TEXT, region TEXT, circle TEXT, district TEXT, state_code TEXT, state TEXT,
  lat REAL, lng REAL
);
CREATE INDEX pincodes_pincode ON pincodes(pincode);
CREATE INDEX pincodes_office ON pincodes(office COLLATE NOCASE);
CREATE INDEX pincodes_district ON pincodes(state_code, district COLLATE NOCASE);
CREATE INDEX pincodes_place ON pincodes(lat, lng) WHERE lat IS NOT NULL;`,
		Build:  buildPincodes,
		Import: importPincode,
		// The portal refuses scripted downloads, so a server cannot refresh this one.
		Main: "post_offices", MaxShrink: 0.05,
	})
}

func buildPincodes(b *Builder) error {
	records, malformed, err := readGzipCSV(b.Path("pincode", "pincode.csv.gz"))
	if err != nil {
		return err
	}
	if malformed > 0 {
		b.Warnf("dropped %d rows with the wrong number of columns", malformed)
	}
	col, err := columns(records[0], "circlename", "regionname", "divisionname", "officename", "pincode", "officetype", "delivery", "district", "statename", "latitude", "longitude")
	if err != nil {
		return err
	}

	insert, err := b.Tx.Prepare(`INSERT INTO pincodes VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()

	pincodes := map[string]bool{}
	unknownStates := map[string]int{}
	skipped, placed := 0, 0
	for _, row := range records[1:] {
		pincode := strings.TrimSpace(row[col["pincode"]])
		// The published file carries a placeholder row ("TEST OFFICE", 999999).
		if len(pincode) != 6 || strings.Trim(pincode, "0123456789") != "" || strings.EqualFold(strings.TrimSpace(row[col["officename"]]), "TEST OFFICE") {
			skipped++
			continue
		}
		stateName := strings.TrimSpace(row[col["statename"]])
		var stateCode any
		if state, ok := india.Lookup(stateName); ok {
			stateCode, stateName = state.Code, state.Name
		} else {
			unknownStates[stateName]++
			stateName = titleCase(stateName)
		}
		lat, lng := coordinate(row[col["latitude"]], 6, 38), coordinate(row[col["longitude"]], 68, 98)
		// Half a position is no position.
		if lat == nil || lng == nil {
			lat, lng = nil, nil
		} else {
			placed++
		}
		if _, err := insert.Exec(
			pincode,
			strings.TrimSpace(row[col["officename"]]),
			nullable(officeType(row[col["officetype"]])),
			strings.EqualFold(strings.TrimSpace(row[col["delivery"]]), "Delivery"),
			nullable(titleCase(strings.TrimSuffix(strings.TrimSpace(row[col["divisionname"]]), " Division"))),
			nullable(titleCase(strings.TrimSuffix(strings.TrimSpace(row[col["regionname"]]), " Region"))),
			nullable(titleCase(strings.TrimSuffix(strings.TrimSpace(row[col["circlename"]]), " Circle"))),
			nullable(titleCase(row[col["district"]])),
			stateCode, nullable(stateName), lat, lng,
		); err != nil {
			return err
		}
		pincodes[pincode] = true
		b.Add("post_offices", 1)
	}
	b.Count("pincodes", len(pincodes))
	b.Count("post_offices_with_position", placed)
	if skipped > 0 {
		b.Warnf("skipped %d rows without a six-digit pincode", skipped)
	}
	if len(unknownStates) > 0 {
		b.Warnf("%d rows have a state name that is not a state: %s", sumValues(unknownStates), topKeys(unknownStates, 8))
	}
	return nil
}

func officeType(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "HO":
		return "Head Office"
	case "SO":
		return "Sub Office"
	case "BO":
		return "Branch Office"
	case "PO":
		return "Post Office"
	}
	return strings.TrimSpace(code)
}

// coordinate keeps a latitude or longitude only when it is a number inside India's bounds.
func coordinate(value string, min, max float64) any {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || number < min || number > max {
		return nil
	}
	return number
}

const pincodeFileURL = "https://www.data.gov.in/files/ogdpv2dms/s3fs-public/dataurl03122020/pincode.csv"

func importPincode(im *Importer) (Source, error) {
	var raw []byte
	var err error
	if im.File != "" {
		raw, err = os.ReadFile(im.File)
	} else {
		raw, err = im.Get(pincodeFileURL)
		if err != nil {
			err = fmt.Errorf("%w\nthe portal often refuses scripted downloads: save the CSV from "+
				"https://www.data.gov.in/resource/all-india-pincode-directory-till-last-month in a browser and pass it with -file", err)
		}
	}
	if err != nil {
		return Source{}, err
	}
	if !bytes.Contains(bytes.ToLower(raw[:min(len(raw), 400)]), []byte("pincode")) {
		return Source{}, errors.New("this does not look like the pincode directory: no Pincode column in the header")
	}
	if err := im.WriteGzip("pincode/pincode.csv.gz", raw); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "All India Pincode Directory",
		Publisher:   "Department of Posts, Ministry of Communications, Government of India",
		URL:         "https://www.data.gov.in/resource/all-india-pincode-directory-till-last-month",
		Licence:     "Government Open Data License - India (GODL)",
		LicenceURL:  "https://www.data.gov.in/Godl",
		Version:     fmt.Sprintf("%d bytes", len(raw)),
		SHA256:      digest(raw),
		Attribution: "Contains data from the All India Pincode Directory, Department of Posts, published on data.gov.in under the Government Open Data License - India.",
	}, nil
}
