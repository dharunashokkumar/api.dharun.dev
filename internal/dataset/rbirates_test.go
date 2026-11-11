package dataset

import (
	"os"
	"testing"
)

// The rates box of the RBI home page as it was on 2026-11-11.
func TestParseRBIRates(t *testing.T) {
	page, err := os.ReadFile("testdata/rbi-home.html")
	if err != nil {
		t.Fatal(err)
	}
	rates, err := ParseRBIRates(page, "2026-11-11")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]RBIRate{}
	for _, r := range rates {
		got[r.Kind+"/"+r.Name] = r
	}
	check := func(key, value, date string) {
		t.Helper()
		if r := got[key]; r.Value != value || r.Date != date {
			t.Errorf("%s = %+v", key, r)
		}
	}
	check("policy/Policy Repo Rate", "5.50%", "2026-11-11")
	check("ratio/CRR", "3.00%", "2026-11-11")
	check("fx/INR / 1 USD", "93.8075", "2026-11-11")
	check("fx/INR / 1 RUB", "1.1405", "2026-11-09")
	check("lending/Base Rate", "8.40% - 10.00%", "2026-11-11")
	check("tbill/91 day T-bills", "5.8681%", "2026-11-11")
	check("fx/INR / 1 EUR", "104.6660", "2026-11-11")
	check("bond/6.94% GS 2036", "7.1671%", "2026-11-09")
	if got["fx/INR / 1 USD"].Number == nil || *got["fx/INR / 1 USD"].Number != 93.8075 {
		t.Error("number")
	}
	for k := range got {
		if k == "stop/S&P BSE Sensex" {
			t.Error("the Sensex was read")
		}
	}
	t.Logf("%d figures", len(rates))
}
