package validate

import "testing"

func TestGSTIN(t *testing.T) {
	// Real registrations, so their check characters are known to be correct.
	for _, value := range []string{"27AAPFU0939F1ZV", "27AAACR4849R1ZL", "27AAACR5055K1Z7", "24AAACC1206D1ZM", " 27aapfu0939f1zv "} {
		if r := GSTIN(value); !r.Valid {
			t.Errorf("%q should be valid: %s", value, r.Reason)
		}
	}
	r := GSTIN("27AAPFU0939F1ZV")
	if r.Parts["state"] != "Maharashtra" || r.Parts["pan"] != "AAPFU0939F" || r.Parts["holderType"] != "Firm or Limited Liability Partnership" {
		t.Errorf("unexpected parts: %v", r.Parts)
	}

	invalid := map[string]string{
		"27AAPFU0939F1ZW":  "wrong check character",
		"27AAPFU0939F1Z":   "too short",
		"00AAPFU0939F1ZV":  "unknown state code",
		"27AAPXU0939F1ZV":  "bad PAN holder type",
		"27AAPFU0939F1YV":  "14th character must be Z",
		"27AAPFU0939F0ZV":  "entity character cannot be 0",
		"":                 "empty",
		"22AAAAA0000A1Z5":  "the widely copied format illustration, whose check character is not real",
		"27AAPFU0939F1ZVV": "too long",
	}
	for value, why := range invalid {
		if r := GSTIN(value); r.Valid || r.Reason == "" {
			t.Errorf("%q should be invalid (%s), got %+v", value, why, r)
		}
	}
	// Swapping any two adjacent, different characters must break the checksum.
	base := "27AAPFU0939F1ZV"
	for i := 0; i < 13; i++ {
		if base[i] == base[i+1] {
			continue
		}
		swapped := []byte(base)
		swapped[i], swapped[i+1] = swapped[i+1], swapped[i]
		if GSTIN(string(swapped)).Valid {
			t.Errorf("transposition at %d went undetected: %s", i, swapped)
		}
	}
}

func TestPAN(t *testing.T) {
	if r := PAN("abcpe1234f"); !r.Valid || r.Normalized != "ABCPE1234F" || r.Parts["holderType"] != "Individual" {
		t.Errorf("unexpected: %+v", r)
	}
	for _, value := range []string{"ABCXE1234F", "ABCPE12345", "ABCP1234F", "1BCPE1234F", ""} {
		if PAN(value).Valid {
			t.Errorf("%q should be invalid", value)
		}
	}
}

func TestIFSC(t *testing.T) {
	if r := IFSC("sbin0000001"); !r.Valid || r.Parts["bankCode"] != "SBIN" || r.Parts["branchCode"] != "000001" {
		t.Errorf("unexpected: %+v", r)
	}
	for _, value := range []string{"SBIN1000001", "SBI00000001", "SBIN000001", "SBIN00000011", ""} {
		if IFSC(value).Valid {
			t.Errorf("%q should be invalid", value)
		}
	}
}

func TestChecksums(t *testing.T) {
	// 2341 2341 2346 is the widely published Verhoeff sample; UIDAI's sample numbers follow the same rule.
	if !VerhoeffValid("234123412346") || VerhoeffValid("234123412347") || VerhoeffValid("23412341234a") {
		t.Error("Verhoeff check wrong")
	}
	if !VerhoeffValid("2363") || VerhoeffValid("2364") { // the textbook example: 236 has check digit 3
		t.Error("Verhoeff textbook example wrong")
	}
	if !LuhnValid("4111111111111111") || !LuhnValid("79927398713") || LuhnValid("4111111111111112") || LuhnValid("0") {
		t.Error("Luhn check wrong")
	}
}
