package panchang

import (
	"math"
	"time"

	sampa "github.com/hablullah/go-sampa"
)

// SunMoon is the plain astronomical day at a place, without the almanac: when the sun
// and moon rise and set, twilight, and the moon's phase.
type SunMoon struct {
	Date string  `json:"date"`
	Sun  SunDay  `json:"sun"`
	Moon MoonDay `json:"moon"`
}

// SunDay is the sun's day. Twilight times are null where the sun does not get that far
// below the horizon.
type SunDay struct {
	Sunrise          string   `json:"sunrise"`
	Sunset           string   `json:"sunset"`
	SolarNoon        string   `json:"solarNoon"`
	DayLengthMinutes float64  `json:"dayLengthMinutes"`
	NoonElevation    float64  `json:"noonElevation"`
	Civil            Twilight `json:"civilTwilight"`
	Nautical         Twilight `json:"nauticalTwilight"`
	Astronomical     Twilight `json:"astronomicalTwilight"`
}

// Twilight is when the sun is a given depth below the horizon, morning and evening.
type Twilight struct {
	Dawn *string `json:"dawn"`
	Dusk *string `json:"dusk"`
}

// MoonDay is the moon's day. Moonrise or moonset is null on the day about once a month
// when the moon does not rise or set.
type MoonDay struct {
	Moonrise *string `json:"moonrise"`
	Moonset  *string `json:"moonset"`
	// Phase is new, waxing crescent, first quarter, waxing gibbous, full, waning gibbous,
	// last quarter or waning crescent. A principal phase names the whole date it falls on.
	Phase string `json:"phase"`
	// Illumination is the lit fraction of the disc at local noon, in percent.
	Illumination float64 `json:"illumination"`
	// Age is the days since the last new moon, at local noon.
	Age  float64 `json:"ageDays"`
	Next Phases  `json:"nextPhases"`
}

// Phases are the next moments of each principal phase after the start of the date.
type Phases struct {
	NewMoon      string `json:"newMoon"`
	FirstQuarter string `json:"firstQuarter"`
	FullMoon     string `json:"fullMoon"`
	LastQuarter  string `json:"lastQuarter"`
}

var phaseNames = []string{"new moon", "first quarter", "full moon", "last quarter"}

// ComputeSunMoon works out the sun and moon for the given date (read in the place's
// time zone).
func ComputeSunMoon(date time.Time, place Place) (SunMoon, error) {
	if place.Zone == nil {
		place.Zone = time.UTC
	}
	midnight := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, place.Zone)
	noon := midnight.Add(12 * time.Hour)
	where := sampa.Location{Latitude: place.Latitude, Longitude: place.Longitude}

	depth := func(name string, degrees float64, before bool) sampa.CustomSunEvent {
		return sampa.CustomSunEvent{Name: name, BeforeTransit: before, Elevation: func(sampa.SunPosition) float64 { return degrees }}
	}
	sun, err := sampa.GetSunEvents(midnight, where, options(noon),
		depth("civilDawn", -6, true), depth("civilDusk", -6, false),
		depth("nauticalDawn", -12, true), depth("nauticalDusk", -12, false),
		depth("astronomicalDawn", -18, true), depth("astronomicalDusk", -18, false))
	if err != nil {
		return SunMoon{}, err
	}
	if sun.Sunrise.IsZero() || sun.Sunset.IsZero() {
		return SunMoon{}, ErrNoSunrise
	}
	optional := func(position sampa.SunPosition) *string {
		if position.IsZero() {
			return nil
		}
		value := stamp(position.DateTime, place.Zone)
		return &value
	}
	twilight := func(kind string) Twilight {
		return Twilight{Dawn: optional(sun.Others[kind+"Dawn"]), Dusk: optional(sun.Others[kind+"Dusk"])}
	}
	result := SunMoon{
		Date: midnight.Format("2006-01-02"),
		Sun: SunDay{
			Sunrise: stamp(sun.Sunrise.DateTime, place.Zone), Sunset: stamp(sun.Sunset.DateTime, place.Zone),
			SolarNoon:        stamp(sun.Transit.DateTime, place.Zone),
			DayLengthMinutes: math.Round(sun.Sunset.DateTime.Sub(sun.Sunrise.DateTime).Minutes()*10) / 10,
			NoonElevation:    math.Round(sun.Transit.TopocentricElevationAngle*100) / 100,
			Civil:            twilight("civil"), Nautical: twilight("nautical"), Astronomical: twilight("astronomical"),
		},
	}

	moon, err := sampa.GetMoonEvents(midnight, where, options(noon))
	if err != nil {
		return SunMoon{}, err
	}
	if !moon.Moonrise.IsZero() {
		rise := stamp(moon.Moonrise.DateTime, place.Zone)
		result.Moon.Moonrise = &rise
	}
	if !moon.Moonset.IsZero() {
		set := stamp(moon.Moonset.DateTime, place.Zone)
		result.Moon.Moonset = &set
	}
	atNoon, err := sampa.GetMoonPosition(noon, where, options(noon))
	if err != nil {
		return SunMoon{}, err
	}
	// go-sampa's "percent" is the lit fraction, 0 to 1 (Meeus 48.1 and 48.3).
	result.Moon.Illumination = math.Round(atNoon.PercentIlluminated*1000) / 10

	// The principal phases are the moments the moon's elongation from the sun reaches
	// 0, 90, 180 and 270 degrees.
	next := make([]time.Time, 4)
	for i := range next {
		if next[i], err = crossing(elongation, float64(i)*90, midnight, 32*24*time.Hour, 24*time.Hour); err != nil {
			return SunMoon{}, err
		}
	}
	result.Moon.Next = Phases{
		NewMoon: stamp(next[0], place.Zone), FirstQuarter: stamp(next[1], place.Zone),
		FullMoon: stamp(next[2], place.Zone), LastQuarter: stamp(next[3], place.Zone),
	}
	newMoon, err := crossingBefore(elongation, 0, noon, 31*24*time.Hour, 24*time.Hour)
	if err != nil {
		return SunMoon{}, err
	}
	result.Moon.Age = math.Round(noon.Sub(newMoon).Hours()/24*10) / 10

	nextMidnight := midnight.AddDate(0, 0, 1)
	for i, at := range next {
		if at.Before(nextMidnight) {
			result.Moon.Phase = phaseNames[i]
		}
	}
	if result.Moon.Phase == "" {
		sunLongitude, moonLongitude, err := longitudes(noon)
		if err != nil {
			return SunMoon{}, err
		}
		switch e := elongation(sunLongitude, moonLongitude); {
		case e < 90:
			result.Moon.Phase = "waxing crescent"
		case e < 180:
			result.Moon.Phase = "waxing gibbous"
		case e < 270:
			result.Moon.Phase = "waning gibbous"
		default:
			result.Moon.Phase = "waning crescent"
		}
	}
	return result, nil
}
