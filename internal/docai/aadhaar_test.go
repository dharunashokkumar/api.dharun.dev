package docai

import (
	"strconv"
	"testing"

	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
)

// withCheck appends the Verhoeff check digit.
func withCheck(digits string) string {
	for d := 0; d <= 9; d++ {
		if s := digits + strconv.Itoa(d); validate.VerhoeffValid(s) {
			return s
		}
	}
	panic("no check digit")
}

func wordRow(texts ...string) []Word {
	var out []Word
	for i, t := range texts {
		out = append(out, Word{Text: t, X0: float64(i * 50), X1: float64(i*50 + 40), Y1: 10})
	}
	return out
}

func TestFindIDNumbers(t *testing.T) {
	aadhaar := withCheck("23456789012")
	vid := withCheck("912345678901234")
	wrong := aadhaar[:11] + strconv.Itoa((int(aadhaar[11]-'0')+1)%10)
	lines := [][]Word{
		wordRow("Your", "Aadhaar", "No.", aadhaar[:4], aadhaar[4:8], aadhaar[8:]),
		wordRow("VID", ":", vid[:4], vid[4:8], vid[8:12], vid[12:]),
		wordRow(wrong[:4], wrong[4:8], wrong[8:]),
		wordRow("Phone", "9876", "5432", "1098"),
		wordRow(aadhaar),
	}
	found := FindIDNumbers(lines)
	if len(found) != 3 {
		t.Fatalf("found %d: %+v", len(found), found)
	}
	if found[0].Kind != "aadhaar" || len(found[0].Words) != 3 || found[0].Words[0].Text != aadhaar[:4] {
		t.Errorf("first = %+v", found[0])
	}
	if found[1].Kind != "vid" || len(found[1].Digits) != 4 {
		t.Errorf("second = %+v", found[1])
	}
	if found[2].Kind != "aadhaar" || found[2].Digits[0] != aadhaar {
		t.Errorf("third = %+v", found[2])
	}
}
