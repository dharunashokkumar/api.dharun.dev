// Package panchang computes the Hindu almanac for a date and place from the positions of
// the Sun and the Moon: the five elements (tithi, nakshatra, yoga, karana, weekday) with
// the moments they change, sunrise, sunset, moonrise and moonset, the inauspicious periods,
// and the solar (Tamil) and lunar months.
//
// Positions come from NREL's Solar Position Algorithm and Meeus's lunar theory, through
// go-sampa; the zodiac is sidereal with the Lahiri (Chitrapaksha) ayanamsa used by the
// Indian national calendar. Results follow modern (drik) reckoning; traditional
// vakya almanacs can differ by hours, and festival rules are not applied.
package panchang

import (
	"errors"
	"math"
	"time"

	sampa "github.com/hablullah/go-sampa"
)

// Place is where the almanac is for. Sunrise, and with it every element, depends on it.
type Place struct {
	Latitude, Longitude float64
	Zone                *time.Location
}

// deltaT is the difference between terrestrial and universal time, in seconds. Within
// this century it has stayed close to 69 s; older dates use Espenak and Meeus's fit.
func deltaT(t time.Time) float64 {
	y := float64(t.Year()) + float64(t.YearDay())/365.25
	switch {
	case y >= 2005 && y < 2100:
		return 64.7 + 0.2*(y-2005)
	case y >= 1986 && y < 2005:
		u := y - 2000
		return 63.86 + 0.3345*u - 0.060374*u*u + 0.0017275*u*u*u + 0.000651814*u*u*u*u + 0.00002373599*u*u*u*u*u
	case y >= 1961 && y < 1986:
		u := y - 1975
		return 45.45 + 1.067*u - u*u/260 - u*u*u/718
	case y >= 1900 && y < 1961:
		u := y - 1950
		return 29.07 + 0.407*u - u*u/233 + u*u*u/2547
	}
	return 69
}

func options(t time.Time) *sampa.Options { return &sampa.Options{DeltaT: deltaT(t)} }

// trueAyanamsa is the ayanamsa including nutation, as subtracted from apparent positions.
func trueAyanamsa(t time.Time) float64 {
	s, err := sampa.GetSunPosition(t, sampa.Location{}, options(t))
	if err != nil {
		return ayanamsa(t)
	}
	return ayanamsa(t) + s.NutationLongitude
}

// ayanamsa is the mean Lahiri ayanamsa in degrees: 23°15'00.658" at 1956-03-21 0h TT (as
// adopted by the Calendar Reform Committee), carried by the general precession in
// longitude (Lieske 1977).
func ayanamsa(t time.Time) float64 {
	const t0 = 2435553.5 // 1956-03-21 0h, Julian day
	jd := julianDay(t)
	century := func(jd float64) float64 { return (jd - 2451545.0) / 36525 }
	precession := func(T float64) float64 { return (5029.0966*T + 1.11113*T*T) / 3600 }
	return 23.250182778 - 0.004658035 + precession(century(jd)) - precession(century(t0))
}

func julianDay(t time.Time) float64 {
	return float64(t.UTC().UnixNano())/86400e9 + 2440587.5
}

func normalise(degrees float64) float64 {
	degrees = math.Mod(degrees, 360)
	if degrees < 0 {
		degrees += 360
	}
	return degrees
}

// longitudes returns the sidereal longitudes of the Sun and the Moon at a moment.
func longitudes(t time.Time) (sun, moon float64, err error) {
	where := sampa.Location{} // geocentric longitudes do not depend on the observer
	s, err := sampa.GetSunPosition(t, where, options(t))
	if err != nil {
		return 0, 0, err
	}
	m, err := sampa.GetMoonPosition(t, where, options(t))
	if err != nil {
		return 0, 0, err
	}
	// Apparent longitudes include nutation; so does the true ayanamsa, so it cancels and
	// sidereal positions stay fixed against the stars.
	a := ayanamsa(t) + s.NutationLongitude
	return normalise(s.ApparentLongitude - a), normalise(m.ApparentLongitude - a), nil
}

// An angle is one of the quantities the elements are read from, in degrees.
type angle func(sun, moon float64) float64

var (
	elongation angle = func(sun, moon float64) float64 { return normalise(moon - sun) }
	moonAngle  angle = func(_, moon float64) float64 { return moon }
	sumAngle   angle = func(sun, moon float64) float64 { return normalise(sun + moon) }
	sunAngle   angle = func(sun, _ float64) float64 { return sun }
)

var errNoCrossing = errors.New("no crossing found")

// crossing finds the first moment after start when the angle reaches target (mod 360),
// searching up to limit ahead, to within a second. The angles only grow, so each step
// checks whether the remaining distance to the target has been covered.
func crossing(f angle, target float64, start time.Time, limit, step time.Duration) (time.Time, error) {
	value := func(t time.Time) (float64, error) {
		sun, moon, err := longitudes(t)
		return f(sun, moon), err
	}
	first, err := value(start)
	if err != nil {
		return time.Time{}, err
	}
	distance := normalise(target - first) // how far the angle still has to go
	if distance == 0 {
		distance = 360
	}
	travelled := func(t time.Time) (float64, error) {
		v, err := value(t)
		return normalise(v - first), err
	}
	low, high := start, start
	for {
		high = low.Add(step)
		if high.Sub(start) > limit {
			return time.Time{}, errNoCrossing
		}
		moved, err := travelled(high)
		if err != nil {
			return time.Time{}, err
		}
		if moved >= distance {
			break
		}
		low = high
	}
	for high.Sub(low) > time.Second {
		middle := low.Add(high.Sub(low) / 2)
		moved, err := travelled(middle)
		if err != nil {
			return time.Time{}, err
		}
		if moved >= distance {
			high = middle
		} else {
			low = middle
		}
	}
	return high.Truncate(time.Second), nil
}

// crossingBefore finds the last moment before end, within window, when the angle
// reached target.
func crossingBefore(f angle, target float64, end time.Time, window, step time.Duration) (time.Time, error) {
	found := time.Time{}
	from := end.Add(-window)
	for {
		next, err := crossing(f, target, from, end.Sub(from), step)
		if err != nil || next.After(end) {
			break
		}
		found, from = next, next.Add(time.Minute)
	}
	if found.IsZero() {
		return found, errNoCrossing
	}
	return found, nil
}
