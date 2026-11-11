package indic

import (
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/translit"
)

// Bharati braille gives every Indian script the same cells, by sound: क, க, ক and ক
// are all dots 1-3. Text in another Indian script is first carried into Devanagari
// letter for letter (internal/translit), then written cell by cell. A consonant's
// inherent a is not written; a vowel sign is written as the vowel's own cell after the
// consonant, and a consonant without a vowel (with virama) takes the virama cell.

// dots builds a braille cell from its dot numbers, such as "13".
func dots(numbers string) rune {
	cell := rune(0x2800)
	for _, n := range numbers {
		cell |= 1 << (n - '1')
	}
	return cell
}

var brailleLetters = map[rune]string{
	// Vowels, and their signs after a consonant.
	'अ': "1", 'आ': "345", 'ा': "345", 'इ': "24", 'ि': "24", 'ई': "35", 'ी': "35", 'उ': "136", 'ु': "136", 'ऊ': "1256", 'ू': "1256",
	'ए': "15", 'े': "15", 'ऐ': "34", 'ै': "34", 'ओ': "135", 'ो': "135", 'औ': "246", 'ौ': "246",
	'ऎ': "26", 'ॆ': "26", 'ऒ': "1346", 'ॊ': "1346", 'ऑ': "345", 'ॉ': "345",
	// Consonants.
	'क': "13", 'ख': "46", 'ग': "1245", 'घ': "126", 'ङ': "346",
	'च': "14", 'छ': "16", 'ज': "245", 'झ': "356", 'ञ': "25",
	'ट': "23456", 'ठ': "2456", 'ड': "1246", 'ढ': "123456", 'ण': "3456",
	'त': "2345", 'थ': "1456", 'द': "145", 'ध': "2346", 'न': "1345",
	'प': "1234", 'फ': "235", 'ब': "12", 'भ': "45", 'म': "134",
	'य': "13456", 'र': "1235", 'ल': "123", 'व': "1236",
	'श': "146", 'ष': "12346", 'स': "234", 'ह': "125", 'ळ': "456",
	// Signs.
	'ं': "56", 'ः': "6", 'ँ': "3", '्': "4", '।': "256", '॥': "256 256",
	// Punctuation shared with English braille.
	',': "2", ';': "23", ':': "25", '.': "256", '?': "236", '!': "235", '-': "36", '\'': "3", '"': "236",
}

// Two-cell letters.
var brailleSequences = map[string]string{"ऋ": "5 1235", "ृ": "5 1235", "क्ष": "12345", "ज्ञ": "156"}

const numberSign = "3456"

// digitCells are the cells for 1 to 9 and 0 after the number sign (the letters a to j).
var digitCells = map[rune]string{'1': "1", '2': "12", '3': "14", '4': "145", '5': "15", '6': "124", '7': "1245", '8': "125", '9': "24", '0': "245"}

// Braille is a text written in Bharati braille.
type Braille struct {
	Braille   string   `json:"braille"`
	Cells     int      `json:"cells"`
	Script    string   `json:"script"`
	Unwritten []string `json:"unwritten"` // letters with no Bharati cell here, left as they were
}

// ToBraille writes text in Bharati braille. ASCII digits and Indian-script digits get the
// number sign; Latin letters are left as they are.
func ToBraille(text string) Braille {
	script := "devanagari"
	if s, ok := translit.Detect(text); ok && s.ID != "devanagari" {
		script = s.ID
		if deva, found := translit.Lookup("devanagari"); found {
			text = translit.ToScript(text, deva)
		}
	}
	text = ToASCIIDigits(text)
	var out strings.Builder
	cells := 0
	unwritten := map[string]bool{}
	write := func(pattern string) {
		for _, cell := range strings.Fields(pattern) {
			out.WriteRune(dots(cell))
			cells++
		}
	}
	runes := []rune(text)
	inNumber := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if cell, ok := digitCells[r]; ok {
			if !inNumber {
				write(numberSign)
				inNumber = true
			}
			write(cell)
			continue
		}
		inNumber = false
		if r == ' ' || r == '\n' || r == '\t' {
			out.WriteRune(r)
			continue
		}
		// क्ष and ज्ञ have cells of their own.
		if i+2 < len(runes) {
			if pattern, ok := brailleSequences[string(runes[i:i+3])]; ok {
				write(pattern)
				i += 2
				continue
			}
		}
		if pattern, ok := brailleSequences[string(r)]; ok {
			write(pattern)
			continue
		}
		// A nukta is dropped: the base letter's cell is the nearest Bharati has.
		if r == '़' {
			continue
		}
		if pattern, ok := brailleLetters[r]; ok {
			write(pattern)
			continue
		}
		if r < 128 {
			out.WriteRune(r)
			continue
		}
		unwritten[string(r)] = true
		out.WriteRune(r)
	}
	list := []string{}
	for letter := range unwritten {
		list = append(list, letter)
	}
	return Braille{Braille: out.String(), Cells: cells, Script: script, Unwritten: list}
}

// ToASCIIDigits turns every Indian-script digit into an ASCII one.
func ToASCIIDigits(s string) string {
	return strings.Map(func(r rune) rune {
		for _, zero := range []rune{0x0966, 0x09E6, 0x0A66, 0x0AE6, 0x0B66, 0x0BE6, 0x0C66, 0x0CE6, 0x0D66} {
			if r >= zero && r <= zero+9 {
				return '0' + r - zero
			}
		}
		return r
	}, s)
}
