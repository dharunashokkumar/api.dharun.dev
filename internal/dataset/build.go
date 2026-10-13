package dataset

import (
	"compress/gzip"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// PNGMasterSize is the edge length of the pre-rendered PNG stored for each logo.
// Requests for smaller sizes are scaled down from it.
const PNGMasterSize = 512

// BuildOptions says where the inputs are and how strict to be.
type BuildOptions struct {
	DataDir  string // holds sources.json, pincode.csv.gz, ifsc.csv.gz, banks.json, rto.json, holidays/
	LogosDir string // holds nse/ and bse/ SVG folders
	Out      string // database file to write
	// Resvg is the path of the resvg executable used to pre-render PNG logos.
	// Empty means PNG output is not built.
	Resvg string
	// Strict makes a missing dataset an error instead of a warning.
	Strict bool
	Log    func(format string, args ...any)
}

// BuildStats counts what went into the database.
type BuildStats struct {
	Logos, LogosWithPNG, Pincodes, PostOffices, Banks, Branches, RTOs, Holidays int
	Warnings                                                                    []string
}

const schema = `
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;

CREATE TABLE logos (
  id TEXT PRIMARY KEY, exchange TEXT NOT NULL, ticker TEXT NOT NULL, company TEXT, sector TEXT,
  industry TEXT, one_line TEXT, isin TEXT, currency TEXT, market_cap REAL,
  is_index_constituent INTEGER NOT NULL DEFAULT 0, indexes TEXT NOT NULL DEFAULT '[]',
  svg BLOB NOT NULL, png BLOB
) WITHOUT ROWID;

CREATE TABLE pincodes (
  pincode TEXT NOT NULL, office TEXT NOT NULL, office_type TEXT, delivery INTEGER,
  division TEXT, region TEXT, circle TEXT, district TEXT, state_code TEXT, state TEXT,
  lat REAL, lng REAL
);
CREATE INDEX pincodes_pincode ON pincodes(pincode);
CREATE INDEX pincodes_office ON pincodes(office COLLATE NOCASE);
CREATE INDEX pincodes_district ON pincodes(state_code, district COLLATE NOCASE);

CREATE TABLE banks (
  code TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT, micr TEXT, upi INTEGER, branches INTEGER NOT NULL DEFAULT 0
) WITHOUT ROWID;

CREATE TABLE ifsc (
  code TEXT PRIMARY KEY, bank_code TEXT NOT NULL, branch TEXT, centre TEXT, district TEXT,
  state_code TEXT, state TEXT, city TEXT, address TEXT, contact TEXT, micr TEXT, swift TEXT,
  imps INTEGER, neft INTEGER, rtgs INTEGER, upi INTEGER
) WITHOUT ROWID;
CREATE INDEX ifsc_bank ON ifsc(bank_code, state_code, city COLLATE NOCASE);
CREATE INDEX ifsc_micr ON ifsc(micr) WHERE micr IS NOT NULL;

CREATE TABLE rto (
  code TEXT PRIMARY KEY, prefix TEXT NOT NULL, state_code TEXT, state TEXT,
  office TEXT NOT NULL, jurisdiction TEXT, notes TEXT
) WITHOUT ROWID;

CREATE TABLE holidays (
  year INTEGER NOT NULL, date TEXT NOT NULL, name TEXT NOT NULL, type TEXT NOT NULL,
  region TEXT NOT NULL, states TEXT NOT NULL, notes TEXT
);
CREATE INDEX holidays_year ON holidays(year, type, date);
`

// Anything executable or remote inside an SVG would be served to browsers, so it fails the build.
var unsafeSVG = []struct {
	pattern *regexp.Regexp
	label   string
}{
	{regexp.MustCompile(`(?i)<script`), "script element"},
	{regexp.MustCompile(`(?i)\son[a-z]+\s*=`), "event handler attribute"},
	{regexp.MustCompile(`(?i)javascript:`), "javascript: URL"},
	{regexp.MustCompile(`(?i)<(foreignObject|iframe|object|embed)\b`), "embedded document"},
	{regexp.MustCompile(`(?i)(href|src)\s*=\s*["']\s*(https?:)?//`), "external reference"},
}

var (
	svgOpenTag = regexp.MustCompile(`<svg\b[^>]*>`)
	svgViewBox = regexp.MustCompile(`\sviewBox\s*=`)
	svgWidth   = regexp.MustCompile(`\swidth="(\d+(?:\.\d+)?)(?:px)?"`)
	svgHeight  = regexp.MustCompile(`\sheight="(\d+(?:\.\d+)?)(?:px)?"`)
)

// Build writes a fresh database. It builds into a temporary file and renames it into
// place, so a failed build never leaves a half-written database behind.
func Build(opts BuildOptions) (BuildStats, error) {
	var stats BuildStats
	if opts.Log == nil {
		opts.Log = func(string, ...any) {}
	}
	warn := func(format string, args ...any) {
		message := fmt.Sprintf(format, args...)
		stats.Warnings = append(stats.Warnings, message)
		opts.Log("warning: %s", message)
	}

	tmp := opts.Out + ".tmp"
	os.Remove(tmp)
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return stats, err
	}
	defer func() {
		db.Close()
		os.Remove(tmp)
	}()
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{"PRAGMA journal_mode=OFF", "PRAGMA synchronous=OFF", "PRAGMA page_size=4096"} {
		if _, err := db.Exec(pragma); err != nil {
			return stats, err
		}
	}
	if _, err := db.Exec(schema); err != nil {
		return stats, err
	}

	sources, err := ReadSources(opts.DataDir)
	if err != nil {
		return stats, err
	}

	steps := []struct {
		name string
		run  func(tx *sql.Tx) error
	}{
		{"logos", func(tx *sql.Tx) error { return buildLogos(tx, opts, &stats) }},
		{"pincode", func(tx *sql.Tx) error { return buildPincodes(tx, opts, &stats, warn) }},
		{"ifsc", func(tx *sql.Tx) error { return buildIFSC(tx, opts, &stats, warn) }},
		{"rto", func(tx *sql.Tx) error { return buildRTO(tx, opts, &stats, warn) }},
		{"holidays", func(tx *sql.Tx) error { return buildHolidays(tx, opts, &stats) }},
	}
	for _, step := range steps {
		tx, err := db.Begin()
		if err != nil {
			return stats, err
		}
		err = step.run(tx)
		if errors.Is(err, os.ErrNotExist) && !opts.Strict {
			warn("%s: source file missing, dataset left empty (%v)", step.name, err)
			tx.Rollback()
			continue
		}
		if err != nil {
			tx.Rollback()
			return stats, fmt.Errorf("%s: %w", step.name, err)
		}
		if source, ok := sources[step.name]; ok {
			value, _ := json.Marshal(source)
			if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)`, "source:"+step.name, string(value)); err != nil {
				tx.Rollback()
				return stats, err
			}
		} else {
			warn("%s: no entry in %s, responses will carry no source credit", step.name, sourcesFile)
		}
		if err := tx.Commit(); err != nil {
			return stats, err
		}
	}

	if _, err := db.Exec(`UPDATE banks SET branches = (SELECT COUNT(*) FROM ifsc WHERE ifsc.bank_code = banks.code)`); err != nil {
		return stats, err
	}
	if _, err := db.Exec("ANALYZE"); err != nil {
		return stats, err
	}
	if _, err := db.Exec("VACUUM"); err != nil {
		return stats, err
	}
	if err := db.Close(); err != nil {
		return stats, err
	}
	os.Remove(opts.Out)
	return stats, os.Rename(tmp, opts.Out)
}

// ---- logos ----

type logoMetadata struct {
	Logos []struct {
		ID                 string   `json:"id"`
		Exchange           string   `json:"exchange"`
		Ticker             string   `json:"ticker"`
		File               string   `json:"file"`
		Company            string   `json:"company_name"`
		Sector             string   `json:"sector"`
		Industry           string   `json:"industry"`
		OneLine            string   `json:"one_liner"`
		ISIN               string   `json:"isin"`
		Currency           string   `json:"currency"`
		MarketCap          *float64 `json:"market_cap"`
		IsIndexConstituent bool     `json:"is_index_constituent"`
		Indexes            []struct {
			Name string `json:"name"`
		} `json:"indexes"`
	} `json:"logos"`
}

func buildLogos(tx *sql.Tx, opts BuildOptions, stats *BuildStats) error {
	raw, err := os.ReadFile(filepath.Join(opts.DataDir, "logo_metadata.full.json"))
	if err != nil {
		return err
	}
	var metadata logoMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return err
	}

	svgs := make([][]byte, len(metadata.Logos))
	for i, logo := range metadata.Logos {
		svg, err := os.ReadFile(filepath.Join(opts.LogosDir, filepath.FromSlash(logo.File)))
		if err != nil {
			return err
		}
		svg, err = prepareSVG(svg)
		if err != nil {
			return fmt.Errorf("%s: %w", logo.File, err)
		}
		svgs[i] = svg
	}

	pngs := make([][]byte, len(svgs))
	if opts.Resvg != "" {
		opts.Log("rendering %d PNG masters with %s", len(svgs), opts.Resvg)
		if err := renderPNGs(opts.Resvg, svgs, pngs); err != nil {
			return err
		}
	}

	insert, err := tx.Prepare(`INSERT INTO logos VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for i, logo := range metadata.Logos {
		names := []string{}
		for _, index := range logo.Indexes {
			if index.Name != "" {
				names = append(names, index.Name)
			}
		}
		indexes, _ := json.Marshal(names)
		var png any
		if pngs[i] != nil {
			png = pngs[i]
			stats.LogosWithPNG++
		}
		if _, err := insert.Exec(logo.ID, logo.Exchange, logo.Ticker, logo.Company, logo.Sector, logo.Industry,
			logo.OneLine, nullable(logo.ISIN), logo.Currency, logo.MarketCap, logo.IsIndexConstituent, string(indexes), svgs[i], png); err != nil {
			return fmt.Errorf("%s: %w", logo.ID, err)
		}
		stats.Logos++
	}
	return nil
}

// prepareSVG rejects unsafe content and makes sure the image can scale.
func prepareSVG(svg []byte) ([]byte, error) {
	svg = []byte(strings.TrimSpace(string(svg)))
	if !strings.HasPrefix(string(svg), "<svg") {
		return nil, errors.New("does not start with <svg")
	}
	for _, check := range unsafeSVG {
		if check.pattern.Match(svg) {
			return nil, fmt.Errorf("contains %s", check.label)
		}
	}
	open := svgOpenTag.Find(svg)
	if open == nil {
		return nil, errors.New("no <svg> tag")
	}
	if svgViewBox.Match(open) {
		return svg, nil
	}
	width, height := svgWidth.FindSubmatch(open), svgHeight.FindSubmatch(open)
	if width == nil || height == nil {
		return nil, errors.New("has neither a viewBox nor a numeric width and height")
	}
	fixed := fmt.Sprintf(`<svg viewBox="0 0 %s %s"%s`, width[1], height[1], open[len("<svg"):])
	return append([]byte(fixed), svg[len(open):]...), nil
}

func renderPNGs(resvg string, svgs, pngs [][]byte) error {
	dir, err := os.MkdirTemp("", "logo-png-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	jobs := make(chan int)
	var firstErr error
	var once sync.Once
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			in := filepath.Join(dir, fmt.Sprintf("in-%d.svg", worker))
			out := filepath.Join(dir, fmt.Sprintf("out-%d.png", worker))
			for i := range jobs {
				err := os.WriteFile(in, svgs[i], 0o600)
				if err == nil {
					var output []byte
					output, err = exec.Command(resvg, "--width", strconv.Itoa(PNGMasterSize), in, out).CombinedOutput()
					if err != nil {
						err = fmt.Errorf("resvg failed on logo %d: %v: %s", i, err, strings.TrimSpace(string(output)))
					}
				}
				if err == nil {
					pngs[i], err = os.ReadFile(out)
				}
				if err != nil {
					once.Do(func() { firstErr = err })
				}
			}
		}(w)
	}
	for i := range svgs {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return firstErr
}

// ---- pincodes ----

func buildPincodes(tx *sql.Tx, opts BuildOptions, stats *BuildStats, warn func(string, ...any)) error {
	records, malformed, err := readGzipCSV(filepath.Join(opts.DataDir, "pincode.csv.gz"))
	if err != nil {
		return err
	}
	if malformed > 0 {
		warn("pincode: dropped %d rows with the wrong number of columns", malformed)
	}
	col, err := columns(records[0], "circlename", "regionname", "divisionname", "officename", "pincode", "officetype", "delivery", "district", "statename", "latitude", "longitude")
	if err != nil {
		return err
	}

	insert, err := tx.Prepare(`INSERT INTO pincodes VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()

	pincodes := map[string]bool{}
	unknownStates := map[string]int{}
	skipped := 0
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
		if _, err := insert.Exec(
			pincode,
			strings.TrimSpace(row[col["officename"]]),
			nullable(officeType(row[col["officetype"]])),
			strings.EqualFold(strings.TrimSpace(row[col["delivery"]]), "Delivery"),
			nullable(titleCase(strings.TrimSuffix(strings.TrimSpace(row[col["divisionname"]]), " Division"))),
			nullable(titleCase(strings.TrimSuffix(strings.TrimSpace(row[col["regionname"]]), " Region"))),
			nullable(titleCase(strings.TrimSuffix(strings.TrimSpace(row[col["circlename"]]), " Circle"))),
			nullable(titleCase(row[col["district"]])),
			stateCode, nullable(stateName),
			coordinate(row[col["latitude"]], 6, 38), coordinate(row[col["longitude"]], 68, 98),
		); err != nil {
			return err
		}
		pincodes[pincode] = true
		stats.PostOffices++
	}
	stats.Pincodes = len(pincodes)
	if skipped > 0 {
		warn("pincode: skipped %d rows without a six-digit pincode", skipped)
	}
	if len(unknownStates) > 0 {
		warn("pincode: %d rows have a state name that is not a state: %s", sumValues(unknownStates), topKeys(unknownStates, 8))
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

// ---- IFSC ----

type bankRecord struct {
	Type string `json:"type"`
	MICR string `json:"micr"`
	UPI  bool   `json:"upi"`
}

func buildIFSC(tx *sql.Tx, opts BuildOptions, stats *BuildStats, warn func(string, ...any)) error {
	raw, err := os.ReadFile(filepath.Join(opts.DataDir, "banks.json"))
	if err != nil {
		return err
	}
	banks := map[string]bankRecord{}
	if err := json.Unmarshal(raw, &banks); err != nil {
		return fmt.Errorf("banks.json: %w", err)
	}

	records, malformed, err := readGzipCSV(filepath.Join(opts.DataDir, "ifsc.csv.gz"))
	if err != nil {
		return err
	}
	if malformed > 0 {
		warn("ifsc: dropped %d rows with the wrong number of columns", malformed)
	}
	col, err := columns(records[0], "ifsc", "branch", "centre", "district", "state", "address", "contact", "imps", "rtgs", "city", "neft", "micr", "upi", "swift")
	if err != nil {
		return err
	}

	// Names live in their own file and cover more bank codes than banks.json does.
	rawNames, err := os.ReadFile(filepath.Join(opts.DataDir, "banknames.json"))
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

	insertBank, err := tx.Prepare(`INSERT INTO banks (code, name, type, micr, upi) VALUES (?,?,?,?,?)`)
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
		stats.Banks++
	}
	if unnamed > 0 {
		warn("ifsc: %d bank codes have no name in banknames.json and are shown by code", unnamed)
	}

	insert, err := tx.Prepare(`INSERT OR IGNORE INTO ifsc VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
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
		bankCode := code[:4]
		stateName := strings.TrimSpace(row[col["state"]])
		var stateCode any
		if state, ok := india.Lookup(stateName); ok {
			stateCode, stateName = state.Code, state.Name
		} else {
			unknownStates[stateName]++
			stateName = titleCase(stateName)
		}
		flag := func(name string) bool { return strings.EqualFold(strings.TrimSpace(row[col[name]]), "true") }
		if _, err := insert.Exec(code, bankCode,
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
		stats.Branches++
	}
	if len(unknownStates) > 0 {
		warn("ifsc: %d branches have a state name that is not a state: %s", sumValues(unknownStates), topKeys(unknownStates, 8))
	}
	return nil
}

// ---- RTO ----

func buildRTO(tx *sql.Tx, opts BuildOptions, stats *BuildStats, warn func(string, ...any)) error {
	raw, err := os.ReadFile(filepath.Join(opts.DataDir, "rto.json"))
	if err != nil {
		return err
	}
	var offices []RTO
	if err := json.Unmarshal(raw, &offices); err != nil {
		return fmt.Errorf("rto.json: %w", err)
	}
	insert, err := tx.Prepare(`INSERT INTO rto VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, office := range offices {
		var stateCode, stateName any
		if state, ok := india.ByVehicleCode(office.Prefix); ok {
			stateCode, stateName = state.Code, state.Name
		} else {
			warn("rto: no state for prefix %s (%s)", office.Prefix, office.Code)
		}
		if _, err := insert.Exec(office.Code, office.Prefix, stateCode, stateName, office.Office, nullable(office.Jurisdiction), nullable(office.Notes)); err != nil {
			return fmt.Errorf("%s: %w", office.Code, err)
		}
		stats.RTOs++
	}
	return nil
}

// ---- holidays ----

var (
	holidayTypes = map[string]bool{"bank": true, "central_gazetted": true, "central_restricted": true, "market": true, "state": true}
	isoDate      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func buildHolidays(tx *sql.Tx, opts BuildOptions, stats *BuildStats) error {
	files, err := filepath.Glob(filepath.Join(opts.DataDir, "holidays", "*.json"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no files in %s: %w", filepath.Join(opts.DataDir, "holidays"), os.ErrNotExist)
	}
	sort.Strings(files)

	insert, err := tx.Prepare(`INSERT INTO holidays VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()

	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var content HolidayFile
		if err := json.Unmarshal(raw, &content); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
		for i, holiday := range content.Holidays {
			where := fmt.Sprintf("%s entry %d (%s)", filepath.Base(file), i+1, holiday.Name)
			if !isoDate.MatchString(holiday.Date) || !strings.HasPrefix(holiday.Date, strconv.Itoa(content.Year)) {
				return fmt.Errorf("%s: date %q is not a day in %d", where, holiday.Date, content.Year)
			}
			if !holidayTypes[holiday.Type] {
				return fmt.Errorf("%s: unknown type %q", where, holiday.Type)
			}
			if holiday.Name == "" || holiday.Region == "" || len(holiday.States) == 0 {
				return fmt.Errorf("%s: name, region and states are required", where)
			}
			for _, code := range holiday.States {
				if _, ok := india.Lookup(code); !ok && code != "*" {
					return fmt.Errorf("%s: unknown state code %q", where, code)
				}
			}
			// Stored as ",TN,PY," so a state can be matched with LIKE '%,TN,%'.
			states := "," + strings.Join(holiday.States, ",") + ","
			if _, err := insert.Exec(content.Year, holiday.Date, holiday.Name, holiday.Type, holiday.Region, states, nullable(holiday.Notes)); err != nil {
				return err
			}
			stats.Holidays++
		}
	}
	return nil
}

// ---- helpers ----

// readGzipCSV returns the header plus every row as wide as the header, and how many
// rows it dropped for having a different width.
func readGzipCSV(path string) ([][]string, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	defer unzipped.Close()

	reader := csv.NewReader(unzipped)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	var records [][]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		records = append(records, record)
	}
	if len(records) < 2 {
		return nil, 0, fmt.Errorf("%s: no data rows", filepath.Base(path))
	}
	width := len(records[0])
	kept := records[:1]
	for _, record := range records[1:] {
		if len(record) == width {
			kept = append(kept, record)
		}
	}
	return kept, len(records) - len(kept), nil
}

// Some exported CSV files start with a UTF-8 byte order mark glued to the first header.
var byteOrderMark = string([]byte{0xEF, 0xBB, 0xBF})

// columns maps the wanted header names (lower case) to their positions.
func columns(header []string, wanted ...string) (map[string]int, error) {
	positions := map[string]int{}
	for i, name := range header {
		positions[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, byteOrderMark)))] = i
	}
	result := map[string]int{}
	for _, name := range wanted {
		position, ok := positions[name]
		if !ok {
			return nil, fmt.Errorf("column %q not found in header %v", name, header)
		}
		result[name] = position
	}
	return result, nil
}

func nullable(value string) any {
	if value == "" || strings.EqualFold(value, "NA") {
		return nil
	}
	return value
}

// titleCase turns "TAMIL NADU" into "Tamil Nadu". Source files shout most names.
func titleCase(value string) string {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	for i, word := range words {
		runes := []rune(word)
		upperNext := true
		for j, r := range runes {
			if upperNext && r >= 'a' && r <= 'z' {
				runes[j] = r - 32
			}
			upperNext = r == '-' || r == '(' || r == '.' || r == '/'
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func sumValues(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func topKeys(counts map[string]int, limit int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(a, b int) bool { return counts[keys[a]] > counts[keys[b]] })
	if len(keys) > limit {
		keys = keys[:limit]
	}
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = fmt.Sprintf("%q x%d", key, counts[key])
	}
	return strings.Join(parts, ", ")
}
