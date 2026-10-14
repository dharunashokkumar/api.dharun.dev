package api

import (
	"net/http"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["get_sun_and_moon"] = `{"date":"2026-10-14","pincode":"600001"}`
}

func TestSunMoon(t *testing.T) {
	data := expectStatus(t, "/v1/sun-moon/2026-10-14?pincode=600001", http.StatusOK).data(t)
	sun := data["sun"].(map[string]any)
	if !strings.HasPrefix(sun["sunrise"].(string), "2026-10-14T05:5") || sun["civilTwilight"].(map[string]any)["dawn"] == nil {
		t.Errorf("sun: %v", sun)
	}
	if moon := data["moon"].(map[string]any); moon["phase"] != "waxing crescent" || moon["nextPhases"].(map[string]any)["fullMoon"] == nil {
		t.Errorf("moon: %v", moon)
	}
	if data["place"].(map[string]any)["pincode"] != "600001" {
		t.Errorf("place: %v", data["place"])
	}
	london := expectStatus(t, "/v1/sun-moon/2026-06-21?lat=51.5&lng=-0.13&tz=Europe/London", http.StatusOK).data(t)
	if astronomical := london["sun"].(map[string]any)["astronomicalTwilight"].(map[string]any); astronomical["dawn"] != nil {
		t.Errorf("London at midsummer never gets astronomically dark: %v", astronomical)
	}
	expectStatus(t, "/v1/sun-moon/today", http.StatusOK)
	expectStatus(t, "/v1/sun-moon/2026-13-01", http.StatusBadRequest)
	expectStatus(t, "/v1/sun-moon/2026-10-14?lat=80&lng=0", http.StatusBadRequest)
}
