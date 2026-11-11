package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func init() {
	fakeZone["pages.site.in. A"] = []string{"pages.site.in. 300 IN A " + testSiteAddress}
	fakeZone["closed-robots.site.in. A"] = []string{"closed-robots.site.in. 300 IN A " + testSiteAddress}
	mcpTestCalls["convert_data_format"] = `{"text":"a: 1\nb: [x, y]\n","to":"json"}`
	mcpTestCalls["compare_texts"] = `{"a":"one\ntwo\n","b":"one\nthree\n"}`
	mcpTestCalls["explain_cron"] = `{"expression":"0 9 * * 1","count":2}`
	mcpTestCalls["convert_time_zone"] = `{"at":"2026-11-11 15:00","to":"Europe/Berlin"}`
	mcpTestCalls["make_ids"] = `{"kind":"ulid","count":2}`
	mcpTestCalls["decode_jwt"] = `{"token":"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJkZW1vIn0.c2ln"}`
	mcpTestCalls["hash_text"] = `{"text":"abc","algo":"sha256"}`
	mcpTestCalls["check_color_contrast"] = `{"fg":"777777","bg":"ffffff"}`
	mcpTestCalls["preview_link"] = `{"url":"https://dharun.dev"}`
	mcpTestCalls["read_web_page"] = `{"url":"https://dharun.dev"}`
}

func TestConvertAndDiff(t *testing.T) {
	got := request(t, http.MethodPost, "/v1/convert?to=json", "name: India API\nport: 8080\n", "Content-Type", "text/plain")
	if got.status != http.StatusOK || !strings.Contains(string(got.body), `\"port\": 8080`) || got.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("convert %d %s", got.status, got.body)
	}
	raw := request(t, http.MethodPost, "/v1/convert?from=json&to=csv&raw=true", `[{"a":"1","b":"2"}]`, "Content-Type", "text/plain")
	if raw.status != http.StatusOK || string(raw.body) != "a,b\n1,2\n" || !strings.HasPrefix(raw.header.Get("Content-Type"), "text/csv") {
		t.Errorf("raw csv %d %q %s", raw.status, raw.body, raw.header.Get("Content-Type"))
	}
	if got := request(t, http.MethodPost, "/v1/convert?from=json", "{broken", "Content-Type", "text/plain"); got.status != http.StatusUnprocessableEntity {
		t.Errorf("broken json = %d", got.status)
	}
	diff := request(t, http.MethodPost, "/v1/diff", `{"a":{"x":1,"y":[1,2]},"b":{"x":2,"y":[1]}}`)
	if diff.status != http.StatusOK || !strings.Contains(string(diff.body), `"path":"/x"`) || !strings.Contains(string(diff.body), `"path":"/y/1"`) {
		t.Errorf("json diff %s", diff.body)
	}
}

func TestDeveloperUtilities(t *testing.T) {
	cron := expectStatus(t, "/v1/cron?expr=30%209%20*%20*%201-5&from=2026-11-13T10:00:00%2B05:30&count=1", http.StatusOK).data(t)
	if cron["next"].([]any)[0] != "2026-11-16T09:30:00+05:30" || !strings.HasPrefix(cron["description"].(string), "At 09:30") {
		t.Errorf("cron %v", cron)
	}
	expectStatus(t, "/v1/cron?expr=61%20*%20*%20*%20*", http.StatusBadRequest)
	ids := expectStatus(t, "/v1/uuid?kind=uuid7&count=3", http.StatusOK).data(t)["ids"].([]any)
	if len(ids) != 3 || len(ids[0].(string)) != 36 {
		t.Errorf("ids %v", ids)
	}
	if read := expectStatus(t, "/v1/uuid/"+ids[0].(string), http.StatusOK).data(t); read["version"].(float64) != 7 {
		t.Errorf("read %v", read)
	}
	hash := request(t, http.MethodPost, "/v1/hash?algo=sha256", "abc", "Content-Type", "text/plain")
	if !strings.Contains(string(hash.body), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad") {
		t.Errorf("hash %s", hash.body)
	}
	jwt := request(t, http.MethodPost, "/v1/jwt/decode", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0.", "Content-Type", "text/plain")
	if jwt.status != http.StatusOK || !strings.Contains(string(jwt.body), "not signed") {
		t.Errorf("jwt %d %s", jwt.status, jwt.body)
	}
	ua := expectStatus(t, "/v1/user-agent?ua=Mozilla%2F5.0%20(iPhone%3B%20CPU%20iPhone%20OS%2017_4%20like%20Mac%20OS%20X)%20AppleWebKit%2F605.1.15%20(KHTML%2C%20like%20Gecko)%20Version%2F17.4%20Mobile%2F15E148%20Safari%2F604.1", http.StatusOK).data(t)
	agent := ua["agent"].(map[string]any)
	if agent["browser"] != "Safari" || agent["os"] != "iOS" || agent["osVersion"] != "17.4" || agent["device"] != "mobile" || agent["vendor"] != "Apple" {
		t.Errorf("agent %v", agent)
	}
	when := expectStatus(t, "/v1/time?at=2026-11-11%2015:00&from=IST&to=America/New_York,Asia/Tokyo", http.StatusOK).data(t)
	zones := when["zones"].([]any)
	if len(zones) != 3 || zones[1].(map[string]any)["local"] != "Wed 11 Nov 2026, 04:30" || zones[2].(map[string]any)["local"] != "Wed 11 Nov 2026, 18:30" {
		t.Errorf("time %v", zones)
	}
	md := request(t, http.MethodPost, "/v1/markdown?to=markdown", "<h2>Hi</h2><p>a <em>b</em></p>", "Content-Type", "text/plain")
	if !strings.Contains(string(md.body), `"markdown":"## Hi\n\na *b*"`) {
		t.Errorf("markdown %s", md.body)
	}
	colour := expectStatus(t, "/v1/color/saffron", http.StatusOK).data(t)
	if colour["hex"] != "#ff9933" || colour["textColor"] != "#000000" {
		t.Errorf("colour %v", colour)
	}
	contrast := expectStatus(t, "/v1/color/contrast?fg=999&bg=fff", http.StatusOK).data(t)["contrast"].(map[string]any)
	if contrast["passes"].(map[string]any)["AA"] != false || contrast["suggestion"] == "" {
		t.Errorf("contrast %v", contrast)
	}
}

func TestPictureAndSheetTools(t *testing.T) {
	photo := sampleOf(t, "photo.jpg")
	if got := postFiles(t, "/v1/color/palette?count=3", nil, photo); got.status != http.StatusOK || len(got.json(t)["data"].([]any)) == 0 {
		t.Errorf("palette %d %s", got.status, clip(string(got.body), 200))
	}
	thumb := postFiles(t, "/v1/image/thumbhash", nil, photo)
	if thumb.status != http.StatusOK || thumb.json(t)["data"].(map[string]any)["bytes"].(float64) < 20 {
		t.Errorf("thumbhash %d %s", thumb.status, thumb.body)
	}
	icons := postFiles(t, "/v1/image/icons?name=Test", nil, photo)
	archive, err := zip.NewReader(bytes.NewReader(icons.body), int64(len(icons.body)))
	if icons.status != http.StatusOK || err != nil || len(archive.File) != 7 {
		t.Fatalf("icons %d %v", icons.status, err)
	}
	if got := postFiles(t, "/v1/image/svg-to-png", nil, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><image href="file:///etc/passwd"/></svg>`)); got.status != http.StatusUnprocessableEntity {
		t.Errorf("an SVG naming a file = %d", got.status)
	}
	sheet := postFiles(t, "/v1/sheet/to-json", nil, sampleOf(t, "table.csv"))
	if sheet.status != http.StatusOK || !strings.Contains(string(sheet.body), `"district":"Namakkal"`) {
		t.Errorf("sheet %d %s", sheet.status, sheet.body)
	}
	book := request(t, http.MethodPost, "/v1/sheet/from-json", `{"records":[{"a":1,"b":"x"}]}`, "Content-Type", "application/json")
	if book.status != http.StatusOK || !bytes.HasPrefix(book.body, []byte("PK")) {
		t.Fatalf("workbook %d", book.status)
	}
	back := postFiles(t, "/v1/sheet/to-json", nil, book.body)
	if !strings.Contains(string(back.body), `"records":[{"a":"1","b":"x"}]`) {
		t.Errorf("round trip %s", back.body)
	}
}

func TestPDFExtras(t *testing.T) {
	document := sampleOf(t, "document.pdf")
	for _, path := range []string{"/v1/pdf/watermark?text=DRAFT", "/v1/pdf/page-numbers?style=of"} {
		if got := postFiles(t, path, nil, document); got.status != http.StatusOK || !bytes.HasPrefix(got.body, []byte("%PDF-")) {
			t.Errorf("%s = %d %s", path, got.status, clip(string(got.body), 200))
		}
	}
	if got := postFiles(t, "/v1/pdf/unlock", map[string]string{"password": "wrong"}, sampleOf(t, "locked.pdf")); got.status != http.StatusUnprocessableEntity {
		t.Errorf("wrong password = %d", got.status)
	}
	if got := postFiles(t, "/v1/pdf/from-images?page=fit", nil, sampleOf(t, "photo.jpg"), sampleOf(t, "codes.png")); got.status != http.StatusOK {
		t.Errorf("from images = %d %s", got.status, clip(string(got.body), 200))
	}
}

func TestMediaTools(t *testing.T) {
	got := postFiles(t, "/v1/media/waveform?width=300&height=60", nil, sampleOf(t, "speech.mp3"))
	if missingTool(got) {
		t.Skip("ffmpeg is not installed")
	}
	if got.status != http.StatusOK || !bytes.HasPrefix(got.body, []byte("\x89PNG")) {
		t.Errorf("waveform %d", got.status)
	}
	if got := postFiles(t, "/v1/media/thumbnail?at=0.5", nil, sampleOf(t, "clip.mp4")); got.status != http.StatusOK || !bytes.HasPrefix(got.body, []byte{0xFF, 0xD8}) {
		t.Errorf("thumbnail %d", got.status)
	}
	expectStatus(t, "/v1/media/convert", http.StatusMethodNotAllowed)
	if got := postFiles(t, "/v1/media/convert?format=exe", nil, sampleOf(t, "speech.mp3")); got.status != http.StatusBadRequest {
		t.Errorf("unknown format = %d", got.status)
	}
}

func TestRequestBins(t *testing.T) {
	made := request(t, http.MethodPost, "/v1/hooks", "{}", "Content-Type", "application/json")
	if made.status != http.StatusOK {
		t.Fatalf("make %d %s", made.status, made.body)
	}
	bin := made.json(t)["data"].(map[string]any)
	id, secret := bin["id"].(string), bin["secret"].(string)
	if caught := request(t, http.MethodPost, "/v1/hook/"+id+"?order=42", `{"paid":true}`, "Content-Type", "application/json", "CF-Connecting-IP", "49.204.0.9"); caught.status != http.StatusOK {
		t.Fatalf("catch %d", caught.status)
	}
	if got := request(t, http.MethodGet, "/v1/hooks/"+id, "", "X-Secret", "wrong"); got.status != http.StatusNotFound {
		t.Errorf("wrong secret = %d", got.status)
	}
	read := request(t, http.MethodGet, "/v1/hooks/"+id, "", "X-Secret", secret)
	if read.status != http.StatusOK || !strings.Contains(string(read.body), `"body":"{\"paid\":true}"`) || strings.Contains(string(read.body), "49.204.0.9") {
		t.Errorf("read %s", read.body)
	}
	if got := request(t, http.MethodPost, "/v1/hooks/"+id+"/delete", "{}", "X-Secret", secret); got.status != http.StatusOK {
		t.Errorf("delete %d", got.status)
	}
	if got := request(t, http.MethodPost, "/v1/hook/"+id, "x"); got.status != http.StatusNotFound {
		t.Errorf("deleted bin still catches: %d", got.status)
	}
}

func TestScratchDatabase(t *testing.T) {
	made := request(t, http.MethodPost, "/v1/db", "{}", "Content-Type", "application/json")
	if made.status != http.StatusOK {
		t.Fatalf("make %d %s", made.status, made.body)
	}
	db := made.json(t)["data"].(map[string]any)
	query := func(sql string) result {
		body, _ := json.Marshal(map[string]any{"sql": sql})
		return request(t, http.MethodPost, "/v1/db/"+db["id"].(string)+"/query", string(body), "X-Secret", db["secret"].(string), "Content-Type", "application/json")
	}
	if got := query("CREATE TABLE shop (name TEXT, rupees INTEGER)"); got.status != http.StatusOK {
		t.Fatalf("create %s", got.body)
	}
	query("INSERT INTO shop VALUES ('rice', 300), ('dal', 150)")
	if got := query("SELECT name FROM shop ORDER BY rupees DESC"); !strings.Contains(string(got.body), `"rows":[["rice"],["dal"]]`) {
		t.Errorf("select %s", got.body)
	}
	for _, bad := range []string{"ATTACH DATABASE '/etc/passwd' AS x", "PRAGMA max_page_count = 100000", "VACUUM INTO '/tmp/x.db'", "SELECT load_extension('x')"} {
		if got := query(bad); got.status != http.StatusBadRequest {
			t.Errorf("%s = %d", bad, got.status)
		}
	}
	if got := query("WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n) SELECT count(*) FROM n"); got.status != http.StatusUnprocessableEntity {
		t.Errorf("an endless query = %d", got.status)
	}
	demo := request(t, http.MethodPost, "/v1/db/demo/query?secret=demo", `{"sql":"DELETE FROM states"}`)
	if demo.status != http.StatusUnprocessableEntity {
		t.Errorf("the demo database took a write: %d %s", demo.status, demo.body)
	}
	request(t, http.MethodPost, "/v1/db/"+db["id"].(string)+"/delete", "{}", "X-Secret", db["secret"].(string))
}

func TestUnfurlAndRead(t *testing.T) {
	sites := useTestSites(t)
	sites.serve("pages.site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nDisallow: /private\n"))
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<html lang="ta"><head><title>Kadai</title><meta property="og:title" content="Sri Murugan Stores"><meta property="og:image" content="/shop.jpg">
<link rel="apple-touch-icon" href="/touch.png"></head><body><nav>Menu</nav><main><h1>Rice</h1><p>Ponni, 60 a kilo.</p></main></body></html>`))
		}
	}))
	un := expectStatus(t, "/v1/unfurl?url=https://pages.site.in/shop", http.StatusOK).data(t)
	if un["title"] != "Sri Murugan Stores" || un["image"] != "https://pages.site.in/shop.jpg" || un["icon"] != "https://pages.site.in/touch.png" || un["language"] != "ta" {
		t.Errorf("unfurl %v", un)
	}
	read := expectStatus(t, "/v1/read?url=https://pages.site.in/shop", http.StatusOK).data(t)
	if read["markdown"] != "# Rice\n\nPonni, 60 a kilo." || read["title"] != "Kadai" {
		t.Errorf("read %v", read)
	}
	expectStatus(t, "/v1/read?url=https://pages.site.in/private/page", http.StatusForbidden)
	expectStatus(t, "/v1/unfurl?url=https://inside.site.in/", http.StatusUnprocessableEntity)
	expectStatus(t, "/v1/screenshot?url=https://inside.site.in/", http.StatusUnprocessableEntity)
}

func TestRobots(t *testing.T) {
	rules := "User-agent: *\nDisallow: /\n\nUser-agent: IndiaAPI-Reader\nAllow: /docs\nDisallow: /docs/private$\n"
	for path, want := range map[string]bool{"/": true, "/docs/x": true, "/docs/private": false, "/docs/private/more": true} {
		if got := robotsAllows(rules, "indiaapi-reader", path); got != want {
			t.Errorf("%s = %v", path, got)
		}
	}
	if !robotsAllows("User-agent: *\nDisallow:\n", "indiaapi-reader", "/a") || robotsAllows("User-agent: *\nDisallow: /*.pdf$\n", "x", "/a/b.pdf") {
		t.Error("empty disallow or wildcard")
	}
}
