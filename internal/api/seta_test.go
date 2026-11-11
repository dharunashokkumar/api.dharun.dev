package api

import (
	"net/http"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["format_indian_number"] = `{"number":"1234567.5"}`
	mcpTestCalls["indian_date_info"] = `{"from":"1995-08-15","to":"2026-11-11"}`
	mcpTestCalls["lookup_upi_handle"] = `{"handle":"someone@ybl"}`
	mcpTestCalls["read_payment_qr_text"] = `{"text":"upi://pay?pa=shop@okaxis&pn=Shop&am=10"}`
	mcpTestCalls["parse_indian_address"] = `{"address":"12, Gandhi Road, Namakkal, Tamil Nadu 637001"}`
}

func TestFormatNumber(t *testing.T) {
	data := expectStatus(t, "/v1/format/number/123456789.5", http.StatusOK).json(t)["data"].(map[string]any)
	if data["indian"] != "12,34,56,789.5" || data["rupees"] != "₹12,34,56,789.50" || data["short"].(map[string]any)["short"] != "12.35 Cr" {
		t.Errorf("%v", data)
	}
	if data["scripts"].(map[string]any)["deva"] != "१२,३४,५६,७८९.५" {
		t.Errorf("deva %v", data["scripts"])
	}
	expectStatus(t, "/v1/format/number/abc", http.StatusBadRequest)
	expectStatus(t, "/v1/format/number/12?decimals=99", http.StatusBadRequest)
}

func TestDates(t *testing.T) {
	data := expectStatus(t, "/v1/dates/2026-11-11", http.StatusOK).json(t)["data"].(map[string]any)
	if data["saka"].(map[string]any)["text"] != "20 Kartika 1948 Saka" || data["financialYear"].(map[string]any)["year"] != "2026-27" {
		t.Errorf("%v", data)
	}
	if tamil := data["tamil"].(map[string]any); tamil["month"] != "Aippasi" {
		t.Errorf("tamil %v", tamil)
	}
	span := expectStatus(t, "/v1/dates/diff?from=1995-08-15&to=2026-11-11", http.StatusOK).json(t)["data"].(map[string]any)
	if s := span["span"].(map[string]any); s["years"].(float64) != 31 || s["months"].(float64) != 2 || s["days"].(float64) != 27 {
		t.Errorf("span %v", span)
	}
	expectStatus(t, "/v1/dates/2026-13-01", http.StatusBadRequest)
	expectStatus(t, "/v1/dates/diff", http.StatusBadRequest)
}

func TestUPIHandlesAndQR(t *testing.T) {
	data := expectStatus(t, "/v1/upi-handles/priya.r@okhdfcbank", http.StatusOK).json(t)["data"].(map[string]any)
	if data["app"] != "Google Pay" || data["bank"] != "HDFC Bank" {
		t.Errorf("%v", data)
	}
	expectStatus(t, "/v1/upi-handles/nosuchhandle", http.StatusNotFound)
	got := request(t, http.MethodPost, "/v1/qr/parse", "upi://pay?pa=shop@okaxis&pn=Shop&am=10", "Content-Type", "text/plain")
	if got.status != http.StatusOK || !strings.Contains(string(got.body), `"payee":"shop@okaxis"`) || got.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("qr %d %s", got.status, got.body)
	}
	expectStatus(t, "/v1/qr/parse?text=hello", http.StatusBadRequest)
}

func TestBadges(t *testing.T) {
	got := expectStatus(t, "/v1/badge?label=made%20in&message=India&color=orange", http.StatusOK)
	if !strings.HasPrefix(got.header.Get("Content-Type"), "image/svg+xml") || strings.Contains(string(got.body), "<text") {
		t.Errorf("badge %s", got.header.Get("Content-Type"))
	}
	expectStatus(t, "/v1/badge/dataset/pincode", http.StatusOK)
	expectStatus(t, "/v1/badge/holiday?type=market", http.StatusOK)
	expectStatus(t, "/v1/badge/dataset/nosuch", http.StatusNotFound)
	expectStatus(t, "/v1/badge", http.StatusBadRequest)
	expectStatus(t, "/v1/badge/nosuch", http.StatusNotFound)
}

func TestAddressParse(t *testing.T) {
	data := expectStatus(t, "/v1/address/parse?text=No.%2012%2C%20Gandhi%20Road%2C%20Near%20Bus%20Stand%2C%20Namakkal%2C%20Tamil%20Nadu%20637001", http.StatusOK).json(t)["data"].(map[string]any)
	if data["pincode"] != "637001" || data["house"] != "No. 12" || data["street"] != "Gandhi Road" || data["landmark"] != "Near Bus Stand" || data["city"] != "Namakkal" || data["state"] != "Tamil Nadu" || data["pincodeChecked"] != true {
		t.Errorf("%v", data)
	}
	wrong := expectStatus(t, "/v1/address/parse?text=5%20MG%20Road%2C%20Pune%2C%20Kerala%20637001", http.StatusOK).json(t)["data"].(map[string]any)
	if len(wrong["warnings"].([]any)) == 0 {
		t.Errorf("no warning for a pincode in another state: %v", wrong)
	}
}
