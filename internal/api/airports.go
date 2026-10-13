package api

import (
	"database/sql"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type airportView struct {
	IATA        *string  `json:"iata"`
	ICAO        *string  `json:"icao"`
	Ident       string   `json:"ident"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	City        *string  `json:"city"`
	StateCode   *string  `json:"stateCode,omitempty"`
	Country     string   `json:"country"`
	CountryName *string  `json:"countryName"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	ElevationFt *int64   `json:"elevationFt"`
	Scheduled   bool     `json:"scheduledService"`
	Wikipedia   *string  `json:"wikipedia,omitempty"`
	Website     *string  `json:"website,omitempty"`
}

const airportColumns = `a.iata, a.icao, a.ident, a.name, a.type, a.municipality, a.state_code, a.country, c.name,
	a.lat, a.lng, a.elevation_ft, a.scheduled, a.wikipedia, a.website
	FROM airports a LEFT JOIN countries c ON c.code = a.country`

func scanAirport(scan func(...any) error) (airportView, error) {
	var view airportView
	var iata, icao, city, state, countryName, wikipedia, website sql.NullString
	var lat, lng sql.NullFloat64
	var elevation sql.NullInt64
	err := scan(&iata, &icao, &view.Ident, &view.Name, &view.Type, &city, &state, &view.Country, &countryName,
		&lat, &lng, &elevation, &view.Scheduled, &wikipedia, &website)
	view.IATA, view.ICAO, view.City, view.StateCode, view.CountryName = str(iata), str(icao), str(city), str(state), str(countryName)
	view.Wikipedia, view.Website = str(wikipedia), str(website)
	view.Latitude, view.Longitude = number(lat), number(lng)
	if elevation.Valid {
		view.ElevationFt = &elevation.Int64
	}
	return view, err
}

var (
	iataShape = regexp.MustCompile(`^[A-Z]{3}$`)
	icaoShape = regexp.MustCompile(`^[A-Z]{4}$`)
)

var airportTypes = []string{"large_airport", "medium_airport", "small_airport", "heliport", "seaplane_base"}

func (s *Server) handleAirport(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "airports")
	if set == nil {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	info(r).subject = code
	var statement string
	switch {
	case iataShape.MatchString(code):
		// Prefer the airport that still has scheduled flights when a code was reused.
		statement = `SELECT ` + airportColumns + ` WHERE a.iata = ? ORDER BY a.scheduled DESC LIMIT 1`
	case icaoShape.MatchString(code):
		statement = `SELECT ` + airportColumns + ` WHERE a.icao = ? OR a.ident = ? LIMIT 1`
	default:
		s.badRequest(w, r, "Give a three-letter IATA code (MAA) or a four-letter ICAO code (VOMM).")
		return
	}
	args := []any{code}
	if len(code) == 4 {
		args = append(args, code)
	}
	view, err := scanAirport(set.row(statement, args...).Scan)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "No open airport has the code "+code+". Search by name or city at /v1/airports?q=.")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, view, meta{Source: s.source("airports")}, cacheLong)
}

func (s *Server) handleAirportSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "airports")
	if set == nil {
		return
	}
	query := r.URL.Query()
	text := strings.TrimSpace(query.Get("q"))
	country := strings.ToUpper(strings.TrimSpace(query.Get("country")))
	kind := strings.ToLower(strings.TrimSpace(query.Get("type")))
	limit, offset, ok := s.listParams(w, r, 20)
	if !ok {
		return
	}
	stateCode, stateOK := stateParam(r)
	if !stateOK {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return
	}
	if country != "" && !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(country) {
		s.badRequest(w, r, "country is a two-letter ISO code, such as IN.")
		return
	}
	if kind != "" && !contains(airportTypes, kind) {
		s.badRequest(w, r, "type must be one of: "+strings.Join(airportTypes, ", ")+".")
		return
	}
	if len(text) < 2 && country == "" && stateCode == "" {
		s.badRequest(w, r, "Give q (at least 2 characters of a name, city or code), a country or a state.")
		return
	}
	if stateCode != "" {
		country = "IN"
	}
	info(r).subject = text

	where := []string{"1 = 1"}
	var args []any
	if text != "" {
		pattern := likePattern(text, true)
		where = append(where, `(a.name LIKE ? ESCAPE '\' OR a.municipality LIKE ? ESCAPE '\' OR a.keywords LIKE ? ESCAPE '\' OR a.iata = ? OR a.icao = ?)`)
		args = append(args, pattern, pattern, pattern, strings.ToUpper(text), strings.ToUpper(text))
	}
	if country != "" {
		where = append(where, `a.country = ?`)
		args = append(args, country)
	}
	if stateCode != "" {
		where = append(where, `a.state_code = ?`)
		args = append(args, stateCode)
	}
	if kind != "" {
		where = append(where, `a.type = ?`)
		args = append(args, kind)
	}
	condition := strings.Join(where, " AND ")

	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM airports a WHERE `+condition, args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	// Airports with flights first, then by size, so "chennai" leads with MAA.
	rows, err := set.db.Query(`SELECT `+airportColumns+` WHERE `+condition+`
		ORDER BY a.scheduled DESC, CASE a.type WHEN 'large_airport' THEN 0 WHEN 'medium_airport' THEN 1 WHEN 'small_airport' THEN 2 ELSE 3 END, a.name
		LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	airports := []airportView{}
	for rows.Next() {
		view, err := scanAirport(rows.Scan)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		airports = append(airports, view)
	}
	m := paging(total, limit, offset)
	m.Source = s.source("airports")
	s.ok(w, r, airports, m, cacheAPI)
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "lookup_airport",
			Description: "Look up an airport by IATA code (MAA) or ICAO code (VOMM): name, city, state, country, coordinates and whether it has scheduled flights.",
			InputSchema: schema([]string{"code"}, map[string]any{"code": text("IATA or ICAO code.")}),
			path: func(args map[string]any) (string, error) {
				code, err := required(args, "code")
				return "/v1/airports/" + url.PathEscape(code), err
			},
		},
		mcpTool{
			Name:        "search_airports",
			Description: "Search airports by name or city, worldwide (those with IATA codes) and every Indian airfield and heliport. Filter by country (ISO code) or Indian state.",
			InputSchema: schema(nil, map[string]any{
				"query": text("Name, city or code."), "country": text("Two-letter ISO country code, such as IN."), "state": stateArg,
			}),
			path: func(args map[string]any) (string, error) {
				return "/v1/airports" + query(args, map[string]string{"query": "q", "country": "country", "state": "state"}), nil
			},
		},
	)
}
