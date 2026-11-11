package indiafmt

import (
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNumbers(t *testing.T) {
	for _, c := range []struct{ in, indian, international, rupees string }{
		{"1234567.5", "12,34,567.5", "1,234,567.5", "₹12,34,567.50"},
		{"100", "100", "100", "₹100.00"},
		{"-98765432101", "-98,76,54,32,101", "-98,765,432,101", "-₹98,76,54,32,101.00"},
		{"१२३४५", "12,345", "12,345", "₹12,345.00"},
		{"₹ 1,00,000.999", "1,00,000.999", "100,000.999", "₹1,00,001.00"},
		{"0.004", "0.004", "0.004", "₹0.00"},
	} {
		n, err := Parse(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if n.Indian() != c.indian || n.International() != c.international || n.Rupees() != c.rupees {
			t.Errorf("%s = %s %s %s", c.in, n.Indian(), n.International(), n.Rupees())
		}
	}
	for _, bad := range []string{"", "abc", "1.2.3", "--1"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	if got := ShortForm(12500000, 2); got.Short != "1.25 Cr" || got.Long != "1.25 crore" {
		t.Errorf("short %v", got)
	}
	if got := ShortForm(340000, 2); got.Short != "3.4 L" {
		t.Errorf("short %v", got)
	}
	if got, _ := Digits("2026", "taml"); got != "௨௦௨௬" {
		t.Errorf("tamil digits %s", got)
	}
}

func TestCalendars(t *testing.T) {
	if got := Saka(date("2026-11-11")); got.Text != "20 Kartika 1948 Saka" {
		t.Errorf("saka %v", got)
	}
	if got := Saka(date("2024-03-21")); got.Text != "1 Chaitra 1946 Saka" {
		t.Errorf("saka leap %v", got)
	}
	if got := Saka(date("2026-03-21")); got.Text != "30 Phalguna 1947 Saka" {
		t.Errorf("saka before new year %v", got)
	}
	if got := Hijri(date("2024-07-08")); got.Text != "1 Muharram 1446 AH" {
		t.Errorf("hijri %v", got)
	}
	fy := FY(date("2026-02-10"))
	if fy.Year != "2025-26" || fy.Quarter != "Q4" || fy.AssessmentYear != "2026-27" || fy.Half != "H2" || fy.QuarterStarts != "2026-01-01" {
		t.Errorf("fy %+v", fy)
	}
	span := Between(date("1990-02-28"), date("2026-11-11"))
	if span.Years != 36 || span.Months != 8 || span.Days != 14 {
		t.Errorf("span %+v", span)
	}
}
