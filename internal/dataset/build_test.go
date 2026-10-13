package dataset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareSVG(t *testing.T) {
	scaled, err := prepareSVG([]byte(`<svg width="18" height="18" xmlns="http://www.w3.org/2000/svg"><path d="M0 0h18v18H0z"/></svg>` + "\n"))
	if err != nil || !strings.HasPrefix(string(scaled), `<svg viewBox="0 0 18 18" width="18" height="18"`) {
		t.Errorf("viewBox not added: %s (%v)", scaled, err)
	}
	untouched := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><image href="data:image/png;base64,AAAA"/></svg>`
	if got, err := prepareSVG([]byte(untouched)); err != nil || string(got) != untouched {
		t.Errorf("a safe SVG with a viewBox should pass unchanged: %s (%v)", got, err)
	}

	rejected := map[string]string{
		"script":            `<svg viewBox="0 0 1 1"><script>alert(1)</script></svg>`,
		"event handler":     `<svg viewBox="0 0 1 1" onload="alert(1)"></svg>`,
		"javascript URL":    `<svg viewBox="0 0 1 1"><a href="javascript:alert(1)"/></svg>`,
		"foreign object":    `<svg viewBox="0 0 1 1"><foreignObject/></svg>`,
		"remote image":      `<svg viewBox="0 0 1 1"><image href="https://example.com/x.png"/></svg>`,
		"protocol-relative": `<svg viewBox="0 0 1 1"><image href="//example.com/x.png"/></svg>`,
		"not an svg":        `<html></html>`,
		"cannot scale":      `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
	}
	for name, svg := range rejected {
		if _, err := prepareSVG([]byte(svg)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{
		"TAMIL NADU":           "Tamil Nadu",
		"  the   nilgiris ":    "The Nilgiris",
		"SAHIBZADA AJIT-SINGH": "Sahibzada Ajit-Singh",
		"y.s.r. (kadapa)":      "Y.S.R. (Kadapa)",
		"":                     "",
	}
	for input, want := range cases {
		if got := titleCase(input); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildFromFixtures(t *testing.T) {
	fixtures := filepath.Join("..", "api", "testdata")
	out := filepath.Join(t.TempDir(), "api.db")
	stats, err := Build(BuildOptions{DataDir: filepath.Join(fixtures, "data"), LogosDir: filepath.Join(fixtures, "logos"), Out: out, Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Logos < 10 || stats.Pincodes < 3 || stats.Branches < 10 || stats.RTOs < 100 || stats.Holidays < 50 {
		t.Errorf("unexpected counts: %+v", stats)
	}
	if _, err := os.Stat(out + ".tmp"); err == nil {
		t.Error("the temporary database was left behind")
	}

	db, err := Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sources, err := Sources(db)
	if err != nil || len(sources) != 5 || sources["rto"].Licence != "CC BY-SA 4.0" {
		t.Errorf("sources were not stored: %v (%v)", sources, err)
	}
	var placeholder int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pincodes WHERE pincode = '999999'`).Scan(&placeholder); err != nil || placeholder != 0 {
		t.Errorf("placeholder post office was imported (%d, %v)", placeholder, err)
	}
	var unnamed int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ifsc i LEFT JOIN banks b ON b.code = i.bank_code WHERE b.code IS NULL`).Scan(&unnamed); err != nil || unnamed != 0 {
		t.Errorf("%d branches have no bank row (%v)", unnamed, err)
	}
}

func TestBuildStrictness(t *testing.T) {
	empty := t.TempDir()
	out := filepath.Join(t.TempDir(), "api.db")

	if _, err := Build(BuildOptions{DataDir: empty, LogosDir: empty, Out: out, Strict: true}); err == nil {
		t.Error("a strict build with no data should fail")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a failed build left a database behind")
	}

	stats, err := Build(BuildOptions{DataDir: empty, LogosDir: empty, Out: out})
	if err != nil {
		t.Fatalf("a lenient build should produce an empty database: %v", err)
	}
	if len(stats.Warnings) < 5 {
		t.Errorf("each missing dataset should be reported, got %v", stats.Warnings)
	}
}

func TestBuildRejectsBadHolidays(t *testing.T) {
	cases := map[string]string{
		"wrong year":    `{"year":2026,"holidays":[{"date":"2027-01-26","name":"Republic Day","type":"market","region":"NSE","states":["*"]}]}`,
		"unknown type":  `{"year":2026,"holidays":[{"date":"2026-01-26","name":"Republic Day","type":"school","region":"NSE","states":["*"]}]}`,
		"unknown state": `{"year":2026,"holidays":[{"date":"2026-01-26","name":"Republic Day","type":"state","region":"X","states":["ZZ"]}]}`,
		"no states":     `{"year":2026,"holidays":[{"date":"2026-01-26","name":"Republic Day","type":"market","region":"NSE","states":[]}]}`,
		"bad date":      `{"year":2026,"holidays":[{"date":"26-01-2026","name":"Republic Day","type":"market","region":"NSE","states":["*"]}]}`,
	}
	for name, content := range cases {
		dataDir := t.TempDir()
		os.MkdirAll(filepath.Join(dataDir, "holidays"), 0o755)
		os.WriteFile(filepath.Join(dataDir, "holidays", "2026.json"), []byte(content), 0o644)
		if _, err := Build(BuildOptions{DataDir: dataDir, LogosDir: dataDir, Out: filepath.Join(t.TempDir(), "api.db")}); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
