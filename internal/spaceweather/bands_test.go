package spaceweather

import (
	"fmt"
	"testing"
)

func summary(bands []Band) string {
	out := ""
	for _, band := range bands {
		out += fmt.Sprintf("%s %s/%s; ", band.Band, band.Day, band.Night)
	}
	return out
}

func TestEstimate(t *testing.T) {
	cases := []struct {
		name string
		in   Conditions
		want string
	}{
		{"solar minimum, quiet", Conditions{SolarFlux: 70, K: 1, XrayFlux: 1e-7},
			"80m-40m fair/good; 30m-20m good/poor; 17m-15m poor/poor; 12m-10m poor/poor; "},
		{"solar maximum, quiet", Conditions{SolarFlux: 180, K: 2, XrayFlux: 5e-7},
			"80m-40m fair/good; 30m-20m good/good; 17m-15m good/fair; 12m-10m good/poor; "},
		{"middling flux, unsettled", Conditions{SolarFlux: 120, K: 4, XrayFlux: 1e-6},
			"80m-40m fair/fair; 30m-20m good/fair; 17m-15m good/poor; 12m-10m fair/poor; "},
		{"geomagnetic storm", Conditions{SolarFlux: 180, K: 7, XrayFlux: 1e-6},
			"80m-40m poor/poor; 30m-20m poor/poor; 17m-15m poor/poor; 12m-10m poor/poor; "},
		{"M flare", Conditions{SolarFlux: 150, K: 2, XrayFlux: 2e-5},
			"80m-40m poor/good; 30m-20m fair/good; 17m-15m good/fair; 12m-10m good/poor; "},
		{"X flare", Conditions{SolarFlux: 200, K: 2, XrayFlux: 1.2e-4},
			"80m-40m poor/good; 30m-20m poor/good; 17m-15m poor/fair; 12m-10m poor/fair; "},
	}
	for _, c := range cases {
		if got := summary(Estimate(c.in)); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}
