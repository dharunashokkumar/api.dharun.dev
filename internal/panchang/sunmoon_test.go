package panchang

import (
	"math"
	"testing"
	"time"
)

// Reference values from Skyfield 1.55 with the JPL DE421 ephemeris, for Chennai
// (13.0827 N, 80.2707 E) on 2026-10-14: almanac.sunrise_sunset, dark_twilight_day,
// risings_and_settings for the moon, moon_phases and fraction_illuminated at noon IST.
func TestSunMoonAgainstJPL(t *testing.T) {
	day, err := ComputeSunMoon(time.Date(2026, 10, 14, 0, 0, 0, 0, ist), chennai)
	if err != nil {
		t.Fatal(err)
	}
	second := 2 * time.Second
	near(t, "sunrise", day.Sun.Sunrise, "2026-10-14T05:59:08+05:30", second)
	near(t, "sunset", day.Sun.Sunset, "2026-10-14T17:50:38+05:30", second)
	near(t, "civil dawn", *day.Sun.Civil.Dawn, "2026-10-14T05:37:44+05:30", second)
	near(t, "civil dusk", *day.Sun.Civil.Dusk, "2026-10-14T18:12:02+05:30", second)
	near(t, "nautical dawn", *day.Sun.Nautical.Dawn, "2026-10-14T05:12:57+05:30", second)
	near(t, "nautical dusk", *day.Sun.Nautical.Dusk, "2026-10-14T18:36:49+05:30", second)
	near(t, "astronomical dawn", *day.Sun.Astronomical.Dawn, "2026-10-14T04:48:13+05:30", second)
	near(t, "astronomical dusk", *day.Sun.Astronomical.Dusk, "2026-10-14T19:01:33+05:30", second)
	// Meeus's lunar theory and go-sampa's rising rule land within a couple of minutes.
	near(t, "moonrise", *day.Moon.Moonrise, "2026-10-14T08:56:55+05:30", 2*time.Minute)
	near(t, "moonset", *day.Moon.Moonset, "2026-10-14T20:25:16+05:30", 2*time.Minute)
	near(t, "first quarter", day.Moon.Next.FirstQuarter, "2026-10-18T21:42:41+05:30", 30*time.Second)
	near(t, "full moon", day.Moon.Next.FullMoon, "2026-10-26T09:41:48+05:30", 30*time.Second)
	near(t, "last quarter", day.Moon.Next.LastQuarter, "2026-11-02T01:58:27+05:30", 30*time.Second)
	near(t, "new moon", day.Moon.Next.NewMoon, "2026-11-09T12:32:06+05:30", 30*time.Second)
	if math.Abs(day.Moon.Illumination-12.97) > 0.5 {
		t.Errorf("illumination %.1f%%, want 13.0%%", day.Moon.Illumination)
	}
	if day.Moon.Phase != "waxing crescent" || day.Moon.Age < 3 || day.Moon.Age > 4 {
		t.Errorf("phase %q, age %.1f days", day.Moon.Phase, day.Moon.Age)
	}
	if day.Sun.DayLengthMinutes < 711 || day.Sun.DayLengthMinutes > 712 {
		t.Errorf("day length %.1f minutes", day.Sun.DayLengthMinutes)
	}
}

func TestPrincipalPhaseNamesItsDate(t *testing.T) {
	full, err := ComputeSunMoon(time.Date(2026, 10, 26, 0, 0, 0, 0, ist), chennai)
	if err != nil {
		t.Fatal(err)
	}
	if full.Moon.Phase != "full moon" || full.Moon.Illumination < 99 {
		t.Errorf("26 October 2026: %q, %.1f%% lit", full.Moon.Phase, full.Moon.Illumination)
	}
	// The next full moon counts from the start of the date, so it is the one that day.
	if full.Moon.Next.FullMoon[:10] != "2026-10-26" {
		t.Errorf("next full moon from that date: %s", full.Moon.Next.FullMoon)
	}
}
