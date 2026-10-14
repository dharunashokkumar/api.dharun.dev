package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

func init() {
	mcpTestCalls["get_satellite_passes"] = `{"satellite":"ISS","pincode":"600001","days":2}`
	mcpTestCalls["find_satellites"] = `{"query":"so-50"}`
	mcpTestCalls["where_is_satellite"] = `{"satellite":"ISS"}`
}

// buildSatelliteDataset builds the satellites from CelesTrak's element sets of
// 2026-10-14, each moved to an epoch of now, so that predictions from now have fresh
// elements whenever the tests run. internal/orbit checks the predictions themselves.
func buildSatelliteDataset(t interface{ Fatal(...any) }, liveDir string) {
	source, err := os.MkdirTemp("", "satellites-source-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(source)
	now := time.Now().UTC()
	epoch := fmt.Sprintf("%02d%012.8f", now.Year()%100, float64(now.YearDay())+float64(now.Sub(now.Truncate(24*time.Hour)))/86400e9)
	for _, file := range []string{"stations.tle", "amateur.tle"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "data", "satellites", file))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.ReplaceAll(string(raw), "\r", ""), "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "1 ") && len(line) == 69 {
				line = line[:18] + epoch + line[32:68]
				lines[i] = line + string(rune('0'+tleChecksum(line)))
			}
		}
		if err := os.MkdirAll(filepath.Join(source, "satellites"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "satellites", file), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	buildLiveFixture(t, liveDir, "satellites", source, dataset.Source{
		Name: "CelesTrak current GP element sets (test)", Licence: "None stated by CelesTrak",
		Attribution: "Orbit elements from CelesTrak (celestrak.org), based on US Space Force data from Space-Track.org.",
	})
}

func tleChecksum(line string) int {
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

func TestSatellitePasses(t *testing.T) {
	got := expectStatus(t, "/v1/sat-pass?sat=ISS&pincode=600001&days=2&minElevation=0", http.StatusOK)
	data := got.data(t)
	if satellite := data["satellite"].(map[string]any); satellite["number"].(float64) != 25544 || satellite["name"] != "ISS (ZARYA)" {
		t.Errorf("satellite: %v", satellite)
	}
	passes := data["passes"].([]any)
	// The ISS crosses the sky over Chennai four to six times a day.
	if len(passes) < 6 || len(passes) > 14 {
		t.Fatalf("%d passes in two days", len(passes))
	}
	last := ""
	for _, raw := range passes {
		pass := raw.(map[string]any)
		top := pass["culmination"].(map[string]any)
		if pass["rise"] != nil && pass["set"] != nil {
			rise, set := pass["rise"].(map[string]any), pass["set"].(map[string]any)
			if !(rise["time"].(string) < top["time"].(string) && top["time"].(string) < set["time"].(string)) {
				t.Errorf("pass out of order: %v", pass)
			}
			if !strings.HasSuffix(rise["time"].(string), "+05:30") {
				t.Errorf("times are in IST by default: %v", rise["time"])
			}
			if duration := pass["durationSeconds"].(float64); duration <= 0 || duration > 12*60 {
				t.Errorf("an ISS pass lasts minutes: %v", duration)
			}
		}
		if top["time"].(string) < last {
			t.Errorf("passes are not in order")
		}
		last = top["time"].(string)
	}
	if strings.Contains(string(got.body), "line1") || strings.Contains(string(got.body), "1 25544U") {
		t.Error("the orbit elements themselves must never be returned")
	}
	if source := got.json(t)["meta"].(map[string]any)["source"].(map[string]any); !strings.Contains(source["attribution"].(string), "CelesTrak") {
		t.Errorf("source: %v", source)
	}

	high := expectStatus(t, "/v1/sat-pass?sat=25544&lat=13.08&lng=80.27&days=2&minElevation=30", http.StatusOK).data(t)
	if len(high["passes"].([]any)) >= len(passes) {
		t.Error("minElevation should leave out low passes")
	}
	for _, raw := range expectStatus(t, "/v1/sat-pass?sat=iss&pincode=600001&days=3&visible=true", http.StatusOK).data(t)["passes"].([]any) {
		if raw.(map[string]any)["visible"] == nil {
			t.Errorf("visible=true listed a pass that cannot be seen: %v", raw)
		}
	}
	// Places far from India work too.
	expectStatus(t, "/v1/sat-pass?lat=70&lng=20&tz=Europe/Oslo", http.StatusOK)

	expectStatus(t, "/v1/sat-pass?sat=99999", http.StatusNotFound)
	expectStatus(t, "/v1/sat-pass?sat=TEVEL2", http.StatusBadRequest) // nine satellites match
	expectStatus(t, "/v1/sat-pass?days=11", http.StatusBadRequest)
	expectStatus(t, "/v1/sat-pass?minElevation=95", http.StatusBadRequest)
	expectStatus(t, "/v1/sat-pass?visible=maybe", http.StatusBadRequest)
}

func TestSatelliteLookup(t *testing.T) {
	stations := expectStatus(t, "/v1/satellites?group=stations", http.StatusOK).json(t)["data"].([]any)
	if len(stations) < 10 || stations[0].(map[string]any)["name"] != "ISS (ZARYA)" {
		t.Errorf("stations: %v", stations)
	}
	list := expectStatus(t, "/v1/satellites?q=so-50", http.StatusOK)
	found := list.json(t)["data"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["number"].(float64) != 27607 {
		t.Errorf("SO-50: %v", found)
	}
	expectStatus(t, "/v1/satellites?group=weather", http.StatusBadRequest)

	now := expectStatus(t, "/v1/satellites/ISS", http.StatusOK).data(t)
	if altitude := now["altitudeKm"].(float64); altitude < 350 || altitude > 460 {
		t.Errorf("ISS altitude %v km", altitude)
	}
	if speed := now["speedKmPerSecond"].(float64); speed < 7.5 || speed > 7.8 {
		t.Errorf("ISS speed %v km/s", speed)
	}
	if latitude := now["latitude"].(float64); latitude < -52 || latitude > 52 {
		t.Errorf("the ISS never goes beyond 51.6°: %v", latitude)
	}
	for _, name := range []string{"25544", "iss (zarya)", "zarya", "ISS"} {
		if got := expectStatus(t, "/v1/satellites/"+strings.ReplaceAll(name, " ", "%20"), http.StatusOK).data(t); got["number"].(float64) != 25544 {
			t.Errorf("%s: %v", name, got["number"])
		}
	}
	// A geostationary satellite never sets; internal/orbit tests its passes with its real epoch.
	if got := expectStatus(t, "/v1/satellites/QO-100", http.StatusOK).data(t); got["number"].(float64) != 43700 {
		t.Errorf("QO-100 is Es'hail 2: %v", got["number"])
	}
	if got := expectStatus(t, "/v1/satellites/AO-7", http.StatusOK).data(t); got["number"].(float64) != 7530 {
		t.Errorf("AO-7 by its bracketed designation: %v", got["number"])
	}
}

func TestCompassDirection(t *testing.T) {
	for azimuth, want := range map[float64]string{0: "N", 11.2: "N", 11.3: "NNE", 90: "E", 200: "SSW", 348.7: "NNW", 359.9: "N"} {
		if got := direction(azimuth); got != want {
			t.Errorf("%.1f°: %s, want %s", azimuth, got, want)
		}
	}
}
