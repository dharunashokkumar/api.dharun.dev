package dataset

import (
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
