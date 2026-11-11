package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["get_mandi_prices"] = `{"commodity":"Tomato","state":"Tamil Nadu"}`
	mcpTestCalls["get_air_quality"] = `{"city":"Chennai"}`
}

func TestMandi(t *testing.T) {
	got := expectStatus(t, "/v1/mandi?commodity=tomato&state=tamil%20nadu", http.StatusOK).json(t)
	rows := got["data"].([]any)
	if len(rows) == 0 {
		t.Fatalf("no tomato prices: %v", got)
	}
	for _, r := range rows {
		row := r.(map[string]any)
		if row["commodity"] != "Tomato" || row["state"] != "Tamil Nadu" || row["modal_price"] == nil {
			t.Errorf("row %v", row)
		}
	}
	commodities := expectStatus(t, "/v1/mandi/commodities", http.StatusOK).json(t)["data"].([]any)
	if len(commodities) != 3 {
		t.Errorf("commodities %v", commodities)
	}
	if got := expectStatus(t, "/v1/mandi?commodity=Durian", http.StatusOK).json(t); got["meta"].(map[string]any)["total"] != 0.0 {
		t.Errorf("durian %v", got)
	}
	expectStatus(t, "/v1/mandi?date=yesterday", http.StatusBadRequest)
	expectStatus(t, "/v1/mandi?limit=5000", http.StatusBadRequest)
}

func TestAirQuality(t *testing.T) {
	stations := expectStatus(t, "/v1/air-quality?city=chennai", http.StatusOK).json(t)["data"].([]any)
	if len(stations) == 0 {
		t.Fatal("no Chennai stations")
	}
	for _, s := range stations {
		st := s.(map[string]any)
		if st["city"] != "Chennai" || len(st["pollutants"].(map[string]any)) == 0 {
			t.Errorf("station %v", st)
		}
		if aqi, ok := st["aqi"].(float64); ok && st["category"] != aqiCategory(int(aqi)) {
			t.Errorf("category %v for %v", st["category"], aqi)
		}
	}
	near := expectStatus(t, "/v1/air-quality?near=28.61,77.21", http.StatusOK).json(t)["data"].([]any)
	if len(near) != 5 || near[0].(map[string]any)["city"] != "Delhi" || near[0].(map[string]any)["distance_km"].(float64) > near[4].(map[string]any)["distance_km"].(float64) {
		t.Errorf("near Delhi: %v", near)
	}
	expectStatus(t, "/v1/air-quality?near=north", http.StatusBadRequest)
}

func TestAQIRule(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	st := airStation{Pollutants: map[string]pollutantView{"PM2.5": {Avg: v(180)}, "NO2": {Avg: v(40)}, "CO": {Avg: v(60)}}}
	st.setAQI()
	if st.AQI == nil || *st.AQI != 180 || st.Category != "Moderate" || st.Main != "PM2.5" {
		t.Errorf("aqi %+v", st)
	}
	two := airStation{Pollutants: map[string]pollutantView{"PM10": {Avg: v(90)}, "NO2": {Avg: v(40)}}}
	two.setAQI()
	if two.AQI != nil {
		t.Error("an index from two pollutants")
	}
	none := airStation{Pollutants: map[string]pollutantView{"SO2": {Avg: v(10)}, "NO2": {Avg: v(40)}, "CO": {Avg: v(60)}}}
	none.setAQI()
	if none.AQI != nil {
		t.Error("an index without PM2.5 or PM10")
	}
}
