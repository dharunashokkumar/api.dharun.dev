package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["list_index_constituents"] = `{"index":"nifty-bank"}`
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
