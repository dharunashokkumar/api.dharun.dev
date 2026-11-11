package api

// Mandi prices (Agmarknet) and air quality (CPCB), both from data.gov.in: server-only
// datasets mandi and air-quality.

import (
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var mandiDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (s *Server) registerGovData() {
	s.handle("mandi", costLight, "GET /v1/mandi", s.handleMandi)
	s.handle("mandi_commodities", costLight, "GET /v1/mandi/commodities", s.handleMandiCommodities)
	s.handle("air_quality", costLight, "GET /v1/air-quality", s.handleAirQuality)
}

type mandiView struct {
	Date      string   `json:"date"`
	State     string   `json:"state"`
	District  string   `json:"district"`
	Market    string   `json:"market"`
	Commodity string   `json:"commodity"`
	Variety   string   `json:"variety"`
	Grade     string   `json:"grade"`
	Min       *float64 `json:"min_price"`
	Max       *float64 `json:"max_price"`
	Modal     *float64 `json:"modal_price"`
}

const mandiNote = "Rupees a quintal (100 kg), as the market reported to Agmarknet: modal_price is the price most trades were made at. Not every market reports every day."

func (s *Server) handleMandi(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "mandi")
	if set == nil {
		return
	}
	query := r.URL.Query()
	limit := 100
	if raw := query.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 1000 {
			s.badRequest(w, r, "limit is 1 to 1000.")
			return
		}
		limit = n
	}
	date := query.Get("date")
	if date == "" {
		if err := set.row(`SELECT max(date) FROM prices`).Scan(&date); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
	} else if !mandiDate.MatchString(date) {
		s.badRequest(w, r, "date is YYYY-MM-DD.")
		return
	}
	where, args := []string{"date = ?"}, []any{date}
	for _, f := range []struct{ param, column string }{{"commodity", "commodity"}, {"state", "state"}, {"district", "district"}} {
		if v := strings.TrimSpace(query.Get(f.param)); v != "" {
			where, args = append(where, f.column+" = ? COLLATE NOCASE"), append(args, v)
		}
	}
	if v := strings.TrimSpace(query.Get("market")); v != "" {
		where, args = append(where, "market LIKE ?"), append(args, "%"+v+"%")
	}
	info(r).subject = query.Get("commodity")
	var total int
	if err := set.row(`SELECT count(*) FROM prices WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.rows(`SELECT date, state, district, market, commodity, variety, grade, min_price, max_price, modal_price FROM prices WHERE `+
		strings.Join(where, " AND ")+` ORDER BY state, district, market, commodity, variety LIMIT ?`, append(args, limit)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	out := []mandiView{}
	for rows.Next() {
		var v mandiView
		if rows.Scan(&v.Date, &v.State, &v.District, &v.Market, &v.Commodity, &v.Variety, &v.Grade, &v.Min, &v.Max, &v.Modal) == nil {
			out = append(out, v)
		}
	}
	note := mandiNote
	if total == 0 {
		note = "No prices for these filters on " + date + ". /v1/mandi/commodities lists what was reported. " + mandiNote
	}
	s.ok(w, r, out, meta{Total: &total, Source: s.source("mandi"), Note: note}, "public, max-age=1800")
}

func (s *Server) handleMandiCommodities(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "mandi")
	if set == nil {
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	query := `SELECT commodity, count(DISTINCT market), min(modal_price), max(modal_price), date FROM prices WHERE date = (SELECT max(date) FROM prices)`
	var args []any
	if state != "" {
		query += ` AND state = ? COLLATE NOCASE`
		args = append(args, state)
	}
	rows, err := set.rows(query+` GROUP BY commodity ORDER BY commodity`, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	type commodity struct {
		Commodity string   `json:"commodity"`
		Markets   int      `json:"markets"`
		LowModal  *float64 `json:"lowest_modal_price"`
		HighModal *float64 `json:"highest_modal_price"`
		Date      string   `json:"date"`
	}
	out := []commodity{}
	for rows.Next() {
		var c commodity
		if rows.Scan(&c.Commodity, &c.Markets, &c.LowModal, &c.HighModal, &c.Date) == nil {
			out = append(out, c)
		}
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("mandi"), Note: "What the markets reported on the latest day, with the range of modal prices across them. " + mandiNote}, "public, max-age=1800")
}

// ---- air quality ----

type pollutantView struct {
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
	Avg *float64 `json:"avg"`
}

type airStation struct {
	Station    string                   `json:"station"`
	City       string                   `json:"city"`
	State      string                   `json:"state"`
	Latitude   *float64                 `json:"latitude"`
	Longitude  *float64                 `json:"longitude"`
	Updated    string                   `json:"updated"`
	AQI        *int                     `json:"aqi"`
	Category   string                   `json:"category,omitempty"`
	Main       string                   `json:"main_pollutant,omitempty"`
	Pollutants map[string]pollutantView `json:"pollutants"`
	DistanceKM *float64                 `json:"distance_km,omitempty"`
}

// aqiCategory names CPCB's National Air Quality Index bands.
func aqiCategory(aqi int) string {
	switch {
	case aqi <= 50:
		return "Good"
	case aqi <= 100:
		return "Satisfactory"
	case aqi <= 200:
		return "Moderate"
	case aqi <= 300:
		return "Poor"
	case aqi <= 400:
		return "Very poor"
	}
	return "Severe"
}

// setAQI applies CPCB's rule: the index is the highest sub-index, given at least three
// pollutants, one of them PM2.5 or PM10.
func (st *airStation) setAQI() {
	count, particulate, best, main := 0, false, -1.0, ""
	for name, p := range st.Pollutants {
		if p.Avg == nil {
			continue
		}
		count++
		if name == "PM2.5" || name == "PM10" {
			particulate = true
		}
		if *p.Avg > best {
			best, main = *p.Avg, name
		}
	}
	if count >= 3 && particulate {
		aqi := int(math.Round(best))
		st.AQI, st.Category, st.Main = &aqi, aqiCategory(aqi), main
	}
}

func (s *Server) handleAirQuality(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "air-quality")
	if set == nil {
		return
	}
	query := r.URL.Query()
	city, state, station := strings.TrimSpace(query.Get("city")), strings.TrimSpace(query.Get("state")), strings.TrimSpace(query.Get("station"))
	var nearLat, nearLon float64
	near := strings.TrimSpace(query.Get("near"))
	if near != "" {
		lat, lon, ok := strings.Cut(near, ",")
		var err1, err2 error
		nearLat, err1 = strconv.ParseFloat(strings.TrimSpace(lat), 64)
		nearLon, err2 = strconv.ParseFloat(strings.TrimSpace(lon), 64)
		if !ok || err1 != nil || err2 != nil || math.Abs(nearLat) > 90 || math.Abs(nearLon) > 180 {
			s.badRequest(w, r, "near is latitude,longitude, such as 13.08,80.27.")
			return
		}
	}
	where, args := []string{"1 = 1"}, []any{}
	if city != "" {
		where, args = append(where, "city = ? COLLATE NOCASE"), append(args, city)
	}
	if state != "" {
		where, args = append(where, "state = ? COLLATE NOCASE"), append(args, state)
	}
	if station != "" {
		where, args = append(where, "station LIKE ?"), append(args, "%"+station+"%")
	}
	info(r).subject = city + state
	rows, err := set.rows(`SELECT station, city, state, latitude, longitude, updated, pollutant, min, max, avg FROM readings WHERE `+strings.Join(where, " AND ")+` ORDER BY station`, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	byStation := map[string]*airStation{}
	var order []string
	for rows.Next() {
		var st airStation
		var pollutant string
		var p pollutantView
		if rows.Scan(&st.Station, &st.City, &st.State, &st.Latitude, &st.Longitude, &st.Updated, &pollutant, &p.Min, &p.Max, &p.Avg) != nil {
			continue
		}
		have, ok := byStation[st.Station]
		if !ok {
			st.Pollutants = map[string]pollutantView{}
			have = &st
			byStation[st.Station] = have
			order = append(order, st.Station)
		}
		have.Pollutants[pollutant] = p
	}
	out := make([]airStation, 0, len(order))
	for _, name := range order {
		st := byStation[name]
		st.setAQI()
		if near != "" && st.Latitude != nil && st.Longitude != nil {
			d := math.Round(haversineKM(nearLat, nearLon, *st.Latitude, *st.Longitude)*10) / 10
			st.DistanceKM = &d
		}
		out = append(out, *st)
	}
	if near != "" {
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].DistanceKM == nil || out[j].DistanceKM == nil {
				return out[j].DistanceKM == nil && out[i].DistanceKM != nil
			}
			return *out[i].DistanceKM < *out[j].DistanceKM
		})
		out = out[:min(len(out), 5)]
	}
	total := len(out)
	note := "Each pollutant's figure is CPCB's sub-index over the last hours (min, max and average); aqi is the highest average, given at least three pollutants with PM2.5 or PM10 among them, as CPCB's National Air Quality Index rule asks. Updated is India time."
	if total == 0 {
		note = "No station matches. Cities and states are named as CPCB names them (Delhi, Bengaluru, Tamil Nadu). " + note
	}
	s.ok(w, r, out, meta{Total: &total, Source: s.source("air-quality"), Note: note}, "public, max-age=600")
}

// haversineKM is the great-circle distance between two points, in kilometres.
func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const radius = 6371.0
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * radius * math.Asin(math.Sqrt(a))
}
