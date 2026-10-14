package api

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["make_fake_indian_data"] = `{"kind":"companies","count":2,"state":"TN"}`
}

func TestFakeData(t *testing.T) {
	first := expectStatus(t, "/v1/fake/people?count=20&state=TN&seed=42", http.StatusOK)
	again := expectStatus(t, "/v1/fake/people?count=20&state=TN&seed=42", http.StatusOK)
	if string(first.body) != string(again.body) {
		t.Error("the same seed should give the same records")
	}
	if other := expectStatus(t, "/v1/fake/people?count=20&state=TN&seed=43", http.StatusOK); string(other.body) == string(first.body) {
		t.Error("another seed should give other records")
	}
	people := first.list(t)
	if len(people) != 20 {
		t.Fatalf("%d people", len(people))
	}
	pan := regexp.MustCompile(`^[A-Z]{3}P[A-Z][0-9]{4}[A-Z]$`)
	phone := regexp.MustCompile(`^\+91 [6-9][0-9]{4} [0-9]{5}$`)
	for _, raw := range people {
		person := raw.(map[string]any)
		address := person["address"].(map[string]any)
		if person["fake"] != true || address["stateCode"] != "TN" || !regexp.MustCompile(`^6[0-9]{5}$`).MatchString(address["pincode"].(string)) {
			t.Errorf("not a marked Tamil Nadu record: %v", person)
		}
		if !pan.MatchString(person["pan"].(string)) || person["pan"].(string)[4] != person["lastName"].(string)[0] {
			t.Errorf("PAN %v for %v", person["pan"], person["lastName"])
		}
		if !phone.MatchString(person["phone"].(string)) || !strings.Contains(person["email"].(string), "@example.") {
			t.Errorf("phone or email: %v %v", person["phone"], person["email"])
		}
		// Phone numbers come only from series the numbering table leaves unallocated.
		if series := strings.ReplaceAll(person["phone"].(string), " ", "")[3:7]; series != "9300" {
			t.Errorf("phone %v is not in an unallocated series", person["phone"])
		}
	}

	// A fake GSTIN must pass this API's own check, and carry its state's code.
	for _, raw := range expectStatus(t, "/v1/fake/companies?count=10&state=MH&seed=7", http.StatusOK).list(t) {
		company := raw.(map[string]any)
		gstin := company["gstin"].(string)
		checked := expectStatus(t, "/v1/validate/gstin/"+gstin, http.StatusOK).data(t)
		if checked["valid"] != true || !strings.HasPrefix(gstin, "27") || gstin[2:12] != company["pan"] {
			t.Errorf("GSTIN %s (PAN %v): %v", gstin, company["pan"], checked)
		}
	}
	if addresses := expectStatus(t, "/v1/fake/addresses?count=5", http.StatusOK); len(addresses.list(t)) != 5 || addresses.header.Get("Cache-Control") != cacheNone {
		t.Errorf("unseeded addresses: %d, cached %q", len(addresses.list(t)), addresses.header.Get("Cache-Control"))
	}
	// A real state with no post offices in the data gets an answer, not an empty body.
	if none := expectStatus(t, "/v1/fake/addresses?state=KL", http.StatusNotFound); !strings.Contains(string(none.body), "No post offices") {
		t.Errorf("addresses for a state without offices: %s", none.body)
	}
	// Many nationwide addresses draw row numbers near the end of the table too.
	if many := expectStatus(t, "/v1/fake/addresses?count=100&seed=7", http.StatusOK); len(many.list(t)) != 100 {
		t.Errorf("100 addresses gave %d", len(many.list(t)))
	}
	expectStatus(t, "/v1/fake/pets", http.StatusNotFound)
	expectStatus(t, "/v1/fake/people?count=500", http.StatusBadRequest)
	expectStatus(t, "/v1/fake/people?seed=abc", http.StatusBadRequest)
	expectStatus(t, "/v1/fake/people?state=Atlantis", http.StatusBadRequest)
}
