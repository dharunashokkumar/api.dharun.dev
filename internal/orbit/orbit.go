// Package orbit predicts where a satellite is and when it passes over a place, from
// its two-line orbit elements (TLE) with the SGP4 model.
//
// SGP4 itself (sgp4.go, dspace.go, gravity.go, satellite.go) is copied from
// github.com/joshuaferrara/go-satellite under its BSD-2-Clause licence
// (LICENSE.go-satellite). It is copied rather than imported because of three things a
// server cannot live with in the original:
//
//   - a malformed element set ended the program through log.Fatal; elements are parsed
//     and checked here first, and errors are returned
//   - the epoch was truncated to whole seconds, which puts a low satellite several
//     kilometres along its track from where it is; the epoch is kept exact here
//   - time could only be given in whole seconds, and the package also carried a
//     Space-Track client with its own dependencies; both are left out
//
// The SGP4 code is otherwise unchanged and is checked against Vallado's sgp4 library.
package orbit

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	TWOPI   float64 = math.Pi * 2.0
	DEG2RAD float64 = math.Pi / 180.0
	RAD2DEG float64 = 180.0 / math.Pi
	XPDOTP  float64 = 1440.0 / (2.0 * math.Pi)
)

// Vector3 is a position (km) or velocity (km/s), in the TEME frame SGP4 works in.
type Vector3 struct {
	X, Y, Z float64
}

// gstime is the Greenwich mean sidereal time (IAU 1982) in radians, as SGP4 expects.
func gstime(jdut1 float64) float64 {
	tut1 := (jdut1 - 2451545.0) / 36525.0
	temp := -6.2e-6*tut1*tut1*tut1 + 0.093104*tut1*tut1 + (876600.0*3600+8640184.812866)*tut1 + 67310.54841
	temp = math.Mod(temp*DEG2RAD/240.0, TWOPI)
	if temp < 0.0 {
		temp += TWOPI
	}
	return temp
}

// julian turns a moment into a Julian date.
func julian(t time.Time) float64 {
	return float64(t.UnixNano())/86400e9 + 2440587.5
}

// Elements is a parsed and initialised TLE, ready to propagate. It is never changed
// after Parse, so one value can be shared by any number of requests.
type Elements struct {
	Name   string
	Number int
	Epoch  time.Time
	sat    Satellite
}

// ErrDecayed means the model puts the satellite inside the atmosphere at that time.
var ErrDecayed = errors.New("the satellite has decayed by then")

// checksum is the TLE line check digit: digits add their value, minus signs one.
func checksum(line string) int {
	sum := 0
	for _, ch := range line[:68] {
		switch {
		case ch >= '0' && ch <= '9':
			sum += int(ch - '0')
		case ch == '-':
			sum++
		}
	}
	return sum % 10
}

// Parse reads a two-line element set, checking its shape and check digits before any
// field is used.
func Parse(name, line1, line2 string) (*Elements, error) {
	line1, line2 = strings.TrimRight(line1, " \r"), strings.TrimRight(line2, " \r")
	if len(line1) != 69 || len(line2) != 69 || line1[0] != '1' || line2[0] != '2' {
		return nil, errors.New("not a two-line element set (each line has 69 characters, starting 1 and 2)")
	}
	for i, line := range []string{line1, line2} {
		if want := int(line[68] - '0'); line[68] < '0' || line[68] > '9' || checksum(line) != want {
			return nil, fmt.Errorf("line %d fails its check digit", i+1)
		}
	}
	if strings.TrimSpace(line1[2:7]) != strings.TrimSpace(line2[2:7]) {
		return nil, errors.New("the two lines are for different satellites")
	}
	var bad error
	float := func(field string) float64 {
		value, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
		if err != nil && bad == nil {
			bad = fmt.Errorf("field %q is not a number", field)
		}
		return value
	}
	// Fields written as an implied decimal with an exponent: " 12345-4" is 0.12345e-4.
	exponent := func(field string) float64 {
		field = strings.TrimSpace(field)
		if field == "" {
			return 0
		}
		sign := 1.0
		if field[0] == '-' || field[0] == '+' {
			if field[0] == '-' {
				sign = -1
			}
			field = field[1:]
		}
		cut := strings.LastIndexAny(field, "+-")
		if cut <= 0 {
			bad = fmt.Errorf("field %q is not in TLE exponent form", field)
			return 0
		}
		return sign * float("0."+field[:cut]+"e"+field[cut:])
	}

	number, err := strconv.Atoi(strings.TrimSpace(line1[2:7]))
	if err != nil {
		return nil, fmt.Errorf("catalogue number %q", line1[2:7])
	}
	var sat Satellite
	sat.Line1, sat.Line2 = line1, line2
	sat.satnum = int64(number)
	sat.whichconst = getGravConst(GravityWGS72)
	sat.epochyr = int64(float(line1[18:20]))
	sat.epochdays = float(line1[20:32])
	sat.ndot = float(line1[33:43])
	sat.nddot = exponent(line1[44:52])
	sat.bstar = exponent(line1[53:61])
	sat.inclo = float(line2[8:16])
	sat.nodeo = float(line2[17:25])
	sat.ecco = float("." + strings.TrimSpace(line2[26:33]))
	sat.argpo = float(line2[34:42])
	sat.mo = float(line2[43:51])
	sat.no = float(line2[52:63])
	if bad != nil {
		return nil, bad
	}
	if sat.no <= 0 || sat.ecco < 0 || sat.ecco >= 1 || sat.epochdays < 1 || sat.epochdays >= 367 {
		return nil, errors.New("the elements are out of range")
	}

	// The epoch: two-digit years 57 to 99 are 1957 to 1999. Day 1.0 is 1 January 0h.
	year := int(sat.epochyr) + 2000
	if sat.epochyr >= 57 {
		year = int(sat.epochyr) + 1900
	}
	startOfYear := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	sat.jdsatepoch = julian(startOfYear) + sat.epochdays - 1
	epoch := startOfYear.Add(time.Duration((sat.epochdays - 1) * 86400e9))

	sat.no /= XPDOTP
	sat.ndot /= XPDOTP * 1440.0
	sat.nddot /= XPDOTP * 1440.0 * 1440
	sat.inclo *= DEG2RAD
	sat.nodeo *= DEG2RAD
	sat.argpo *= DEG2RAD
	sat.mo *= DEG2RAD
	opsmode := "i"
	sgp4init(&opsmode, sat.jdsatepoch-2433281.5, &sat)
	if sat.Error != 0 {
		return nil, fmt.Errorf("SGP4 cannot start from these elements: %s", sat.ErrorStr)
	}
	return &Elements{Name: strings.TrimSpace(name), Number: number, Epoch: epoch, sat: sat}, nil
}

// Propagate gives the satellite's position and velocity at a moment, in TEME.
func (e *Elements) Propagate(t time.Time) (position, velocity Vector3, err error) {
	sat := e.sat // sgp4 writes into the record, so each call works on its own copy
	minutes := (julian(t) - sat.jdsatepoch) * 1440
	position, velocity = sgp4(&sat, minutes)
	if sat.Error != 0 {
		if sat.Error == 6 {
			return position, velocity, ErrDecayed
		}
		return position, velocity, errors.New(sat.ErrorStr)
	}
	return position, velocity, nil
}

// Period is the time for one orbit.
func (e *Elements) Period() time.Duration {
	return time.Duration(TWOPI / e.sat.no * 60e9)
}

// Inclination is the orbit's inclination in degrees.
func (e *Elements) Inclination() float64 { return e.sat.inclo * RAD2DEG }
