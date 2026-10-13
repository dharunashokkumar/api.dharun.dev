package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// Holiday lists come from three publishers with three rhythms, so they are three
// datasets with the same table:
//
//	holidays-bank     Reserve Bank of India holiday matrix    fetched by a program
//	holidays-market   National Stock Exchange lists           fetched by a program
//	holidays-central  Department of Personnel and Training    typed in by hand each year
//
// Each keeps one file per year under data/holidays/<kind>/<year>.json.

// Holiday is one entry of a holiday file.
type Holiday struct {
	Date string `json:"date"`
	Name string `json:"name"`
	// Type is one of: bank, market, clearing, central_gazetted, central_restricted, state.
	Type string `json:"type"`
	// Region names who observes it: "India", an RBI office, an exchange or a state.
	Region string `json:"region"`
	// States lists the state codes covered; ["*"] means the whole country.
	States []string `json:"states"`
	Notes  string   `json:"notes,omitempty"`
}

// HolidayFile is the shape of data/holidays/<kind>/<year>.json.
type HolidayFile struct {
	Year     int       `json:"year"`
	Holidays []Holiday `json:"holidays"`
}

// HolidayDatasets lists the holiday datasets and the entry types each may hold.
var HolidayDatasets = map[string][]string{
	"holidays-bank":    {"bank"},
	"holidays-market":  {"market", "clearing"},
	"holidays-central": {"central_gazetted", "central_restricted"},
}

const holidayDDL = `
CREATE TABLE holidays (
  year INTEGER NOT NULL, date TEXT NOT NULL, name TEXT NOT NULL, type TEXT NOT NULL,
  region TEXT NOT NULL, states TEXT NOT NULL, notes TEXT
);
CREATE INDEX holidays_year ON holidays(year, type, date);`

func init() {
	register(&Spec{
		Name: "holidays-bank", Schema: 1, DDL: holidayDDL,
		Build:  func(b *Builder) error { return buildHolidays(b, "bank") },
		Import: importBankHolidays,
		Every:  30 * 24 * time.Hour,
		Main:   "holidays", MaxShrink: 0.2,
	})
	register(&Spec{
		Name: "holidays-market", Schema: 1, DDL: holidayDDL,
		Build:  func(b *Builder) error { return buildHolidays(b, "market") },
		Import: importMarketHolidays,
		Every:  30 * 24 * time.Hour,
		Main:   "holidays", MaxShrink: 0.2,
	})
	register(&Spec{
		Name: "holidays-central", Schema: 1, DDL: holidayDDL,
		Build: func(b *Builder) error { return buildHolidays(b, "central") },
	})
}

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func buildHolidays(b *Builder, kind string) error {
	allowed := map[string]bool{}
	for _, name := range HolidayDatasets["holidays-"+kind] {
		allowed[name] = true
	}
	dir := b.Path("holidays", kind)
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no files in %s: %w", dir, os.ErrNotExist)
	}
	sort.Strings(files)

	insert, err := b.Tx.Prepare(`INSERT INTO holidays VALUES (?,?,?,?,?,?,?)`)
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
			where := fmt.Sprintf("%s/%s entry %d (%s)", kind, filepath.Base(file), i+1, holiday.Name)
			if !isoDate.MatchString(holiday.Date) || !strings.HasPrefix(holiday.Date, strconv.Itoa(content.Year)) {
				return fmt.Errorf("%s: date %q is not a day in %d", where, holiday.Date, content.Year)
			}
			if _, err := time.Parse("2006-01-02", holiday.Date); err != nil {
				return fmt.Errorf("%s: date %q does not exist", where, holiday.Date)
			}
			if !allowed[holiday.Type] {
				return fmt.Errorf("%s: type %q does not belong in the %s list", where, holiday.Type, kind)
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
			b.Add("holidays", 1)
			b.Add("year:"+strconv.Itoa(content.Year), 1)
		}
	}
	return nil
}

// writeHolidayYear saves one year of one kind, one holiday per line so that a refresh
// is a readable diff.
func writeHolidayYear(im *Importer, kind string, year int, holidays []Holiday) error {
	sort.SliceStable(holidays, func(a, b int) bool {
		if holidays[a].Date != holidays[b].Date {
			return holidays[a].Date < holidays[b].Date
		}
		if holidays[a].Type != holidays[b].Type {
			return holidays[a].Type < holidays[b].Type
		}
		return holidays[a].Region < holidays[b].Region
	})
	var out strings.Builder
	fmt.Fprintf(&out, "{\n  \"year\": %d,\n  \"holidays\": [\n", year)
	for i, holiday := range holidays {
		line, err := json.Marshal(holiday)
		if err != nil {
			return err
		}
		out.WriteString("    " + string(line))
		if i < len(holidays)-1 {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("  ]\n}\n")
	return im.Write(fmt.Sprintf("holidays/%s/%d.json", kind, year), []byte(out.String()))
}

// carryForward writes out the years held by the copy in use that this import did not
// fetch. A source only publishes the current year; without this, a server's refreshed
// copy would forget every earlier one.
func carryForward(im *Importer, kind string, fetched map[int]bool) error {
	if im.Current == nil {
		return nil
	}
	rows, err := im.Current.Query(`SELECT year, date, name, type, region, states, COALESCE(notes, '') FROM holidays ORDER BY year, date`)
	if err != nil {
		return err
	}
	defer rows.Close()
	years := map[int][]Holiday{}
	for rows.Next() {
		var year int
		var states string
		var holiday Holiday
		if err := rows.Scan(&year, &holiday.Date, &holiday.Name, &holiday.Type, &holiday.Region, &states, &holiday.Notes); err != nil {
			return err
		}
		holiday.States = strings.Split(strings.Trim(states, ","), ",")
		years[year] = append(years[year], holiday)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for year, holidays := range years {
		if fetched[year] {
			continue
		}
		if err := writeHolidayYear(im, kind, year, holidays); err != nil {
			return err
		}
	}
	return nil
}

// ---- Reserve Bank of India ----

// rbiOfficeStates maps each Reserve Bank regional office to the states it certainly
// covers. Union territories without an office of their own are left out rather than guessed.
var rbiOfficeStates = map[string][]string{
	"Agartala": {"TR"}, "Ahmedabad": {"GJ"}, "Aizawl": {"MZ"}, "Belapur": {"MH"}, "Bengaluru": {"KA"},
	"Bhopal": {"MP"}, "Bhubaneswar": {"OD"}, "Chandigarh": {"CH", "PB", "HR"}, "Chennai": {"TN", "PY"},
	"Dehradun": {"UK"}, "Gangtok": {"SK"}, "Guwahati": {"AS"}, "Hyderabad": {"TS"}, "Imphal": {"MN"},
	"Itanagar": {"AR"}, "Jaipur": {"RJ"}, "Jammu": {"JK"}, "Kanpur": {"UP"}, "Kochi": {"KL"},
	"Kohima": {"NL"}, "Kolkata": {"WB"}, "Lucknow": {"UP"}, "Mumbai": {"MH"}, "Nagpur": {"MH"},
	"New Delhi": {"DL"}, "Panaji": {"GA"}, "Patna": {"BR"}, "Raipur": {"CG"}, "Ranchi": {"JH"},
	"Shillong": {"ML"}, "Shimla": {"HP"}, "Srinagar": {"JK"}, "Thiruvananthapuram": {"KL"}, "Vijayawada": {"AP"},
}

const rbiHolidayURL = "https://www.rbi.org.in/Scripts/HolidayMatrixDisplay.aspx"

var (
	rbiMatrix      = regexp.MustCompile(`(?s)<table[^>]*class="tablebgforholiday"[^>]*>(.*?)</table>`)
	rbiHeaderCell  = regexp.MustCompile(`(?s)<span class="dop_header">\s*(.*?)\s*</span>`)
	rbiRow         = regexp.MustCompile(`(?s)<tr>\s*<td\s*>([^<]+)</td>(.*?)</tr>`)
	rbiCell        = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	rbiDescription = regexp.MustCompile(`(?s)<tr>\s*<td>([^<]+)</td>\s*<td>\s*(\d{1,2})\s*</td>\s*</tr>`)
	hiddenField    = regexp.MustCompile(`name="(__[A-Z]+)"[^>]*value="([^"]*)"`)
)

// importYears is which years a holiday import fetches: the one asked for, or the
// current year plus the next once the year is nearly over. The second value says
// whether a year that the source has not published yet is an error.
func importYears(im *Importer) (years []int, strict bool) {
	if im.Year != 0 {
		return []int{im.Year}, true
	}
	now := im.Now
	if now.IsZero() {
		now = time.Now()
	}
	years = []int{now.Year()}
	if now.Month() >= time.November {
		years = append(years, now.Year()+1)
	}
	return years, false
}

func importBankHolidays(im *Importer) (Source, error) {
	years, strict := importYears(im)
	fetched := map[int]bool{}
	total := 0
	for i, year := range years {
		holidays, err := fetchBankHolidays(im, year)
		if err != nil {
			// Next year's matrix appears some time in December; until then it is simply absent.
			if !strict && i > 0 {
				im.Logf("bank holidays for %d are not published yet (%v)", year, err)
				continue
			}
			return Source{}, err
		}
		if err := writeHolidayYear(im, "bank", year, holidays); err != nil {
			return Source{}, err
		}
		fetched[year] = true
		total += len(holidays)
	}
	if err := carryForward(im, "bank", fetched); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "Bank holidays in India",
		Publisher:   "Reserve Bank of India",
		URL:         rbiHolidayURL,
		Licence:     "Official publication",
		Version:     yearList(fetched),
		Attribution: "Bank holidays from the Reserve Bank of India holiday matrix, by regional office.",
		Notes:       "Holidays under the Negotiable Instruments Act and the banks' closing of accounts. An office's holidays are attributed to the states it covers; union territories without an office are not guessed.",
	}, nil
}

func fetchBankHolidays(im *Importer, year int) ([]Holiday, error) {
	raw, err := im.Browse(rbiHolidayURL)
	if err != nil {
		return nil, err
	}
	page := string(raw)
	var holidays []Holiday
	for month := 1; month <= 12; month++ {
		form := url.Values{"drRegionalOffice": {"0"}, "drMonth": {strconv.Itoa(month)}, "drYear": {strconv.Itoa(year)}, "btnGo": {"GO"}}
		for _, field := range hiddenField.FindAllStringSubmatch(page, -1) {
			form.Set(field[1], html.UnescapeString(field[2]))
		}
		raw, err := im.Post(rbiHolidayURL, form)
		if err != nil {
			return nil, err
		}
		page = string(raw)
		monthly, err := parseRBIMonth(page, year, month)
		if err != nil {
			return nil, fmt.Errorf("%d-%02d: %w", year, month, err)
		}
		holidays = append(holidays, monthly...)
		time.Sleep(1500 * time.Millisecond) // one polite request per month
	}
	if len(holidays) < 100 {
		return nil, fmt.Errorf("found only %d bank holidays for %d: not published yet, or the page layout changed", len(holidays), year)
	}
	return holidays, nil
}

// parseRBIMonth reads one month of the matrix: a grid of offices against dates, and a
// separate list naming what each date is.
func parseRBIMonth(page string, year, month int) ([]Holiday, error) {
	matrix := rbiMatrix.FindStringSubmatch(page)
	if matrix == nil {
		return nil, errors.New("holiday matrix not found")
	}
	header := rbiHeaderCell.FindAllStringSubmatch(matrix[1], -1)
	if len(header) == 0 || !strings.Contains(header[0][1], strconv.Itoa(year)) {
		return nil, errors.New("matrix header is not for the requested year")
	}
	if !strings.HasPrefix(header[0][1], time.Month(month).String()) {
		return nil, fmt.Errorf("matrix is for %q, not %s", header[0][1], time.Month(month))
	}
	if len(header) == 1 {
		return nil, nil // a month with no holidays
	}
	days := make([]int, len(header)-1)
	for i, cell := range header[1:] {
		day, err := strconv.Atoi(cell[1])
		if err != nil {
			return nil, fmt.Errorf("unexpected day %q", cell[1])
		}
		days[i] = day
	}

	names := map[int]string{}
	for _, row := range rbiDescription.FindAllStringSubmatch(page, -1) {
		day, _ := strconv.Atoi(row[2])
		names[day] = strings.TrimSpace(html.UnescapeString(row[1]))
	}

	var holidays []Holiday
	for _, row := range rbiRow.FindAllStringSubmatch(matrix[1], -1) {
		office := strings.TrimSpace(html.UnescapeString(row[1]))
		states, known := rbiOfficeStates[office]
		if !known {
			return nil, fmt.Errorf("unknown RBI office %q: add it to rbiOfficeStates", office)
		}
		cells := rbiCell.FindAllStringSubmatch(row[2], -1)
		if len(cells) != len(days) {
			return nil, fmt.Errorf("%s has %d cells for %d dates", office, len(cells), len(days))
		}
		for i, cell := range cells {
			var note string
			switch {
			case strings.Contains(cell[1], "Negotiable Instruments Act"):
			case strings.Contains(cell[1], "Closing of Accounts"):
				note = "Banks' closing of accounts: branches are closed to the public."
			default:
				continue
			}
			name := names[days[i]]
			if name == "" {
				return nil, fmt.Errorf("no description for day %d", days[i])
			}
			holidays = append(holidays, Holiday{
				Date:   fmt.Sprintf("%04d-%02d-%02d", year, month, days[i]),
				Name:   name,
				Type:   "bank",
				Region: "RBI " + office,
				States: states,
				Notes:  note,
			})
		}
	}
	return holidays, nil
}

// ---- National Stock Exchange ----

const (
	nseHolidayPage = "https://www.nseindia.com/resources/exchange-communication-holidays"
	nseHolidayAPI  = "https://www.nseindia.com/api/holiday-master?type="
)

type nseHoliday struct {
	Date        string `json:"tradingDate"`
	Description string `json:"description"`
}

// The exchange publishes two lists for the equities segment. On a trading holiday the
// market is shut. On a clearing holiday it trades but nothing settles, which moves
// settlement dates.
func importMarketHolidays(im *Importer) (Source, error) {
	// The exchange's API answers more reliably to a session that has loaded the page first.
	if _, err := im.Browse(nseHolidayPage); err != nil {
		im.Logf("could not open the exchange's holiday page first: %v", err)
	}
	byYear := map[int][]Holiday{}
	trading := map[int]int{}
	for _, list := range []struct{ query, kind string }{{"trading", "market"}, {"clearing", "clearing"}} {
		raw, err := im.Browse(nseHolidayAPI + list.query)
		if err != nil {
			return Source{}, err
		}
		holidays, err := parseNSEHolidays(raw, list.kind)
		if err != nil {
			return Source{}, fmt.Errorf("%s holidays: %w", list.query, err)
		}
		for _, holiday := range holidays {
			year, _ := strconv.Atoi(holiday.Date[:4])
			byYear[year] = append(byYear[year], holiday)
			if list.kind == "market" {
				trading[year]++
			}
		}
	}

	fetched := map[int]bool{}
	for year, holidays := range byYear {
		if im.Year != 0 && year != im.Year {
			continue
		}
		// A real year has well over eight trading holidays; fewer is a list still being filled in.
		if trading[year] < 8 {
			continue
		}
		if err := writeHolidayYear(im, "market", year, holidays); err != nil {
			return Source{}, err
		}
		fetched[year] = true
	}
	if len(fetched) == 0 {
		return Source{}, errors.New("the exchange does not list a full year of trading holidays for the year asked for")
	}
	if err := carryForward(im, "market", fetched); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "Trading and clearing holidays, NSE equities",
		Publisher:   "National Stock Exchange of India",
		URL:         nseHolidayPage,
		Licence:     "Official publication",
		Version:     yearList(fetched),
		Attribution: "Trading and clearing holidays from the National Stock Exchange of India, capital market segment.",
	}, nil
}

func parseNSEHolidays(raw []byte, kind string) ([]Holiday, error) {
	var segments map[string][]nseHoliday
	if err := json.Unmarshal(raw, &segments); err != nil {
		return nil, fmt.Errorf("unexpected response from the exchange: %w", err)
	}
	entries, ok := segments["CM"] // CM is the equities (capital market) segment
	if !ok {
		return nil, errors.New("the response has no capital market segment")
	}
	var holidays []Holiday
	for _, entry := range entries {
		date, err := time.Parse("02-Jan-2006", strings.TrimSpace(entry.Date))
		if err != nil {
			return nil, fmt.Errorf("unexpected date %q", entry.Date)
		}
		description := strings.TrimSpace(entry.Description)
		holiday := Holiday{
			Date:   date.Format("2006-01-02"),
			Name:   strings.TrimSpace(strings.TrimSuffix(description, "*")),
			Type:   kind,
			Region: "NSE equities",
			States: []string{"*"},
		}
		if kind == "market" && strings.HasSuffix(description, "*") {
			holiday.Notes = "A special Muhurat trading session is held on this day; the exchange announces its timings separately."
		}
		holidays = append(holidays, holiday)
	}
	return holidays, nil
}

func yearList(years map[int]bool) string {
	sorted := make([]int, 0, len(years))
	for year := range years {
		sorted = append(sorted, year)
	}
	sort.Ints(sorted)
	parts := make([]string, len(sorted))
	for i, year := range sorted {
		parts[i] = strconv.Itoa(year)
	}
	return strings.Join(parts, ", ")
}
