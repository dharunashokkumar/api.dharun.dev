package api

import (
	"database/sql"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// India's extent, with a margin; a point outside it cannot have a post office nearby.
const (
	indiaMinLat, indiaMaxLat = 6.0, 38.0
	indiaMinLng, indiaMaxLng = 68.0, 98.0
	earthRadiusKm            = 6371.0
)

// coordinateParams reads lat and lng. It writes the error response itself and returns
// false when they are missing or outside India.
func (s *Server) coordinateParams(w http.ResponseWriter, r *http.Request) (lat, lng float64, ok bool) {
	query := r.URL.Query()
	lat, errLat := strconv.ParseFloat(strings.TrimSpace(query.Get("lat")), 64)
	lng, errLng := strconv.ParseFloat(strings.TrimSpace(query.Get("lng")), 64)
	if errLat != nil || errLng != nil || math.IsNaN(lat) || math.IsNaN(lng) {
		s.badRequest(w, r, "Give lat and lng as decimal degrees, for example lat=11.2189&lng=78.1674.")
		return 0, 0, false
	}
	if lat < indiaMinLat || lat > indiaMaxLat || lng < indiaMinLng || lng > indiaMaxLng {
		s.badRequest(w, r, "That point is outside India.")
		return 0, 0, false
	}
	return lat, lng, true
}

// distanceKm is the great-circle distance between two points.
func distanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	rad := math.Pi / 180
	dLat, dLng := (lat2-lat1)*rad, (lng2-lng1)*rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}

type nearbyOffice struct {
	postOffice
	DistanceKm float64 `json:"distanceKm"`
}

func (s *Server) handlePincodeNear(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "pincode")
	if set == nil {
		return
	}
	lat, lng, ok := s.coordinateParams(w, r)
	if !ok {
		return
	}
	radius, err := intParam(r, "radius", 5, 1, 50)
	if err != nil {
		s.badRequest(w, r, "radius is in kilometres, 1 to 50.")
		return
	}
	limit, err := intParam(r, "limit", 10, 1, 100)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	// Coordinates are rounded in the log: the request is about a place, not a person,
	// but a precise point can still be someone's home.
	info(r).subject = strconv.FormatFloat(lat, 'f', 2, 64) + "," + strconv.FormatFloat(lng, 'f', 2, 64)

	// A box around the point narrows the search through the index; the exact distance
	// then decides.
	dLat := float64(radius) / 111.0
	dLng := dLat / math.Max(0.1, math.Cos(lat*math.Pi/180))
	rows, err := set.rows(`SELECT pincode, office, office_type, delivery, division, region, circle, district, state_code, state, lat, lng
		FROM pincodes WHERE lat BETWEEN ? AND ? AND lng BETWEEN ? AND ? AND lat IS NOT NULL`,
		lat-dLat, lat+dLat, lng-dLng, lng+dLng)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()

	found := []nearbyOffice{}
	for rows.Next() {
		var office nearbyOffice
		var officeType, division, region, circle, district, stateCode, state sql.NullString
		var officeLat, officeLng float64
		if err := rows.Scan(&office.Pincode, &office.Name, &officeType, &office.Delivery, &division, &region, &circle, &district, &stateCode, &state, &officeLat, &officeLng); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		distance := distanceKm(lat, lng, officeLat, officeLng)
		if distance > float64(radius) {
			continue
		}
		office.Type, office.Division, office.Region, office.Circle = str(officeType), str(division), str(region), str(circle)
		office.District, office.StateCode, office.State = str(district), str(stateCode), str(state)
		office.Latitude, office.Longitude = &officeLat, &officeLng
		office.DistanceKm = math.Round(distance*100) / 100
		found = append(found, office)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	sort.SliceStable(found, func(a, b int) bool { return found[a].DistanceKm < found[b].DistanceKm })
	total := len(found)
	if len(found) > limit {
		found = found[:limit]
	}
	zero := 0
	s.ok(w, r, found, meta{Total: &total, Limit: &limit, Offset: &zero, Source: s.source("pincode"),
		Note: "Positions come from the Department of Posts directory, where about one office in ten has none and some are approximate. The nearest office is not always the one whose pincode covers the point."}, cacheAPI)
}
