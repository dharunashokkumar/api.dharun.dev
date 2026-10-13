package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["lookup_airport"] = `{"code":"maa"}`
	mcpTestCalls["search_airports"] = `{"query":"chennai"}`
}

func TestAirports(t *testing.T) {
	maa := expectStatus(t, "/v1/airports/maa", http.StatusOK).data(t)
	if maa["icao"] != "VOMM" || maa["city"] != "Chennai" || maa["stateCode"] != "TN" || maa["countryName"] != "India" || maa["scheduledService"] != true {
		t.Errorf("MAA: %v", maa)
	}
	if byICAO := expectStatus(t, "/v1/airports/VOMM", http.StatusOK).data(t); byICAO["iata"] != "MAA" {
		t.Errorf("VOMM: %v", byICAO)
	}
	if lhr := expectStatus(t, "/v1/airports/LHR", http.StatusOK).data(t); lhr["country"] != "GB" || lhr["stateCode"] != nil {
		t.Errorf("LHR: %v", lhr)
	}
	expectStatus(t, "/v1/airports/ZZZ", http.StatusNotFound)
	expectStatus(t, "/v1/airports/M1", http.StatusBadRequest)

	chennai := expectStatus(t, "/v1/airports?q=chennai", http.StatusOK).list(t)
	if len(chennai) == 0 || chennai[0].(map[string]any)["iata"] != "MAA" {
		t.Errorf("searching chennai should lead with MAA: %v", chennai)
	}
	tamilNadu := expectStatus(t, "/v1/airports?state=TN", http.StatusOK).list(t)
	heliports, withFlights := 0, 0
	for _, raw := range tamilNadu {
		airport := raw.(map[string]any)
		if airport["stateCode"] != "TN" {
			t.Errorf("outside Tamil Nadu: %v", airport)
		}
		if airport["type"] == "heliport" {
			heliports++
		}
		if airport["scheduledService"] == true && heliports > 0 {
			t.Error("airports with flights should come before heliports")
		}
		if airport["scheduledService"] == true {
			withFlights++
		}
	}
	if heliports < 1 || withFlights < 4 {
		t.Errorf("Tamil Nadu: %d heliports, %d with flights", heliports, withFlights)
	}
	if onlyHeliports := expectStatus(t, "/v1/airports?state=TN&type=heliport", http.StatusOK).list(t); len(onlyHeliports) != heliports {
		t.Errorf("type filter: %d", len(onlyHeliports))
	}
	if byCode := expectStatus(t, "/v1/airports?q=sin", http.StatusOK).list(t); len(byCode) == 0 {
		t.Error("a search should match an IATA code")
	}
	expectStatus(t, "/v1/airports", http.StatusBadRequest)
	expectStatus(t, "/v1/airports?country=IND", http.StatusBadRequest)
	expectStatus(t, "/v1/airports?q=chennai&type=spaceport", http.StatusBadRequest)
}
