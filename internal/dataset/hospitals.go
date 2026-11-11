package dataset

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The National Hospital Directory of the Ministry of Health (Central Bureau of Health
// Intelligence), as of May 2016, from data.gov.in (Government Open Data License - India):
// about 30,000 hospitals with their kind, system of medicine, address, phone numbers and
// position. Old, and some rows have their address columns shifted, as published; the
// pages say so. The nodal person's name, phone and email are a person's details and are
// left out. A server's own copy (NoSeed), with DATA_GOV_IN_KEY.

const hospitalsResource = "37670b6f-c236-49a7-8cd7-cc2dc610e32d"

func init() {
	register(&Spec{
		Name:   "hospitals",
		Schema: 1,
		DDL: `
CREATE TABLE hospitals (id INTEGER PRIMARY KEY, name TEXT NOT NULL, category TEXT NOT NULL, care_type TEXT NOT NULL, system TEXT NOT NULL,
  address TEXT NOT NULL, state TEXT NOT NULL, district TEXT NOT NULL, pincode TEXT NOT NULL, telephone TEXT NOT NULL, emergency TEXT NOT NULL,
  ambulance TEXT NOT NULL, website TEXT NOT NULL, specialties TEXT NOT NULL, facilities TEXT NOT NULL, beds INTEGER, latitude REAL, longitude REAL);
CREATE INDEX hospitals_pincode ON hospitals(pincode);
CREATE INDEX hospitals_place ON hospitals(state COLLATE NOCASE, district COLLATE NOCASE);
CREATE INDEX hospitals_position ON hospitals(latitude, longitude);`,
		Build:  buildHospitals,
		Import: importHospitals,
		Every:  30 * 24 * time.Hour,
		Main:   "hospitals", MaxShrink: 0.05,
		NoSeed: true,
	})
}

// hospitalText is a field with the platform's placeholders taken out.
func hospitalText(r map[string]any, name string) string {
	s := strings.TrimSpace(govText(r, name))
	switch strings.ToUpper(s) {
	case "NA", "N/A", "0", "-", "NIL":
		return ""
	}
	return s
}

func buildHospitals(b *Builder) error {
	insert, err := b.Tx.Prepare(`INSERT OR REPLACE INTO hospitals VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	count := 0
	err = eachLine(filepath.Join(b.Path("hospitals"), "hospitals.jsonl.gz"), func(r map[string]any) error {
		id := lgdCode(r, "sr_no")
		name := hospitalText(r, "hospital_name")
		if id == 0 || name == "" {
			return nil
		}
		var lat, lon *float64
		if a, c, ok := strings.Cut(hospitalText(r, "_location_coordinates"), ","); ok {
			x, err1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
			y, err2 := strconv.ParseFloat(strings.TrimSpace(c), 64)
			// Only positions inside India's box: some rows hold 0,0 or swapped values.
			if err1 == nil && err2 == nil && x > 6 && x < 38 && y > 68 && y < 98 {
				lat, lon = &x, &y
			}
		}
		var beds *int
		if n, err := strconv.Atoi(hospitalText(r, "_total_num_beds")); err == nil && n > 0 {
			beds = &n
		}
		address := hospitalText(r, "_address_original_first_line")
		if address == "" {
			address = hospitalText(r, "_location")
		}
		pin := hospitalText(r, "_pincode")
		if len(pin) != 6 {
			pin = ""
		}
		_, err := insert.Exec(id, name, hospitalText(r, "hospital_category"), hospitalText(r, "_hospital_care_type"), hospitalText(r, "_discipline_systems_of_medicine"),
			address, hospitalText(r, "state"), hospitalText(r, "district"), pin, hospitalText(r, "telephone"), hospitalText(r, "emergency_num"),
			hospitalText(r, "_ambulance_phone_no"), hospitalText(r, "website"), hospitalText(r, "specialties"), hospitalText(r, "facilities"), beds, lat, lon)
		b.Add("hospitals", 1)
		count++
		return err
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("hospitals: no hospital could be read from the source file")
	}
	return nil
}

func importHospitals(im *Importer) (Source, error) {
	count, err := govStream(im, hospitalsResource, "hospitals/hospitals.jsonl.gz")
	if errors.Is(err, errNoKey) {
		im.Logf("hospitals: %v; skipped", err)
		return Source{}, ErrNotChanged
	}
	if err != nil {
		return Source{}, err
	}
	if count < 10000 {
		return Source{}, fmt.Errorf("the hospital directory gave %d rows: fewer than usual", count)
	}
	version := strconv.Itoa(count)
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	return govSource("National Hospital Directory with geo code (May 2016)", "Central Bureau of Health Intelligence, Ministry of Health and Family Welfare, via data.gov.in",
		hospitalsResource, version, "National Hospital Directory (May 2016) from the Ministry of Health and Family Welfare, published on data.gov.in under the Government Open Data License - India."), nil
}
