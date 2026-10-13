package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// Holiday data has no single source. Two parts can be fetched:
//
//	apidata import bank-holidays -year 2026     Reserve Bank of India holiday matrix
//	apidata import market-holidays -year 2026   National Stock Exchange trading holidays
//
// Each replaces only its own kind of entry in data/holidays/<year>.json. Central
// government holidays are typed into that file by hand from the yearly memorandum.

const browserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

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

func importBankHolidays(dataDir string, year int) (int, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: time.Minute}
	page, err := fetchPage(client, http.MethodGet, rbiHolidayURL, nil)
	if err != nil {
		return 0, err
	}

	var holidays []dataset.Holiday
	for month := 1; month <= 12; month++ {
		form := url.Values{"drRegionalOffice": {"0"}, "drMonth": {strconv.Itoa(month)}, "drYear": {strconv.Itoa(year)}, "btnGo": {"GO"}}
		for _, field := range hiddenField.FindAllStringSubmatch(page, -1) {
			form.Set(field[1], html.UnescapeString(field[2]))
		}
		if page, err = fetchPage(client, http.MethodPost, rbiHolidayURL, form); err != nil {
			return 0, err
		}
		monthly, err := parseRBIMonth(page, year, month)
		if err != nil {
			return 0, fmt.Errorf("%d-%02d: %w", year, month, err)
		}
		holidays = append(holidays, monthly...)
		time.Sleep(1500 * time.Millisecond) // one polite request per month
	}
	if len(holidays) < 100 {
		return 0, fmt.Errorf("found only %d bank holidays for %d: the page layout has probably changed", len(holidays), year)
	}
	return len(holidays), replaceHolidays(dataDir, year, holidays, "bank")
}

// parseRBIMonth reads one month of the matrix: a grid of offices against dates, and a
// separate list naming what each date is.
func parseRBIMonth(page string, year, month int) ([]dataset.Holiday, error) {
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

	var holidays []dataset.Holiday
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
			holidays = append(holidays, dataset.Holiday{
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

func importMarketHolidays(dataDir string, year int) (int, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: time.Minute}
	// The exchange's API only answers a session that has loaded the page first.
	if _, err := fetchPage(client, http.MethodGet, "https://www.nseindia.com/resources/exchange-communication-holidays", nil); err != nil {
		return 0, err
	}
	raw, err := fetchPage(client, http.MethodGet, "https://www.nseindia.com/api/holiday-master?type=trading", nil)
	if err != nil {
		return 0, err
	}
	var segments map[string][]struct {
		Date        string `json:"tradingDate"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(raw), &segments); err != nil {
		return 0, fmt.Errorf("unexpected response from the exchange: %w", err)
	}

	var holidays []dataset.Holiday
	for _, entry := range segments["CM"] { // CM is the equities (capital market) segment
		date, err := time.Parse("02-Jan-2006", entry.Date)
		if err != nil {
			return 0, fmt.Errorf("unexpected date %q", entry.Date)
		}
		if date.Year() != year {
			continue
		}
		holiday := dataset.Holiday{
			Date:   date.Format("2006-01-02"),
			Name:   strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(entry.Description), "*")),
			Type:   "market",
			Region: "NSE equities",
			States: []string{"*"},
		}
		if strings.HasSuffix(strings.TrimSpace(entry.Description), "*") {
			holiday.Notes = "A special Muhurat trading session is held on this day; the exchange announces its timings separately."
		}
		holidays = append(holidays, holiday)
	}
	if len(holidays) < 8 {
		return 0, fmt.Errorf("the exchange lists only %d trading holidays for %d: it may not have published that year yet", len(holidays), year)
	}
	return len(holidays), replaceHolidays(dataDir, year, holidays, "market")
}

// replaceHolidays swaps every entry of one type in data/holidays/<year>.json and
// leaves the other types, including the hand-maintained ones, untouched.
func replaceHolidays(dataDir string, year int, fresh []dataset.Holiday, kind string) error {
	path := filepath.Join(dataDir, "holidays", fmt.Sprintf("%d.json", year))
	file := dataset.HolidayFile{Year: year}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &file); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	kept := fresh
	for _, holiday := range file.Holidays {
		if holiday.Type != kind {
			kept = append(kept, holiday)
		}
	}
	sort.SliceStable(kept, func(a, b int) bool {
		if kept[a].Date != kept[b].Date {
			return kept[a].Date < kept[b].Date
		}
		if kept[a].Type != kept[b].Type {
			return kept[a].Type < kept[b].Type
		}
		return kept[a].Region < kept[b].Region
	})
	file.Holidays = kept

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeHolidayFile(path, file)
}

// writeHolidayFile keeps one holiday per line, so a yearly refresh is a readable diff.
func writeHolidayFile(path string, file dataset.HolidayFile) error {
	var out strings.Builder
	fmt.Fprintf(&out, "{\n  \"year\": %d,\n  \"holidays\": [\n", file.Year)
	for i, holiday := range file.Holidays {
		line, err := json.Marshal(holiday)
		if err != nil {
			return err
		}
		out.WriteString("    " + string(line))
		if i < len(file.Holidays)-1 {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("  ]\n}\n")
	return os.WriteFile(path, []byte(out.String()), 0o644)
}

func fetchPage(client *http.Client, method, address string, form url.Values) (string, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := http.NewRequest(method, address, body)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", browserAgent)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("Referer", address)
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s %s: %s", method, address, response.Status)
	}
	raw, err := io.ReadAll(response.Body)
	return string(raw), err
}
