package api

import (
	"net/http"
	"net/url"
	"testing"
)

func init() {
	mcpTestCalls["list_index_constituents"] = `{"index":"nifty-bank"}`
	mcpTestCalls["get_rbi_rates"] = `{"currency":"USD"}`
	mcpTestCalls["get_gold_silver_rates"] = `{}`
	mcpTestCalls["get_mutual_fund_returns"] = `{"code":"122639"}`
	mcpTestCalls["list_derivative_expiries"] = `{"year":2026,"month":3}`
}

func TestIndices(t *testing.T) {
	bank := expectStatus(t, "/v1/indices/nifty-bank", http.StatusOK).data(t)
	if bank["name"] != "Nifty Bank" || len(bank["members"].([]any)) < 10 {
		t.Errorf("bank %v", bank)
	}
	expectStatus(t, "/v1/indices/niftybank", http.StatusOK)
	of := expectStatus(t, "/v1/indices?symbol=AXISBANK", http.StatusOK).json(t)["data"].([]any)
	if len(of) != 2 {
		t.Errorf("AXISBANK is in %v", of)
	}
	expectStatus(t, "/v1/indices/nifty-moon", http.StatusNotFound)
	expectStatus(t, "/v1/indices?symbol=NOSUCH", http.StatusNotFound)
}

func TestNAVReturns(t *testing.T) {
	got := expectStatus(t, "/v1/mutual-funds/INF879O01027/returns", http.StatusOK).data(t)
	byPeriod := map[string]map[string]any{}
	for _, r := range got["returns"].([]any) {
		byPeriod[r.(map[string]any)["period"].(string)] = r.(map[string]any)
	}
	// The fixture grows 15% a year.
	for _, period := range []string{"1y", "3y", "5y"} {
		if cagr := byPeriod[period]["cagrPercent"].(float64); cagr < 14.5 || cagr > 15.5 {
			t.Errorf("%s CAGR %v", period, cagr)
		}
	}
	sips := got["sip"].([]any)
	if len(sips) < 3 {
		t.Fatalf("sip %v", sips)
	}
	if x := sips[0].(map[string]any)["xirrPercent"].(float64); x < 13 || x > 17 {
		t.Errorf("SIP XIRR %v", x)
	}
	history := expectStatus(t, "/v1/mutual-funds/122639/history?from=2026-10-01", http.StatusOK).json(t)
	if n := history["meta"].(map[string]any)["total"].(float64); n < 8 {
		t.Errorf("history %v", n)
	}
	expectStatus(t, "/v1/mutual-funds/130503/returns", http.StatusNotFound)
}

func TestBullion(t *testing.T) {
	got := expectStatus(t, "/v1/bullion", http.StatusOK).data(t)
	if got["date"] != "2026-11-11" {
		t.Fatalf("bullion %v", got)
	}
	first := got["rates"].([]any)[0].(map[string]any)
	if first["metal"] != "gold" || first["purity"] != "999" || first["karat"] != "24K" || first["perGram"].(float64) != 14080.5 || first["change"].(float64) != 685 {
		t.Errorf("gold %v", first)
	}
	history := expectStatus(t, "/v1/bullion/history?metal=gold&purity=999", http.StatusOK).json(t)
	if history["meta"].(map[string]any)["total"].(float64) != 3 {
		t.Errorf("history %v", history)
	}
	expectStatus(t, "/v1/bullion/history?metal=copper", http.StatusNotFound)
}

func TestRatesAndExpiries(t *testing.T) {
	rates := expectStatus(t, "/v1/rates", http.StatusOK).data(t)
	policy := rates["policy"].([]any)
	if len(policy) < 4 || len(rates["governmentBonds"].([]any)) < 3 {
		t.Errorf("rates %v", rates)
	}
	usd := expectStatus(t, "/v1/fx?currency=USD", http.StatusOK).json(t)["data"].([]any)
	if len(usd) != 1 || usd[0].(map[string]any)["rupees"].(float64) != 93.8075 {
		t.Errorf("usd %v", usd)
	}
	if history := expectStatus(t, "/v1/fx?currency=USD&days=7", http.StatusOK).json(t)["data"].([]any); len(history) != 2 {
		t.Errorf("history %v", history)
	}
	expectStatus(t, "/v1/fx?currency=XYZ", http.StatusBadRequest)
	march := expectStatus(t, "/v1/expiries?exchange=nse&year=2026&month=3", http.StatusOK).json(t)["data"].([]any)
	last := march[len(march)-1].(map[string]any)
	if len(march) != 5 || last["kind"] != "monthly" || last["date"] != "2026-03-31" && last["moved"] == nil {
		t.Errorf("march %v", march)
	}
	for _, e := range march {
		if e.(map[string]any)["moved"] != nil {
			t.Logf("moved: %v", e)
		}
	}
	expectStatus(t, "/v1/expiries?year=2024", http.StatusBadRequest)
}

func TestBankCodesAndSectors(t *testing.T) {
	micr := expectStatus(t, "/v1/micr/400240003", http.StatusOK).json(t)["data"].([]any)
	if len(micr) != 1 || micr[0].(map[string]any)["ifsc"] != "HDFC0000001" {
		t.Errorf("micr %v", micr)
	}
	if swift := expectStatus(t, "/v1/swift/HDFCINBB", http.StatusOK).json(t)["data"].([]any); len(swift) < 2 {
		t.Errorf("swift %v", swift)
	}
	expectStatus(t, "/v1/micr/12345", http.StatusBadRequest)
	sectors := expectStatus(t, "/v1/sectors", http.StatusOK).json(t)["data"].([]any)
	if len(sectors) == 0 {
		t.Fatal("no sectors")
	}
	name := sectors[0].(map[string]any)["sector"].(string)
	one := expectStatus(t, "/v1/sectors?sector="+url.QueryEscape(name), http.StatusOK).json(t)["data"].([]any)[0].(map[string]any)
	if len(one["largest"].([]any)) == 0 {
		t.Errorf("sector %v", one)
	}
}
