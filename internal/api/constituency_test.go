package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["find_constituency"] = `{"pincode":"637001"}`
}

func TestConstituency(t *testing.T) {
	namakkal := expectStatus(t, "/v1/constituency?pincode=637001", http.StatusOK).data(t)
	assembly, _ := namakkal["assembly"].(map[string]any)
	parliament, _ := namakkal["parliament"].(map[string]any)
	if assembly == nil || assembly["name"] != "Namakkal" || assembly["stateCode"] != "TN" || assembly["parliamentName"] != "Namakkal" {
		t.Errorf("assembly seat of 637001: %v", assembly)
	}
	if parliament == nil || parliament["name"] != "Namakkal" || parliament["reserved"] != "GEN" || parliament["nameHindi"] == nil {
		t.Errorf("Lok Sabha seat of 637001: %v", parliament)
	}

	// Hyderabad: the assembly map files it under Andhra Pradesh; the answer must say Telangana.
	hyderabad := expectStatus(t, "/v1/constituency?lat=17.3850&lng=78.4867", http.StatusOK).data(t)
	if hyderabad["assembly"].(map[string]any)["stateCode"] != "TS" || hyderabad["parliament"].(map[string]any)["stateCode"] != "TS" {
		t.Errorf("Hyderabad: %v", hyderabad)
	}
	// A reserved seat: Rasipuram (SC), with the marker taken out of the name.
	rasipuram := expectStatus(t, "/v1/constituency?lat=11.4600&lng=78.1800", http.StatusOK).data(t)
	if seat := rasipuram["assembly"].(map[string]any); seat["name"] != "Rasipuram" || seat["reserved"] != "SC" {
		t.Errorf("Rasipuram: %v", seat)
	}
	// A pre-delimitation state is marked as such.
	guwahati := expectStatus(t, "/v1/constituency?lat=26.1445&lng=91.7362", http.StatusOK).data(t)
	if guwahati["assembly"].(map[string]any)["predelimitation"] != true {
		t.Errorf("Assam's assembly seats predate the 2023 delimitation: %v", guwahati["assembly"])
	}
	expectStatus(t, "/v1/constituency?lat=13.0&lng=81.5", http.StatusNotFound) // Bay of Bengal
	expectStatus(t, "/v1/constituency", http.StatusBadRequest)
	expectStatus(t, "/v1/constituency?pincode=99", http.StatusBadRequest)
}
