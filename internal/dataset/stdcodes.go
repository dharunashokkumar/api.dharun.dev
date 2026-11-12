package dataset

import "strings"

// STD codes: every Short Distance Charging Area (SDCA) with its trunk code, the Long
// Distance Charging Area it belongs to and the licensed service area (circle), from
// Annex II of the Department of Telecommunications' National Numbering Plan (2003),
// read by tools/std/parse_nnp.py. Landline codes have stayed as the plan set them;
// service areas are the 2003 ones (Andhra Pradesh includes Telangana). Fixed.

func init() {
	register(&Spec{
		Name:   "std-codes",
		Schema: 1,
		DDL: `
CREATE TABLE sdca (code TEXT PRIMARY KEY, sdca TEXT NOT NULL, ldca TEXT NOT NULL, circle TEXT NOT NULL,
  circle_name TEXT NOT NULL, search TEXT NOT NULL) WITHOUT ROWID;
CREATE INDEX sdca_circle ON sdca(circle);`,
		Build: buildSTDCodes,
		Main:  "codes", MaxShrink: 0.02,
		Fixed: true,
	})
}

func buildSTDCodes(b *Builder) error {
	var rows []struct {
		Code, SDCA, LDCA, Circle string
		CircleName               string `json:"circle_name"`
	}
	if err := readJSONFile(b.Path("std-codes", "sdca.json"), &rows); err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT INTO sdca VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, r := range rows {
		search := strings.ToLower(r.SDCA + " " + r.LDCA)
		if _, err := insert.Exec(r.Code, r.SDCA, r.LDCA, r.Circle, r.CircleName, search); err != nil {
			return err
		}
		b.Add("codes", 1)
	}
	return nil
}
