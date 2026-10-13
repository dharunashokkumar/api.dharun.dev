package translit

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Romanisation tables, by offset within a block. ISO 15919 is the reference; IAST and
// the plain ASCII scheme are written as differences from it.

var isoConsonants = map[rune]string{
	0x15: "k", 0x16: "kh", 0x17: "g", 0x18: "gh", 0x19: "ṅ",
	0x1A: "c", 0x1B: "ch", 0x1C: "j", 0x1D: "jh", 0x1E: "ñ",
	0x1F: "ṭ", 0x20: "ṭh", 0x21: "ḍ", 0x22: "ḍh", 0x23: "ṇ",
	0x24: "t", 0x25: "th", 0x26: "d", 0x27: "dh", 0x28: "n", 0x29: "ṉ",
	0x2A: "p", 0x2B: "ph", 0x2C: "b", 0x2D: "bh", 0x2E: "m",
	0x2F: "y", 0x30: "r", 0x31: "ṟ", 0x32: "l", 0x33: "ḷ", 0x34: "ḻ", 0x35: "v",
	0x36: "ś", 0x37: "ṣ", 0x38: "s", 0x39: "h",
	0x58: "q", 0x59: "k͟h", 0x5A: "ġ", 0x5B: "z", 0x5C: "ṛ", 0x5D: "ṛh", 0x5E: "f", 0x5F: "ẏ",
}

// withNukta is what a consonant becomes with a nukta dot under it.
var withNukta = map[rune]rune{0x15: 0x58, 0x16: 0x59, 0x17: 0x5A, 0x1C: 0x5B, 0x21: 0x5C, 0x22: 0x5D, 0x2B: 0x5E, 0x2F: 0x5F}

// isoVowels covers both independent vowels and vowel signs, keyed by the independent
// vowel's offset; vowelSign maps a sign to that offset.
var isoVowels = map[rune]string{
	0x04: "a", 0x05: "a", 0x06: "ā", 0x07: "i", 0x08: "ī", 0x09: "u", 0x0A: "ū",
	0x0B: "r̥", 0x60: "r̥̄", 0x0C: "l̥", 0x61: "l̥̄",
	0x0D: "ê", 0x0E: "e", 0x0F: "ē", 0x10: "ai", 0x11: "ô", 0x12: "o", 0x13: "ō", 0x14: "au",
}

var vowelSign = map[rune]rune{
	0x3E: 0x06, 0x3F: 0x07, 0x40: 0x08, 0x41: 0x09, 0x42: 0x0A, 0x43: 0x0B, 0x44: 0x60,
	0x62: 0x0C, 0x63: 0x61, 0x45: 0x0D, 0x46: 0x0E, 0x47: 0x0F, 0x48: 0x10, 0x49: 0x11,
	0x4A: 0x12, 0x4B: 0x13, 0x4C: 0x14, 0x57: 0x14,
}

var isoMarks = map[rune]string{oCandrabindu: "m̐", oAnusvara: "ṁ", oVisarga: "ḥ", oAvagraha: "’", 0x50: "ōṁ", 0x64: ".", 0x65: ".."}

var iastDifferences = map[string]string{
	"r̥": "ṛ", "r̥̄": "ṝ", "l̥": "ḷ", "l̥̄": "ḹ", "ē": "e", "ō": "o", "ṁ": "ṃ", "’": "'",
}

var asciiConsonants = map[string]string{
	"ṅ": "ng", "c": "ch", "ch": "chh", "ñ": "ny", "ṭ": "t", "ṭh": "th", "ḍ": "d", "ḍh": "dh", "ṇ": "n", "ṉ": "n",
	"ṟ": "r", "ḷ": "l", "ḻ": "zh", "ś": "sh", "ṣ": "sh", "k͟h": "kh", "ġ": "g", "ṛ": "r", "ṛh": "rh", "ẏ": "y",
}

var asciiVowels = map[string]string{
	"ā": "aa", "ī": "ii", "ū": "uu", "r̥": "ri", "r̥̄": "rii", "l̥": "li", "l̥̄": "lii",
	"ê": "e", "ē": "e", "ô": "o", "ō": "o",
}

var asciiMarks = map[string]string{"m̐": "n", "ṁ": "m", "ḥ": "h", "’": "", "ōṁ": "om"}

func inScheme(roman, scheme string, kind byte) string {
	switch scheme {
	case "iast":
		if alternative, ok := iastDifferences[roman]; ok {
			return alternative
		}
	case "ascii":
		table := map[byte]map[string]string{'c': asciiConsonants, 'v': asciiVowels, 'm': asciiMarks}[kind]
		if alternative, ok := table[roman]; ok {
			return alternative
		}
	}
	return roman
}

// romanise writes units in a Roman scheme. In the ASCII scheme, Hindi-style word-final
// inherent vowels are dropped (भारत is bharat), as people write them.
func romanise(units []unit, scheme string) string {
	var out strings.Builder
	for i := 0; i < len(units); i++ {
		u := units[i]
		o := u.offset
		switch {
		case o < 0:
			out.WriteRune(u.r)
		case o == 0x4E: // Bengali khanda ta: a t with no vowel
			out.WriteString("t")
		case isConsonant(o):
			if i+1 < len(units) && units[i+1].offset == oNukta {
				if nukta, ok := withNukta[o]; ok {
					o = nukta
				}
				i++
			}
			out.WriteString(inScheme(isoConsonants[o], scheme, 'c'))
			next := rune(-2)
			if i+1 < len(units) {
				next = units[i+1].offset
			}
			switch {
			case next == oVirama:
				i++
			case vowelSign[next] != 0:
				out.WriteString(inScheme(isoVowels[vowelSign[next]], scheme, 'v'))
				i++
			default:
				if scheme == "ascii" && schwaDropping[u.script] && wordEnds(units, i+1) && syllables(units, i) > 1 {
					break // final inherent vowel, silent in Hindi
				}
				out.WriteString("a")
			}
		case isIndependentVowel(o):
			out.WriteString(inScheme(isoVowels[o], scheme, 'v'))
		case vowelSign[o] != 0:
			// A sign without a consonant, in broken input.
			out.WriteString(inScheme(isoVowels[vowelSign[o]], scheme, 'v'))
		case o >= 0x66 && o <= 0x6F:
			out.WriteRune('0' + (o - 0x66))
		case o == oAnusvara && scheme == "ascii":
			out.WriteString(asciiNasal(units, i+1))
		case isoMarks[o] != "":
			out.WriteString(inScheme(isoMarks[o], scheme, 'm'))
		case o == oVirama || o == oNukta:
			// Dangling marks carry no sound of their own.
		}
	}
	return out.String()
}

// schwaDropping are the scripts of languages that leave a word-final inherent vowel
// unspoken (Hindi, Punjabi, Gujarati), so the ASCII scheme leaves it out.
var schwaDropping = map[string]bool{"devanagari": true, "gurmukhi": true, "gujarati": true}

// asciiNasal is how an anusvara sounds before the letter at i: the nasal of that
// letter's group, as people spell it (gaṅgā is ganga, sampūrṇa is sampurna).
func asciiNasal(units []unit, i int) string {
	if i < len(units) && isConsonant(units[i].offset) {
		o := units[i].offset
		switch {
		case o >= 0x15 && o <= 0x18:
			return "n" // before k, g: ng is written n in everyday spelling
		case o >= 0x2A && o <= 0x2E:
			return "m"
		case o <= 0x28:
			return "n"
		}
	}
	return "m"
}

// wordEnds reports whether nothing but marks of the same syllable follow position i.
func wordEnds(units []unit, i int) bool {
	for ; i < len(units); i++ {
		o := units[i].offset
		if o == oAnusvara || o == oCandrabindu || o == oVisarga {
			return false // a nasal or visarga keeps the vowel
		}
		return o < 0 || !(isConsonant(o) || isIndependentVowel(o) || vowelSign[o] != 0 || o == oNukta || o == oVirama)
	}
	return true
}

// syllables counts the vowels of the word ending at position i.
func syllables(units []unit, i int) int {
	count := 0
	for ; i >= 0 && units[i].offset >= 0; i-- {
		o := units[i].offset
		if isIndependentVowel(o) || vowelSign[o] != 0 || (isConsonant(o) && !(i+1 < len(units) && units[i+1].offset == oVirama)) {
			count++
		}
	}
	return count
}

// Romanise writes Indian-script text in a Roman scheme: iso15919, iast or ascii.
func Romanise(text, scheme string) (string, error) {
	if scheme != "iso15919" && scheme != "iast" && scheme != "ascii" {
		return "", ErrNotSupported
	}
	return norm.NFC.String(romanise(toUnits(text), scheme)), nil
}

// ---- Roman to script ----

type romanToken struct {
	text   string
	offset rune
	kind   byte // 'c' consonant, 'v' vowel, 'm' mark
}

// readingTable lists every Roman spelling a scheme accepts, longest first.
func readingTable(scheme string) []romanToken {
	var tokens []romanToken
	add := func(text string, offset rune, kind byte) {
		tokens = append(tokens, romanToken{norm.NFC.String(text), offset, kind})
	}
	for offset, roman := range isoConsonants {
		if scheme == "iast" && (offset == 0x33 || offset == 0x5C || offset == 0x5D) {
			continue // in IAST ḷ and ṛ are vowels, not the consonants ळ and ड़
		}
		add(roman, offset, 'c')
	}
	for offset, roman := range isoVowels {
		if offset == 0x04 {
			continue
		}
		spelling := roman
		if scheme == "iast" {
			spelling = inScheme(roman, "iast", 'v')
			if offset == 0x0E || offset == 0x12 {
				continue // IAST has no short e and o; e and o are the long ones
			}
		}
		add(spelling, offset, 'v')
	}
	for offset, roman := range isoMarks {
		if offset == 0x64 || offset == 0x65 || offset == 0x50 {
			continue
		}
		add(roman, offset, 'm')
		add(inScheme(roman, "iast", 'm'), offset, 'm')
	}
	sort.SliceStable(tokens, func(a, b int) bool {
		return len([]rune(tokens[a].text)) > len([]rune(tokens[b].text))
	})
	return tokens
}

// FromRoman reads ISO 15919 or IAST text and writes it in an Indian script.
func FromRoman(text, scheme string, target Script) (string, error) {
	if scheme != "iso15919" && scheme != "iast" {
		return "", ErrNotSupported
	}
	table := readingTable(scheme)
	source := []rune(norm.NFC.String(strings.ToLower(text)))
	const dev = 0x0900
	var out strings.Builder
	pending := false // a consonant waiting for its vowel
	closeConsonant := func() {
		if pending {
			out.WriteRune(dev + oVirama)
			pending = false
		}
	}
	for i := 0; i < len(source); {
		var found *romanToken
		for t := range table {
			candidate := []rune(table[t].text)
			if i+len(candidate) <= len(source) && string(source[i:i+len(candidate)]) == table[t].text {
				found = &table[t]
				break
			}
		}
		if found == nil {
			closeConsonant()
			out.WriteRune(source[i])
			i++
			continue
		}
		i += len([]rune(found.text))
		switch found.kind {
		case 'c':
			closeConsonant()
			out.WriteRune(dev + found.offset)
			pending = true
		case 'v':
			if pending {
				if found.offset != 0x05 { // the inherent a needs no sign
					for sign, vowel := range vowelSign {
						if vowel == found.offset && sign != 0x57 {
							out.WriteRune(dev + sign)
							break
						}
					}
				}
				pending = false
			} else {
				out.WriteRune(dev + found.offset)
			}
		case 'm':
			pending = false
			out.WriteRune(dev + found.offset)
		}
	}
	closeConsonant()
	devanagari := out.String()
	if target.ID == "devanagari" {
		return norm.NFC.String(devanagari), nil
	}
	return ToScript(devanagari, target), nil
}

// HasIndianLetters reports whether text contains any letter of the nine scripts.
func HasIndianLetters(text string) bool {
	for _, r := range text {
		if _, ok := scriptOf(r); ok && unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
