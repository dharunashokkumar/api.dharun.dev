package orbit

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The element sets are the ones in the API's test data, as CelesTrak published them on
// 2026-10-14. Reference values come from Vallado's sgp4 library 2.27 (Python, WGS 72,
// "i" mode) and Skyfield 1.55 with JPL DE421.

func load(t *testing.T) map[string]*Elements {
	t.Helper()
	all := map[string]*Elements{}
	for _, file := range []string{"stations.tle", "amateur.tle"} {
		f, err := os.Open(filepath.Join("..", "api", "testdata", "data", "satellites", file))
		if err != nil {
			t.Fatal(err)
		}
		var lines []string
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		f.Close()
		for i := 0; i+2 < len(lines); i += 3 {
			elements, err := Parse(lines[i], lines[i+1], lines[i+2])
			if err != nil {
				t.Fatalf("%s: %v", lines[i], err)
			}
			all[elements.Name] = elements
		}
	}
	return all
}

func TestPropagateMatchesVallado(t *testing.T) {
	satellites := load(t)
	cases := []struct {
		name    string
		minutes float64
		x, y, z float64
	}{
		{"ISS (ZARYA)", 0, 1987.057511, 6502.989259, 0.001597},
		{"ISS (ZARYA)", 720, 3973.313780, -1426.450285, -5328.681874},
		{"ISS (ZARYA)", 4320, -3650.364209, -5734.966689, 95.822312},
		{"OSCAR 7 (AO-7)", 1440, 3783.571620, -3668.931255, -5787.348871},
		// Deep space: a highly eccentric orbit and a geostationary one.
		{"PHASE 3B (AO-10)", 720, -28328.983875, -13733.255249, 1344.047848},
		{"PHASE 3B (AO-10)", 4320, -3631.400567, -14959.438456, 6228.139508},
		{"ES'HAIL 2", 1440, -12141.522848, 40379.332991, -11.972059},
	}
	for _, c := range cases {
		elements := satellites[c.name]
		at := elements.Epoch.Add(time.Duration(c.minutes * 60e9))
		p, _, err := elements.Propagate(at)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if off := math.Sqrt(math.Pow(p.X-c.x, 2) + math.Pow(p.Y-c.y, 2) + math.Pow(p.Z-c.z, 2)); off > 0.001 {
			t.Errorf("%s at %+.0f min: off by %.4f km (%v)", c.name, c.minutes, off, p)
		}
	}
}

func TestParseRefusesDamagedElements(t *testing.T) {
	l1 := "1 25544U 98067A   26286.82178372  .00006592  00000+0  12831-3 0  9991"
	l2 := "2 25544  51.6325  73.0089 0006710 259.3258 100.6975 15.48855783590140"
	if _, err := Parse("ISS", l1, l2); err != nil {
		t.Fatalf("good elements: %v", err)
	}
	for name, lines := range map[string][2]string{
		"check digit":    {strings.Replace(l1, "26286.82", "26286.83", 1), l2},
		"short line":     {l1[:60], l2},
		"lines swapped":  {l2, l1},
		"two satellites": {l1, strings.Replace(l2, "25544", "25545", 1)[:68] + "1"},
		"not a number":   {l1, l2[:8] + " 51.6x25" + l2[16:68] + "0"},
	} {
		if _, err := Parse("ISS", lines[0], lines[1]); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPassesMatchSkyfield(t *testing.T) {
	iss := load(t)["ISS (ZARYA)"]
	chennai := Observer{Latitude: 13.0827, Longitude: 80.2707}
	from := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	passes, err := iss.Passes(chennai, PassOptions{From: from, Until: from.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		rise, top, set string
		elevation      float64
	}{
		{"00:29:48", "00:31:11", "00:32:33", 0.63},
		{"08:42:42", "08:48:06", "08:53:27", 45.41},
		{"10:21:01", "10:24:35", "10:28:10", 5.59},
		{"22:01:20", "22:06:45", "22:12:10", 60.02},
		{"23:40:08", "23:43:40", "23:47:12", 5.25},
		{"07:55:42", "08:00:46", "08:05:49", 21.09},
		{"09:32:43", "09:37:22", "09:42:01", 13.58},
		{"21:14:21", "21:19:30", "21:24:39", 25.40},
		{"22:51:41", "22:56:13", "23:00:46", 11.88},
	}
	if len(passes) != len(want) {
		t.Fatalf("%d passes, want %d", len(passes), len(want))
	}
	near := func(what string, got time.Time, clock string) {
		t.Helper()
		wanted, _ := time.Parse("2006-01-02 15:04:05", got.Format("2006-01-02")+" "+clock)
		if off := got.Sub(wanted); off > 2*time.Second || off < -2*time.Second {
			t.Errorf("%s: %s, want %s", what, got.Format("15:04:05"), clock)
		}
	}
	for i, pass := range passes {
		near("rise", pass.Rise.Time, want[i].rise)
		near("culmination", pass.Culmination.Time, want[i].top)
		near("set", pass.Set.Time, want[i].set)
		if math.Abs(pass.Culmination.Elevation-want[i].elevation) > 0.05 {
			t.Errorf("pass %d peaks at %.2f°, want %.2f°", i, pass.Culmination.Elevation, want[i].elevation)
		}
	}
	// Skyfield: at 22:01 the ISS is in shadow, by 22:12 sunlit, and at 03:30 IST the sky
	// over Chennai is dark, so the pass is visible in its later part.
	morning := passes[3]
	if morning.VisibleFrom == nil || morning.VisibleFrom.Before(morning.Rise.Time) || morning.VisibleUntil.Sub(morning.Set.Time) < -15*time.Second {
		t.Errorf("visible: %v to %v", morning.VisibleFrom, morning.VisibleUntil)
	}
	// The 08:42 UTC pass is at 14:12 IST, in daylight.
	if passes[1].VisibleFrom != nil {
		t.Errorf("a daytime pass is not visible to the eye")
	}

	high, _ := iss.Passes(chennai, PassOptions{From: from, Until: from.Add(48 * time.Hour), MinElevation: 20})
	if len(high) != 4 {
		t.Errorf("%d passes above 20°, want 4", len(high))
	}
}

func TestGeostationaryNeverSets(t *testing.T) {
	eshail := load(t)["ES'HAIL 2"] // over 25.9° E, low in the west from Chennai
	from := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	passes, err := eshail.Passes(Observer{Latitude: 13.0827, Longitude: 80.2707}, PassOptions{From: from, Until: from.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(passes) != 1 || passes[0].Rise != nil || passes[0].Set != nil {
		t.Fatalf("passes: %+v", passes)
	}
	if top := passes[0].Culmination; top.Elevation < 20 || top.Elevation > 40 || top.Azimuth < 240 || top.Azimuth > 270 {
		t.Errorf("look angles %.1f° up at %.1f°", top.Elevation, top.Azimuth)
	}
}

func TestSubPointMatchesSkyfield(t *testing.T) {
	where, err := load(t)["ISS (ZARYA)"].SubPoint(time.Date(2026, 10, 14, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(where.Latitude+36.6424) > 0.01 || math.Abs(where.Longitude-173.9307) > 0.01 || math.Abs(where.Altitude-427.793) > 0.5 {
		t.Errorf("ISS over %.4f, %.4f at %.3f km; want -36.6424, 173.9307, 427.793", where.Latitude, where.Longitude, where.Altitude)
	}
}
