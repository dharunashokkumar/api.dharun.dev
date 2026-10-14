package api

import (
	"database/sql"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/spaceweather"
)

// HF propagation: NOAA's latest indices, refreshed hourly by the server, and a rule of
// thumb rating of each band from them.

type reading struct {
	Value    float64 `json:"value"`
	Observed string  `json:"observed"`
}

type dailyReading struct {
	Value int    `json:"value"`
	Date  string `json:"date"`
}

type kReading struct {
	Value    float64 `json:"value"`
	Level    string  `json:"level"`
	Observed string  `json:"observed"`
}

type xrayReading struct {
	Class    string  `json:"class"`
	Flux     float64 `json:"flux"`
	Observed string  `json:"observed"`
}

type flareReading struct {
	Class string `json:"class"`
	Peak  string `json:"peak"`
}

type noaaScales struct {
	RadioBlackout    string `json:"radioBlackout"`
	SolarRadiation   string `json:"solarRadiation"`
	GeomagneticStorm string `json:"geomagneticStorm"`
	Observed         string `json:"observed"`
}

type kHistory struct {
	Start string  `json:"start"`
	Kp    float64 `json:"kp"`
}

type propagationView struct {
	SolarFlux      reading             `json:"solarFlux"`
	SunspotNumber  *dailyReading       `json:"sunspotNumber"`
	K              kReading            `json:"kIndex"`
	A              *dailyReading       `json:"aIndex"`
	Xray           xrayReading         `json:"xray"`
	LatestFlare    *flareReading       `json:"latestFlare"`
	SolarWindSpeed *reading            `json:"solarWindSpeed"`
	Scales         *noaaScales         `json:"noaaScales"`
	KHistory       []kHistory          `json:"kHistory"`
	Bands          []spaceweather.Band `json:"bands"`
	Rules          []string            `json:"rules"`
}

// kLevel names a K-index the way NOAA does: G1 to G5 storms from 5 up.
func kLevel(k float64) string {
	switch {
	case k >= 9:
		return "extreme storm (G5)"
	case k >= 8:
		return "severe storm (G4)"
	case k >= 7:
		return "strong storm (G3)"
	case k >= 6:
		return "moderate storm (G2)"
	case k >= 5:
		return "minor storm (G1)"
	case k >= 4:
		return "active"
	case k >= 3:
		return "unsettled"
	}
	return "quiet"
}

func (s *Server) handlePropagation(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "space-weather")
	if set == nil {
		return
	}
	fail := func() { s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.") }
	readingRows, err := set.rows(`SELECT name, value, text, observed FROM readings`)
	if err != nil {
		fail()
		return
	}
	readings := map[string]struct {
		value    float64
		text     sql.NullString
		observed string
	}{}
	for readingRows.Next() {
		var name, observed string
		var value float64
		var text sql.NullString
		if err := readingRows.Scan(&name, &value, &text, &observed); err != nil {
			readingRows.Close()
			fail()
			return
		}
		readings[name] = struct {
			value    float64
			text     sql.NullString
			observed string
		}{value, text, observed}
	}
	readingRows.Close()

	view := propagationView{Rules: spaceweather.Rules, KHistory: []kHistory{}}
	flux, haveFlux := readings["solar_flux"]
	xray, haveXray := readings["xray"]
	if !haveFlux || !haveXray {
		fail() // the build refuses a file without these
		return
	}
	view.SolarFlux = reading{flux.value, flux.observed}
	view.Xray = xrayReading{Class: xray.text.String, Flux: xray.value, Observed: xray.observed}
	if flare, ok := readings["xray_latest_flare"]; ok {
		view.LatestFlare = &flareReading{Class: flare.text.String, Peak: flare.observed}
	}
	if wind, ok := readings["solar_wind_speed"]; ok {
		view.SolarWindSpeed = &reading{wind.value, wind.observed}
	}
	if radio, ok := readings["scale_r"]; ok {
		scale := func(name, letter string) string {
			return letter + strconv.Itoa(int(readings[name].value))
		}
		view.Scales = &noaaScales{RadioBlackout: scale("scale_r", "R"), SolarRadiation: scale("scale_s", "S"),
			GeomagneticStorm: scale("scale_g", "G"), Observed: radio.observed}
	}

	var sunspots dailyReading
	var sunspotValue sql.NullInt64
	if err := set.row(`SELECT date, sunspots FROM daily WHERE sunspots IS NOT NULL ORDER BY date DESC LIMIT 1`).Scan(&sunspots.Date, &sunspotValue); err == nil {
		sunspots.Value = int(sunspotValue.Int64)
		view.SunspotNumber = &sunspots
	}

	// The K-index every three hours; the last day of them is returned, newest last.
	kRows, err := set.rows(`SELECT time, kp, a FROM kp ORDER BY time DESC LIMIT 24`)
	if err != nil {
		fail()
		return
	}
	aByDay := map[string][]int{}
	var days []string
	for kRows.Next() {
		var at string
		var kp float64
		var a int
		if err := kRows.Scan(&at, &kp, &a); err != nil {
			kRows.Close()
			fail()
			return
		}
		if len(view.KHistory) < 8 {
			view.KHistory = append([]kHistory{{at, kp}}, view.KHistory...)
		}
		day := strings.SplitN(at, "T", 2)[0]
		if _, seen := aByDay[day]; !seen {
			days = append(days, day)
		}
		aByDay[day] = append(aByDay[day], a)
	}
	kRows.Close()
	if len(view.KHistory) == 0 {
		fail()
		return
	}
	latest := view.KHistory[len(view.KHistory)-1]
	view.K = kReading{Value: latest.Kp, Level: kLevel(latest.Kp), Observed: latest.Start}
	// The daily A index is the mean of the day's eight three-hourly a values; the
	// newest day with all eight is given.
	for _, day := range days {
		if values := aByDay[day]; len(values) == 8 {
			sum := 0
			for _, value := range values {
				sum += value
			}
			view.A = &dailyReading{Value: int(math.Round(float64(sum) / 8)), Date: day}
			break
		}
	}

	view.Bands = spaceweather.Estimate(spaceweather.Conditions{SolarFlux: flux.value, K: latest.Kp, XrayFlux: xray.value})
	s.ok(w, r, view, meta{Source: s.source("space-weather"), Note: propagationNote}, "public, max-age=600")
}

const propagationNote = "Indices from NOAA SWPC, fetched every hour (meta.source.retrieved says when). The band ratings are a rule of thumb " +
	"from the solar flux, the K-index and the X-ray flux, listed in rules; real conditions depend on the path, season, time of day and antenna. " +
	"Each kIndex reading covers three hours from the time given."

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "get_hf_propagation",
		Description: "Current space weather from NOAA (solar flux, sunspot number, K and A indices, X-ray class, NOAA R/S/G scales) and a rule of thumb good/fair/poor rating of HF amateur radio bands by day and night.",
		InputSchema: schema([]string{}, map[string]any{}),
		path:        func(map[string]any) (string, error) { return "/v1/propagation", nil },
	})
}
