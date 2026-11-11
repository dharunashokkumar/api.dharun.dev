package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["list_indian_festivals"] = `{"year":2025,"month":10}`
	mcpTestCalls["write_bharati_braille"] = `{"text":"नमस्ते"}`
	mcpTestCalls["convert_indian_keyboard"] = `{"text":"vphhkf","layout":"tamil99"}`
	mcpTestCalls["count_indian_letters"] = `{"text":"தமிழ்நாடு"}`
}

func TestFestivalsEndpoint(t *testing.T) {
	got := expectStatus(t, "/v1/festivals/2025?month=10&kind=hindu", http.StatusOK)
	if !strings.Contains(string(got.body), `"date":"2025-10-20","name":"Diwali (Lakshmi Puja)"`) {
		t.Errorf("diwali missing: %s", clip(string(got.body), 400))
	}
	tn := string(expectStatus(t, "/v1/festivals/2025?state=TN", http.StatusOK).body)
	if strings.Contains(tn, "Chhath") || !strings.Contains(tn, "Thaipusam") {
		t.Error("state filter")
	}
	feed := expectStatus(t, "/v1/festivals/2025.ics?kind=tamil", http.StatusOK)
	if !strings.Contains(string(feed.body), "SUMMARY:Karthigai Deepam") {
		t.Errorf("feed %s", clip(string(feed.body), 300))
	}
	expectStatus(t, "/v1/festivals/1800", http.StatusBadRequest)
	expectStatus(t, "/v1/festivals/2025?kind=martian", http.StatusBadRequest)
	today := expectStatus(t, "/v1/today?date=2025-10-20", http.StatusOK).data(t)
	if !strings.Contains(toJSON(today["festivals"]), "Diwali") {
		t.Errorf("today festivals %v", today["festivals"])
	}
}

func TestIndicEndpoints(t *testing.T) {
	if got := expectStatus(t, "/v1/keyboard/convert?layout=tamil99&text=vphhkf", http.StatusOK).data(t); got["text"] != "வணக்கம்" {
		t.Errorf("keyboard %v", got)
	}
	if got := expectStatus(t, "/v1/braille?text=%E0%A4%95", http.StatusOK).data(t); got["braille"] != "⠅" {
		t.Errorf("braille %v", got)
	}
	if got := expectStatus(t, "/v1/indic/aksharas?text=%E0%AE%A4%E0%AE%AE%E0%AE%BF%E0%AE%B4%E0%AF%8D", http.StatusOK).data(t); got["count"].(map[string]any)["aksharas"].(float64) != 3 {
		t.Errorf("aksharas %v", got)
	}
	sorted := request(t, http.MethodPost, "/v1/indic/sort", `{"words":["கடல்","அம்மா"],"lang":"ta"}`)
	if !strings.Contains(string(sorted.body), `["அம்மா","கடல்"]`) {
		t.Errorf("sort %s", sorted.body)
	}
	if got := expectStatus(t, "/v1/helplines?q=cyber", http.StatusOK).json(t)["data"].([]any); len(got) != 1 {
		t.Errorf("helplines %v", got)
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
