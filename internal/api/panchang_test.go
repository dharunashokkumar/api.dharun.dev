package api

import (
	"net/http"
	"strings"
	"testing"
	_ "time/tzdata"
)

func init() {
	mcpTestCalls["get_panchang"] = `{"date":"2026-01-14","pincode":"600001"}`
}

func TestPanchang(t *testing.T) {
	pongal := expectStatus(t, "/v1/panchang/2026-01-14?pincode=600001", http.StatusOK).data(t)
	place := pongal["place"].(map[string]any)
	if place["pincode"] != "600001" || place["latitude"].(float64) < 13 || place["latitude"].(float64) > 13.2 {
		t.Errorf("place from the pincode: %v", place)
	}
	if month := pongal["tamilMonth"].(map[string]any); month["name"] != "Thai" || month["day"].(float64) != 1 || month["tamil"] != "தை" {
		t.Errorf("Thai Pongal 2026: %v", month)
	}
	if !strings.HasPrefix(pongal["sunrise"].(string), "2026-01-14T06:3") || !strings.HasSuffix(pongal["sunrise"].(string), "+05:30") {
		t.Errorf("Chennai sunrise: %v", pongal["sunrise"])
	}
	if tithi := pongal["tithi"].([]any)[0].(map[string]any); tithi["name"] != "Ekadashi" {
		t.Errorf("tithi: %v", tithi)
	}

	byDefault := expectStatus(t, "/v1/panchang/2026-01-14", http.StatusOK).data(t)
	if byDefault["place"].(map[string]any)["name"] != "New Delhi" || byDefault["sunrise"] == pongal["sunrise"] {
		t.Errorf("default place: %v", byDefault["place"])
	}
	london := expectStatus(t, "/v1/panchang/2026-06-21?lat=51.5074&lng=-0.1278&tz=Europe/London", http.StatusOK).data(t)
	if !strings.HasSuffix(london["sunrise"].(string), "+01:00") {
		t.Errorf("London in summer time: %v", london["sunrise"])
	}
	expectStatus(t, "/v1/panchang/today", http.StatusOK)
	for _, bad := range []string{
		"/v1/panchang/14-01-2026", "/v1/panchang/1800-01-01", "/v1/panchang/2026-01-14?tz=Mars/Base",
		"/v1/panchang/2026-01-14?lat=80&lng=10", "/v1/panchang/2026-01-14?lat=13", "/v1/panchang/2026-01-14?pincode=12345",
	} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
	expectStatus(t, "/v1/panchang/2026-01-14?pincode=999998", http.StatusNotFound)
}
