package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

type stationZoneView struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type stationView struct {
	Code       string           `json:"code"`
	Name       string           `json:"name"`
	OtherNames []string         `json:"otherNames,omitempty"`
	StateCode  *string          `json:"stateCode"`
	State      *string          `json:"state"`
	Zone       *stationZoneView `json:"zone"`
	ElevationM *float64         `json:"elevationM"`
}

func scanStation(scan func(...any) error) (stationView, error) {
	var view stationView
	var state, zone, others sql.NullString
	var elevation sql.NullFloat64
	err := scan(&view.Code, &view.Name, &state, &zone, &elevation, &others)
	if state.Valid {
		view.StateCode = &state.String
		if found, ok := india.Lookup(state.String); ok {
			view.State = &found.Name
		}
	}
	if zone.Valid {
		for _, z := range dataset.Zones {
			if z.Code == zone.String {
				view.Zone = &stationZoneView{z.Code, z.Name}
			}
		}
	}
	view.ElevationM = number(elevation)
	if others.Valid {
		json.Unmarshal([]byte(others.String), &view.OtherNames)
	}
	return view, err
}

const (
	stationColumns = `code, name, state_code, zone, elevation_m, other_names FROM stations`
	stationNote    = "From a community-kept list: a few codes are out of date (Mumbai CSMT is listed under its old code CSTM) and many stations lack a state or zone."
)

var stationCodeInput = regexp.MustCompile(`^[A-Z]{1,5}$`)

func (s *Server) handleStation(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "stations")
	if set == nil {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	info(r).subject = code
	if !stationCodeInput.MatchString(code) {
		s.badRequest(w, r, "A station code is 1 to 5 letters, such as MAS or NDLS.")
		return
	}
	view, err := scanStation(set.row(`SELECT `+stationColumns+` WHERE code = ?`, code).Scan)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "No station with the code "+code+". Search by name at /v1/stations?q=.")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, view, meta{Source: s.source("stations"), Note: stationNote}, cacheLong)
}

func (s *Server) handleStationSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "stations")
	if set == nil {
		return
	}
	query := r.URL.Query()
	text := strings.TrimSpace(query.Get("q"))
	zone := strings.TrimSpace(query.Get("zone"))
	limit, offset, ok := s.listParams(w, r, 20)
	if !ok {
		return
	}
	stateCode, stateOK := stateParam(r)
	if !stateOK {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return
	}
	if zone != "" {
		found := false
		for _, z := range dataset.Zones {
			if strings.EqualFold(z.Code, zone) {
				zone, found = z.Code, true
			}
		}
		if !found {
			s.badRequest(w, r, "zone is a zone code, such as SR or NFR. See /v1/stations/zones.")
			return
		}
	}
	if len(text) < 2 && stateCode == "" && zone == "" {
		s.badRequest(w, r, "Give q (at least 2 characters of a name), a state or a zone.")
		return
	}
	info(r).subject = text
	where := []string{"1 = 1"}
	var args []any
	order := "name"
	if text != "" {
		where = append(where, `(search LIKE ? ESCAPE '\' OR code = ?)`)
		args = append(args, likePattern(strings.ToLower(text), true), strings.ToUpper(text))
		// An exact code first, then names that start with the text.
		order = `code = ? DESC, search LIKE ? ESCAPE '\' DESC, name`
	}
	if stateCode != "" {
		where = append(where, `state_code = ?`)
		args = append(args, stateCode)
	}
	if zone != "" {
		where = append(where, `zone = ?`)
		args = append(args, zone)
	}
	condition := strings.Join(where, " AND ")
	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM stations WHERE `+condition, args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	orderArgs := []any{}
	if text != "" {
		orderArgs = append(orderArgs, strings.ToUpper(text), likePattern(strings.ToLower(text), false))
	}
	rows, err := set.db.Query(`SELECT `+stationColumns+` WHERE `+condition+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		append(append(args, orderArgs...), limit, offset)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	stations := []stationView{}
	for rows.Next() {
		view, err := scanStation(rows.Scan)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		stations = append(stations, view)
	}
	if rows.Err() != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	m := paging(total, limit, offset)
	m.Source, m.Note = s.source("stations"), stationNote
	s.ok(w, r, stations, m, cacheAPI)
}

func (s *Server) handleStationZones(w http.ResponseWriter, r *http.Request) {
	zones := make([]stationZoneView, len(dataset.Zones))
	for i, z := range dataset.Zones {
		zones[i] = stationZoneView{z.Code, z.Name}
	}
	total := len(zones)
	s.ok(w, r, zones, meta{Total: &total}, cacheLong)
}

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "lookup_railway_station",
			Description: "Look up an Indian Railways station by its code (MAS, NDLS): name, state, railway zone and elevation.",
			InputSchema: schema([]string{"code"}, map[string]any{"code": text("Station code.")}),
			path: func(args map[string]any) (string, error) {
				code, err := required(args, "code")
				return "/v1/stations/" + url.PathEscape(code), err
			},
		},
		mcpTool{
			Name:        "search_railway_stations",
			Description: "Find Indian railway stations and their codes by name, state or railway zone.",
			InputSchema: schema(nil, map[string]any{"query": text("Station name."), "state": stateArg, "zone": text("Zone code, such as SR.")}),
			path: func(args map[string]any) (string, error) {
				return "/v1/stations" + query(args, map[string]string{"query": "q", "state": "state", "zone": "zone"}), nil
			},
		},
	)
}
