package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

const testOrigin = "https://api.example.test"

var (
	testServer *Server
	testHTTP   *httptest.Server
	testDBPath string
)

// TestMain builds a database from the small real-data fixtures in testdata and serves it.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "api-test-")
	if err != nil {
		panic(err)
	}
	testDBPath = filepath.Join(dir, "api.db")
	if _, err := dataset.Build(dataset.BuildOptions{
		DataDir: filepath.Join("testdata", "data"), LogosDir: filepath.Join("testdata", "logos"), Out: testDBPath, Strict: true,
	}); err != nil {
		panic(err)
	}
	testServer = newTestServer(Config{Version: "1.2.3", PublicOrigin: testOrigin})
	testHTTP = httptest.NewServer(testServer)

	code := m.Run()
	testHTTP.Close()
	testServer.Close()
	testServer.db.Close()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newTestServer(cfg Config) *Server {
	db, err := dataset.Open(testDBPath)
	if err != nil {
		panic(err)
	}
	if cfg.PNGCacheBytes == 0 {
		cfg.PNGCacheBytes = 1 << 20
	}
	server, err := New(cfg, db)
	if err != nil {
		panic(err)
	}
	return server
}

type result struct {
	status int
	header http.Header
	body   []byte
}

// json decodes the body into a generic value for assertions.
func (r result) json(t *testing.T) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(r.body, &decoded); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, r.body)
	}
	return decoded
}

func (r result) data(t *testing.T) map[string]any {
	t.Helper()
	data, ok := r.json(t)["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is not an object: %s", r.body)
	}
	return data
}

func (r result) list(t *testing.T) []any {
	t.Helper()
	list, ok := r.json(t)["data"].([]any)
	if !ok {
		t.Fatalf("data is not a list: %s", r.body)
	}
	return list
}

func request(t *testing.T, method, path string, body string, headers ...string) result {
	t.Helper()
	req, err := http.NewRequest(method, testHTTP.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	return result{response.StatusCode, response.Header, raw}
}

func get(t *testing.T, path string) result { return request(t, http.MethodGet, path, "") }

func expectStatus(t *testing.T, path string, want int) result {
	t.Helper()
	got := get(t, path)
	if got.status != want {
		t.Errorf("GET %s = %d, want %d: %s", path, got.status, want, clip(string(got.body), 200))
	}
	return got
}

func TestEveryDocumentedExampleWorks(t *testing.T) {
	documented := map[string]bool{}
	for _, group := range catalog {
		for _, e := range group.Endpoints {
			documented["GET "+e.Path] = true
			got := expectStatus(t, e.Example, http.StatusOK)
			if e.Image {
				if !strings.HasPrefix(got.header.Get("Content-Type"), "image/") {
					t.Errorf("%s: content type %q", e.Example, got.header.Get("Content-Type"))
				}
				continue
			}
			envelope := got.json(t)
			if _, ok := envelope["data"]; !ok {
				t.Errorf("%s: no data in envelope", e.Example)
			}
			if _, ok := envelope["meta"].(map[string]any); !ok {
				t.Errorf("%s: no meta in envelope", e.Example)
			}
			if group.SourceKey != "" && envelope["meta"].(map[string]any)["source"] == nil {
				t.Errorf("%s: response does not credit its source", e.Example)
			}
		}
	}
	// Every data route must be documented, or it would be invisible to docs, OpenAPI and llms.txt.
	for pattern := range testServer.routes {
		if strings.HasPrefix(pattern, "GET /v1/") && !documented[pattern] {
			t.Errorf("route %q is not in the catalog", pattern)
		}
	}
}

func TestLogos(t *testing.T) {
	logo := expectStatus(t, "/v1/logos/TCS", http.StatusOK)
	if !bytes.HasPrefix(logo.body, []byte("<svg")) || !bytes.Contains(logo.body, []byte("viewBox")) {
		t.Errorf("unexpected SVG: %s", clip(string(logo.body), 80))
	}
	for name, want := range map[string]string{
		"Access-Control-Allow-Origin": "*", "X-Content-Type-Options": "nosniff", "Cache-Control": cacheLong, "Content-Security-Policy": svgPolicy,
	} {
		if got := logo.header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if again := request(t, http.MethodGet, "/v1/logos/TCS", "", "If-None-Match", logo.header.Get("ETag")); again.status != http.StatusNotModified {
		t.Errorf("revalidation = %d, want 304", again.status)
	}

	for _, symbol := range []string{"tcs", "NSE:TCS", "NSE_TCS", "TCS.NS", "INE467B01029", "TCS.svg", "TCS?exchange=nse"} {
		if got := expectStatus(t, "/v1/logos/"+symbol, http.StatusOK); !bytes.Equal(got.body, logo.body) {
			t.Errorf("%s returned a different image", symbol)
		}
	}
	expectStatus(t, "/v1/logos/M%26M", http.StatusOK)
	expectStatus(t, "/v1/logos/MM", http.StatusOK)
	expectStatus(t, "/v1/logos/BAJAJ-AUTO", http.StatusOK)
	expectStatus(t, "/v1/logos/TCS?exchange=bse", http.StatusNotFound)

	missing := expectStatus(t, "/v1/logos/NOTLISTED", http.StatusNotFound)
	if code := missing.json(t)["error"].(map[string]any)["code"]; code != "not_found" {
		t.Errorf("error code = %v", code)
	}
	fallback := expectStatus(t, "/v1/logos/NOTLISTED?fallback=monogram", http.StatusOK)
	if fallback.header.Get("X-Logo-Fallback") != "monogram" || !bytes.Contains(fallback.body, []byte(">NO</text>")) {
		t.Errorf("unexpected monogram: %s", fallback.body)
	}
	hostile := expectStatus(t, "/v1/logos/%22%3E%3Cscript%3Ealert(1)%3C%2Fscript%3E?fallback=monogram", http.StatusOK)
	if bytes.Contains(hostile.body, []byte("<script")) || bytes.Contains(hostile.body, []byte("alert")) {
		t.Errorf("monogram echoed markup: %s", hostile.body)
	}

	for _, path := range []string{
		"/v1/logos/TCS?format=gif", "/v1/logos/TCS?format=png&size=8", "/v1/logos/TCS?format=png&size=4096",
		"/v1/logos/TCS?format=png&size=abc", "/v1/logos/TCS?fallback=blank", "/v1/logos/TCS?exchange=nyse",
	} {
		expectStatus(t, path, http.StatusBadRequest)
	}
}

func TestPNG(t *testing.T) {
	// The fixtures are built without the PNG renderer, which must be reported, not hidden.
	expectStatus(t, "/v1/logos/TCS?format=png", http.StatusNotImplemented)

	monogram := expectStatus(t, "/v1/logos/NOTLISTED.png?size=48&fallback=monogram", http.StatusOK)
	decoded, err := png.Decode(bytes.NewReader(monogram.body))
	if err != nil || decoded.Bounds().Dx() != 48 || decoded.Bounds().Dy() != 48 {
		t.Fatalf("monogram PNG: %v %v", err, decoded)
	}

	// Scaling a master down keeps it a valid PNG of exactly the requested size.
	master := image.NewNRGBA(image.Rect(0, 0, dataset.PNGMasterSize, dataset.PNGMasterSize))
	for i := range master.Pix {
		master.Pix[i] = 0x80
	}
	master.Set(10, 10, color.NRGBA{255, 0, 0, 255})
	encoded, _ := encodePNG(master)
	for _, size := range []int{16, 128, dataset.PNGMasterSize} {
		scaled, err := resizePNG(encoded, size)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(scaled))
		if err != nil || img.Bounds().Dx() != size {
			t.Errorf("size %d: %v %v", size, err, img.Bounds())
		}
	}
}

func TestCompanies(t *testing.T) {
	company := expectStatus(t, "/v1/companies/reliance", http.StatusOK).data(t)
	if company["id"] != "NSE:RELIANCE" || company["isin"] == "" {
		t.Errorf("unexpected company: %v", company)
	}
	if svg := company["logo"].(map[string]any)["svg"]; svg != testOrigin+"/v1/logos/NSE:RELIANCE" {
		t.Errorf("logo.svg = %v", svg)
	}
	ampersand := expectStatus(t, "/v1/companies/M%26M", http.StatusOK).data(t)
	if svg := ampersand["logo"].(map[string]any)["svg"]; svg != testOrigin+"/v1/logos/NSE:M%26M" {
		t.Errorf("logo.svg = %v", svg)
	}
	// Links the API hands out must resolve back to the same company.
	for _, symbol := range []string{"TCS", "M%26M", "BIRET.RR", "BAJAJ_AUTO"} {
		data := expectStatus(t, "/v1/companies/"+symbol, http.StatusOK).data(t)
		self := data["links"].(map[string]any)["self"].(string)
		if again := expectStatus(t, strings.TrimPrefix(self, testOrigin), http.StatusOK).data(t); again["id"] != data["id"] {
			t.Errorf("%s: self link resolved to %v", symbol, again["id"])
		}
		expectStatus(t, strings.TrimPrefix(data["logo"].(map[string]any)["svg"].(string), testOrigin), http.StatusOK)
	}
	expectStatus(t, "/v1/companies/NOTLISTED", http.StatusNotFound)

	search := expectStatus(t, "/v1/companies?q=tata&limit=2", http.StatusOK)
	if list := search.list(t); len(list) != 2 {
		t.Errorf("limit not applied: %d results", len(list))
	}
	meta := search.json(t)["meta"].(map[string]any)
	if meta["total"].(float64) < 3 || meta["limit"].(float64) != 2 {
		t.Errorf("unexpected paging: %v", meta)
	}
	if first := expectStatus(t, "/v1/companies?q=TCS", http.StatusOK).list(t)[0].(map[string]any); first["id"] != "NSE:TCS" {
		t.Errorf("exact ticker should rank first, got %v", first["id"])
	}
	for _, c := range expectStatus(t, "/v1/companies?exchange=bse", http.StatusOK).list(t) {
		if c.(map[string]any)["exchange"] != "BSE" {
			t.Errorf("exchange filter leaked %v", c.(map[string]any)["id"])
		}
	}
	expectStatus(t, "/v1/companies?limit=0", http.StatusBadRequest)
	expectStatus(t, "/v1/companies?limit=101", http.StatusBadRequest)
	expectStatus(t, "/v1/companies?offset=-1", http.StatusBadRequest)
}

func TestPincode(t *testing.T) {
	data := expectStatus(t, "/v1/pincode/637001", http.StatusOK).data(t)
	if data["district"] != "Namakkal" || data["stateCode"] != "TN" || data["state"] != "Tamil Nadu" {
		t.Errorf("unexpected pincode: %v", data)
	}
	offices := data["offices"].([]any)
	first := offices[0].(map[string]any)
	if first["name"] != "Namakkal H.O" || first["type"] != "Head Office" || first["latitude"] == nil {
		t.Errorf("head office should come first with coordinates: %v", first)
	}
	expectStatus(t, "/v1/pincode/637%20001", http.StatusOK)
	expectStatus(t, "/v1/pincode/123", http.StatusBadRequest)
	expectStatus(t, "/v1/pincode/012345", http.StatusBadRequest)
	expectStatus(t, "/v1/pincode/100000", http.StatusNotFound)
	// The published file carries a placeholder office under 999999; it must not be served.
	expectStatus(t, "/v1/pincode/999999", http.StatusNotFound)

	for _, office := range expectStatus(t, "/v1/pincode?q=namakkal&state=tamil%20nadu", http.StatusOK).list(t) {
		entry := office.(map[string]any)
		if !strings.HasPrefix(strings.ToLower(entry["name"].(string)), "namakkal") || entry["pincode"] == "" {
			t.Errorf("unexpected search hit: %v", entry)
		}
	}
	if byPrefix := expectStatus(t, "/v1/pincode?q=6370", http.StatusOK).list(t); len(byPrefix) == 0 {
		t.Error("pincode prefix search found nothing")
	}
	if none := expectStatus(t, "/v1/pincode?q=namakkal&state=KA", http.StatusOK).list(t); len(none) != 0 {
		t.Errorf("state filter leaked %d results", len(none))
	}
	expectStatus(t, "/v1/pincode?q=n", http.StatusBadRequest)
	expectStatus(t, "/v1/pincode?q=namakkal&state=Atlantis", http.StatusBadRequest)
	// A search string is data, never a pattern.
	if wild := expectStatus(t, "/v1/pincode?q=%25%25", http.StatusOK).list(t); len(wild) != 0 {
		t.Errorf("wildcards in q matched %d rows", len(wild))
	}
}

func TestIFSCAndBanks(t *testing.T) {
	branch := expectStatus(t, "/v1/ifsc/sbin0000001", http.StatusOK).data(t)
	bank := branch["bank"].(map[string]any)
	if branch["ifsc"] != "SBIN0000001" || bank["name"] != "State Bank of India" || branch["stateCode"] != "WB" || branch["neft"] != true {
		t.Errorf("unexpected branch: %v", branch)
	}
	expectStatus(t, "/v1/ifsc/SBIN1000001", http.StatusBadRequest)
	expectStatus(t, "/v1/ifsc/SBIN0999999", http.StatusNotFound)

	for _, entry := range expectStatus(t, "/v1/ifsc?bank=sbin&state=TN&q=namakkal", http.StatusOK).list(t) {
		b := entry.(map[string]any)
		if b["stateCode"] != "TN" || !strings.HasPrefix(b["ifsc"].(string), "SBIN") {
			t.Errorf("unexpected branch in search: %v", b["ifsc"])
		}
	}
	expectStatus(t, "/v1/ifsc?q=namakkal", http.StatusBadRequest)
	expectStatus(t, "/v1/ifsc?bank=SBIN&state=Atlantis", http.StatusBadRequest)

	hdfc := expectStatus(t, "/v1/banks/hdfc", http.StatusOK).data(t)
	if hdfc["name"] != "HDFC Bank" || hdfc["branches"].(float64) < 1 {
		t.Errorf("unexpected bank: %v", hdfc)
	}
	// A bank that appears only in the branch list still resolves, by name.
	if rural := expectStatus(t, "/v1/banks/PKGB", http.StatusOK).data(t); rural["name"] == "" {
		t.Errorf("unexpected bank: %v", rural)
	}
	expectStatus(t, "/v1/banks/ZZZZ", http.StatusNotFound)
	expectStatus(t, "/v1/banks/SBI", http.StatusBadRequest)
	if found := expectStatus(t, "/v1/banks?q=state%20bank", http.StatusOK).list(t); len(found) != 1 {
		t.Errorf("bank search found %d", len(found))
	}
}

func TestRTO(t *testing.T) {
	for _, input := range []string{"TN-28", "tn28", "TN%2028", "TN28AB1234", "TN-28-AB-1234", "TN28A1234"} {
		data := expectStatus(t, "/v1/rto/"+input, http.StatusOK).data(t)
		if data["code"] != "TN-28" || data["office"] != "Namakkal (North)" || data["stateCode"] != "TN" {
			t.Errorf("%s resolved to %v", input, data)
		}
	}
	if unit := expectStatus(t, "/v1/rto/TN28Z", http.StatusOK).data(t); unit["code"] != "TN-28Z" {
		t.Errorf("suffix office not preferred: %v", unit)
	}
	if statewide := expectStatus(t, "/v1/rto/AP39", http.StatusOK).data(t); !strings.Contains(statewide["jurisdiction"].(string), "Guntur") {
		t.Errorf("merged jurisdiction missing: %v", statewide)
	}
	expectStatus(t, "/v1/rto/TN-00", http.StatusNotFound)
	expectStatus(t, "/v1/rto/hello", http.StatusBadRequest)

	list := expectStatus(t, "/v1/rto?state=TN&limit=100", http.StatusOK)
	if total := list.json(t)["meta"].(map[string]any)["total"].(float64); total < 100 {
		t.Errorf("Tamil Nadu has %v codes", total)
	}
	if hits := expectStatus(t, "/v1/rto?q=namakkal", http.StatusOK).list(t); len(hits) < 2 {
		t.Errorf("text search found %d", len(hits))
	}
	expectStatus(t, "/v1/rto", http.StatusBadRequest)
}

func TestHolidays(t *testing.T) {
	all := expectStatus(t, "/v1/holidays/2026", http.StatusOK)
	if len(all.list(t)) < 50 {
		t.Errorf("only %d holidays", len(all.list(t)))
	}
	previous := ""
	for _, entry := range all.list(t) {
		h := entry.(map[string]any)
		if h["date"].(string) < previous {
			t.Fatalf("holidays are not sorted by date at %v", h["date"])
		}
		previous = h["date"].(string)
	}

	market := expectStatus(t, "/v1/holidays/2026?type=market", http.StatusOK).list(t)
	republic := false
	for _, entry := range market {
		h := entry.(map[string]any)
		if h["type"] != "market" {
			t.Errorf("type filter leaked %v", h["type"])
		}
		republic = republic || (h["date"] == "2026-01-26" && h["weekday"] == "Monday")
	}
	if !republic {
		t.Error("Republic Day missing from market holidays")
	}

	// A state sees national holidays and its own, never another state's.
	for _, entry := range expectStatus(t, "/v1/holidays/2026?state=TN&month=1", http.StatusOK).list(t) {
		h := entry.(map[string]any)
		states := fmt.Sprint(h["states"])
		if !strings.Contains(states, "TN") && !strings.Contains(states, "*") {
			t.Errorf("state filter leaked %v (%v)", h["name"], states)
		}
		if !strings.HasPrefix(h["date"].(string), "2026-01-") {
			t.Errorf("month filter leaked %v", h["date"])
		}
	}
	pongal := false
	for _, entry := range expectStatus(t, "/v1/holidays/2026?state=TN&type=bank&month=1", http.StatusOK).list(t) {
		pongal = pongal || strings.Contains(entry.(map[string]any)["name"].(string), "Pongal")
	}
	if !pongal {
		t.Error("Pongal missing from Tamil Nadu bank holidays")
	}

	missing := expectStatus(t, "/v1/holidays/1999", http.StatusNotFound)
	if !strings.Contains(string(missing.body), "2026") {
		t.Errorf("404 should list the years available: %s", missing.body)
	}
	expectStatus(t, "/v1/holidays/abcd", http.StatusBadRequest)
	expectStatus(t, "/v1/holidays/2026?type=school", http.StatusBadRequest)
	expectStatus(t, "/v1/holidays/2026?month=13", http.StatusBadRequest)
	expectStatus(t, "/v1/holidays/2026?state=Atlantis", http.StatusBadRequest)
}

func TestWords(t *testing.T) {
	cases := map[string]string{
		"/v1/words/100000":                      "one lakh",
		"/v1/words/1,00,000":                    "one lakh",
		"/v1/words/1%2000%20000":                "one lakh",
		"/v1/words/-12.50":                      "minus twelve point five zero",
		"/v1/words/1250.5?style=currency":       "one thousand two hundred fifty rupees and fifty paise only",
		"/v1/words/100000?lang=hi":              "एक लाख",
		"/v1/words/100000?lang=ta":              "ஒரு லட்சம்",
		"/v1/words/000":                         "zero",
		"/v1/words/999999999999999":             "",
		"/v1/words/1500?lang=TA&style=CURRENCY": "ஆயிரத்து ஐநூறு ரூபாய் மட்டும்",
	}
	for path, want := range cases {
		data := expectStatus(t, path, http.StatusOK).data(t)
		if want != "" && data["words"] != want {
			t.Errorf("%s = %q, want %q", path, data["words"], want)
		}
	}
	for _, path := range []string{
		"/v1/words/abc", "/v1/words/1.2.3", "/v1/words/1234567890123456", "/v1/words/1?lang=fr",
		"/v1/words/1?style=roman", "/v1/words/-5?style=currency", "/v1/words/1.234?style=currency",
	} {
		expectStatus(t, path, http.StatusBadRequest)
	}
}

func TestValidate(t *testing.T) {
	gstin := expectStatus(t, "/v1/validate/gstin/27aapfu0939f1zv", http.StatusOK)
	data := gstin.data(t)
	if data["valid"] != true || data["normalized"] != "27AAPFU0939F1ZV" || data["parts"].(map[string]any)["state"] != "Maharashtra" {
		t.Errorf("unexpected: %v", data)
	}
	if _, echoed := data["input"]; echoed {
		t.Error("the raw input must not be echoed")
	}
	if cache := gstin.header.Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Errorf("identifier checks must not be cached, got %q", cache)
	}
	if bad := expectStatus(t, "/v1/validate/gstin/27AAPFU0939F1ZW", http.StatusOK).data(t); bad["valid"] != false || bad["reason"] == "" {
		t.Errorf("wrong check character accepted: %v", bad)
	}
	if pan := expectStatus(t, "/v1/validate/pan/ABCPE1234F", http.StatusOK).data(t); pan["valid"] != true {
		t.Errorf("unexpected: %v", pan)
	}
	if known := expectStatus(t, "/v1/validate/ifsc/SBIN0000001", http.StatusOK).data(t); known["valid"] != true || known["exists"] != true {
		t.Errorf("unexpected: %v", known)
	}
	if unknown := expectStatus(t, "/v1/validate/ifsc/SBIN0999999", http.StatusOK).data(t); unknown["valid"] != true || unknown["exists"] != false {
		t.Errorf("unexpected: %v", unknown)
	}
	expectStatus(t, "/v1/validate/aadhaar/123412341234", http.StatusNotFound)
}

func TestStates(t *testing.T) {
	if states := expectStatus(t, "/v1/states", http.StatusOK).list(t); len(states) != 36 {
		t.Errorf("%d states and union territories, want 36", len(states))
	}
	for _, input := range []string{"TN", "tn", "Tamil%20Nadu", "tamilnadu"} {
		if state := expectStatus(t, "/v1/states/"+input, http.StatusOK).data(t); state["code"] != "TN" || state["gstCode"] != "33" {
			t.Errorf("%s resolved to %v", input, state)
		}
	}
	if telangana := expectStatus(t, "/v1/states/TG", http.StatusOK).data(t); telangana["name"] != "Telangana" {
		t.Errorf("vehicle prefix TG resolved to %v", telangana)
	}
	expectStatus(t, "/v1/states/Atlantis", http.StatusNotFound)
}

func TestDocuments(t *testing.T) {
	home := expectStatus(t, "/", http.StatusOK)
	if !strings.Contains(string(home.body), siteTagline) || !strings.Contains(string(home.body), testOrigin+"/v1/pincode/637001") {
		t.Error("home page is missing its headline or example")
	}
	page := expectStatus(t, "/docs/pincode", http.StatusOK)
	if !strings.Contains(string(page.body), `<link rel="canonical" href="`+testOrigin+`/docs/pincode">`) || !strings.Contains(string(page.body), "Department of Posts") {
		t.Error("pincode page is missing its canonical link or source credit")
	}
	expectStatus(t, "/docs/nothing", http.StatusNotFound)

	var openapi struct {
		OpenAPI string
		Servers []struct{ URL string }
		Paths   map[string]any
	}
	if err := json.Unmarshal(expectStatus(t, "/openapi.json", http.StatusOK).body, &openapi); err != nil {
		t.Fatal(err)
	}
	endpoints := 0
	for _, group := range catalog {
		endpoints += len(group.Endpoints)
	}
	if openapi.OpenAPI != "3.1.0" || openapi.Servers[0].URL != testOrigin || len(openapi.Paths) != endpoints {
		t.Errorf("unexpected OpenAPI document: %s %v %d paths for %d endpoints", openapi.OpenAPI, openapi.Servers, len(openapi.Paths), endpoints)
	}

	llms := string(expectStatus(t, "/llms.txt", http.StatusOK).body)
	if !strings.HasPrefix(llms, "# "+siteName) || !strings.Contains(llms, testOrigin+"/v1/rto/TN-28") {
		t.Error("llms.txt is missing its title or an example")
	}
	robots := string(expectStatus(t, "/robots.txt", http.StatusOK).body)
	if !strings.Contains(robots, "User-agent: ClaudeBot") || !strings.Contains(robots, "Sitemap: "+testOrigin+"/sitemap.xml") {
		t.Error("robots.txt is missing an AI crawler or the sitemap")
	}
	if sitemap := string(expectStatus(t, "/sitemap.xml", http.StatusOK).body); strings.Count(sitemap, "<url>") != len(catalog)+1 {
		t.Errorf("sitemap has %d URLs", strings.Count(sitemap, "<url>"))
	}

	index := expectStatus(t, "/v1", http.StatusOK).data(t)
	if len(index["apis"].([]any)) != len(catalog) || index["records"].(map[string]any)["logos"].(float64) < 10 {
		t.Errorf("unexpected index: %v", index["records"])
	}
	health := expectStatus(t, "/healthz", http.StatusOK)
	if !strings.Contains(string(health.body), `"version":"1.2.3"`) {
		t.Errorf("health check does not report the version: %s", health.body)
	}

	gz := request(t, http.MethodGet, "/openapi.json", "", "Accept-Encoding", "gzip")
	if gz.header.Get("Content-Encoding") != "gzip" {
		t.Error("large JSON was not compressed")
	}
}

func TestMethodsAndUnknownPaths(t *testing.T) {
	head := request(t, http.MethodHead, "/v1/companies/TCS", "")
	if head.status != http.StatusOK || len(head.body) != 0 || head.header.Get("Content-Length") == "0" {
		t.Errorf("HEAD: %d, %d bytes, length %s", head.status, len(head.body), head.header.Get("Content-Length"))
	}
	preflight := request(t, http.MethodOptions, "/mcp", "")
	if preflight.status != http.StatusNoContent || !strings.Contains(preflight.header.Get("Access-Control-Allow-Methods"), "POST") {
		t.Errorf("preflight: %d %v", preflight.status, preflight.header)
	}
	for _, method := range []string{http.MethodPost, http.MethodDelete, http.MethodPut} {
		if got := request(t, method, "/v1/logos/TCS", ""); got.status != http.StatusMethodNotAllowed {
			t.Errorf("%s = %d, want 405", method, got.status)
		}
	}
	for _, path := range []string{"/nothing", "/v1/nothing", "/metrics", "/stats", "/api.db", "/v1/logos/a/b", "/go.mod"} {
		expectStatus(t, path, http.StatusNotFound)
	}
	// A path that climbs out is normalised by the router and never reaches a file.
	if got := get(t, "/../go.mod"); got.status == http.StatusOK {
		t.Errorf("path traversal returned 200")
	}
}

func TestMCP(t *testing.T) {
	rpc := func(body string) (int, map[string]any) {
		got := request(t, http.MethodPost, "/mcp", body, "Content-Type", "application/json", "Accept", "application/json, text/event-stream")
		if len(got.body) == 0 {
			return got.status, nil
		}
		return got.status, got.json(t)
	}

	_, init := rpc(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`)
	if init["result"].(map[string]any)["protocolVersion"] != "2025-06-18" {
		t.Errorf("unexpected initialize result: %v", init)
	}
	if _, future := rpc(`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2099-01-01"}}`); future["result"].(map[string]any)["protocolVersion"] != mcpDefaultVersion {
		t.Errorf("unknown protocol version not negotiated down: %v", future)
	}
	if status, body := rpc(`{"jsonrpc":"2.0","method":"notifications/initialized"}`); status != http.StatusAccepted || body != nil {
		t.Errorf("notification: %d %v", status, body)
	}

	_, listed := rpc(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	tools := listed["result"].(map[string]any)["tools"].([]any)
	if len(tools) != len(mcpTools) {
		t.Fatalf("%d tools listed", len(tools))
	}

	// Every tool, called with sensible arguments, returns the same envelope as its REST endpoint.
	calls := map[string]string{
		"get_company_logo":     `{"symbol":"tcs.ns"}`,
		"search_companies":     `{"query":"tata","limit":2}`,
		"lookup_pincode":       `{"pincode":"637001"}`,
		"search_post_offices":  `{"query":"namakkal","state":"TN"}`,
		"lookup_ifsc":          `{"ifsc":"SBIN0000001"}`,
		"search_bank_branches": `{"bank":"SBIN","state":"TN"}`,
		"search_banks":         `{"query":"hdfc"}`,
		"lookup_rto":           `{"code":"TN28AB1234"}`,
		"list_holidays":        `{"year":2026,"type":"market"}`,
		"number_to_words":      `{"number":125000.5,"style":"currency"}`,
		"validate_identifier":  `{"kind":"gstin","value":"27AAPFU0939F1ZV"}`,
	}
	for _, tool := range mcpTools {
		arguments, ok := calls[tool.Name]
		if !ok {
			t.Errorf("no test call for tool %s", tool.Name)
			continue
		}
		_, called := rpc(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"` + tool.Name + `","arguments":` + arguments + `}}`)
		outcome := called["result"].(map[string]any)
		if outcome["isError"] == true {
			t.Errorf("%s failed: %v", tool.Name, outcome["content"])
			continue
		}
		if _, ok := outcome["structuredContent"].(map[string]any)["data"]; !ok {
			t.Errorf("%s returned no data", tool.Name)
		}
	}
	_, words := rpc(`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"number_to_words","arguments":{"number":125000.5,"style":"currency"}}}`)
	if spelled := words["result"].(map[string]any)["structuredContent"].(map[string]any)["data"].(map[string]any)["words"]; spelled != "one lakh twenty-five thousand rupees and fifty paise only" {
		t.Errorf("number passed as JSON number spelled %q", spelled)
	}

	_, missing := rpc(`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"lookup_pincode","arguments":{"pincode":"100000"}}}`)
	if missing["result"].(map[string]any)["isError"] != true {
		t.Errorf("a failed lookup should be a tool error: %v", missing)
	}
	_, noArgument := rpc(`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"lookup_pincode","arguments":{}}}`)
	if noArgument["result"].(map[string]any)["isError"] != true {
		t.Errorf("a missing argument should be a tool error: %v", noArgument)
	}
	if _, unknown := rpc(`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"nope"}}`); unknown["error"].(map[string]any)["code"].(float64) != -32602 {
		t.Errorf("unknown tool: %v", unknown)
	}
	if _, method := rpc(`{"jsonrpc":"2.0","id":14,"method":"resources/list"}`); method["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Errorf("unknown method: %v", method)
	}
	if status, garbage := rpc(`not json`); status != http.StatusBadRequest || garbage["error"].(map[string]any)["code"].(float64) != -32700 {
		t.Errorf("parse error: %d %v", status, garbage)
	}
	if status, _ := rpc(strings.Repeat("x", mcpBodyLimit+10)); status != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body = %d, want 413", status)
	}
	if got := get(t, "/mcp"); got.status != http.StatusMethodNotAllowed {
		t.Errorf("GET /mcp = %d, want 405", got.status)
	}
}

func TestMetricsAndLifetimeTotals(t *testing.T) {
	stateDir := t.TempDir()
	server := newTestServer(Config{Version: "9.9.9", StateDir: stateDir})
	serve := func(path string) {
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	serve("/v1/logos/TCS")
	serve("/v1/logos/NOTLISTED")
	serve("/v1/pincode/637001")
	serve("/healthz")

	recorder := httptest.NewRecorder()
	server.MetricsHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	text := recorder.Body.String()
	for _, want := range []string{
		`api_build_info{version="9.9.9"} 1`,
		`api_http_requests_total{route="logo",method="GET",status="200"} 1`,
		`api_http_requests_total{route="logo",method="GET",status="404"} 1`,
		`api_logo_requests_total{format="svg",result="not_found"} 1`,
		`api_http_request_duration_seconds_bucket{route="pincode",le="+Inf"} 1`,
		`api_lifetime_requests_total{route="logo"} 2`,
		`api_dataset_records{dataset="rto_codes"}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics are missing %q", want)
		}
	}
	// What was looked up is for the log, never for metric labels.
	if strings.Contains(text, "TCS") || strings.Contains(text, "637001") || strings.Contains(text, "NOTLISTED") {
		t.Error("a looked-up value leaked into the metrics")
	}

	// Totals are written on shutdown and picked up by the next start; health checks do not count.
	server.Close()
	server.db.Close()
	restarted := newTestServer(Config{Version: "9.9.9", StateDir: stateDir})
	defer func() { restarted.Close(); restarted.db.Close() }()
	totals := restarted.metrics.lifetime()
	if totals.Total != 3 || totals.Requests["logo"] != 2 || totals.Requests["pincode"] != 1 || totals.Requests["health"] != 0 {
		t.Errorf("totals did not survive a restart: %+v", totals)
	}
	restarted.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/states", nil))
	if got := restarted.metrics.lifetime().Total; got != 4 {
		t.Errorf("total after restart = %d, want 4", got)
	}

	stats := httptest.NewRecorder()
	restarted.MetricsHandler().ServeHTTP(stats, httptest.NewRequest(http.MethodGet, "/stats", nil))
	if !strings.Contains(stats.Body.String(), `"total":4`) {
		t.Errorf("unexpected /stats: %s", stats.Body.String())
	}
}

func TestRateLimit(t *testing.T) {
	server := newTestServer(Config{Version: "t", RateLimitPerMinute: 3})
	defer func() { server.Close(); server.db.Close() }()
	hit := func(path, remote string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = remote
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, req)
		return recorder.Code
	}
	var statuses []int
	for i := 0; i < 5; i++ {
		statuses = append(statuses, hit("/v1/logos/TCS", "203.0.113.5:1000"))
	}
	if fmt.Sprint(statuses) != "[200 200 200 429 429]" {
		t.Errorf("statuses = %v", statuses)
	}
	// Another client, and the pages a throttled client needs, are unaffected.
	if hit("/v1/logos/TCS", "203.0.113.6:1000") != http.StatusOK || hit("/", "203.0.113.5:1000") != http.StatusOK || hit("/healthz", "203.0.113.5:1000") != http.StatusOK {
		t.Error("the limit leaked beyond the throttled client's data requests")
	}
}

func TestClientKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.9:5555"
	req.Header.Set("CF-Connecting-IP", "198.51.100.7")

	direct := &Server{cfg: Config{}}
	if key := direct.clientKey(req); key != "10.0.0.9" {
		t.Errorf("without a trusted proxy the header must be ignored, got %q", key)
	}
	proxied := &Server{cfg: Config{TrustProxy: true}}
	if key := proxied.clientKey(req); key != "198.51.100.7" {
		t.Errorf("behind a trusted proxy the header is the client, got %q", key)
	}
}
