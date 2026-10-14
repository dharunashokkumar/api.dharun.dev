package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestWorkingDayBank(t *testing.T) {
	// Pongal week in Tamil Nadu: 15 Jan Pongal, 16 Thiruvalluvar Day, 17 Uzhavar Thirunal
	// (a third Saturday), 18 a Sunday.
	day := expectStatus(t, "/v1/working-days/2026-01-16?calendar=bank&state=TN&add=3", http.StatusOK).data(t)
	if day["working"] != false || !strings.Contains(day["reasons"].([]any)[0].(string), "Thiruvalluvar") {
		t.Errorf("16 January in Tamil Nadu: %v", day)
	}
	if day["previousWorkingDay"] != "2026-01-14" || day["nextWorkingDay"] != "2026-01-19" || day["shifted"] != "2026-01-21" {
		t.Errorf("neighbours or shift wrong: %v", day)
	}
	if day["state"] != "TN" || day["calendar"] != "bank" || day["weekday"] != "Friday" {
		t.Errorf("echo of the request wrong: %v", day)
	}
	// The same day is an ordinary working day for banks in Maharashtra.
	if mh := expectStatus(t, "/v1/working-days/2026-01-16?calendar=bank&state=Maharashtra", http.StatusOK).data(t); mh["working"] != true {
		t.Errorf("16 January in Maharashtra: %v", mh)
	}

	second := expectStatus(t, "/v1/working-days/2026-10-10?state=TN", http.StatusOK).data(t)
	if second["working"] != false || second["reasons"].([]any)[0] != "Second Saturday" {
		t.Errorf("second Saturday: %v", second)
	}
	if first := expectStatus(t, "/v1/working-days/2026-10-03?state=TN", http.StatusOK).data(t); first["working"] != true || len(first["reasons"].([]any)) != 0 {
		t.Errorf("a first Saturday is a bank working day: %v", first)
	}
	if fourth := expectStatus(t, "/v1/working-days/2026-10-24?state=TN", http.StatusOK).data(t); fourth["reasons"].([]any)[0] != "Fourth Saturday" {
		t.Errorf("fourth Saturday: %v", fourth)
	}
	if fifth := expectStatus(t, "/v1/working-days/2026-10-31?state=TN", http.StatusOK).data(t); fifth["working"] != true {
		t.Errorf("a fifth Saturday is a bank working day: %v", fifth)
	}
	backwards := expectStatus(t, "/v1/working-days/2026-01-19?state=TN&add=-1", http.StatusOK).data(t)
	if backwards["shifted"] != "2026-01-14" {
		t.Errorf("one working day before 19 January: %v", backwards["shifted"])
	}

	expectStatus(t, "/v1/working-days/2026-01-16", http.StatusBadRequest)               // bank needs a state
	expectStatus(t, "/v1/working-days/2026-01-16?state=LD", http.StatusNotFound)        // no Reserve Bank office covers Lakshadweep
	expectStatus(t, "/v1/working-days/2031-01-16?state=TN", http.StatusNotFound)        // no list for that year
	expectStatus(t, "/v1/working-days/2026-12-20?state=TN&add=30", http.StatusNotFound) // walks into an unlisted year
	expectStatus(t, "/v1/working-days/16-01-2026?state=TN", http.StatusBadRequest)      // date format
	expectStatus(t, "/v1/working-days/2026-02-30?state=TN", http.StatusBadRequest)      // no such day
	expectStatus(t, "/v1/working-days/2026-01-16?calendar=school&state=TN", http.StatusBadRequest)
	expectStatus(t, "/v1/working-days/2026-01-16?state=TN&add=400", http.StatusBadRequest)
}

// Jammu and Kashmir has two Reserve Bank offices. On 2026-03-04 only Jammu is closed;
// on 2026-03-13 both are, and the holiday is listed once.
func TestWorkingDayBankOffices(t *testing.T) {
	merged := expectStatus(t, "/v1/working-days/2026-03-04?state=JK", http.StatusOK)
	day := merged.data(t)
	if day["working"] != false || !strings.Contains(fmt.Sprint(day["reasons"]), "(Jammu office only)") {
		t.Errorf("merged state answer: %v", day)
	}
	if note := merged.json(t)["meta"].(map[string]any)["note"].(string); !strings.Contains(note, "Jammu and Srinagar") {
		t.Errorf("the note should name the offices: %s", note)
	}
	srinagar := expectStatus(t, "/v1/working-days/2026-03-04?state=JK&office=Srinagar", http.StatusOK).data(t)
	if srinagar["working"] != true || srinagar["office"] != "Srinagar" {
		t.Errorf("Srinagar on 2026-03-04: %v", srinagar)
	}
	jammu := expectStatus(t, "/v1/working-days/2026-03-04?state=JK&office=rbi%20jammu", http.StatusOK).data(t)
	if jammu["working"] != false || jammu["office"] != "Jammu" || strings.Contains(fmt.Sprint(jammu["reasons"]), "office only") {
		t.Errorf("Jammu on 2026-03-04: %v", jammu)
	}
	both := expectStatus(t, "/v1/working-days/2026-03-13?state=JK", http.StatusOK).data(t)
	if reasons := both["reasons"].([]any); len(reasons) != 1 || strings.Contains(fmt.Sprint(reasons), "office") {
		t.Errorf("a holiday both offices keep: %v", reasons)
	}
	span := expectStatus(t, "/v1/working-days?from=2026-03-02&to=2026-03-06&state=JK&office=Srinagar", http.StatusOK).data(t)
	if span["workingDays"] != float64(5) || span["office"] != "Srinagar" {
		t.Errorf("Srinagar's week: %v", span)
	}

	expectStatus(t, "/v1/working-days/2026-03-04?state=JK&office=Kochi", http.StatusBadRequest)
	expectStatus(t, "/v1/working-days/2026-03-04?calendar=market&office=Jammu", http.StatusBadRequest)
}

func TestWorkingDayMarketAndSettlement(t *testing.T) {
	republic := expectStatus(t, "/v1/working-days/2026-01-26?calendar=market", http.StatusOK).data(t)
	if republic["working"] != false || republic["reasons"].([]any)[0] != "Republic Day" {
		t.Errorf("Republic Day: %v", republic)
	}
	// 19 February is a clearing holiday only: the market trades, nothing settles.
	if trading := expectStatus(t, "/v1/working-days/2026-02-19?calendar=market", http.StatusOK).data(t); trading["working"] != true {
		t.Errorf("19 February is a trading day: %v", trading)
	}
	t1 := expectStatus(t, "/v1/working-days/2026-02-18?calendar=settlement&add=1", http.StatusOK).data(t)
	if t1["shifted"] != "2026-02-20" {
		t.Errorf("a T+1 trade on 18 February settles on the 20th, got %v", t1["shifted"])
	}
	// A state is irrelevant to the exchange and is ignored rather than refused.
	if ignored := expectStatus(t, "/v1/working-days/2026-01-26?calendar=market&state=TN", http.StatusOK).data(t); ignored["state"] != nil {
		t.Errorf("state should be dropped for the market calendar: %v", ignored)
	}
	central := expectStatus(t, "/v1/working-days/2026-01-26?calendar=central", http.StatusOK).data(t)
	if central["working"] != false {
		t.Errorf("Republic Day for central government offices: %v", central)
	}

	// January 2026: nine weekend days, and trading holidays on the 15th and the 26th.
	month := expectStatus(t, "/v1/working-days?from=2026-01-01&to=2026-01-31&calendar=market", http.StatusOK).data(t)
	if month["days"].(float64) != 31 || month["workingDays"].(float64) != 20 || len(month["daysOff"].([]any)) != 11 {
		t.Errorf("January 2026: %v days, %v working, %d off", month["days"], month["workingDays"], len(month["daysOff"].([]any)))
	}
	expectStatus(t, "/v1/working-days?from=2026-01-31&to=2026-01-01&calendar=market", http.StatusBadRequest)
	expectStatus(t, "/v1/working-days?from=2026-01-01&to=2027-06-01&calendar=market", http.StatusBadRequest)
	expectStatus(t, "/v1/working-days?from=2026-01-01&calendar=market", http.StatusBadRequest)
}

func TestMarketStatus(t *testing.T) {
	cases := []struct {
		at, session, nextOpen string
	}{
		{"2026-10-13T10:00:00+05:30", "open", ""},
		{"2026-10-13T04:30:00Z", "open", ""}, // the same moment in UTC
		{"2026-10-13T09:05:00+05:30", "pre_open", "2026-10-13T09:15:00+05:30"},
		{"2026-10-13T15:45:00+05:30", "closing", "2026-10-14T09:15:00+05:30"},
		{"2026-10-13T20:00:00+05:30", "closed", "2026-10-14T09:15:00+05:30"},
		{"2026-01-26T10:00:00+05:30", "closed", "2026-01-27T09:15:00+05:30"},
		{"2026-10-17T11:00:00+05:30", "closed", "2026-10-19T09:15:00+05:30"},
	}
	for _, c := range cases {
		status := expectStatus(t, "/v1/market/status?at="+strings.ReplaceAll(c.at, "+", "%2B"), http.StatusOK)
		data := status.data(t)
		if data["session"] != c.session || data["open"] != (c.session == "open") || (data["nextOpen"] != nil && data["nextOpen"] != c.nextOpen) || (c.nextOpen != "" && data["nextOpen"] == nil) {
			t.Errorf("%s: %v", c.at, data)
		}
	}
	republic := expectStatus(t, "/v1/market/status?at=2026-01-26T10:00:00%2B05:30", http.StatusOK).data(t)
	if republic["reasons"].([]any)[0] != "Republic Day" {
		t.Errorf("closed without its reason: %v", republic)
	}
	muhurat := expectStatus(t, "/v1/market/status?at=2026-11-08T18:00:00%2B05:30", http.StatusOK).data(t)
	if !strings.Contains(muhurat["note"].(string), "Muhurat") {
		t.Errorf("the Muhurat session note is missing: %v", muhurat)
	}
	// A live status is cached briefly; a status for a fixed moment can be cached long.
	if now := get(t, "/v1/market/status"); now.status == http.StatusOK && now.header.Get("Cache-Control") != "public, max-age=30" {
		t.Errorf("current status cached as %q", now.header.Get("Cache-Control"))
	}
	expectStatus(t, "/v1/market/status?at=yesterday", http.StatusBadRequest)
}

func TestPincodeNear(t *testing.T) {
	result := expectStatus(t, "/v1/pincode/near?lat=11.2189&lng=78.1674&radius=5", http.StatusOK)
	offices := result.list(t)
	if len(offices) < 3 {
		t.Fatalf("expected several post offices near Namakkal, got %d", len(offices))
	}
	last := 0.0
	for _, raw := range offices {
		office := raw.(map[string]any)
		distance := office["distanceKm"].(float64)
		if distance < last || distance > 5 {
			t.Errorf("not sorted or outside the radius: %v", office)
		}
		last = distance
		if pin := office["pincode"].(string); pin != "637001" && pin != "637002" {
			t.Errorf("an office far from Namakkal was returned: %v", office)
		}
	}
	if meta := result.json(t)["meta"].(map[string]any); meta["total"].(float64) < float64(len(offices)) || meta["source"] == nil {
		t.Errorf("meta: %v", meta)
	}
	if few := expectStatus(t, "/v1/pincode/near?lat=11.2189&lng=78.1674&radius=50&limit=2", http.StatusOK).list(t); len(few) != 2 {
		t.Errorf("limit ignored: %d", len(few))
	}
	// The ocean off Chennai has no post office within a kilometre.
	if empty := expectStatus(t, "/v1/pincode/near?lat=13.0&lng=80.6&radius=1", http.StatusOK).list(t); len(empty) != 0 {
		t.Errorf("found offices in the sea: %v", empty)
	}
	expectStatus(t, "/v1/pincode/near?lat=51.5&lng=-0.12", http.StatusBadRequest) // London
	expectStatus(t, "/v1/pincode/near?lat=11.2", http.StatusBadRequest)
	expectStatus(t, "/v1/pincode/near?lat=11.2&lng=78.1&radius=500", http.StatusBadRequest)
	// The literal path wins over the pincode pattern.
	expectStatus(t, "/v1/pincode/637001", http.StatusOK)
}

func init() {
	mcpTestCalls["check_working_day"] = `{"date":"2026-02-18","calendar":"settlement","add":1}`
	mcpTestCalls["count_working_days"] = `{"from":"2026-01-01","to":"2026-01-31","calendar":"market"}`
	mcpTestCalls["indian_market_status"] = `{"at":"2026-01-26T10:00:00+05:30"}`
	mcpTestCalls["find_post_offices_near"] = `{"lat":11.2189,"lng":78.1674}`
}
