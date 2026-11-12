package api

import (
	"net/http"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["census_2011"] = `{"q":"coimbatore"}`
	mcpTestCalls["find_std_code"] = `{"code":"0422"}`
	mcpTestCalls["card_bin_lookup"] = `{"prefix":"605067"}`
	mcpTestCalls["indian_classics"] = `{"text":"gita","ref":"2.47"}`
}

func TestCensus(t *testing.T) {
	d := expectStatus(t, "/v1/census/district/632", http.StatusOK).json(t)["data"].(map[string]any)
	if d["name"] != "Coimbatore" || d["state"].(map[string]any)["name"] != "Tamil Nadu" {
		t.Errorf("district 632: %v %v", d["name"], d["state"])
	}
	total := d["figures"].(map[string]any)["total"].(map[string]any)
	if total["persons"] != float64(3458045) || total["households"] != float64(958035) {
		t.Errorf("Coimbatore figures: %v", total)
	}
	rates := d["rates"].(map[string]any)
	if rates["sex_ratio"] != float64(1000) || rates["literacy_rate"] != 83.98 {
		t.Errorf("Coimbatore rates: %v", rates)
	}
	if children := d["children"].([]any); len(children) < 5 || children[0].(map[string]any)["level"] != "subdistrict" {
		t.Errorf("Coimbatore children: %v", children)
	}
	india := expectStatus(t, "/v1/census/india/0", http.StatusOK).json(t)["data"].(map[string]any)
	if india["figures"].(map[string]any)["total"].(map[string]any)["persons"] != float64(1210854977) || len(india["children"].([]any)) != 35 {
		t.Errorf("India: %v", india["figures"])
	}
	if got := expectStatus(t, "/v1/census/state/33", http.StatusOK).json(t)["data"].(map[string]any); got["name"] != "Tamil Nadu" {
		t.Errorf("state 33: %v", got["name"])
	}
	search := expectStatus(t, "/v1/census?q=coimbatore", http.StatusOK).json(t)["data"].([]any)
	if len(search) < 2 || search[0].(map[string]any)["code"] == "" {
		t.Errorf("search: %v", search)
	}
	if towns := expectStatus(t, "/v1/census?q=pollachi&level=town", http.StatusOK).json(t)["data"].([]any); len(towns) == 0 ||
		towns[0].(map[string]any)["level"] != "town" || towns[0].(map[string]any)["subdistrict"] != nil {
		t.Errorf("town search: %v", towns)
	}
	expectStatus(t, "/v1/census/district/999", http.StatusNotFound)
	expectStatus(t, "/v1/census/planet/1", http.StatusNotFound)
	expectStatus(t, "/v1/census/state/abc", http.StatusBadRequest)
	expectStatus(t, "/v1/census?q=ab", http.StatusBadRequest)
}

func TestSTDCodes(t *testing.T) {
	d := expectStatus(t, "/v1/std-codes/0422", http.StatusOK).json(t)["data"].(map[string]any)
	if d["area"] != "Coimbatore" || d["dial"] != "0422" || d["circle"] != "TN" {
		t.Errorf("0422: %v", d)
	}
	if d := expectStatus(t, "/v1/std-codes/+91%2044%202345%206789", http.StatusOK).json(t)["data"].(map[string]any); d["code"] != "44" {
		t.Errorf("Chennai number: %v", d)
	}
	if d := expectStatus(t, "/v1/std-codes/04259123456", http.StatusOK).json(t)["data"].(map[string]any); d["area"] != "Pollachi" {
		t.Errorf("Pollachi number: %v", d)
	}
	search := expectStatus(t, "/v1/std-codes?q=pollachi", http.StatusOK).json(t)["data"].([]any)
	if len(search) != 1 || search[0].(map[string]any)["code"] != "4259" {
		t.Errorf("search: %v", search)
	}
	if circle := expectStatus(t, "/v1/std-codes?circle=TN", http.StatusOK).json(t)["data"].([]any); len(circle) < 100 {
		t.Errorf("TN circle: %d", len(circle))
	}
	expectStatus(t, "/v1/std-codes/4", http.StatusNotFound)
	expectStatus(t, "/v1/std-codes/42", http.StatusNotFound) // 422 is a code; 42 is not
	expectStatus(t, "/v1/std-codes/abc", http.StatusBadRequest)
	expectStatus(t, "/v1/std-codes", http.StatusBadRequest)
}

func TestBins(t *testing.T) {
	d := expectStatus(t, "/v1/bin/605067", http.StatusOK).json(t)["data"].(map[string]any)
	if d["network"] != "RUPAY" || d["type"] != "DEBIT" || d["country"] != "IN" || !strings.Contains(d["issuer"].(string), "HDFC") {
		t.Errorf("605067: %v", d)
	}
	if d := expectStatus(t, "/v1/bin/60506712", http.StatusOK).json(t)["data"].(map[string]any); d["bin"] != "605067" {
		t.Errorf("8 digits: %v", d)
	}
	expectStatus(t, "/v1/bin/4111111111111111", http.StatusBadRequest)
	expectStatus(t, "/v1/bin/12345", http.StatusBadRequest)
	expectStatus(t, "/v1/bin/999999", http.StatusNotFound)
}

func TestClassics(t *testing.T) {
	list := expectStatus(t, "/v1/classics", http.StatusOK).json(t)["data"].([]any)
	if len(list) != 3 {
		t.Errorf("texts: %v", list)
	}
	v := expectStatus(t, "/v1/classics/gita/2.47", http.StatusOK).json(t)["data"].(map[string]any)
	lines := v["lines"].([]any)
	if v["ref"] != "2.47" || !strings.HasPrefix(lines[0].(string), "कर्मण्येवाधिकारस्ते") {
		t.Errorf("Gita 2.47: %v", v)
	}
	if tr := v["transliteration"].([]any)[0].(string); !strings.HasPrefix(tr, "karmaṇ") {
		t.Errorf("Gita 2.47 transliteration: %q", tr)
	}
	if chapter := expectStatus(t, "/v1/classics/gita/2", http.StatusOK).json(t)["data"].([]any); len(chapter) != 72 {
		t.Errorf("Gita 2: %d verses", len(chapter))
	}
	a := expectStatus(t, "/v1/classics/aathichoodi/1", http.StatusOK).json(t)["data"].(map[string]any)
	if a["lines"].([]any)[0] != "அறம் செய விரும்பு" || a["ref"] != "1" {
		t.Errorf("Aathichoodi 1: %v", a)
	}
	if k := expectStatus(t, "/v1/classics/kabir/1.1", http.StatusOK).json(t)["data"].(map[string]any); len(k["lines"].([]any)) != 2 {
		t.Errorf("Kabir 1.1: %v", k)
	}
	if one := expectStatus(t, "/v1/classics/kabir/15", http.StatusOK).json(t)["data"].([]any); len(one) != 1 {
		t.Errorf("a part with one couplet is still a list: %v", one)
	}
	expectStatus(t, "/v1/classics/gita/random", http.StatusOK)
	if parts := expectStatus(t, "/v1/classics/gita", http.StatusOK).json(t)["data"].(map[string]any)["parts"].([]any); len(parts) != 18 {
		t.Errorf("Gita parts: %d", len(parts))
	}
	expectStatus(t, "/v1/classics/gita/19.1", http.StatusNotFound)
	expectStatus(t, "/v1/classics/vedas", http.StatusNotFound)
	expectStatus(t, "/v1/classics/gita/x", http.StatusBadRequest)
}
