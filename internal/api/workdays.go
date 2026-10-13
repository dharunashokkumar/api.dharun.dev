package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Working days are worked out from the holiday lists the server already holds, plus the
// weekly rules each calendar follows. A calendar only answers for years it has a list for:
// guessing that an unlisted year has no holidays would be a confident wrong answer.

type workCalendar struct {
	ID        string
	Title     string
	dataset   string
	kind      string // holiday type in that dataset
	needState bool
	// offDay reports whether a date is a weekly day off, and names the rule.
	offDay func(day time.Time) string
}

var workCalendars = []workCalendar{
	{
		ID: "bank", Title: "Banks in a state", dataset: "holidays-bank", kind: "bank", needState: true,
		offDay: func(day time.Time) string {
			switch {
			case day.Weekday() == time.Sunday:
				return "Sunday"
			case day.Weekday() == time.Saturday && (day.Day()+6)/7 == 2:
				return "Second Saturday"
			case day.Weekday() == time.Saturday && (day.Day()+6)/7 == 4:
				return "Fourth Saturday"
			}
			return ""
		},
	},
	{ID: "market", Title: "NSE equities trading", dataset: "holidays-market", kind: "market", offDay: weekend},
	{ID: "settlement", Title: "NSE equities settlement (clearing)", dataset: "holidays-market", kind: "clearing", offDay: weekend},
	{ID: "central", Title: "Central government offices", dataset: "holidays-central", kind: "central_gazetted", offDay: weekend},
}

func weekend(day time.Time) string {
	switch day.Weekday() {
	case time.Saturday:
		return "Saturday"
	case time.Sunday:
		return "Sunday"
	}
	return ""
}

func workCalendarIDs() []string {
	ids := make([]string, len(workCalendars))
	for i, calendar := range workCalendars {
		ids[i] = calendar.ID
	}
	return ids
}

// workBook is one calendar's holidays for a span of years, loaded for one request.
type workBook struct {
	calendar workCalendar
	state    string
	holidays map[string][]string // date -> names
	years    map[int]bool        // years the list covers
}

const isoDay = "2006-01-02"

// openWorkBook reads a calendar's holidays for the years from..to. It writes the error
// response itself and returns nil when the request cannot be answered.
func (s *Server) openWorkBook(w http.ResponseWriter, r *http.Request, id string, from, to int) *workBook {
	var calendar *workCalendar
	for i := range workCalendars {
		if workCalendars[i].ID == id {
			calendar = &workCalendars[i]
		}
	}
	if calendar == nil {
		s.badRequest(w, r, "calendar must be one of: "+strings.Join(workCalendarIDs(), ", ")+".")
		return nil
	}
	state, ok := stateParam(r)
	if !ok {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return nil
	}
	if calendar.needState && state == "" {
		s.badRequest(w, r, "The bank calendar needs a state: bank holidays differ from state to state. Add state=TN, for example.")
		return nil
	}
	if !calendar.needState {
		state = ""
	}
	set := s.need(w, r, calendar.dataset)
	if set == nil {
		return nil
	}

	book := &workBook{calendar: *calendar, state: state, holidays: map[string][]string{}, years: map[int]bool{}}
	rows, err := set.rows(`SELECT year, date, name, states FROM holidays WHERE type = ? AND year BETWEEN ? AND ?`, calendar.kind, from, to)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var year int
		var date, name, states string
		if err := rows.Scan(&year, &date, &name, &states); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return nil
		}
		// A year counts as covered for a state only if some entry names that state; a
		// state without its own list (no Reserve Bank office) is never covered.
		if state != "" && states != ",*," && !strings.Contains(states, ","+state+",") {
			continue
		}
		book.years[year] = true
		book.holidays[date] = append(book.holidays[date], name)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return nil
	}
	return book
}

// covers reports whether the book can answer for a date's year.
func (b *workBook) covers(day time.Time) bool { return b.years[day.Year()] }

func (b *workBook) uncovered(day time.Time) string {
	scope := b.calendar.Title
	if b.state != "" {
		scope += " in " + b.state
	}
	return fmt.Sprintf("No holiday list for %s in %d, so working days cannot be worked out for that year.", scope, day.Year())
}

// reasons says why a day is not a working day; nil means it is one.
func (b *workBook) reasons(day time.Time) []string {
	var why []string
	if rule := b.calendar.offDay(day); rule != "" {
		why = append(why, rule)
	}
	return append(why, b.holidays[day.Format(isoDay)]...)
}

// step moves from a day by n working days (n may be negative). It fails when the walk
// leaves the years the book covers.
func (b *workBook) step(day time.Time, n int) (time.Time, bool) {
	direction := 1
	if n < 0 {
		direction, n = -1, -n
	}
	for n > 0 {
		day = day.AddDate(0, 0, direction)
		if !b.covers(day) {
			return day, false
		}
		if len(b.reasons(day)) == 0 {
			n--
		}
	}
	return day, true
}

type workDayView struct {
	Date     string   `json:"date"`
	Weekday  string   `json:"weekday"`
	Calendar string   `json:"calendar"`
	State    string   `json:"state,omitempty"`
	Working  bool     `json:"working"`
	Reasons  []string `json:"reasons"`
	Previous string   `json:"previousWorkingDay"`
	Next     string   `json:"nextWorkingDay"`
	// Shifted is set when add= was given: the day that many working days later.
	Add     *int   `json:"add,omitempty"`
	Shifted string `json:"shifted,omitempty"`
}

// calendarParam is the calendar a request asks for; banks when it names none.
func calendarParam(r *http.Request) string {
	if id := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("calendar"))); id != "" {
		return id
	}
	return "bank"
}

func parseDay(value string) (time.Time, error) {
	day, err := time.Parse(isoDay, strings.TrimSpace(value))
	if err != nil {
		return day, fmt.Errorf("dates are written YYYY-MM-DD, for example 2026-10-13")
	}
	return day, nil
}

func (s *Server) handleWorkingDay(w http.ResponseWriter, r *http.Request) {
	info(r).subject = r.PathValue("date")
	day, err := parseDay(r.PathValue("date"))
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	add, err := intParam(r, "add", 0, -366, 366)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	// add may walk up to a year and a half away; load the neighbouring years too.
	book := s.openWorkBook(w, r, calendarParam(r), day.Year()-2, day.Year()+2)
	if book == nil {
		return
	}
	if !book.covers(day) {
		s.notFound(w, r, book.uncovered(day))
		return
	}
	reasons := book.reasons(day)
	view := workDayView{
		Date: day.Format(isoDay), Weekday: day.Weekday().String(), Calendar: book.calendar.ID, State: book.state,
		Working: len(reasons) == 0, Reasons: append([]string{}, reasons...),
	}
	if previous, ok := book.step(day, -1); ok {
		view.Previous = previous.Format(isoDay)
	}
	if next, ok := book.step(day, 1); ok {
		view.Next = next.Format(isoDay)
	}
	if r.URL.Query().Get("add") != "" {
		shifted, ok := book.step(day, add)
		if !ok {
			s.notFound(w, r, book.uncovered(shifted))
			return
		}
		view.Add, view.Shifted = &add, shifted.Format(isoDay)
	}
	s.ok(w, r, view, meta{Source: s.source(book.calendar.dataset), Note: workDayNote(book.calendar.ID)}, cacheAPI)
}

func workDayNote(calendar string) string {
	switch calendar {
	case "bank":
		return "Sundays, the second and fourth Saturdays, and the Reserve Bank's holidays for the state's regional office."
	case "market":
		return "Weekends and NSE trading holidays. Special sessions, such as Muhurat trading or a Saturday budget session, are announced separately by the exchange."
	case "settlement":
		return "Weekends and NSE clearing holidays: days on which trades do not settle. A T+1 trade settles on add=1."
	}
	return "Weekends and gazetted holidays for central government offices. Restricted holidays are optional and count as working days."
}

type workRangeView struct {
	From        string           `json:"from"`
	To          string           `json:"to"`
	Calendar    string           `json:"calendar"`
	State       string           `json:"state,omitempty"`
	Days        int              `json:"days"`
	WorkingDays int              `json:"workingDays"`
	DaysOff     []workDayOffView `json:"daysOff"`
}

type workDayOffView struct {
	Date    string   `json:"date"`
	Weekday string   `json:"weekday"`
	Reasons []string `json:"reasons"`
}

func (s *Server) handleWorkingDays(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	from, err := parseDay(query.Get("from"))
	if err == nil {
		var to time.Time
		to, err = parseDay(query.Get("to"))
		if err == nil {
			s.workingDaysBetween(w, r, from, to)
			return
		}
	}
	s.badRequest(w, r, "Give from and to: "+err.Error()+".")
}

func (s *Server) workingDaysBetween(w http.ResponseWriter, r *http.Request, from, to time.Time) {
	info(r).subject = from.Format(isoDay) + ".." + to.Format(isoDay)
	if to.Before(from) {
		s.badRequest(w, r, "to must not be before from.")
		return
	}
	if to.Sub(from) > 366*24*time.Hour {
		s.badRequest(w, r, "The range may span at most 366 days.")
		return
	}
	book := s.openWorkBook(w, r, calendarParam(r), from.Year(), to.Year())
	if book == nil {
		return
	}
	view := workRangeView{From: from.Format(isoDay), To: to.Format(isoDay), Calendar: book.calendar.ID, State: book.state, DaysOff: []workDayOffView{}}
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		if !book.covers(day) {
			s.notFound(w, r, book.uncovered(day))
			return
		}
		view.Days++
		if reasons := book.reasons(day); len(reasons) > 0 {
			view.DaysOff = append(view.DaysOff, workDayOffView{Date: day.Format(isoDay), Weekday: day.Weekday().String(), Reasons: reasons})
		} else {
			view.WorkingDays++
		}
	}
	s.ok(w, r, view, meta{Source: s.source(book.calendar.dataset), Note: workDayNote(book.calendar.ID)}, cacheAPI)
}

// ---- market status ----

var indiaTime = time.FixedZone("IST", 5*3600+1800)

// NSE equities sessions, in minutes after midnight IST.
var marketSessions = []struct {
	name       string
	start, end int
}{
	{"pre_open", 9 * 60, 9*60 + 15},
	{"open", 9*60 + 15, 15*60 + 30},
	{"closing", 15*60 + 30, 16 * 60},
}

type marketStatusView struct {
	At       string   `json:"at"`
	Exchange string   `json:"exchange"`
	Session  string   `json:"session"`
	Open     bool     `json:"open"`
	Reasons  []string `json:"reasons,omitempty"`
	Hours    struct {
		PreOpen string `json:"preOpen"`
		Normal  string `json:"normal"`
		Closing string `json:"closing"`
	} `json:"hours"`
	NextOpen string `json:"nextOpen,omitempty"`
	Note     string `json:"note,omitempty"`
}

func (s *Server) handleMarketStatus(w http.ResponseWriter, r *http.Request) {
	at := time.Now()
	if raw := r.URL.Query().Get("at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			s.badRequest(w, r, "at is a moment with its offset, for example 2026-10-13T10:30:00+05:30.")
			return
		}
		at = parsed
	}
	at = at.In(indiaTime).Truncate(time.Second)
	info(r).subject = at.Format(time.RFC3339)
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)

	book := s.openWorkBook(w, r, "market", day.Year(), day.Year()+1)
	if book == nil {
		return
	}
	if !book.covers(day) {
		s.notFound(w, r, book.uncovered(day))
		return
	}

	view := marketStatusView{At: at.Format(time.RFC3339), Exchange: "NSE equities", Session: "closed"}
	view.Hours.PreOpen, view.Hours.Normal, view.Hours.Closing = "09:00-09:15", "09:15-15:30", "15:30-16:00"
	view.Reasons = book.reasons(day)
	minute := at.Hour()*60 + at.Minute()
	if len(view.Reasons) == 0 {
		for _, session := range marketSessions {
			if minute >= session.start && minute < session.end {
				view.Session = session.name
			}
		}
	}
	view.Open = view.Session == "open"
	if notes := s.marketDayNotes(day); notes != "" {
		view.Note = notes
	}

	// The next moment normal trading starts: today if it has not started yet, otherwise
	// the next trading day.
	if !view.Open {
		next := day
		if len(view.Reasons) > 0 || minute >= marketSessions[1].start {
			var ok bool
			if next, ok = book.step(day, 1); !ok {
				next = time.Time{}
			}
		}
		if !next.IsZero() {
			view.NextOpen = time.Date(next.Year(), next.Month(), next.Day(), 9, 15, 0, 0, indiaTime).Format(time.RFC3339)
		}
	}
	// A status is only good for a short while, unless it was asked for a fixed moment.
	cache := "public, max-age=30"
	if r.URL.Query().Get("at") != "" {
		cache = cacheAPI
	}
	s.ok(w, r, view, meta{Source: s.source("holidays-market"),
		Note: "Regular NSE equities hours. Special sessions (Muhurat trading, Saturday budget sessions, mock sessions) are announced separately by the exchange."}, cache)
}

// marketDayNotes repeats the exchange's own note for a day, such as a Muhurat session.
func (s *Server) marketDayNotes(day time.Time) string {
	set := s.data("holidays-market")
	if set == nil {
		return ""
	}
	var notes string
	set.row(`SELECT COALESCE(group_concat(notes, ' '), '') FROM holidays WHERE type = 'market' AND date = ? AND notes IS NOT NULL`, day.Format(isoDay)).Scan(&notes)
	return notes
}

func init() {
	calendarArg := oneOf("bank (needs state), market (NSE trading), settlement (NSE clearing) or central (central government offices).", workCalendarIDs()...)
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "check_working_day",
			Description: "Check whether a date is a working day in India for banks in a state, NSE trading, NSE settlement or central government offices; get the previous and next working days, or the date N working days away (T+1 settlement: calendar settlement, add 1).",
			InputSchema: schema([]string{"date"}, map[string]any{
				"date":     text("Date, YYYY-MM-DD."),
				"calendar": calendarArg,
				"state":    stateArg,
				"add":      map[string]any{"type": "integer", "minimum": -366, "maximum": 366, "description": "Working days to add."},
			}),
			path: func(args map[string]any) (string, error) {
				date, err := required(args, "date")
				return "/v1/working-days/" + url.PathEscape(date) + query(args, map[string]string{"calendar": "calendar", "state": "state", "add": "add"}), err
			},
		},
		mcpTool{
			Name:        "count_working_days",
			Description: "Count Indian working days between two dates (both included) for a calendar, and list each day off with its reason.",
			InputSchema: schema([]string{"from", "to"}, map[string]any{
				"from": text("First date, YYYY-MM-DD."), "to": text("Last date, YYYY-MM-DD."),
				"calendar": calendarArg, "state": stateArg,
			}),
			path: func(args map[string]any) (string, error) {
				if _, err := required(args, "from"); err != nil {
					return "", err
				}
				_, err := required(args, "to")
				return "/v1/working-days" + query(args, map[string]string{"from": "from", "to": "to", "calendar": "calendar", "state": "state"}), err
			},
		},
		mcpTool{
			Name:        "indian_market_status",
			Description: "Is the Indian stock market (NSE equities) open now or at a given moment: session, reason if closed, and when it next opens.",
			InputSchema: schema(nil, map[string]any{"at": text("Moment with offset, RFC 3339, for example 2026-10-13T10:30:00+05:30. Default now.")}),
			path: func(args map[string]any) (string, error) {
				return "/v1/market/status" + query(args, map[string]string{"at": "at"}), nil
			},
		},
		mcpTool{
			Name:        "find_post_offices_near",
			Description: "Find Indian post offices and their pincodes near a latitude and longitude, nearest first.",
			InputSchema: schema([]string{"lat", "lng"}, map[string]any{
				"lat": map[string]any{"type": "number"}, "lng": map[string]any{"type": "number"},
				"radius": map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "description": "Kilometres. Default 5."},
			}),
			path: func(args map[string]any) (string, error) {
				if _, err := required(args, "lat"); err != nil {
					return "", err
				}
				_, err := required(args, "lng")
				return "/v1/pincode/near" + query(args, map[string]string{"lat": "lat", "lng": "lng", "radius": "radius"}), err
			},
		},
	)
}
