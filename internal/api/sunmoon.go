package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/panchang"
)

type sunMoonView struct {
	Place panchangPlace `json:"place"`
	panchang.SunMoon
}

// handleSunMoon answers the plain astronomy of a day, from the same code and the same
// place rules as the panchang.
func (s *Server) handleSunMoon(w http.ResponseWriter, r *http.Request) {
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
	day, err := panchang.ComputeSunMoon(date, place)
	if errors.Is(err, panchang.ErrNoSunrise) {
		s.fail(w, r, http.StatusUnprocessableEntity, "no_sunrise", "The sun does not rise and set there on that date.")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, sunMoonView{Place: view, SunMoon: day}, meta{Note: sunMoonNote}, cache)
}

const sunMoonNote = "Computed from the positions of the Sun and the Moon (NREL SPA and Meeus). Sunrise, sunset and twilight agree with " +
	"the JPL ephemeris to within seconds; moonrise and moonset to within a couple of minutes. Rising and setting are for the " +
	"upper edge of the disc with standard refraction, at sea level."

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "get_sun_and_moon",
		Description: "Sunrise, sunset, solar noon, day length, civil, nautical and astronomical twilight, moonrise, moonset, moon phase, illumination, age and the next new, first quarter, full and last quarter moons, for a date at an Indian pincode or any latitude and longitude.",
		InputSchema: schema([]string{"date"}, map[string]any{
			"date":    text("YYYY-MM-DD or today."),
			"pincode": text("Indian pincode of the place."),
			"lat":     map[string]any{"type": "number"},
			"lng":     map[string]any{"type": "number"},
			"tz":      text("IANA time zone. Default Asia/Kolkata."),
		}),
		path: func(args map[string]any) (string, error) {
			date, err := required(args, "date")
			return "/v1/sun-moon/" + url.PathEscape(date) + query(args, map[string]string{"pincode": "pincode", "lat": "lat", "lng": "lng", "tz": "tz"}), err
		},
	})
}
