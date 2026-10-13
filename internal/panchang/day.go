package panchang

import (
	"errors"
	"math"
	"time"

	sampa "github.com/hablullah/go-sampa"
)

// Element is one tithi, nakshatra, yoga or karana, with when it began and ends.
type Element struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	Tamil  string `json:"tamil,omitempty"`
	Paksha string `json:"paksha,omitempty"` // for tithis: shukla (waxing) or krishna (waning)
	Pada   int    `json:"pada,omitempty"`   // for the nakshatra at sunrise: quarter, 1 to 4
	Start  string `json:"start"`
	End    string `json:"end"`
}

// Period is a span of the day.
type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Month is a calendar month and the day of it.
type Month struct {
	Name  string `json:"name"`
	Tamil string `json:"tamil,omitempty"`
	Day   int    `json:"day,omitempty"`
}

// LunarMonth names the lunar month both ways it is reckoned.
type LunarMonth struct {
	// Amanta months run new moon to new moon (south and west India).
	Amanta string `json:"amanta"`
	// Purnimanta months run full moon to full moon (north India); in the waning half they
	// already carry the next month's name.
	Purnimanta string `json:"purnimanta"`
	// Adhika is set for an extra (leap) month: one in which the sun changes no sign.
	Adhika bool `json:"adhika"`
}

// Day is the almanac for one date at one place.
type Day struct {
	Date       string     `json:"date"`
	Weekday    Name       `json:"weekday"`
	Sunrise    string     `json:"sunrise"`
	Sunset     string     `json:"sunset"`
	Moonrise   *string    `json:"moonrise"`
	Moonset    *string    `json:"moonset"`
	Tithi      []Element  `json:"tithi"`
	Nakshatra  []Element  `json:"nakshatra"`
	Yoga       []Element  `json:"yoga"`
	Karana     []Element  `json:"karana"`
	SunSign    Name       `json:"sunSign"`
	MoonSign   Name       `json:"moonSign"`
	TamilMonth Month      `json:"tamilMonth"`
	TamilYear  Name       `json:"tamilYear"`
	Lunar      LunarMonth `json:"lunarMonth"`
	Shaka      int        `json:"shakaYear"`
	Vikram     int        `json:"vikramSamvat"`
	RahuKalam  Period     `json:"rahuKalam"`
	Yamagandam Period     `json:"yamagandam"`
	Gulika     Period     `json:"gulikaKalam"`
	Abhijit    Period     `json:"abhijitMuhurta"`
	Ayanamsa   float64    `json:"ayanamsa"`
}

// ErrNoSunrise is returned for places and dates where the sun does not rise or set.
var ErrNoSunrise = errors.New("the sun does not rise and set at that place on that date")

type sunTimes struct{ rise, set time.Time }

func sunOn(date time.Time, place Place) (sunTimes, error) {
	where := sampa.Location{Latitude: place.Latitude, Longitude: place.Longitude}
	midnight := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, place.Zone)
	events, err := sampa.GetSunEvents(midnight, where, options(midnight))
	if err != nil {
		return sunTimes{}, err
	}
	if events.Sunrise.IsZero() || events.Sunset.IsZero() {
		return sunTimes{}, ErrNoSunrise
	}
	return sunTimes{events.Sunrise.DateTime, events.Sunset.DateTime}, nil
}

// span is how many degrees one element of a kind covers, and how many there are.
type kind struct {
	f     angle
	width float64
	count int
	step  time.Duration
}

var (
	tithiKind     = kind{elongation, 12, 30, 2 * time.Hour}
	karanaKind    = kind{elongation, 6, 60, time.Hour}
	nakshatraKind = kind{moonAngle, 360.0 / 27, 27, 2 * time.Hour}
	yogaKind      = kind{sumAngle, 360.0 / 27, 27, 2 * time.Hour}
)

// sequence lists the elements of a kind from the one current at from to the one current
// just before until, each with its start and end.
func sequence(k kind, from, until time.Time, zone *time.Location) ([]int, []Period, error) {
	sun, moon, err := longitudes(from)
	if err != nil {
		return nil, nil, err
	}
	index := int(math.Floor(k.f(sun, moon) / k.width))
	start, err := crossingBefore(k.f, float64(index)*k.width, from, 3*24*time.Hour, k.step)
	if err != nil {
		return nil, nil, err
	}
	var indexes []int
	var periods []Period
	at := from
	for {
		end, err := crossing(k.f, float64(index+1)*k.width, at, 3*24*time.Hour, k.step)
		if err != nil {
			return nil, nil, err
		}
		indexes = append(indexes, index%k.count)
		periods = append(periods, Period{stamp(start, zone), stamp(end, zone)})
		if !end.Before(until) {
			break
		}
		start, at, index = end, end, (index+1)%k.count
	}
	return indexes, periods, nil
}

func stamp(t time.Time, zone *time.Location) string { return t.In(zone).Format(time.RFC3339) }

// Compute works out the almanac for the given date (its year, month and day are read in
// the place's time zone).
func Compute(date time.Time, place Place) (Day, error) {
	if place.Zone == nil {
		place.Zone = time.UTC
	}
	local := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, place.Zone)
	today, err := sunOn(local, place)
	if err != nil {
		return Day{}, err
	}
	tomorrow, err := sunOn(local.AddDate(0, 0, 1), place)
	if err != nil {
		return Day{}, err
	}
	sunrise, sunset := today.rise, today.set
	day := Day{
		Date: local.Format("2006-01-02"), Weekday: weekdayNames[local.Weekday()],
		Sunrise: stamp(sunrise, place.Zone), Sunset: stamp(sunset, place.Zone),
		Ayanamsa: math.Round(trueAyanamsa(sunrise)*1e6) / 1e6,
	}

	where := sampa.Location{Latitude: place.Latitude, Longitude: place.Longitude}
	if moon, err := sampa.GetMoonEvents(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, place.Zone), where, options(local)); err == nil {
		if !moon.Moonrise.IsZero() {
			rise := stamp(moon.Moonrise.DateTime, place.Zone)
			day.Moonrise = &rise
		}
		if !moon.Moonset.IsZero() {
			set := stamp(moon.Moonset.DateTime, place.Zone)
			day.Moonset = &set
		}
	}

	// The five elements, from sunrise to the next sunrise.
	until := tomorrow.rise
	indexes, periods, err := sequence(tithiKind, sunrise, until, place.Zone)
	if err != nil {
		return Day{}, err
	}
	for i, index := range indexes {
		name, paksha := tithiNames[index%15], "shukla"
		if index >= 15 {
			paksha = "krishna"
		}
		if index == 29 {
			name = amavasya
		}
		day.Tithi = append(day.Tithi, Element{Number: index + 1, Name: name.English, Tamil: name.Tamil, Paksha: paksha, Start: periods[i].Start, End: periods[i].End})
	}
	indexes, periods, err = sequence(nakshatraKind, sunrise, until, place.Zone)
	if err != nil {
		return Day{}, err
	}
	_, moonAtSunrise, _ := longitudes(sunrise)
	for i, index := range indexes {
		element := Element{Number: index + 1, Name: nakshatraNames[index].English, Tamil: nakshatraNames[index].Tamil, Start: periods[i].Start, End: periods[i].End}
		if i == 0 {
			element.Pada = int(math.Mod(moonAtSunrise, 360.0/27)/(360.0/108)) + 1
		}
		day.Nakshatra = append(day.Nakshatra, element)
	}
	indexes, periods, err = sequence(yogaKind, sunrise, until, place.Zone)
	if err != nil {
		return Day{}, err
	}
	for i, index := range indexes {
		day.Yoga = append(day.Yoga, Element{Number: index + 1, Name: yogaNames[index], Start: periods[i].Start, End: periods[i].End})
	}
	indexes, periods, err = sequence(karanaKind, sunrise, until, place.Zone)
	if err != nil {
		return Day{}, err
	}
	for i, index := range indexes {
		day.Karana = append(day.Karana, Element{Number: index + 1, Name: karanaName(index), Start: periods[i].Start, End: periods[i].End})
	}

	sunLongitude, moonLongitude, err := longitudes(sunrise)
	if err != nil {
		return Day{}, err
	}
	day.SunSign, day.MoonSign = rashiNames[int(sunLongitude/30)], rashiNames[int(moonLongitude/30)]

	if err := solarMonth(&day, local, sunset, place); err != nil {
		return Day{}, err
	}
	if err := lunarMonth(&day, local, sunrise); err != nil {
		return Day{}, err
	}

	// Rahu kalam and the rest: eighths of the daytime, by weekday.
	part := sunset.Sub(sunrise) / 8
	eighth := func(n int) Period {
		start := sunrise.Add(time.Duration(n-1) * part)
		return Period{stamp(start, place.Zone), stamp(start.Add(part), place.Zone)}
	}
	weekday := int(local.Weekday())
	day.RahuKalam, day.Yamagandam, day.Gulika = eighth(rahuKalamPart[weekday]), eighth(yamagandamPart[weekday]), eighth(gulikaPart[weekday])
	// Abhijit muhurta: the eighth of the fifteen muhurtas of the day, around midday.
	midday, muhurta := sunrise.Add(sunset.Sub(sunrise)/2), sunset.Sub(sunrise)/15
	day.Abhijit = Period{stamp(midday.Add(-muhurta/2), place.Zone), stamp(midday.Add(muhurta/2), place.Zone)}
	return day, nil
}

// solarMonth sets the Tamil month: it begins on the day the sun enters a sign, if it
// does so before sunset, or else on the next day.
func solarMonth(day *Day, local, sunset time.Time, place Place) error {
	sun, _, err := longitudes(sunset)
	if err != nil {
		return err
	}
	sign := int(sun / 30)
	entered, err := crossingBefore(sunAngle, float64(sign)*30, sunset, 33*24*time.Hour, 24*time.Hour)
	if err != nil {
		return err
	}
	enteredLocal := entered.In(place.Zone)
	start := time.Date(enteredLocal.Year(), enteredLocal.Month(), enteredLocal.Day(), 12, 0, 0, 0, place.Zone)
	that, err := sunOn(start, place)
	if err != nil {
		return err
	}
	if entered.After(that.set) {
		start = start.AddDate(0, 0, 1)
	}
	dayNumber := int(math.Round(local.Sub(start).Hours()/24)) + 1
	day.TamilMonth = Month{Name: tamilMonths[sign].English, Tamil: tamilMonths[sign].Tamil, Day: dayNumber}

	// The Tamil year begins with Chithirai, in April.
	year := local.Year()
	if local.Month() <= time.April && sign >= 8 {
		year--
	}
	day.TamilYear = tamilYears[((year-1987)%60+60)%60]
	return nil
}

// lunarMonth names the lunar month from the sign the sun is in at the new moon that
// begins it, and marks it adhika when the next new moon finds the sun in the same sign.
func lunarMonth(day *Day, local, sunrise time.Time) error {
	previous, err := crossingBefore(elongation, 0, sunrise, 31*24*time.Hour, 6*time.Hour)
	if err != nil {
		return err
	}
	next, err := crossing(elongation, 0, sunrise, 31*24*time.Hour, 6*time.Hour)
	if err != nil {
		return err
	}
	sunThen, _, err := longitudes(previous)
	if err != nil {
		return err
	}
	sunNext, _, err := longitudes(next)
	if err != nil {
		return err
	}
	signThen, signNext := int(sunThen/30), int(sunNext/30)
	index := (signThen + 1) % 12
	day.Lunar = LunarMonth{Amanta: lunarMonths[index], Purnimanta: lunarMonths[index], Adhika: signThen == signNext}
	if len(day.Tithi) > 0 && day.Tithi[0].Paksha == "krishna" {
		day.Lunar.Purnimanta = lunarMonths[(index+1)%12]
	}
	// The Shaka year begins with Chaitra, in March or April.
	day.Shaka = local.Year() - 78
	if index >= 9 && local.Month() <= time.April {
		day.Shaka--
	}
	day.Vikram = day.Shaka + 135
	return nil
}
