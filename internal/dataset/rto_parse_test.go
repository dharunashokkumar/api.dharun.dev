package dataset

import (
	"strings"
	"testing"
)

// A cut-down page with the table shapes the real article uses.
func sampleRTOPage() string {
	var page strings.Builder
	// The parser refuses a page with too few sections, so pad with empty ones.
	for _, prefix := range []string{"AR", "AS", "BR", "CG", "DD", "DL", "GA", "GJ", "HP", "HR", "JH", "JK", "KA", "KL", "LA", "LD", "MH", "ML", "MN", "MP", "MZ", "NL", "OD", "PB", "PY", "RJ", "SK", "TG", "TR", "UK"} {
		page.WriteString("==" + prefix + "—Somewhere==\nNo table here.\n")
	}
	page.WriteString(`==TN—Tamil Nadu==
Intro text mentioning TN-99 that is not a table row.
{| class="wikitable sortable"
|-
! scope="col" style="width:10%"  | Code
! scope="col" | Location
! scope="col" | District
! scope="col" | Zone
! scope="col" class="unsortable" | Annotations
|-
| TN-01
| [[Chennai]] (Central): [[Ayanavaram]]
| [[Chennai district|Chennai]]
| Chennai North
| [[State Express Transport Corporation|SETC]] buses<ref>{{Cite web|title=x}}</ref> '''here'''
|-
| TN 28
| [[Namakkal]] (North)
| [[Namakkal]]
| Erode
|
|-
| TN-28Z || [[Rasipuram]] || [[Namakkal]] || Erode || Unit office
|}
==AP—Andhra Pradesh==
{| class="wikitable"
! Code !! Office location !! Jurisdiction area !! Annotations
|-
| rowspan="2" | AP-39, AP-40<ref name="a" />
| rowspan="2" |RTA
| [[Guntur district]]
|
|-
| [[Krishna district]]
|
|}
==CH—Chandigarh==
{| class="wikitable"
! Code !! Office location !! Jurisdiction area !! Annotations
|-
| CH-01 <br /> CH-02
|[[Chandigarh]]
|
|| First cell empty, note in the last
|-
| XX-01
| Wrong prefix for this section
|
|
|}
==UP—Uttar Pradesh==
{| class="wikitable"
! Code !! Jurisdiction
|-
| UP-32 || [[Lucknow]]
|}
==WB—West Bengal==
{| class="wikitable"
! Scheme !! Meaning
|-
| WB-01 A || not an office table
|}
`)
	return page.String()
}

func TestParseRTO(t *testing.T) {
	offices, err := parseRTO(sampleRTOPage())
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]int{}
	for i, office := range offices {
		byCode[office.Code] = i
	}
	if len(offices) != 8 {
		t.Fatalf("parsed %d codes, want 8: %v", len(offices), byCode)
	}

	check := func(code, office, jurisdiction, notes string) {
		t.Helper()
		i, ok := byCode[code]
		if !ok {
			t.Errorf("%s missing", code)
			return
		}
		got := offices[i]
		if got.Office != office || got.Jurisdiction != jurisdiction || got.Notes != notes {
			t.Errorf("%s = %q / %q / %q, want %q / %q / %q", code, got.Office, got.Jurisdiction, got.Notes, office, jurisdiction, notes)
		}
	}
	check("TN-01", "Chennai (Central): Ayanavaram", "Chennai", "SETC buses here") // links, refs, templates and bold removed
	check("TN-28", "Namakkal (North)", "Namakkal", "")                            // "TN 28" normalised
	check("TN-28Z", "Rasipuram", "Namakkal", "Unit office")                       // cells on one line
	check("AP-39", "RTA", "Guntur district, Krishna district", "")                // rowspan rows merged
	check("AP-40", "RTA", "Guntur district, Krishna district", "")                // several codes in one cell
	check("CH-01", "Chandigarh", "", "First cell empty, note in the last")
	check("CH-02", "Chandigarh", "", "First cell empty, note in the last")
	check("UP-32", "Lucknow", "Lucknow", "") // a jurisdiction-only table

	for i := 1; i < len(offices); i++ {
		if offices[i-1].Code >= offices[i].Code {
			t.Errorf("codes are not sorted: %s before %s", offices[i-1].Code, offices[i].Code)
		}
	}
}

func TestParseRTORejectsAChangedPage(t *testing.T) {
	if _, err := parseRTO("==TN—Tamil Nadu==\nnothing else"); err == nil {
		t.Error("a page with one section should be refused")
	}
}
