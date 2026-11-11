package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["find_hospitals"] = `{"pincode":"637001"}`
}

func TestHospitals(t *testing.T) {
	byPin := expectStatus(t, "/v1/hospitals?pincode=637001", http.StatusOK).json(t)["data"].([]any)
	if len(byPin) == 0 || byPin[0].(map[string]any)["district"] != "Namakkal" {
		t.Errorf("by pincode %v", byPin)
	}
	district := expectStatus(t, "/v1/hospitals?district=namakkal&state=Tamil%20Nadu", http.StatusOK).json(t)["data"].([]any)
	if len(district) != 15 {
		t.Errorf("by district: %d", len(district))
	}
	near := expectStatus(t, "/v1/hospitals?near=12.99,77.58", http.StatusOK).json(t)["data"].([]any)
	if len(near) != 10 {
		t.Fatalf("near: %d", len(near))
	}
	first, last := near[0].(map[string]any)["distance_km"].(float64), near[9].(map[string]any)["distance_km"].(float64)
	if first > last || first > 10 {
		t.Errorf("near not sorted or too far: %v .. %v", first, last)
	}
	if got := expectStatus(t, "/v1/hospitals?q=care", http.StatusOK).json(t)["data"].([]any); len(got) == 0 {
		t.Error("name search found nothing")
	}
	expectStatus(t, "/v1/hospitals", http.StatusBadRequest)
	expectStatus(t, "/v1/hospitals?near=x", http.StatusBadRequest)
	expectStatus(t, "/v1/hospitals?pincode=12", http.StatusBadRequest)
}
