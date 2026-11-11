package dataset

import (
	"encoding/json"
	"os"
	"testing"
)

// TestWriteRBIFixture writes the API tests' rbi-rates fixture from the saved page when
// UPDATE_FIXTURES is set.
func TestWriteRBIFixture(t *testing.T) {
	if os.Getenv("UPDATE_FIXTURES") == "" {
		t.Skip("set UPDATE_FIXTURES=1 to rewrite the fixture")
	}
	page, _ := os.ReadFile("testdata/rbi-home.html")
	rates, err := ParseRBIRates(page, "2026-11-11")
	if err != nil {
		t.Fatal(err)
	}
	older := 92.9
	rates = append(rates, RBIRate{Date: "2026-11-10", Kind: "fx", Name: "INR / 1 USD", Value: "92.9000", Number: &older})
	raw, _ := json.MarshalIndent(rates, "", " ")
	if err := os.WriteFile("../api/testdata/data/rbi-rates/rates.json", raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
