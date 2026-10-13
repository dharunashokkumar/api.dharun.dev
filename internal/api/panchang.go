package api

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/panchang"
)

type panchangPlace struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	TimeZone  string  `json:"timeZone"`
	Pincode   string  `json:"pincode,omitempty"`
	Name      string  `json:"name,omitempty"`
}

type panchangView struct {
	Place panchangPlace `json:"place"`
	panchang.Day
}

// New Delhi, when a request names no place.
const defaultLatitude, defaultLongitude = 28.6139, 77.2090

// panchangPlace reads where the almanac is for: a pincode, or lat and lng, or New Delhi.
func (s *Server) panchangPlace(w http.ResponseWriter, r *http.Request) (panchang.Place, panchangPlace, bool) {
	query := r.URL.Query()
	zoneName := strings.TrimSpace(query.Get("tz"))
	if zoneName == "" {
		zoneName = "Asia/Kolkata"
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		s.badRequest(w, r, "tz is an IANA time zone name, such as Asia/Kolkata or Europe/London.")
		return panchang.Place{}, panchangPlace{}, false
	}
	view := panchangPlace{Latitude: defaultLatitude, Longitude: defaultLongitude, TimeZone: zoneName, Name: "New Delhi"}
	switch pincode := strings.TrimSpace(query.Get("pincode")); {
	case pincode != "":
		set := s.need(w, r, "pincode")
		if set == nil {
			return panchang.Place{}, panchangPlace{}, false
		}
		if !sixDigits.MatchString(pincode) {
			s.badRequest(w, r, "A pincode is six digits and does not start with 0.")
			return panchang.Place{}, panchangPlace{}, false
		}
		// The average position of the pincode's post offices that have one.
		var lat, lng *float64
		var name *string
		if err := set.row(`SELECT AVG(lat), AVG(lng), MIN(district) FROM pincodes WHERE pincode = ? AND lat IS NOT NULL`, pincode).Scan(&lat, &lng, &name); err != nil || lat == nil {
			s.notFound(w, r, "No position is known for pincode "+pincode+". Give lat and lng instead.")
			return panchang.Place{}, panchangPlace{}, false
		}
		view = panchangPlace{Latitude: round(*lat, 4), Longitude: round(*lng, 4), TimeZone: zoneName, Pincode: pincode}
		if name != nil {
			view.Name = *name
		}
	case query.Get("lat") != "" || query.Get("lng") != "":
		lat, errLat := strconv.ParseFloat(strings.TrimSpace(query.Get("lat")), 64)
		lng, errLng := strconv.ParseFloat(strings.TrimSpace(query.Get("lng")), 64)
		if errLat != nil || errLng != nil || lat < -66 || lat > 66 || lng < -180 || lng > 180 {
			s.badRequest(w, r, "Give lat (between -66 and 66, where the sun rises and sets every day) and lng in decimal degrees.")
			return panchang.Place{}, panchangPlace{}, false
		}
		view = panchangPlace{Latitude: lat, Longitude: lng, TimeZone: zoneName}
	}
	return panchang.Place{Latitude: view.Latitude, Longitude: view.Longitude, Zone: zone}, view, true
}

func round(value float64, places int) float64 {
	scale := 1.0
	for i := 0; i < places; i++ {
		scale *= 10
	}
	return float64(int64(value*scale+0.5)) / scale
}

func (s *Server) handlePanchang(w http.ResponseWriter, r *http.Request) {
	place, view, ok := s.panchangPlace(w, r)
	if !ok {
		return
	}
	raw := strings.ToLower(strings.TrimSpace(r.PathValue("date")))
	info(r).subject = raw
	var date time.Time
	cache := cacheLong
	if raw == "today" {
		date = time.Now().In(place.Zone)
		cache = "public, max-age=600"
	} else {
		parsed, err := parseDay(raw)
		if err != nil {
			s.badRequest(w, r, err.Error()+", or today.")
			return
		}
		if parsed.Year() < 1900 || parsed.Year() > 2100 {
			s.badRequest(w, r, "Dates from 1900 to 2100 are supported.")
			return
		}
		date = parsed
	}
	day, err := panchang.Compute(time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, place.Zone), place)
	if errors.Is(err, panchang.ErrNoSunrise) {
		s.fail(w, r, http.StatusUnprocessableEntity, "no_sunrise", "The sun does not rise and set there on that date, so there is no panchang day.")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, panchangView{Place: view, Day: day}, meta{Note: panchangNote}, cache)
}

const panchangNote = "Computed from the positions of the Sun and the Moon (NREL SPA and Meeus, within seconds of NASA JPL Horizons), " +
	"sidereal with the Lahiri ayanamsa as defined by the Swiss Ephemeris. Elements are given from sunrise to the next sunrise. " +
	"Published almanacs can differ by a minute or two, by up to about seven minutes at a sankranti, and traditional vakya " +
	"almanacs by hours. Festival dates are not given."

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "get_panchang",
		Description: "Hindu almanac (panchang) for a date and place: tithi, nakshatra, yoga, karana with their end times, sunrise, sunset, moonrise, moonset, Rahu kalam, Yamagandam, Gulika kalam, Abhijit muhurta, Tamil month and day, lunar month, Shaka and Vikram years.",
		InputSchema: schema([]string{"date"}, map[string]any{
			"date":    text("YYYY-MM-DD or today."),
			"pincode": text("Indian pincode of the place."),
			"lat":     map[string]any{"type": "number"},
			"lng":     map[string]any{"type": "number"},
			"tz":      text("IANA time zone. Default Asia/Kolkata."),
		}),
		path: func(args map[string]any) (string, error) {
			date, err := required(args, "date")
			return "/v1/panchang/" + url.PathEscape(date) + query(args, map[string]string{"pincode": "pincode", "lat": "lat", "lng": "lng", "tz": "tz"}), err
		},
	})
}
