package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func init() {
	mcpTestCalls["calculate_indian_money"] = `{"kind":"emi","inputs":{"amount":1000000,"rate":8.5,"years":20}}`
	mcpTestCalls["today_in_india"] = `{"state":"TN","date":"2026-01-15"}`
	mcpTestCalls["batch_lookup"] = `{"kind":"pincode","values":["637001","000000"]}`
	mcpTestCalls["make_chart_image"] = `{"type":"bar","labels":["A","B"],"series":[{"name":"x","values":[1,2]}]}`
}

func TestCalculators(t *testing.T) {
	got := expectStatus(t, "/v1/calc/emi?amount=1000000&rate=8.5&years=20", http.StatusOK).json(t)
	if emi := got["data"].(map[string]any)["emi"].(float64); emi != 8678.23 {
		t.Errorf("EMI %v", emi)
	}
	tax := expectStatus(t, "/v1/calc/income-tax?income=1,275,000&year=2025-26", http.StatusOK).json(t)["data"].(map[string]any)
	if tax["new"].(map[string]any)["total"].(float64) != 0 || tax["better"] != "new" {
		t.Errorf("income tax %v", tax)
	}
	for _, path := range []string{"/v1/calc/emi?amount=abc&rate=8&years=1", "/v1/calc/gst?amount=100&rate=17", "/v1/calc/ppf?yearly=10", "/v1/calc/income-tax?income=1&salaried=maybe"} {
		expectStatus(t, path, http.StatusBadRequest)
	}
	expectStatus(t, "/v1/calc/unknown", http.StatusNotFound)
}

func TestBatch(t *testing.T) {
	got := request(t, http.MethodPost, "/v1/batch", `{"kind":"pincode","values":["637001","000000"]}`, "Content-Type", "application/json")
	if got.status != http.StatusOK {
		t.Fatalf("batch = %d %s", got.status, got.body)
	}
	answers := got.json(t)["data"].([]any)
	first, second := answers[0].(map[string]any), answers[1].(map[string]any)
	if first["status"].(float64) != 200 || first["data"].(map[string]any)["district"] != "Namakkal" || second["status"].(float64) != 400 || second["error"] == nil {
		t.Errorf("answers %v", answers)
	}
	if got.header.Get("Cache-Control") != "private, no-store" {
		t.Error("a batch answer may be cached")
	}
	mixed := request(t, http.MethodPost, "/v1/batch?format=ndjson", `{"lookups":[{"kind":"gstin","value":"27AAPFU0939F1ZV"},{"kind":"ifsc","value":"SBIN0000001"}]}`)
	if lines := strings.Split(strings.TrimSpace(string(mixed.body)), "\n"); len(lines) != 2 || !strings.Contains(lines[0], `"valid":true`) {
		t.Errorf("ndjson %s", mixed.body)
	}
	for _, body := range []string{`{"kind":"nope","values":["1"]}`, `{"kind":"pincode","values":[]}`, `not json`} {
		if got := request(t, http.MethodPost, "/v1/batch", body); got.status != http.StatusBadRequest {
			t.Errorf("%s = %d", body, got.status)
		}
	}
	values := make([]string, batchMax+1)
	for i := range values {
		values[i] = "637001"
	}
	encoded, _ := json.Marshal(map[string]any{"kind": "pincode", "values": values})
	if got := request(t, http.MethodPost, "/v1/batch", string(encoded)); got.status != http.StatusBadRequest {
		t.Errorf("1,001 lookups = %d", got.status)
	}
}

func TestBatchCharges(t *testing.T) {
	server := newTestServer(Config{Version: "t", RateLimitPerMinute: 20})
	defer server.Close()
	values := make([]string, 300)
	for i := range values {
		values[i] = "637001"
	}
	encoded, _ := json.Marshal(map[string]any{"kind": "pincode", "values": values})
	// 300 lookups cost 30 units: more than this allowance.
	if got := send(server, "POST", "/v1/batch", string(encoded)); got.Code != http.StatusTooManyRequests {
		t.Errorf("300 lookups on 20 units = %d", got.Code)
	}
}

func TestToday(t *testing.T) {
	defer func(saved func() time.Time) { todayClock = saved }(todayClock)
	todayClock = func() time.Time { return time.Date(2026, 1, 15, 10, 0, 0, 0, indiaTime) }
	data := expectStatus(t, "/v1/today?state=TN", http.StatusOK).json(t)["data"].(map[string]any)
	if data["date"] != "2026-01-15" || data["financialYear"] != "2025-26" || data["quarter"] != "Q4" || data["assessmentYear"] != "2026-27" {
		t.Errorf("today %v", data)
	}
	if bank, ok := data["working"].(map[string]any)["bank"].(map[string]any); !ok || bank["working"] != false {
		t.Errorf("15 January 2026 (Pongal) should not be a bank day in Tamil Nadu: %v", data["working"])
	}
	expectStatus(t, "/v1/today?pincode=12", http.StatusBadRequest)
	expectStatus(t, "/v1/today?state=XX", http.StatusBadRequest)
}

func TestHolidayCalendar(t *testing.T) {
	got := expectStatus(t, "/v1/holidays/2026.ics?type=bank&state=TN", http.StatusOK)
	body := string(got.body)
	if !strings.HasPrefix(got.header.Get("Content-Type"), "text/calendar") || !strings.HasPrefix(body, "BEGIN:VCALENDAR\r\n") ||
		!strings.Contains(body, "BEGIN:VEVENT") || !strings.HasSuffix(body, "END:VCALENDAR\r\n") {
		t.Errorf("calendar %s", clip(body, 300))
	}
	for _, line := range strings.Split(body, "\r\n") {
		if len(line) > 75 {
			t.Errorf("unfolded line %q", line)
		}
	}
	expectStatus(t, "/v1/holidays/20x6.ics", http.StatusBadRequest)
}

func TestPictures(t *testing.T) {
	avatar := expectStatus(t, "/v1/avatar/Priya%20Raman?shape=rounded", http.StatusOK)
	if !strings.HasPrefix(string(avatar.body), "<svg") || !strings.Contains(string(avatar.body), "<title>Priya Raman</title>") {
		t.Errorf("avatar %s", clip(string(avatar.body), 200))
	}
	if got := initials("ராமன் குமார்"); got != "ராகு" {
		t.Errorf("Tamil initials %q", got)
	}
	if got := initials("क्षमा"); got != "क्ष" {
		t.Errorf("Hindi conjunct initial %q", got)
	}
	expectStatus(t, "/v1/placeholder/600x400?text=Hero", http.StatusOK)
	expectStatus(t, "/v1/placeholder/5x5", http.StatusBadRequest)
	expectStatus(t, "/v1/placeholder/axb", http.StatusBadRequest)
	if named := string(get(t, "/v1/chart?type=bar&values=1,2&values=3,4&names=First,Second&format=svg").body); !strings.Contains(named, "<title>bar chart</title>") || strings.Count(named, "<rect x=") < 6 {
		t.Errorf("two named series: %s", clip(named, 200))
	}
	for _, path := range []string{"/v1/chart?type=line&values=1,3,2&values=2,2,4&names=a,b&format=svg", "/v1/chart?type=pie&values=3,1&labels=x,y&format=svg", "/v1/chart?type=sparkline&values=1,5,3,9&format=svg"} {
		if got := expectStatus(t, path, http.StatusOK); !strings.HasPrefix(string(got.body), "<svg") {
			t.Errorf("%s: %s", path, clip(string(got.body), 100))
		}
	}
	posted := request(t, http.MethodPost, "/v1/chart?format=svg", `{"type":"bar","labels":["Q1","Q2"],"series":[{"values":[150000,2500000]}]}`, "Content-Type", "application/json")
	if posted.status != http.StatusOK || !strings.Contains(string(posted.body), "<rect") {
		t.Errorf("posted chart = %d", posted.status)
	}
	for _, path := range []string{"/v1/chart?values=a,b", "/v1/chart?type=radar&values=1", "/v1/chart"} {
		expectStatus(t, path, http.StatusBadRequest)
	}
	if formatTick(2500000) != "25L" || formatTick(30000000) != "3Cr" || formatTick(1500) != "1.5K" {
		t.Error("Indian axis labels")
	}
}

func TestInteractivePages(t *testing.T) {
	for _, path := range []string{"/play", "/tools", "/tools/gst-invoice", "/tools/image-to-text", "/demos", "/demos/market-clock", "/logos", "/logos/TCS", "/keys", "/changelog"} {
		got := request(t, http.MethodGet, path, "", "Accept", "text/html")
		if got.status != http.StatusOK || !strings.Contains(string(got.body), "<h1") {
			t.Errorf("%s = %d", path, got.status)
		}
	}
	// The playground carries a spec for every endpoint, filled from its example.
	play := string(get(t, "/play").body)
	start := strings.Index(play, `<script type="application/json" id="page-data">`)
	end := strings.Index(play[start:], "</script>")
	var data struct {
		Specs []trySpec `json:"specs"`
	}
	if err := json.Unmarshal([]byte(play[start+len(`<script type="application/json" id="page-data">`):start+end]), &data); err != nil {
		t.Fatalf("page data: %v", err)
	}
	count := 0
	for _, group := range catalog {
		count += len(group.Endpoints)
	}
	if len(data.Specs) != count {
		t.Errorf("%d specs for %d endpoints", len(data.Specs), count)
	}
	for _, spec := range data.Specs {
		for _, p := range spec.Params {
			if p.In == "path" && p.Value == "" {
				t.Errorf("%s: path parameter %s has no example value", spec.Path, p.Name)
			}
		}
		if _, registered := testServer.costs[strings.TrimSuffix(spec.Path, ".ics")]; !registered {
			t.Errorf("%s: no route registered for its cost", spec.Path)
		}
	}
	// Every API page has a try-it panel per endpoint.
	pincode := string(get(t, "/docs/pincode").body)
	if strings.Count(pincode, `class="tryit"`) != 3 || !strings.Contains(pincode, "/assets/try.js") {
		t.Errorf("pincode page panels: %d", strings.Count(pincode, `class="tryit"`))
	}
	if samples := get(t, "/play/samples.json"); samples.status != http.StatusOK || !json.Valid(samples.body) {
		t.Errorf("samples = %d", samples.status)
	}
	// A logo page has one address: other spellings move to it.
	if got := get(t, "/logos/TCS.NS"); got.status != http.StatusMovedPermanently || got.header.Get("Location") != "/logos/TCS" {
		t.Errorf("TCS.NS = %d %s", got.status, got.header.Get("Location"))
	}
	expectStatus(t, "/logos/NOSUCHCOMPANY", http.StatusNotFound)
	expectStatus(t, "/tools/nothing", http.StatusNotFound)
	for _, file := range []string{"try.js", "play.js", "tools.js", "demos.js", "logos.js"} {
		if got := get(t, "/assets/"+file); got.status != http.StatusOK {
			t.Errorf("%s = %d", file, got.status)
		}
	}
}
