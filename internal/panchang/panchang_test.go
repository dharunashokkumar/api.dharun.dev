package panchang

import (
	"math"
	"testing"
	"time"
)

var ist = time.FixedZone("IST", 5*3600+1800)

var chennai = Place{Latitude: 13.0827, Longitude: 80.2707, Zone: ist}

func day(t *testing.T, date string) Day {
	t.Helper()
	parsed, _ := time.ParseInLocation("2006-01-02", date, ist)
	result, err := Compute(parsed, chennai)
	if err != nil {
		t.Fatalf("%s: %v", date, err)
	}
	return result
}

func at(t *testing.T, stamp string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func near(t *testing.T, what, got, want string, tolerance time.Duration) {
	t.Helper()
	if difference := at(t, got).Sub(at(t, want)); difference > tolerance || difference < -tolerance {
		t.Errorf("%s: %s, want %s (off by %s)", what, got, want, difference)
	}
}

func TestAyanamsa(t *testing.T) {
	// The Swiss Ephemeris value of the Lahiri ayanamsa at J2000: 23°51'25.532".
	j2000 := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC).Add(-64 * time.Second)
	if got := ayanamsa(j2000); math.Abs(got-23.857092) > 0.00002 {
		t.Errorf("mean Lahiri ayanamsa at J2000 = %.6f", got)
	}
}

// Reference figures: Sun and Moon positions from NASA JPL Horizons (DE441) put the end of
// Shukla Tritiya on 13 October 2026 at 23:28:14 IST; moonrise and moonset in Chennai at
// 08:03 and 19:40 (JPL, refracted). Sunrise and sunset agree with published almanacs.
func TestAgainstJPLHorizons(t *testing.T) {
	d := day(t, "2026-10-13")
	near(t, "sunrise", d.Sunrise, "2026-10-13T05:59:00+05:30", 60*time.Second)
	near(t, "sunset", d.Sunset, "2026-10-13T17:51:00+05:30", 60*time.Second)
	near(t, "moonrise", *d.Moonrise, "2026-10-13T08:03:00+05:30", 90*time.Second)
	near(t, "moonset", *d.Moonset, "2026-10-13T19:40:00+05:30", 90*time.Second)
	if d.Tithi[0].Name != "Tritiya" || d.Tithi[0].Paksha != "shukla" {
		t.Errorf("tithi at sunrise: %+v", d.Tithi[0])
	}
	near(t, "end of Tritiya", d.Tithi[0].End, "2026-10-13T23:28:14+05:30", 20*time.Second)
	if d.Nakshatra[0].Name != "Vishakha" || d.Yoga[0].Name != "Priti" || d.Karana[0].Name != "Taitila" {
		t.Errorf("elements: %s, %s, %s", d.Nakshatra[0].Name, d.Yoga[0].Name, d.Karana[0].Name)
	}
	if d.Weekday.English != "Mangalavara" || d.Lunar.Amanta != "Ashvina" || d.Shaka != 1948 || d.Vikram != 2083 {
		t.Errorf("calendar: %+v %+v %d %d", d.Weekday, d.Lunar, d.Shaka, d.Vikram)
	}
	// Each element ends where the next begins.
	for _, elements := range [][]Element{d.Tithi, d.Nakshatra, d.Yoga, d.Karana} {
		for i := 1; i < len(elements); i++ {
			if elements[i].Start != elements[i-1].End {
				t.Errorf("gap between %s and %s", elements[i-1].Name, elements[i].Name)
			}
		}
		if !at(t, elements[0].Start).Before(at(t, d.Sunrise)) && elements[0].Start != d.Sunrise {
			t.Errorf("%s should have begun by sunrise", elements[0].Name)
		}
	}
	// Rahu kalam on a Tuesday is the seventh eighth of the day.
	near(t, "Rahu kalam", d.RahuKalam.Start, "2026-10-13T14:53:00+05:30", 60*time.Second)
}

func TestTamilCalendar(t *testing.T) {
	cases := []struct {
		date, month string
		dayOfMonth  int
		year        string
	}{
		{"2026-01-14", "Thai", 1, "Vishvavasu"},      // Thai Pongal 2026
		{"2026-01-13", "Margazhi", 29, "Vishvavasu"}, // Margazhi 1 was 16 December 2025
		{"2026-04-14", "Chithirai", 1, "Parabhava"},  // Tamil New Year 2026
		{"2026-04-13", "Panguni", 30, "Vishvavasu"},  // Panguni 1 was 15 March 2026
		{"2025-04-14", "Chithirai", 1, "Vishvavasu"},
		{"2026-10-13", "Purattasi", 27, "Parabhava"},
	}
	for _, c := range cases {
		d := day(t, c.date)
		if d.TamilMonth.Name != c.month || d.TamilMonth.Day != c.dayOfMonth || d.TamilYear.English != c.year {
			t.Errorf("%s: %s %d, %s; want %s %d, %s", c.date, d.TamilMonth.Name, d.TamilMonth.Day, d.TamilYear.English, c.month, c.dayOfMonth, c.year)
		}
	}
}

func TestLunarCalendar(t *testing.T) {
	// 2026 has an extra Jyeshtha (Adhika masa), from mid-May to mid-June.
	if d := day(t, "2026-05-25"); d.Lunar.Amanta != "Jyeshtha" || !d.Lunar.Adhika {
		t.Errorf("Adhika Jyeshtha 2026: %+v", d.Lunar)
	}
	if d := day(t, "2026-06-25"); d.Lunar.Amanta != "Jyeshtha" || d.Lunar.Adhika {
		t.Errorf("the regular Jyeshtha after it: %+v", d.Lunar)
	}
	// Diwali 2026 falls on the Amavasya of 8 November.
	diwali := day(t, "2026-11-08")
	found := false
	for _, tithi := range diwali.Tithi {
		found = found || tithi.Name == "Amavasya"
	}
	if !found {
		t.Errorf("no Amavasya on 8 November 2026: %+v", diwali.Tithi)
	}
	// The Shaka year turns with Chaitra, in March 2026.
	if before, after := day(t, "2026-03-10"), day(t, "2026-03-25"); before.Shaka != 1947 || after.Shaka != 1948 {
		t.Errorf("Shaka years: %d then %d", before.Shaka, after.Shaka)
	}
	// In the waning half, purnimanta months already carry the next name.
	if d := day(t, "2026-01-14"); d.Lunar.Amanta != "Pausha" || d.Lunar.Purnimanta != "Magha" {
		t.Errorf("waning Pausha: %+v", d.Lunar)
	}
}

func TestKaranaNames(t *testing.T) {
	cases := map[int]string{0: "Kimstughna", 1: "Bava", 7: "Vishti", 8: "Bava", 56: "Vishti", 57: "Shakuni", 58: "Chatushpada", 59: "Naga"}
	for index, want := range cases {
		if got := karanaName(index); got != want {
			t.Errorf("karana %d = %s, want %s", index, got, want)
		}
	}
	if len(tamilYears) != 60 || tamilYears[38].English != "Vishvavasu" {
		t.Error("the Tamil year cycle is off")
	}
}

func TestPlaces(t *testing.T) {
	// Abroad: London keeps its own sunrise, and the date is read in its time zone.
	london, _ := time.LoadLocation("Europe/London")
	d, err := Compute(time.Date(2026, 6, 21, 0, 0, 0, 0, london), Place{Latitude: 51.5074, Longitude: -0.1278, Zone: london})
	if err != nil {
		t.Fatal(err)
	}
	near(t, "London midsummer sunrise", d.Sunrise, "2026-06-21T04:43:00+01:00", 3*time.Minute)
	// Far north in midsummer the sun does not set.
	if _, err := Compute(time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC), Place{Latitude: 78.2, Longitude: 15.6, Zone: time.UTC}); err == nil {
		t.Error("Svalbard in June should have no sunrise")
	}
}
