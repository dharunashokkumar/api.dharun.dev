// Package validate checks the structure of Indian identifiers. It answers "is this
// well-formed?", never "does this exist?": nothing here contacts a government system.
package validate

import (
	"regexp"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// Result is the outcome of one check. Parts explains the pieces of a valid identifier.
type Result struct {
	Input      string            `json:"input,omitempty"`
	Normalized string            `json:"normalized"`
	Valid      bool              `json:"valid"`
	Reason     string            `json:"reason,omitempty"`
	Parts      map[string]string `json:"parts,omitempty"`
}

var (
	panPattern   = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)
	ifscPattern  = regexp.MustCompile(`^[A-Z]{4}0[A-Z0-9]{6}$`)
	gstinPattern = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)
)

var panHolderTypes = map[byte]string{
	'P': "Individual",
	'C': "Company",
	'H': "Hindu Undivided Family",
	'F': "Firm or Limited Liability Partnership",
	'A': "Association of Persons",
	'T': "Trust",
	'B': "Body of Individuals",
	'L': "Local Authority",
	'J': "Artificial Juridical Person",
	'G': "Government",
}

// Invalid bytes become U+FFFD first, so removing spaces cannot join them into a character.
func normalize(input string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.ToValidUTF8(input, "�")), ""))
}

// PAN checks a Permanent Account Number: five letters, four digits, one letter, with
// the fourth letter naming the kind of holder. A PAN has no public checksum, so a
// well-formed value can still be one that was never issued.
func PAN(input string) Result {
	r := Result{Input: input, Normalized: normalize(input)}
	if !panPattern.MatchString(r.Normalized) {
		r.Reason = "A PAN is 10 characters: five letters, four digits, then one letter."
		return r
	}
	holder, ok := panHolderTypes[r.Normalized[3]]
	if !ok {
		r.Reason = "The fourth character must be a holder type: P, C, H, F, A, T, B, L, J or G."
		return r
	}
	r.Valid = true
	r.Parts = map[string]string{"holderType": holder}
	return r
}

// IFSC checks the shape of an Indian Financial System Code: four letters for the bank,
// a zero, then six characters for the branch.
func IFSC(input string) Result {
	r := Result{Input: input, Normalized: normalize(input)}
	if !ifscPattern.MatchString(r.Normalized) {
		r.Reason = "An IFSC is 11 characters: four letters, the digit 0, then six letters or digits."
		return r
	}
	r.Valid = true
	r.Parts = map[string]string{"bankCode": r.Normalized[:4], "branchCode": r.Normalized[5:]}
	return r
}

// GSTIN checks a Goods and Services Tax Identification Number: state code, the holder's
// PAN, an entity number, the letter Z and a check character.
func GSTIN(input string) Result {
	r := Result{Input: input, Normalized: normalize(input)}
	value := r.Normalized
	if !gstinPattern.MatchString(value) {
		r.Reason = "A GSTIN is 15 characters: two-digit state code, a 10-character PAN, an entity character, the letter Z and a check character."
		return r
	}

	stateCode := value[:2]
	stateName := india.GSTOnlyCodes[stateCode]
	if state, ok := india.ByGSTCode(stateCode); ok {
		stateName = state.Name
	}
	if stateName == "" {
		r.Reason = "The first two digits are not a GST state code."
		return r
	}

	pan := PAN(value[2:12])
	if !pan.Valid {
		r.Reason = "Characters 3 to 12 must be a valid PAN. " + pan.Reason
		return r
	}

	if want := gstinCheckChar(value[:14]); value[14] != want {
		r.Reason = "The check character does not match; one of the characters is wrong."
		return r
	}

	r.Valid = true
	r.Parts = map[string]string{
		"stateCode":    stateCode,
		"state":        stateName,
		"pan":          value[2:12],
		"holderType":   pan.Parts["holderType"],
		"entityNumber": value[12:13],
	}
	return r
}

const gstinAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// gstinCheckChar computes the 15th character from the first 14: a base-36 weighted
// sum (weights 1, 2, 1, 2 ...) where each product contributes quotient plus remainder.
// GSTINCheckChar is the check character for the first 14 characters of a GSTIN.
func GSTINCheckChar(first14 string) byte { return gstinCheckChar(first14) }

func gstinCheckChar(first14 string) byte {
	sum := 0
	for i := 0; i < len(first14); i++ {
		value := strings.IndexByte(gstinAlphabet, first14[i])
		product := value * (1 + i%2)
		sum += product/36 + product%36
	}
	return gstinAlphabet[(36-sum%36)%36]
}

// Verhoeff tables (dihedral group D5), as used for the Aadhaar check digit.
var (
	verhoeffMultiply = [10][10]byte{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {1, 2, 3, 4, 0, 6, 7, 8, 9, 5}, {2, 3, 4, 0, 1, 7, 8, 9, 5, 6},
		{3, 4, 0, 1, 2, 8, 9, 5, 6, 7}, {4, 0, 1, 2, 3, 9, 5, 6, 7, 8}, {5, 9, 8, 7, 6, 0, 4, 3, 2, 1},
		{6, 5, 9, 8, 7, 1, 0, 4, 3, 2}, {7, 6, 5, 9, 8, 2, 1, 0, 4, 3}, {8, 7, 6, 5, 9, 3, 2, 1, 0, 4},
		{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
	}
	verhoeffPermute = [8][10]byte{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {1, 5, 7, 6, 2, 8, 3, 0, 9, 4}, {5, 8, 0, 3, 7, 9, 6, 1, 4, 2},
		{8, 9, 1, 6, 0, 4, 3, 5, 2, 7}, {9, 4, 5, 3, 1, 2, 6, 8, 7, 0}, {4, 2, 8, 6, 5, 7, 3, 9, 0, 1},
		{2, 7, 9, 3, 8, 0, 6, 4, 1, 5}, {7, 0, 4, 6, 9, 1, 3, 2, 5, 8},
	}
)

// VerhoeffValid reports whether a string of digits ends in a correct Verhoeff check digit.
func VerhoeffValid(digits string) bool {
	var check byte
	for i := 0; i < len(digits); i++ {
		d := digits[len(digits)-1-i]
		if d < '0' || d > '9' {
			return false
		}
		check = verhoeffMultiply[check][verhoeffPermute[i%8][d-'0']]
	}
	return len(digits) > 0 && check == 0
}

// LuhnValid reports whether a string of digits passes the Luhn check used by payment cards.
func LuhnValid(digits string) bool {
	sum := 0
	for i := 0; i < len(digits); i++ {
		d := int(digits[len(digits)-1-i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		if i%2 == 1 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return len(digits) > 1 && sum%10 == 0
}
