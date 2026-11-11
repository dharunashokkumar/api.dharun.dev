package docai

import (
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
)

// IDNumber is an Aadhaar number (12 digits) or Virtual ID (16 digits) found among OCR
// words, with the words that hold its digits in order.
type IDNumber struct {
	Kind  string // aadhaar or vid
	Words []Word
	// Digits per word, so a mask can cover the first eight digits and leave the last four.
	Digits []string
}

// FindIDNumbers finds Aadhaar numbers and Virtual IDs on lines of OCR words: four-digit
// groups next to each other on one line (or one word of 12 or 16 digits) whose last
// digit is a correct Verhoeff check. An Aadhaar number never starts with 0 or 1.
func FindIDNumbers(lines [][]Word) []IDNumber {
	var out []IDNumber
	for _, line := range lines {
		used := make([]bool, len(line))
		for i := range line {
			if used[i] {
				continue
			}
			digits, words, parts := "", []Word(nil), []string(nil)
			for j := i; j < len(line) && len(digits) < 16; j++ {
				d := onlyDigits(line[j].Text)
				if d == "" || len(d)%4 != 0 || (j > i && len(d) != 4) {
					break
				}
				digits += d
				words, parts = append(words, line[j]), append(parts, d)
				if (len(digits) == 12 || len(digits) == 16) && validID(digits) {
					// Prefer a 16-digit VID when the next group completes one.
					if len(digits) == 12 && j+1 < len(line) {
						if next := onlyDigits(line[j+1].Text); len(next) == 4 && validID(digits+next) && (j+2 >= len(line) || onlyDigits(line[j+2].Text) == "") {
							continue
						}
					}
					kind := "aadhaar"
					if len(digits) == 16 {
						kind = "vid"
					}
					out = append(out, IDNumber{Kind: kind, Words: words, Digits: parts})
					for k := i; k <= j; k++ {
						used[k] = true
					}
					break
				}
			}
		}
	}
	return out
}

func validID(digits string) bool {
	return digits[0] >= '2' && validate.VerhoeffValid(digits)
}

// onlyDigits returns the word's digits when the word is digits with at most stray
// punctuation OCR adds; otherwise "".
func onlyDigits(s string) string {
	s = strings.Trim(s, ".,:;-|()[]")
	for _, r := range s {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return s
}
