package validate

import "testing"

// The validators take whatever a caller sends. None may panic, and a value's normalised
// form must get the same answer as the value itself.
func FuzzValidators(f *testing.F) {
	for _, seed := range []string{"ABCPE1234F", "27AAPFU0939F1ZV", "SBIN0000001", "abcpe 1234 f", "", "ı", "234123412346", "4111111111111111"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		for name, check := range map[string]func(string) Result{"PAN": PAN, "IFSC": IFSC, "GSTIN": GSTIN} {
			r := check(input)
			if r.Valid == (r.Reason != "") {
				t.Errorf("%s(%q): valid %v with reason %q", name, input, r.Valid, r.Reason)
			}
			if again := check(r.Normalized); again.Valid != r.Valid || again.Normalized != r.Normalized {
				t.Errorf("%s(%q) is %v as %q, but %v as its normalised form", name, input, r.Valid, r.Normalized, again.Valid)
			}
		}
		VerhoeffValid(input)
		LuhnValid(input)
		if c := GSTINCheckChar(input); c == 0 {
			t.Errorf("GSTINCheckChar(%q) gave no character", input)
		}
	})
}
