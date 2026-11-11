package api

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

type holidayView struct {
	Date    string   `json:"date"`
	Weekday string   `json:"weekday"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Region  string   `json:"region"`
	States  []string `json:"states"`
	Notes   *string  `json:"notes,omitempty"`
}

// holidayTypes maps each entry type to the dataset that holds it, in display order.
var holidayTypes = []struct{ kind, dataset string }{
	{"bank", "holidays-bank"},
	{"central_gazetted", "holidays-central"},
	{"central_restricted", "holidays-central"},
	{"market", "holidays-market"},
	{"clearing", "holidays-market"},
}

func holidayTypeNames() []string {
	names := make([]string, len(holidayTypes))
	for i, entry := range holidayTypes {
		names[i] = entry.kind
	}
	return names
}

var holidayDatasets = []string{"holidays-bank", "holidays-central", "holidays-market"}

// holidayListNames says what each holiday dataset is, for notes about missing years.
var holidayListNames = map[string]string{
	"holidays-bank":    "bank",
	"holidays-central": "central government",
	"holidays-market":  "NSE market",
}

// holidayYears lists the years any holiday dataset covers. Builders store a count per
// year, so this reads no table.
func (s *Server) holidayYears(datasets ...string) []int {
	if len(datasets) == 0 {
		datasets = holidayDatasets
	}
	seen := map[int]bool{}
	for _, name := range datasets {
		set := s.data(name)
		if set == nil {
			continue
		}
		for key := range set.meta.Counts {
			if year, err := strconv.Atoi(strings.TrimPrefix(key, "year:")); err == nil && strings.HasPrefix(key, "year:") {
				seen[year] = true
			}
		}
	}
	years := make([]int, 0, len(seen))
	for year := range seen {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

func hasYear(years []int, year int) bool {
	for _, known := range years {
		if known == year {
			return true
		}
	}
	return false
}

// holidaySource credits the datasets a holiday answer drew on: the one source when it
// drew on one, otherwise the three together.
func (s *Server) holidaySource(used []string) *sourceRef {
	if len(used) == 1 {
		return s.source(used[0])
	}
	combined := &sourceRef{Name: "Indian holiday lists", URL: "https://github.com/dharunashokkumar/api.dharun.dev#data-sources", Licence: "Official publications, compiled into one list"}
	var credits []string
	for _, name := range used {
		source, ok := s.sourceOf(name)
		if !ok {
			continue
		}
		credits = append(credits, source.Attribution)
		if dataset.Newer(source.Retrieved, combined.Retrieved) {
			combined.Retrieved = source.Retrieved
		}
	}
	combined.Attribution = strings.Join(credits, " ")
	return combined
}

func (s *Server) handleHolidays(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	// /v1/holidays/2027.ics is the same list as a calendar to subscribe to.
	raw, calendar := strings.CutSuffix(r.PathValue("year"), ".ics")
	year, err := strconv.Atoi(raw)
	if err != nil || year < 1900 || year > 2200 {
		s.badRequest(w, r, "The year must be four digits, for example /v1/holidays/2026.")
		return
	}
	info(r).subject = strconv.Itoa(year)

	kind := strings.ToLower(strings.TrimSpace(query.Get("type")))
	wanted := map[string]bool{} // datasets to read
	if kind == "" {
		for _, name := range holidayDatasets {
			wanted[name] = true
		}
	} else {
		for _, entry := range holidayTypes {
			if entry.kind == kind {
				wanted[entry.dataset] = true
			}
		}
		if len(wanted) == 0 {
			s.badRequest(w, r, "type must be one of: "+strings.Join(holidayTypeNames(), ", ")+".")
			return
		}
	}
	stateCode, stateOK := stateParam(r)
	if !stateOK {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return
	}
	month := 0
	if query.Get("month") != "" {
		if month, err = intParam(r, "month", 0, 1, 12); err != nil {
			s.badRequest(w, r, err.Error())
			return
		}
	}

	// Each list answers only for the years it covers: an empty answer from a list that
	// has no data for the year would read as "no holidays".
	var asked, covered, missing []string
	for _, name := range holidayDatasets {
		if wanted[name] {
			asked = append(asked, name)
			if hasYear(s.holidayYears(name), year) {
				covered = append(covered, name)
			} else {
				missing = append(missing, holidayListNames[name])
			}
		}
	}
	if len(covered) == 0 {
		years := s.holidayYears(asked...)
		if len(years) == 0 {
			s.need(w, r, asked[0])
			return
		}
		s.notFound(w, r, fmt.Sprintf("No %s holiday list for %d. Years available: %s.", joinWords(missing), year, joinInts(years)))
		return
	}

	where := []string{`year = ?`}
	args := []any{year}
	if kind != "" {
		where = append(where, `type = ?`)
		args = append(args, kind)
	}
	if stateCode != "" {
		where = append(where, `(states = ',*,' OR states LIKE ?)`)
		args = append(args, "%,"+stateCode+",%")
	}
	if month != 0 {
		where = append(where, `substr(date, 6, 2) = ?`)
		args = append(args, fmt.Sprintf("%02d", month))
	}
	statement := `SELECT date, name, type, region, states, notes FROM holidays WHERE ` + strings.Join(where, " AND ")

	holidays := []holidayView{}
	var used []string
	for _, name := range covered {
		set := s.data(name)
		if set == nil {
			continue
		}
		used = append(used, name)
		rows, err := set.rows(statement, args...)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		for rows.Next() {
			var view holidayView
			var states string
			var notes sql.NullString
			if err := rows.Scan(&view.Date, &view.Name, &view.Type, &view.Region, &states, &notes); err != nil {
				rows.Close()
				s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
				return
			}
			view.States = strings.Split(strings.Trim(states, ","), ",")
			view.Weekday = weekday(view.Date)
			view.Notes = str(notes)
			holidays = append(holidays, view)
		}
		rows.Close()
		if rows.Err() != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
	}
	sort.SliceStable(holidays, func(a, b int) bool {
		if holidays[a].Date != holidays[b].Date {
			return holidays[a].Date < holidays[b].Date
		}
		if holidays[a].Type != holidays[b].Type {
			return holidays[a].Type < holidays[b].Type
		}
		return holidays[a].Region < holidays[b].Region
	})

	total := len(holidays)
	note := "Compiled from official notifications. Dates of festivals fixed by moon sighting can change; check the issuing authority before relying on a date."
	if len(missing) > 0 {
		note = fmt.Sprintf("The %s holiday list for %d is not out yet, so those holidays are not included. ", joinWords(missing), year) + note
	}
	if calendar {
		s.sendCalendar(w, r, year, kind, stateCode, holidays)
		return
	}
	s.ok(w, r, holidays, meta{Total: &total, Source: s.holidaySource(used), Note: note}, cacheAPI)
}

// sendCalendar answers with the holidays as an iCalendar feed (RFC 5545) that Google
// Calendar, Outlook and phones subscribe to and fetch again by themselves.
func (s *Server) sendCalendar(w http.ResponseWriter, r *http.Request, year int, kind, state string, holidays []holidayView) {
	name := "India holidays " + strconv.Itoa(year)
	if kind != "" {
		name = strings.ToUpper(kind[:1]) + kind[1:] + " holidays " + strconv.Itoa(year)
	}
	if state != "" {
		name += " (" + state + ")"
	}
	events := make([]calendarEvent, 0, len(holidays))
	for _, h := range holidays {
		events = append(events, calendarEvent{Date: h.Date, Key: h.Type + "|" + h.Region, Name: h.Name,
			Description: h.Type + " holiday, " + h.Region + ". From " + s.origin(r) + "/v1/holidays/" + strconv.Itoa(year)})
	}
	writeCalendar(w, r, name, "Holidays", events)
}

// calendarEvent is one all-day event of a calendar feed.
type calendarEvent struct {
	Date, Key, Name, Description string
}

// writeCalendar answers with an iCalendar feed (RFC 5545) of all-day events.
func writeCalendar(w http.ResponseWriter, r *http.Request, name, product string, events []calendarEvent) {
	var out strings.Builder
	line := func(text string) {
		// Lines are folded at 75 octets, continuation lines start with a space.
		for len(text) > 75 {
			cut := 75
			for cut > 0 && !utf8.RuneStart(text[cut]) {
				cut--
			}
			out.WriteString(text[:cut] + "\r\n")
			text = " " + text[cut:]
		}
		out.WriteString(text + "\r\n")
	}
	escape := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`)
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//India API//" + product + "//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:" + escape.Replace(name))
	line("X-WR-TIMEZONE:Asia/Kolkata")
	line("REFRESH-INTERVAL;VALUE=DURATION:P1D")
	line("X-PUBLISHED-TTL:P1D")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for _, e := range events {
		day, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			continue
		}
		sum := sha1.Sum([]byte(e.Date + "|" + e.Key + "|" + e.Name))
		line("BEGIN:VEVENT")
		line("UID:" + hex.EncodeToString(sum[:8]) + "@api.dharun.dev")
		line("DTSTAMP:" + stamp)
		line("DTSTART;VALUE=DATE:" + day.Format("20060102"))
		line("DTEND;VALUE=DATE:" + day.AddDate(0, 0, 1).Format("20060102"))
		line("SUMMARY:" + escape.Replace(e.Name))
		line("DESCRIPTION:" + escape.Replace(e.Description))
		line("TRANSP:TRANSPARENT")
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	writeBody(w, r, http.StatusOK, []byte(out.String()), "text/calendar; charset=utf-8", cacheAPI, true)
}

// joinWords joins names as prose: "a", "a and b", "a, b and c".
func joinWords(words []string) string {
	if len(words) <= 1 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

func joinInts(values []int) string {
	if len(values) == 0 {
		return "none yet"
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, ", ")
}
