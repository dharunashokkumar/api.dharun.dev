// Package spaceweather turns solar and geomagnetic indices into a rough estimate of
// shortwave (HF) radio conditions per band, day and night.
//
// It is a rule of thumb, of the kind printed on amateur radio propagation banners, not a
// model of the ionosphere: real conditions depend on the path, the season, the time of
// day at both ends and the antenna. The thresholds below are written out so a caller can
// see exactly what the estimate rests on.
package spaceweather

// Conditions are the indices the estimate reads.
type Conditions struct {
	// SolarFlux is the 10.7 cm solar radio flux in solar flux units. It tracks the
	// ionisation of the F layer, and with it the highest usable frequency.
	SolarFlux float64
	// K is the latest planetary K-index, 0 to 9. A disturbed magnetic field weakens the
	// F layer, most of all on paths in darkness and near the poles.
	K float64
	// XrayFlux is the 0.1 to 0.8 nm X-ray flux in W/m². Flares ionise the D layer on the
	// sunlit side, which absorbs the lower bands first and in a large flare all of them.
	XrayFlux float64
}

// Band is the estimate for one group of amateur bands.
type Band struct {
	Band  string `json:"band"`
	Day   string `json:"day"`
	Night string `json:"night"`
}

const (
	poor = iota
	fair
	good
)

var levelNames = []string{"poor", "fair", "good"}

// Rules lists the estimate's rules in words, for the documentation and the response.
var Rules = []string{
	"80m-40m: fair by day (D-layer absorption), good at night.",
	"30m-20m: good by day; at night good from a solar flux of 120, fair from 80, else poor.",
	"17m-15m: by day good from a solar flux of 110, fair from 80, else poor; at night fair from 150, else poor.",
	"12m-10m: by day good from a solar flux of 150, fair from 110, else poor; at night fair from 200, else poor.",
	"A K-index of 4 lowers every night rating one step; 5 or more lowers every rating one step, and 7 or more two.",
	"An M-class X-ray flux lowers the day rating of 80m-40m and 30m-20m one step; X-class makes every day rating poor.",
}

// Estimate rates each band group by day and by night.
func Estimate(c Conditions) []Band {
	sfi := c.SolarFlux
	from := func(thresholds ...float64) int {
		// thresholds are for good then fair; a missing one means that level is not reached.
		level := poor
		for i, threshold := range thresholds {
			if sfi >= threshold {
				level = max(level, len(thresholds)-i)
			}
		}
		return level
	}
	type rating struct {
		band       string
		day, night int
		lowBand    bool
	}
	ratings := []rating{
		{"80m-40m", fair, good, true},
		{"30m-20m", good, from(120, 80), true},
		{"17m-15m", from(110, 80), min(from(150), fair), false},
		{"12m-10m", from(150, 110), min(from(200), fair), false},
	}

	var bands []Band
	for _, r := range ratings {
		day, night := r.day, r.night
		switch {
		case c.K >= 7:
			day, night = day-2, night-2
		case c.K >= 5:
			day, night = day-1, night-1
		case c.K >= 4:
			night--
		}
		switch {
		case c.XrayFlux >= 1e-4:
			day = poor
		case c.XrayFlux >= 1e-5 && r.lowBand:
			day--
		}
		bands = append(bands, Band{Band: r.band, Day: levelNames[max(day, poor)], Night: levelNames[max(night, poor)]})
	}
	return bands
}
