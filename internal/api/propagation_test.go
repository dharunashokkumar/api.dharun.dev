package api

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

func init() {
	mcpTestCalls["get_hf_propagation"] = `{}`
}

// buildLiveFixture builds a dataset that only ever exists as a server's own copy from
// fixture files laid out as an importer writes them, into the live directory.
func buildLiveFixture(t interface{ Fatal(...any) }, liveDir, name, sourceDir string, source dataset.Source) {
	spec, _ := dataset.Lookup(name)
	source.Retrieved = time.Now().UTC().Format(time.RFC3339)
	if _, _, err := dataset.Build(spec, dataset.BuildOptions{
		Dir: sourceDir, Out: filepath.Join(liveDir, dataset.LiveName(name, time.Now())), Source: source,
	}); err != nil {
		t.Fatal(err)
	}
}

func buildSpaceWeatherDataset(t interface{ Fatal(...any) }, liveDir string) {
	buildLiveFixture(t, liveDir, "space-weather", filepath.Join("testdata", "data"), dataset.Source{
		Name: "Space weather data (test)", Licence: "Public domain (work of the US government)",
		Attribution: "Space weather data from the NOAA Space Weather Prediction Center.",
	})
}

func TestPropagation(t *testing.T) {
	got := expectStatus(t, "/v1/propagation", http.StatusOK)
	data := got.data(t)
	// The fixture was fetched on 2026-10-14: flux 136, sunspot number 102, B7.9.
	if flux := data["solarFlux"].(map[string]any); flux["value"].(float64) != 136 {
		t.Errorf("solar flux: %v", flux)
	}
	if spots := data["sunspotNumber"].(map[string]any); spots["value"].(float64) != 102 || spots["date"] != "2026-10-13" {
		t.Errorf("sunspot number: %v", spots)
	}
	if xray := data["xray"].(map[string]any); !strings.HasPrefix(xray["class"].(string), "B") {
		t.Errorf("x-ray: %v", xray)
	}
	if scales := data["noaaScales"].(map[string]any); scales["radioBlackout"] != "R0" {
		t.Errorf("scales: %v", scales)
	}
	if history := data["kHistory"].([]any); len(history) != 8 {
		t.Errorf("K history has %d readings", len(history))
	}
	if a := data["aIndex"].(map[string]any); a["value"].(float64) < 1 || a["date"] == "" {
		t.Errorf("A index: %v", a)
	}
	bands := data["bands"].([]any)
	if len(bands) != 4 || bands[0].(map[string]any)["band"] != "80m-40m" {
		t.Fatalf("bands: %v", bands)
	}
	if source := got.json(t)["meta"].(map[string]any)["source"].(map[string]any); !strings.Contains(source["attribution"].(string), "NOAA") {
		t.Errorf("source: %v", source)
	}
}

func TestKLevel(t *testing.T) {
	for k, want := range map[float64]string{0.33: "quiet", 3: "unsettled", 4.67: "active", 5: "minor storm (G1)", 9: "extreme storm (G5)"} {
		if got := kLevel(k); got != want {
			t.Errorf("K %.2f: %q, want %q", k, got, want)
		}
	}
}
