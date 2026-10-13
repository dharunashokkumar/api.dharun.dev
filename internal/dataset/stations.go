package dataset

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// Indian Railways station codes from Wikipedia's "List of railway stations in India"
// (CC BY-SA 4.0), chosen by Dharun on 2026-10-13 as no official open list exists. The
// article is hand-edited, so the import cleans it: names out of links and templates,
// states to codes, zones to one spelling, repeated codes merged.

// Station is one station as kept in data/stations/stations.json.
type Station struct {
	Code       string   `json:"code"`
	Name       string   `json:"name"`
	State      string   `json:"state,omitempty"`
	Zone       string   `json:"zone,omitempty"`
	ElevationM *float64 `json:"elevationM,omitempty"`
	OtherNames []string `json:"otherNames,omitempty"`
}

// Zone is one Indian Railways zone, or the Konkan Railway, which is run apart from them.
type Zone struct{ Code, Name string }

// Zones lists the zones by the code Indian Railways uses for them.
var Zones = []Zone{
	{"CR", "Central"}, {"ER", "Eastern"}, {"ECR", "East Central"}, {"ECoR", "East Coast"},
	{"NR", "Northern"}, {"NCR", "North Central"}, {"NER", "North Eastern"}, {"NFR", "Northeast Frontier"},
	{"NWR", "North Western"}, {"SR", "Southern"}, {"SCR", "South Central"}, {"SER", "South Eastern"},
	{"SECR", "South East Central"}, {"SWR", "South Western"}, {"WR", "Western"}, {"WCR", "West Central"},
	{"SCoR", "South Coast"}, {"MR", "Metro Railway Kolkata"}, {"KR", "Konkan Railway"},
}

func init() {
	register(&Spec{
		Name:   "stations",
		Schema: 1,
		DDL: `
CREATE TABLE stations (
  code TEXT PRIMARY KEY, name TEXT NOT NULL, state_code TEXT, zone TEXT, elevation_m REAL,
  other_names TEXT, search TEXT NOT NULL
) WITHOUT ROWID;
CREATE INDEX stations_state ON stations(state_code, zone);`,
		Build:  buildStations,
		Import: importStations,
		Every:  30 * 24 * time.Hour,
		Main:   "stations", MaxShrink: 0.03,
		Check: checkStations,
	})
}

func buildStations(b *Builder) error {
	raw, err := os.ReadFile(b.Path("stations", "stations.json"))
	if err != nil {
		return err
	}
	var stations []Station
	if err := json.Unmarshal(raw, &stations); err != nil {
		return fmt.Errorf("stations.json: %w", err)
	}
	insert, err := b.Tx.Prepare(`INSERT INTO stations VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, station := range stations {
		var others any
		if len(station.OtherNames) > 0 {
			encoded, _ := json.Marshal(station.OtherNames)
			others = string(encoded)
		}
		search := strings.ToLower(strings.Join(append([]string{station.Name}, station.OtherNames...), " "))
		if _, err := insert.Exec(station.Code, station.Name, nullable(station.State), nullable(station.Zone), station.ElevationM, others, search); err != nil {
			return fmt.Errorf("%s: %w", station.Code, err)
		}
		b.Add("stations", 1)
	}
	return nil
}

// checkStations holds back a refresh that renamed or dropped many stations at once.
func checkStations(current, fresh *sql.DB) error {
	load := func(db *sql.DB) (map[string]string, error) {
		rows, err := db.Query(`SELECT code, name FROM stations`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		names := map[string]string{}
		for rows.Next() {
			var code, name string
			if err := rows.Scan(&code, &name); err != nil {
				return nil, err
			}
			names[code] = name
		}
		return names, rows.Err()
	}
	before, err := load(current)
	if err != nil {
		return err
	}
	after, err := load(fresh)
	if err != nil {
		return err
	}
	changed := 0
	for code, name := range before {
		if renamed, ok := after[code]; !ok || renamed != name {
			changed++
		}
	}
	if len(before) > 0 && changed*10 > len(before) {
		return fmt.Errorf("%d of %d stations were renamed or removed since the copy in use", changed, len(before))
	}
	return nil
}

const stationsPage = "List_of_railway_stations_in_India"

var (
	stationLink   = regexp.MustCompile(`\{\{\s*stnlnk\s*\|([^}|]+)[^}]*\}\}`)
	stationCode   = regexp.MustCompile(`^[A-Z]{1,5}$`)
	elevation     = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*m\b`)
	codeWiseTitle = regexp.MustCompile(`(?m)^==\s*List of stations \(Code Wise\)\s*==`)
)

// zoneOf reads the many ways the article names a zone ("SR/Southern", "Southern
// Railway", "[[Konkan Railway|KRCL]]", "Harbour (CR)").
func zoneOf(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	upper := strings.ToUpper(text)
	if strings.Contains(upper, "KONKAN") || strings.Contains(upper, "KRCL") {
		return "KR"
	}
	if strings.Contains(upper, "METRO") {
		return "MR"
	}
	// An abbreviation before a slash or in brackets wins: "WR/Western/Harbour (CR)" is WR.
	for _, part := range regexp.MustCompile(`[A-Za-z]+`).FindAllString(text, -1) {
		for _, zone := range Zones {
			if part == zone.Code || strings.EqualFold(part, zone.Code) && len(part) > 2 {
				return zone.Code
			}
		}
	}
	// Otherwise the longest zone name the text contains: "South East Central" before "Central".
	best := ""
	for _, zone := range Zones {
		if strings.Contains(strings.ToLower(text), strings.ToLower(zone.Name)) && len(zone.Name) > len(zoneName(best)) {
			best = zone.Code
		}
	}
	return best
}

func zoneName(code string) string {
	for _, zone := range Zones {
		if zone.Code == code {
			return zone.Name
		}
	}
	return ""
}

func stationState(text string) string {
	text = strings.TrimSpace(strings.Split(text, "(")[0])
	if state, ok := india.Lookup(text); ok {
		return state.Code
	}
	return ""
}

// parseStations reads the article's tables. The main tables have one station per line,
// cells separated by ||; a short table at the end puts each cell on its own line.
func parseStations(wikitext string) ([]Station, error) {
	wikitext = wikiComment.ReplaceAllString(wikitext, "")
	wikitext = stationLink.ReplaceAllString(wikitext, "$1")
	main, codeWise := wikitext, ""
	if at := codeWiseTitle.FindStringIndex(wikitext); at != nil {
		main, codeWise = wikitext[:at[0]], wikitext[at[0]:]
	}

	byCode := map[string]*Station{}
	var order []string
	add := func(station Station) {
		existing, ok := byCode[station.Code]
		if !ok {
			copied := station
			byCode[station.Code] = &copied
			order = append(order, station.Code)
			return
		}
		// A code listed twice: keep the first name, note the other, fill gaps.
		if station.Name != existing.Name && !contains(existing.OtherNames, station.Name) {
			existing.OtherNames = append(existing.OtherNames, station.Name)
		}
		if existing.State == "" {
			existing.State = station.State
		}
		if existing.Zone == "" {
			existing.Zone = station.Zone
		}
		if existing.ElevationM == nil {
			existing.ElevationM = station.ElevationM
		}
	}
	for _, line := range strings.Split(main, "\n") {
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|-") || strings.HasPrefix(line, "|}") || !strings.Contains(line, "||") {
			continue
		}
		cells := strings.Split(line[1:], "||")
		if len(cells) < 2 {
			continue
		}
		cell := func(i int) string {
			if i < len(cells) {
				return cleanWiki(cells[i])
			}
			return ""
		}
		name := cell(0)
		var height *float64
		if match := elevation.FindStringSubmatch(strings.ReplaceAll(cell(4), "&nbsp;", " ")); match != nil {
			if value, err := strconv.ParseFloat(match[1], 64); err == nil {
				height = &value
			}
		}
		// A cell may carry two codes for one station ("VS/VSI").
		for _, code := range strings.Split(strings.ToUpper(cell(1)), "/") {
			code = strings.TrimSpace(code)
			if !stationCode.MatchString(code) || name == "" {
				continue
			}
			add(Station{Code: code, Name: name, State: stationState(cell(2)), Zone: zoneOf(cells[min(3, len(cells)-1)]), ElevationM: height})
		}
	}
	// The code-wise table: code, name, and (mostly empty) further cells, one per line.
	var row []string
	flush := func() {
		if len(row) >= 2 {
			code, name := strings.ToUpper(cleanWiki(row[0])), cleanWiki(row[1])
			if stationCode.MatchString(code) && name != "" {
				state := ""
				if len(row) > 2 {
					state = stationState(cleanWiki(row[2]))
				}
				add(Station{Code: code, Name: name, State: state})
			}
		}
		row = nil
	}
	for _, line := range strings.Split(codeWise, "\n") {
		switch {
		case strings.HasPrefix(line, "|-"), strings.HasPrefix(line, "|}"):
			flush()
		case strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "|+"):
			row = append(row, line[1:])
		}
	}
	flush()

	stations := make([]Station, 0, len(order))
	for _, code := range order {
		stations = append(stations, *byCode[code])
	}
	sort.Slice(stations, func(a, b int) bool { return stations[a].Code < stations[b].Code })
	return stations, nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func importStations(im *Importer) (Source, error) {
	wikitext, revision, err := im.wikipedia(stationsPage)
	if err != nil {
		return Source{}, err
	}
	if im.CurrentVersion() == "revision "+revision {
		return Source{}, ErrNotChanged
	}
	stations, err := parseStations(string(wikitext))
	if err != nil {
		return Source{}, err
	}
	if len(stations) < 3000 {
		return Source{}, fmt.Errorf("parsed only %d stations, expected about 4,000: the article has probably changed shape", len(stations))
	}
	encoded, err := json.MarshalIndent(stations, "", " ")
	if err != nil {
		return Source{}, err
	}
	if err := im.Write("stations/stations.json", append(encoded, '\n')); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "List of railway stations in India",
		Publisher:   "Wikipedia contributors",
		URL:         wikipediaURL(stationsPage, revision),
		Licence:     "CC BY-SA 4.0",
		LicenceURL:  "https://creativecommons.org/licenses/by-sa/4.0/",
		Version:     "revision " + revision,
		SHA256:      digest(wikitext),
		Attribution: "Station codes adapted from the Wikipedia article \"List of railway stations in India\", licensed CC BY-SA 4.0. This dataset is shared under the same licence.",
	}, nil
}
