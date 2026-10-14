package api

import (
	"database/sql"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/orbit"
)

// Satellite passes and positions, computed from CelesTrak's orbit elements. Only what
// is computed from the elements is ever returned, never the elements themselves.

type satelliteRecord struct {
	Number      int      `json:"number"`
	Name        string   `json:"name"`
	Groups      []string `json:"groups"`
	Period      float64  `json:"periodMinutes"`
	Inclination float64  `json:"inclination"`
	Epoch       string   `json:"elementsEpoch"`
	line1       string
	line2       string
}

const satelliteColumns = `number, name, groups, period_minutes, inclination, epoch, line1, line2 FROM satellites`

func scanSatellite(scan func(...any) error) (satelliteRecord, error) {
	var record satelliteRecord
	var groups string
	err := scan(&record.Number, &record.Name, &groups, &record.Period, &record.Inclination, &record.Epoch, &record.line1, &record.line2)
	record.Groups = strings.Split(groups, ",")
	record.Period = math.Round(record.Period*100) / 100
	record.Inclination = math.Round(record.Inclination*1000) / 1000
	return record, err
}

// satelliteAliases are names people use that CelesTrak's names do not contain.
var satelliteAliases = map[string]int{
	"ISS": 25544, "ZARYA": 25544, "CSS": 48274, "TIANGONG": 48274, "TIANHE": 48274,
	"QO-100": 43700, "QO100": 43700, "ESHAIL 2": 43700, "ESHAIL-2": 43700,
}

var errAmbiguous = errors.New("ambiguous")

// findSatellite reads a satellite from a catalogue number, a name, the designation in
// a name's brackets (SO-50, AO-7) or a part of a name that only one satellite has.
func (s *Server) findSatellite(set *handle, query string) (satelliteRecord, []string, error) {
	query = strings.ToUpper(strings.TrimSpace(query))
	if number, ok := satelliteAliases[query]; ok {
		query = strconv.Itoa(number)
	}
	if number, err := strconv.Atoi(query); err == nil {
		record, err := scanSatellite(set.row(`SELECT `+satelliteColumns+` WHERE number = ?`, number).Scan)
		return record, nil, err
	}
	rows, err := set.rows(`SELECT ` + satelliteColumns + ` ORDER BY number`)
	if err != nil {
		return satelliteRecord{}, nil, err
	}
	defer rows.Close()
	var exact, bracket, partial []satelliteRecord
	for rows.Next() {
		record, err := scanSatellite(rows.Scan)
		if err != nil {
			return satelliteRecord{}, nil, err
		}
		name := strings.ToUpper(record.Name)
		switch {
		case name == query:
			exact = append(exact, record)
		case strings.Contains(name, "("+query+")"):
			bracket = append(bracket, record)
		case strings.Contains(name, query):
			partial = append(partial, record)
		}
	}
	for _, found := range [][]satelliteRecord{exact, bracket, partial} {
		switch {
		case len(found) == 1:
			return found[0], nil, nil
		case len(found) > 1:
			var names []string
			for _, record := range found[:min(len(found), 10)] {
				names = append(names, record.Name+" ("+strconv.Itoa(record.Number)+")")
			}
			return satelliteRecord{}, names, errAmbiguous
		}
	}
	return satelliteRecord{}, nil, sql.ErrNoRows
}

// satelliteFromRequest finds the satellite a request names, answering errors itself.
func (s *Server) satelliteFromRequest(w http.ResponseWriter, r *http.Request, set *handle, query string) (satelliteRecord, *orbit.Elements, bool) {
	record, candidates, err := s.findSatellite(set, query)
	switch {
	case errors.Is(err, errAmbiguous):
		s.badRequest(w, r, "Several satellites match "+query+": "+strings.Join(candidates, ", ")+". Give the catalogue number.")
		return record, nil, false
	case errors.Is(err, sql.ErrNoRows):
		s.notFound(w, r, "No tracked satellite matches "+query+". The space stations and amateur radio satellites are tracked; see /v1/satellites.")
		return record, nil, false
	case err != nil:
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return record, nil, false
	}
	elements, err := orbit.Parse(record.Name, record.line1, record.line2)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return record, nil, false
	}
	return record, elements, true
}

// maxElementsAge is how old orbit elements may be before predictions from them are
// refused. A low orbit drifts by kilometres a day once its elements are out of date.
const maxElementsAge = 30 * 24 * time.Hour

func (s *Server) handleSatellites(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "satellites")
	if set == nil {
		return
	}
	query := r.URL.Query()
	text := strings.ToUpper(strings.TrimSpace(query.Get("q")))
	info(r).subject = text
	group := strings.ToLower(strings.TrimSpace(query.Get("group")))
	groups := map[string]string{"": "", "stations": "space stations", "amateur": "amateur radio"}
	groupName, ok := groups[group]
	if !ok {
		s.badRequest(w, r, "group is stations or amateur.")
		return
	}
	rows, err := set.rows(`SELECT ` + satelliteColumns + ` ORDER BY number`)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	found := []satelliteRecord{}
	for rows.Next() {
		record, err := scanSatellite(rows.Scan)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		if text != "" && !strings.Contains(strings.ToUpper(record.Name), text) && strconv.Itoa(record.Number) != text {
			continue
		}
		if groupName != "" && !contains(record.Groups, groupName) {
			continue
		}
		found = append(found, record)
	}
	total := len(found)
	s.ok(w, r, found, meta{Total: &total, Source: s.source("satellites")}, "public, max-age=3600")
}

type positionView struct {
	satelliteRecord
	At          string  `json:"at"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	AltitudeKm  float64 `json:"altitudeKm"`
	SpeedKmS    float64 `json:"speedKmPerSecond"`
	Sunlit      bool    `json:"sunlit"`
	ElementsAge float64 `json:"elementsAgeDays"`
}

func (s *Server) handleSatellite(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "satellites")
	if set == nil {
		return
	}
	query := r.PathValue("id")
	info(r).subject = strings.ToUpper(strings.TrimSpace(query))
	record, elements, ok := s.satelliteFromRequest(w, r, set, query)
	if !ok {
		return
	}
	now := time.Now().UTC().Truncate(time.Second)
	if now.Sub(elements.Epoch) > maxElementsAge {
		s.fail(w, r, http.StatusServiceUnavailable, "stale_elements", "The orbit elements for "+record.Name+" are more than 30 days old, too old to say where it is.")
		return
	}
	where, err := elements.SubPoint(now)
	if errors.Is(err, orbit.ErrDecayed) {
		s.fail(w, r, http.StatusUnprocessableEntity, "decayed", record.Name+" has re-entered the atmosphere, according to its last orbit elements.")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	view := positionView{
		satelliteRecord: record, At: now.Format(time.RFC3339),
		Latitude: round(where.Latitude, 4), Longitude: round(where.Longitude, 4), AltitudeKm: round(where.Altitude, 1),
		SpeedKmS: round(where.Speed, 3), Sunlit: where.Sunlit, ElementsAge: round(now.Sub(elements.Epoch).Hours()/24, 2),
	}
	s.ok(w, r, view, meta{Source: s.source("satellites"), Note: "Where the satellite is at the moment of the request, over the WGS 84 ellipsoid."}, "public, max-age=5")
}

type passPoint struct {
	Time      string  `json:"time"`
	Azimuth   float64 `json:"azimuth"`
	Direction string  `json:"direction"`
	Elevation float64 `json:"elevation"`
	RangeKm   float64 `json:"rangeKm"`
}

type visibleSpan struct {
	From  string `json:"from"`
	Until string `json:"until"`
}

type passView struct {
	Rise        *passPoint   `json:"rise"`
	Culmination passPoint    `json:"culmination"`
	Set         *passPoint   `json:"set"`
	Duration    *int         `json:"durationSeconds"`
	Visible     *visibleSpan `json:"visible"`
}

type passesView struct {
	Satellite    satelliteRecord `json:"satellite"`
	ElementsAge  float64         `json:"elementsAgeDays"`
	Place        panchangPlace   `json:"place"`
	From         string          `json:"from"`
	Until        string          `json:"until"`
	MinElevation float64         `json:"minElevation"`
	Passes       []passView      `json:"passes"`
}

var compass = []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}

func direction(azimuth float64) string {
	return compass[int(math.Mod(azimuth+11.25, 360)/22.5)%16]
}

func (s *Server) handleSatellitePasses(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "satellites")
	if set == nil {
		return
	}
	place, view, ok := s.panchangPlace(w, r, true)
	if !ok {
		return
	}
	query := r.URL.Query()
	name := strings.TrimSpace(query.Get("sat"))
	if name == "" {
		name = "25544"
	}
	info(r).subject = strings.ToUpper(name)
	days := 3
	if raw := strings.TrimSpace(query.Get("days")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 10 {
			s.badRequest(w, r, "days is from 1 to 10.")
			return
		}
		days = value
	}
	minElevation := 10.0
	if raw := strings.TrimSpace(query.Get("minElevation")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || !(value >= 0 && value <= 90) {
			s.badRequest(w, r, "minElevation is in degrees, from 0 to 90.")
			return
		}
		minElevation = value
	}
	onlyVisible := false
	switch strings.ToLower(strings.TrimSpace(query.Get("visible"))) {
	case "", "false", "0":
	case "true", "1":
		onlyVisible = true
	default:
		s.badRequest(w, r, "visible is true or false.")
		return
	}
	record, elements, ok := s.satelliteFromRequest(w, r, set, name)
	if !ok {
		return
	}
	from := time.Now().UTC().Truncate(time.Minute)
	until := from.Add(time.Duration(days) * 24 * time.Hour)
	if from.Sub(elements.Epoch) > maxElementsAge {
		s.fail(w, r, http.StatusServiceUnavailable, "stale_elements", "The orbit elements for "+record.Name+" are more than 30 days old, too old for useful predictions.")
		return
	}
	passes, err := elements.Passes(orbit.Observer{Latitude: view.Latitude, Longitude: view.Longitude},
		orbit.PassOptions{From: from, Until: until, MinElevation: minElevation})
	if errors.Is(err, orbit.ErrDecayed) {
		s.fail(w, r, http.StatusUnprocessableEntity, "decayed", record.Name+" re-enters the atmosphere within the period, according to its last orbit elements.")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	stamp := func(t time.Time) string { return t.In(place.Zone).Format(time.RFC3339) }
	convert := func(p *orbit.Point) *passPoint {
		if p == nil {
			return nil
		}
		return &passPoint{Time: stamp(p.Time), Azimuth: round(p.Azimuth, 1), Direction: direction(p.Azimuth),
			Elevation: round(p.Elevation, 1), RangeKm: math.Round(p.Range)}
	}
	result := passesView{
		Satellite: record, ElementsAge: round(from.Sub(elements.Epoch).Hours()/24, 2), Place: view,
		From: stamp(from), Until: stamp(until), MinElevation: minElevation, Passes: []passView{},
	}
	for _, pass := range passes {
		if onlyVisible && pass.VisibleFrom == nil {
			continue
		}
		item := passView{Rise: convert(pass.Rise), Culmination: *convert(pass.Culmination), Set: convert(pass.Set)}
		if pass.Rise != nil && pass.Set != nil {
			seconds := int(pass.Set.Time.Sub(pass.Rise.Time).Seconds())
			item.Duration = &seconds
		}
		if pass.VisibleFrom != nil {
			item.Visible = &visibleSpan{From: stamp(*pass.VisibleFrom), Until: stamp(*pass.VisibleUntil)}
		}
		result.Passes = append(result.Passes, item)
	}
	total := len(result.Passes)
	s.ok(w, r, result, meta{Total: &total, Source: s.source("satellites"), Note: satellitePassNote}, "public, max-age=300")
}

const satellitePassNote = "Passes from now, computed with SGP4 from the satellite's latest orbit elements. Rise and set are when it crosses " +
	"the horizon (elevation 0°, no refraction); a pass is listed when its highest point reaches minElevation. visible is when it is " +
	"in sunlight while the sky is dark (sun 6° or more below the horizon), so it can be seen if bright enough. Rise is null for a " +
	"satellite already up when the search starts, set for one still up when it ends. Times agree with Skyfield to about a second."

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "get_satellite_passes",
			Description: "When the ISS, the Chinese space station or an amateur radio satellite passes over a place in the next days: rise, highest point and set times with compass directions and elevations, and when the pass can be seen with the eye.",
			InputSchema: schema([]string{}, map[string]any{
				"satellite":     text("Catalogue number or name, such as ISS, CSS, SO-50, AO-7 or QO-100. Default ISS."),
				"pincode":       text("Indian pincode of the observer."),
				"lat":           map[string]any{"type": "number"},
				"lng":           map[string]any{"type": "number"},
				"days":          map[string]any{"type": "integer", "description": "1 to 10. Default 3."},
				"min_elevation": map[string]any{"type": "number", "description": "Lowest peak elevation listed, degrees. Default 10."},
				"visible_only":  map[string]any{"type": "boolean"},
				"tz":            text("IANA time zone. Default Asia/Kolkata."),
			}),
			path: func(args map[string]any) (string, error) {
				return "/v1/sat-pass" + query(args, map[string]string{"satellite": "sat", "pincode": "pincode", "lat": "lat", "lng": "lng",
					"days": "days", "min_elevation": "minElevation", "visible_only": "visible", "tz": "tz"}), nil
			},
		},
		mcpTool{
			Name:        "find_satellites",
			Description: "List or search the tracked space stations and amateur radio satellites, with catalogue numbers, orbital periods and inclinations.",
			InputSchema: schema([]string{}, map[string]any{"query": text("Part of a name or a catalogue number."), "group": oneOf("", "stations", "amateur")}),
			path: func(args map[string]any) (string, error) {
				return "/v1/satellites" + query(args, map[string]string{"query": "q", "group": "group"}), nil
			},
		},
		mcpTool{
			Name:        "where_is_satellite",
			Description: "Where a tracked satellite such as the ISS is right now: the point on Earth below it, its altitude and speed, and whether it is in sunlight.",
			InputSchema: schema([]string{"satellite"}, map[string]any{"satellite": text("Catalogue number or name, such as ISS.")}),
			path: func(args map[string]any) (string, error) {
				id, err := required(args, "satellite")
				return "/v1/satellites/" + url.PathEscape(id), err
			},
		},
	)
}
