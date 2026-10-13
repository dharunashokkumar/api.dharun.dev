package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["get_thirukkural"] = `{"number":1}`
	mcpTestCalls["search_thirukkural"] = `{"query":"மழை"}`
}

func TestThirukkural(t *testing.T) {
	first := expectStatus(t, "/v1/thirukkural/1", http.StatusOK)
	kural := first.data(t)
	lines := kural["lines"].([]any)
	if lines[0] != "அகர முதல எழுத்தெல்லாம் ஆதி" || lines[1] != "பகவன் முதற்றே உலகு." {
		t.Errorf("kural 1: %v", lines)
	}
	translations := kural["translations"].(map[string]any)
	if !strings.HasPrefix(translations["pope"].(string), "A, as its first of letters") || !strings.HasPrefix(translations["drewLazarus"].(string), "As the letter A") {
		t.Errorf("translations: %v", translations)
	}
	if len(translations) != 2 {
		t.Errorf("only the public-domain translations may be served: %v", translations)
	}
	chapter := kural["chapter"].(map[string]any)
	if chapter["number"].(float64) != 1 || chapter["name"] != "கடவுள் வாழ்த்து" || chapter["translation"] != "The Praise of God" {
		t.Errorf("chapter: %v", chapter)
	}
	if kural["paal"].(map[string]any)["name"] != "அறத்துப்பால்" || kural["iyal"].(map[string]any)["name"] != "பாயிரவியல்" {
		t.Errorf("paal or iyal: %v %v", kural["paal"], kural["iyal"])
	}
	if source := first.json(t)["meta"].(map[string]any)["source"].(map[string]any); !strings.Contains(source["attribution"].(string), "Pope") {
		t.Errorf("source: %v", source)
	}

	last := expectStatus(t, "/v1/thirukkural/1330", http.StatusOK).data(t)
	if last["chapter"].(map[string]any)["number"].(float64) != 133 || last["paal"].(map[string]any)["number"].(float64) != 3 {
		t.Errorf("kural 1330 should be in chapter 133 of the third paal: %v", last["chapter"])
	}
	for _, bad := range []string{"/v1/thirukkural/0", "/v1/thirukkural/1331", "/v1/thirukkural/one"} {
		expectStatus(t, bad, http.StatusBadRequest)
	}

	// Search works in Tamil and English; rain (மழை / வான்) is chapter 2.
	tamil := expectStatus(t, "/v1/thirukkural?q="+url.QueryEscape("மழை"), http.StatusOK)
	if total := tamil.json(t)["meta"].(map[string]any)["total"].(float64); total < 3 {
		t.Errorf("Tamil search found %v kurals", total)
	}
	english := expectStatus(t, "/v1/thirukkural?q=RAIN&limit=50", http.StatusOK).list(t)
	inChapterTwo := 0
	for _, raw := range english {
		if raw.(map[string]any)["chapter"].(map[string]any)["number"].(float64) == 2 {
			inChapterTwo++
		}
	}
	if inChapterTwo < 3 {
		t.Errorf("an English search for rain found %d kurals of chapter 2", inChapterTwo)
	}
	if chapterTwo := expectStatus(t, "/v1/thirukkural?chapter=2", http.StatusOK); len(chapterTwo.list(t)) != 10 {
		t.Errorf("chapter 2 has %d kurals", len(chapterTwo.list(t)))
	}
	if love := expectStatus(t, "/v1/thirukkural?paal=3&limit=1", http.StatusOK).json(t)["meta"].(map[string]any); love["total"].(float64) != 250 {
		t.Errorf("the third paal has 25 chapters of 10 kurals, got %v", love["total"])
	}
	expectStatus(t, "/v1/thirukkural", http.StatusBadRequest)
	expectStatus(t, "/v1/thirukkural?q=a", http.StatusBadRequest)
	expectStatus(t, "/v1/thirukkural?chapter=134", http.StatusBadRequest)
	if none := expectStatus(t, "/v1/thirukkural?q=100%25", http.StatusOK).list(t); len(none) != 0 {
		t.Errorf("a %% in the query must be literal: %d results", len(none))
	}

	today := expectStatus(t, "/v1/thirukkural/today?date=2026-10-13", http.StatusOK).data(t)
	again := expectStatus(t, "/v1/thirukkural/today?date=2026-10-13", http.StatusOK).data(t)
	tomorrow := expectStatus(t, "/v1/thirukkural/today?date=2026-10-14", http.StatusOK).data(t)
	if today["number"] != again["number"] || today["number"] == tomorrow["number"] {
		t.Errorf("kural of the day: %v, %v, then %v", today["number"], again["number"], tomorrow["number"])
	}
	random := expectStatus(t, "/v1/thirukkural/random", http.StatusOK)
	if random.header.Get("Cache-Control") != cacheNone {
		t.Errorf("a random kural must not be cached: %q", random.header.Get("Cache-Control"))
	}

	chapters := expectStatus(t, "/v1/thirukkural/chapters", http.StatusOK).list(t)
	if len(chapters) != 133 {
		t.Fatalf("%d chapters", len(chapters))
	}
	covered := 0
	for _, raw := range chapters {
		chapter := raw.(map[string]any)
		covered += int(chapter["last"].(float64)-chapter["first"].(float64)) + 1
	}
	if covered != 1330 {
		t.Errorf("chapters cover %d kurals", covered)
	}
	two := expectStatus(t, "/v1/thirukkural/chapters/2", http.StatusOK).data(t)
	if two["translation"] != "The Blessing of Rain" || two["first"].(float64) != 11 || !strings.HasSuffix(two["links"].(map[string]any)["kurals"].(string), "/v1/thirukkural?chapter=2") {
		t.Errorf("chapter 2: %v", two)
	}
	expectStatus(t, "/v1/thirukkural/chapters/0", http.StatusBadRequest)
}
