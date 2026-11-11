package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["find_villages"] = `{"pincode":"637001"}`
}

func TestLGD(t *testing.T) {
	states := expectStatus(t, "/v1/lgd/states", http.StatusOK).json(t)["data"].([]any)
	if len(states) != 2 {
		t.Errorf("states %v", states)
	}
	districts := expectStatus(t, "/v1/lgd/districts?state=33", http.StatusOK).json(t)["data"].([]any)
	found := false
	for _, d := range districts {
		if d.(map[string]any)["name"] == "Namakkal" && d.(map[string]any)["code"] == 580.0 {
			found = true
		}
	}
	if !found {
		t.Errorf("no Namakkal (580) in %v", districts)
	}
	subs := expectStatus(t, "/v1/lgd/subdistricts?district=580", http.StatusOK).json(t)["data"].([]any)
	if len(subs) != 8 {
		t.Errorf("subdistricts %v", subs)
	}
	byPin := expectStatus(t, "/v1/lgd/villages?pincode=637001", http.StatusOK).json(t)["data"].([]any)
	if len(byPin) == 0 {
		t.Fatal("no villages for 637001")
	}
	v := byPin[0].(map[string]any)
	if v["pincode"] != "637001" || v["district"] != "Namakkal" || v["state"] != "Tamil Nadu" || v["subdistrict"] != "Namakkal" {
		t.Errorf("village %v", v)
	}
	one := expectStatus(t, "/v1/lgd/villages/634584", http.StatusOK).json(t)["data"].(map[string]any)
	if one["name"] != "Sarkarnattamangalam" || one["district_code"] != 580.0 {
		t.Errorf("634584: %v", one)
	}
	if got := expectStatus(t, "/v1/lgd/villages?q=sarkar&state=33", http.StatusOK).json(t)["data"].([]any); len(got) == 0 {
		t.Error("name search found nothing")
	}
	if got := expectStatus(t, "/v1/lgd/villages?subdistrict=5747", http.StatusOK).json(t)["data"].([]any); len(got) < 10 {
		t.Errorf("subdistrict villages %d", len(got))
	}
	expectStatus(t, "/v1/lgd/villages", http.StatusBadRequest)
	expectStatus(t, "/v1/lgd/villages?pincode=12", http.StatusBadRequest)
	expectStatus(t, "/v1/lgd/districts", http.StatusBadRequest)
	expectStatus(t, "/v1/lgd/villages/1", http.StatusNotFound)
}
