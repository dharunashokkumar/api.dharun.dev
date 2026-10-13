package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

func init() {
	register(&Spec{
		Name:   "ifsc",
		Schema: 1,
		DDL: `
CREATE TABLE banks (
  code TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT, micr TEXT, upi INTEGER, branches INTEGER NOT NULL DEFAULT 0
) WITHOUT ROWID;
CREATE TABLE ifsc (
  code TEXT PRIMARY KEY, bank_code TEXT NOT NULL, branch TEXT, centre TEXT, district TEXT,
  state_code TEXT, state TEXT, city TEXT, address TEXT, contact TEXT, micr TEXT, swift TEXT,
  imps INTEGER, neft INTEGER, rtgs INTEGER, upi INTEGER
) WITHOUT ROWID;
CREATE INDEX ifsc_bank ON ifsc(bank_code, state_code, city COLLATE NOCASE);
CREATE INDEX ifsc_micr ON ifsc(micr) WHERE micr IS NOT NULL;`,
		Build:  buildIFSC,
		Import: importIFSC,
		Every:  7 * 24 * time.Hour,
		Main:   "branches", MaxShrink: 0.05,
	})
}

type bankRecord struct {
	Type string `json:"type"`
	MICR string `json:"micr"`
	UPI  bool   `json:"upi"`
}

func buildIFSC(b *Builder) error {
	raw, err := os.ReadFile(b.Path("banks.json"))
	if err != nil {
		return err
	}
	banks := map[string]bankRecord{}
	if err := json.Unmarshal(raw, &banks); err != nil {
		return fmt.Errorf("banks.json: %w", err)
	}

	records, malformed, err := readGzipCSV(b.Path("ifsc.csv.gz"))
	if err != nil {
		return err
	}
	if malformed > 0 {
		b.Warnf("dropped %d rows with the wrong number of columns", malformed)
	}
	col, err := columns(records[0], "ifsc", "branch", "centre", "district", "state", "address", "contact", "imps", "rtgs", "city", "neft", "micr", "upi", "swift")
	if err != nil {
		return err
	}

	// Names live in their own file and cover more bank codes than banks.json does.
	rawNames, err := os.ReadFile(b.Path("banknames.json"))
	if err != nil {
		return err
	}
	names := map[string]string{}
	if err := json.Unmarshal(rawNames, &names); err != nil {
		return fmt.Errorf("banknames.json: %w", err)
	}

	// Every bank code that appears anywhere gets a row, so a branch always has a bank.
	known := map[string]bool{}
	for code := range names {
		known[code] = true
	}
	for code := range banks {
		known[code] = true
	}
	for _, row := range records[1:] {
		if code := strings.ToUpper(strings.TrimSpace(row[col["ifsc"]])); len(code) == 11 {
			known[code[:4]] = true
		}
	}
	codes := make([]string, 0, len(known))
	for code := range known {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	insertBank, err := b.Tx.Prepare(`INSERT INTO banks (code, name, type, micr, upi) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insertBank.Close()
	unnamed := 0
	for _, code := range codes {
		bank := banks[code]
		name := names[code]
		if name == "" {
			name = code
			unnamed++
		}
		if _, err := insertBank.Exec(code, name, nullable(bank.Type), nullable(bank.MICR), bank.UPI); err != nil {
			return err
		}
	}
	b.Count("banks", len(codes))
	if unnamed > 0 {
		b.Warnf("%d bank codes have no name in banknames.json and are shown by code", unnamed)
	}

	insert, err := b.Tx.Prepare(`INSERT OR IGNORE INTO ifsc VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()

	unknownStates := map[string]int{}
	for _, row := range records[1:] {
		code := strings.ToUpper(strings.TrimSpace(row[col["ifsc"]]))
		if len(code) != 11 {
			continue
		}
		stateName := strings.TrimSpace(row[col["state"]])
		var stateCode any
		if state, ok := india.Lookup(stateName); ok {
			stateCode, stateName = state.Code, state.Name
		} else {
			unknownStates[stateName]++
			stateName = titleCase(stateName)
		}
		flag := func(name string) bool { return strings.EqualFold(strings.TrimSpace(row[col[name]]), "true") }
		if _, err := insert.Exec(code, code[:4],
			nullable(strings.TrimSpace(row[col["branch"]])),
			nullable(titleCase(row[col["centre"]])),
			nullable(titleCase(row[col["district"]])),
			stateCode, nullable(stateName),
			nullable(titleCase(row[col["city"]])),
			nullable(strings.TrimSpace(row[col["address"]])),
			nullable(strings.TrimSpace(row[col["contact"]])),
			nullable(strings.TrimSpace(row[col["micr"]])),
			nullable(strings.TrimSpace(row[col["swift"]])),
			flag("imps"), flag("neft"), flag("rtgs"), flag("upi"),
		); err != nil {
			return err
		}
		b.Add("branches", 1)
	}
	if len(unknownStates) > 0 {
		b.Warnf("%d branches have a state name that is not a state: %s", sumValues(unknownStates), topKeys(unknownStates, 8))
	}
	_, err = b.Tx.Exec(`UPDATE banks SET branches = (SELECT COUNT(*) FROM ifsc WHERE ifsc.bank_code = banks.code)`)
	return err
}

func importIFSC(im *Importer) (Source, error) {
	version := im.Version
	if version == "" {
		raw, err := im.Get("https://api.github.com/repos/razorpay/ifsc/releases/latest")
		if err != nil {
			return Source{}, err
		}
		var release struct {
			Tag string `json:"tag_name"`
		}
		if err := json.Unmarshal(raw, &release); err != nil || release.Tag == "" {
			return Source{}, fmt.Errorf("could not read the latest release tag: %v", err)
		}
		version = release.Tag
	}
	base := "https://github.com/razorpay/ifsc/releases/download/" + version + "/"
	branches, err := im.Get(base + "IFSC.csv")
	if err != nil {
		return Source{}, err
	}
	banks, err := im.Get(base + "banks.json")
	if err != nil {
		return Source{}, err
	}
	names, err := im.Get("https://raw.githubusercontent.com/razorpay/ifsc/" + version + "/src/banknames.json")
	if err != nil {
		return Source{}, err
	}
	if !json.Valid(banks) || !json.Valid(names) {
		return Source{}, errors.New("banks.json or banknames.json is not valid JSON")
	}
	if err := im.WriteGzip("ifsc.csv.gz", branches); err != nil {
		return Source{}, err
	}
	if err := im.Write("banks.json", banks); err != nil {
		return Source{}, err
	}
	if err := im.Write("banknames.json", names); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "IFSC dataset",
		Publisher:   "Razorpay, compiled from Reserve Bank of India and NPCI lists",
		URL:         "https://github.com/razorpay/ifsc/releases/tag/" + version,
		Licence:     "Public domain (dataset), as stated by the publisher",
		LicenceURL:  "https://github.com/razorpay/ifsc#license",
		Version:     version,
		SHA256:      digest(branches),
		Attribution: "IFSC data from the open razorpay/ifsc dataset, compiled from Reserve Bank of India and NPCI publications.",
	}, nil
}
