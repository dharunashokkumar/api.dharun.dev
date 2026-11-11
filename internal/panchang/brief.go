package panchang

import (
	"math"
	"time"
)

// The pieces below let a caller that needs a whole year (the festival calendar) work
// from a few positions a day instead of the full almanac, which searches for every
// element's start and end and takes far longer.

// Positions returns the sidereal longitudes of the sun and the moon, in degrees.
func Positions(t time.Time) (sun, moon float64, err error) { return longitudes(t) }

// TithiAt is the tithi (1 to 30; 15 is purnima, 30 amavasya) at a moment.
func TithiAt(t time.Time) (int, error) {
	sun, moon, err := longitudes(t)
	if err != nil {
		return 0, err
	}
	return int(math.Floor(elongation(sun, moon)/12)) + 1, nil
}

// NakshatraAt is the moon's nakshatra (1 to 27) at a moment.
func NakshatraAt(t time.Time) (int, error) {
	_, moon, err := longitudes(t)
	if err != nil {
		return 0, err
	}
	return int(math.Floor(moon/(360.0/27))) + 1, nil
}

// SunTimes returns sunrise and sunset on a date at a place.
func SunTimes(date time.Time, place Place) (rise, set time.Time, err error) {
	if place.Zone == nil {
		place.Zone = time.UTC
	}
	times, err := sunOn(time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, place.Zone), place)
	return times.rise, times.set, err
}

// NewMoons returns the moments of new moon from start to end.
func NewMoons(start, end time.Time) ([]time.Time, error) {
	var out []time.Time
	at := start
	for {
		next, err := crossing(elongation, 0, at, 32*24*time.Hour, 6*time.Hour)
		if err != nil {
			return nil, err
		}
		if next.After(end) {
			return out, nil
		}
		out = append(out, next)
		at = next.Add(time.Hour)
	}
}

// Sankrantis returns the moments the sun enters each sidereal sign from start to end,
// with the sign entered (0 is Mesha, the start of the Tamil Chithirai).
func Sankrantis(start, end time.Time) ([]time.Time, []int, error) {
	var times []time.Time
	var signs []int
	at := start
	for {
		sun, _, err := longitudes(at)
		if err != nil {
			return nil, nil, err
		}
		sign := (int(sun/30) + 1) % 12
		next, err := crossing(sunAngle, float64(sign)*30, at, 33*24*time.Hour, 24*time.Hour)
		if err != nil {
			return nil, nil, err
		}
		if next.After(end) {
			return times, signs, nil
		}
		times, signs = append(times, next), append(signs, sign)
		at = next.Add(time.Hour)
	}
}

// LunarMonthName is the name of the amanta lunar month that begins when the sun is in
// the given sign at the new moon (sign 11, Meena, gives Chaitra).
func LunarMonthName(sign int) string { return lunarMonths[(sign+1)%12] }

// TamilMonthName is the Tamil month of a sign (0, Mesha, is Chithirai).
func TamilMonthName(sign int) (string, string) {
	return tamilMonths[sign%12].English, tamilMonths[sign%12].Tamil
}
