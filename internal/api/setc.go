package api

// Set C of the idea catalogue (growth plan, 05): India civic and daily life. This file
// holds what is computed or typed rather than downloaded.

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/festivals"
	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

func (s *Server) registerSetC() {
	s.handle("festivals", costHeavy, "GET /v1/festivals/{year}", s.handleFestivals)
	s.registerSetCText()
}

var festivalKinds = []string{"hindu", "tamil", "sikh", "jain", "buddhist", "islamic", "christian", "national"}

const festivalNote = "Computed from the panchang: lunar festivals by the tithi at the time custom names (sunrise for most), reckoned at New Delhi; Tamil festivals by the solar month and nakshatra at Chennai. Local almanacs can differ by a day where a tithi changes near that time. Islamic dates are the tabular calendar's and marked expected: the sighting of the moon decides them. Not the official holiday list: see /v1/holidays for that."

// festivalsOn returns a year's festivals, filtered.
func festivalsOn(year int, kind, state string, month int) ([]festivals.Festival, error) {
	all, err := festivals.Year(year)
	if err != nil {
		return nil, err
	}
	out := []festivals.Festival{}
	for _, f := range all {
		if kind != "" && f.Kind != kind {
			continue
		}
		if month > 0 && f.Date[5:7] != twoDigits(month) {
			continue
		}
		if state != "" && len(f.Regions) > 0 {
			found := false
			for _, region := range f.Regions {
				if region == state || (region == "south" && southStates[state]) || (region == "north" && !southStates[state]) {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, f)
	}
	return out, nil
}

var southStates = map[string]bool{"TN": true, "KL": true, "KA": true, "AP": true, "TG": true, "PY": true, "LD": true, "AN": true}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func (s *Server) handleFestivals(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("year")
	calendar := strings.HasSuffix(raw, ".ics")
	raw = strings.TrimSuffix(raw, ".ics")
	info(r).subject = raw
	year, err := strconv.Atoi(raw)
	if err != nil || year < festivals.MinYear || year > festivals.MaxYear {
		s.badRequest(w, r, "The year is "+strconv.Itoa(festivals.MinYear)+" to "+strconv.Itoa(festivals.MaxYear)+".")
		return
	}
	query := r.URL.Query()
	kind := strings.ToLower(query.Get("kind"))
	if kind != "" {
		known := false
		for _, k := range festivalKinds {
			known = known || k == kind
		}
		if !known {
			s.badRequest(w, r, "kind is one of "+strings.Join(festivalKinds, ", ")+".")
			return
		}
	}
	state := ""
	if raw := query.Get("state"); raw != "" {
		st, ok := india.Lookup(raw)
		if !ok {
			s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
			return
		}
		state = st.Code
	}
	month := 0
	if raw := query.Get("month"); raw != "" {
		if month, err = strconv.Atoi(raw); err != nil || month < 1 || month > 12 {
			s.badRequest(w, r, "month is 1 to 12.")
			return
		}
	}
	list, err := festivalsOn(year, kind, state, month)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	if calendar {
		events := make([]calendarEvent, 0, len(list))
		for _, f := range list {
			description := f.Rule + ". From " + s.origin(r) + "/v1/festivals/" + raw
			if f.Expected {
				description = "Expected, subject to the sighting of the moon. " + description
			}
			events = append(events, calendarEvent{Date: f.Date, Key: f.Kind, Name: f.Name, Description: description})
		}
		name := "Indian festivals " + raw
		if state != "" {
			name += " (" + state + ")"
		}
		writeCalendar(w, r, name, "Festivals", events)
		return
	}
	total := len(list)
	s.ok(w, r, list, meta{Total: &total, Note: festivalNote}, cacheLong)
}

// todaysFestivals lists the festivals on a date, for /v1/today.
func todaysFestivals(day time.Time, state string) []festivals.Festival {
	list, err := festivalsOn(day.Year(), "", state, int(day.Month()))
	if err != nil {
		return nil
	}
	out := []festivals.Festival{}
	for _, f := range list {
		if f.Date == day.Format(time.DateOnly) {
			out = append(out, f)
		}
	}
	return out
}
