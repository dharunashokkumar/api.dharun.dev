package api

import (
	"net/http"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["lookup_railway_station"] = `{"code":"mas"}`
	mcpTestCalls["search_railway_stations"] = `{"query":"chennai"}`
}

func TestStations(t *testing.T) {
	mas := expectStatus(t, "/v1/stations/mas", http.StatusOK).data(t)
	if mas["stateCode"] != "TN" || mas["state"] != "Tamil Nadu" || mas["zone"].(map[string]any)["code"] != "SR" || !strings.Contains(mas["name"].(string), "Central") {
		t.Errorf("MAS: %v", mas)
	}
	if ndls := expectStatus(t, "/v1/stations/NDLS", http.StatusOK).data(t); ndls["stateCode"] != "DL" || ndls["elevationM"].(float64) != 215 {
		t.Errorf("NDLS: %v", ndls)
	}
	expectStatus(t, "/v1/stations/ZZZZ", http.StatusNotFound)
	expectStatus(t, "/v1/stations/M4S", http.StatusBadRequest)

	chennai := expectStatus(t, "/v1/stations?q=chennai", http.StatusOK).list(t)
	if len(chennai) < 2 {
		t.Errorf("chennai: %v", chennai)
	}
	byCode := expectStatus(t, "/v1/stations?q=ms", http.StatusOK).list(t)
	if len(byCode) == 0 || byCode[0].(map[string]any)["code"] != "MS" {
		t.Errorf("an exact code should come first: %v", byCode)
	}
	southern := expectStatus(t, "/v1/stations?zone=sr&state=TN", http.StatusOK).list(t)
	for _, raw := range southern {
		if raw.(map[string]any)["zone"].(map[string]any)["code"] != "SR" {
			t.Errorf("zone filter: %v", raw)
		}
	}
	if zones := expectStatus(t, "/v1/stations/zones", http.StatusOK).list(t); len(zones) < 18 {
		t.Errorf("zones: %d", len(zones))
	}
	expectStatus(t, "/v1/stations?zone=XYZ", http.StatusBadRequest)
	expectStatus(t, "/v1/stations", http.StatusBadRequest)
}
