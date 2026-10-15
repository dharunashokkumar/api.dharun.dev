package words

import (
	"strings"
	"testing"
)

// Every amount in range is spelled in every language, and nothing out of range panics.
func FuzzSpell(f *testing.F) {
	f.Add(int64(100000), "05", false, 50)
	f.Add(int64(0), "", true, 0)
	f.Add(int64(MaxValue), "999999", false, 99)
	f.Add(int64(-1), "1", true, 100)
	f.Fuzz(func(t *testing.T, whole int64, fraction string, negative bool, paise int) {
		// The API only passes the digits after the decimal point.
		fraction = strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, fraction)
		inRange := whole >= 0 && whole <= MaxValue
		for _, lang := range Languages() {
			got, err := Number(lang, negative, whole, fraction)
			if inRange && (err != nil || got == "") {
				t.Errorf("Number(%s, %v, %d, %q) = %q, %v", lang, negative, whole, fraction, got, err)
			}
			if !inRange && err == nil {
				t.Errorf("Number(%s, %d) accepted an amount out of range", lang, whole)
			}
			got, err = Currency(lang, whole, paise)
			if inRange && paise >= 0 && paise <= 99 && (err != nil || got == "") {
				t.Errorf("Currency(%s, %d, %d) = %q, %v", lang, whole, paise, got, err)
			}
		}
	})
}
