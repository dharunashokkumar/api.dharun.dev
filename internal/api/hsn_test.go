package api

import (
	"net/http"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["lookup_hsn_code"] = `{"code":"0902"}`
	mcpTestCalls["search_hsn_codes"] = `{"query":"green tea"}`
}

func TestHSN(t *testing.T) {
	tea := expectStatus(t, "/v1/hsn/0902%2010%2010", http.StatusOK).data(t)
	if tea["code"] != "09021010" || tea["level"] != "tariff_item" || tea["kind"] != "hsn" {
		t.Errorf("09021010: %v", tea)
	}
	parents := tea["parents"].([]any)
	if len(parents) != 3 || parents[0].(map[string]any)["code"] != "09" || parents[2].(map[string]any)["code"] != "090210" {
		t.Errorf("parents should run from the chapter down: %v", parents)
	}
	heading := expectStatus(t, "/v1/hsn/0902", http.StatusOK).data(t)
	if len(heading["children"].([]any)) < 2 || heading["level"] != "heading" {
		t.Errorf("0902 children: %v", heading)
	}
	service := expectStatus(t, "/v1/hsn/995411", http.StatusOK).data(t)
	if service["kind"] != "sac" || !strings.Contains(service["description"].(string), "Construction") || len(service["parents"].([]any)) != 2 {
		t.Errorf("995411: %v", service)
	}
	missing := expectStatus(t, "/v1/hsn/09029999", http.StatusNotFound)
	if !strings.Contains(string(missing.body), "nearest listed code is 0902") {
		t.Errorf("a missing code should point to the nearest one: %s", missing.body)
	}
	expectStatus(t, "/v1/hsn/9", http.StatusBadRequest)
	expectStatus(t, "/v1/hsn/09ab", http.StatusBadRequest)

	green := expectStatus(t, "/v1/hsn?q=tea%20green", http.StatusOK).list(t)
	if len(green) == 0 || green[0].(map[string]any)["code"] != "090210" {
		t.Errorf("words in any order, broader codes first: %v", green)
	}
	if prefix := expectStatus(t, "/v1/hsn?q=0902&limit=100", http.StatusOK).list(t); len(prefix) < 5 || prefix[0].(map[string]any)["code"] != "0902" {
		t.Errorf("code prefix: %v", prefix)
	}
	if services := expectStatus(t, "/v1/hsn?q=construction&kind=sac", http.StatusOK).list(t); len(services) == 0 {
		t.Error("SAC search found nothing")
	}
	expectStatus(t, "/v1/hsn?q=a", http.StatusBadRequest)
	expectStatus(t, "/v1/hsn?q=tea&kind=goods", http.StatusBadRequest)
}
