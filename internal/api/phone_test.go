package api

import (
	"net/http"
	"net/url"
	"testing"
)

func init() {
	mcpTestCalls["check_indian_phone_number"] = `{"number":"+91 95001 12345"}`
}

func TestPhone(t *testing.T) {
	check := func(number string) map[string]any {
		t.Helper()
		got := expectStatus(t, "/v1/phone/"+url.PathEscape(number), http.StatusOK)
		if got.header.Get("Cache-Control") != "private, no-store" {
			t.Error("a phone number must not be shared-cached")
		}
		return got.data(t)
	}
	for _, written := range []string{"+91 95001 12345", "09500112345", "9500112345", "+91-95001-12345", "0091 9500112345", "(950) 011-2345"} {
		mobile := check(written)
		if mobile["valid"] != true || mobile["type"] != "mobile" || mobile["e164"] != "+919500112345" || mobile["national"] != "95001 12345" {
			t.Errorf("%q: %v", written, mobile)
			continue
		}
		operator, _ := mobile["originalOperator"].(map[string]any)
		circle, _ := mobile["circle"].(map[string]any)
		if operator == nil || operator["code"] != "AT" || circle == nil || circle["code"] != "TN" || circle["name"] != "Tamil Nadu" {
			t.Errorf("%q series details: %v %v", written, operator, circle)
		}
	}
	// A series the source leaves blank is still a valid mobile number, just without details.
	if blank := check("9300112345"); blank["valid"] != true || blank["originalOperator"] != nil {
		t.Errorf("blank series: %v", blank)
	}
	if free := check("1800 425 1234"); free["type"] != "toll_free" || free["valid"] != true {
		t.Errorf("toll free: %v", free)
	}
	if fixed := check("044 2345 6789"); fixed["type"] != "fixed_or_unknown" || fixed["valid"] != true || fixed["e164"] != "+914423456789" {
		t.Errorf("Chennai landline: %v", fixed)
	}
	for _, bad := range []string{"12345", "95001123456789", "9500ABCDEF", "0000000000", "98765+43210", "+91+9876543210"} {
		if result := check(bad); result["valid"] != false || result["problem"] == "" {
			t.Errorf("%q should be invalid: %v", bad, result)
		}
	}
}
