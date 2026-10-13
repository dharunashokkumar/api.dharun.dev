// Package translit converts text between the nine major Indian scripts and to and from
// Roman letters, by rule. Unicode lays these scripts out in parallel blocks inherited from
// ISCII, so most letters convert by position; the tables here cover the exceptions and
// the romanisation schemes. It does not guess: turning casual Roman typing ("vanakkam")
// into a script needs a language model, which this package does not have.
package translit

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Script is one Indian script.
type Script struct {
	ID    string
	Name  string
	base  rune
	table *unicode.RangeTable
}

// Scripts lists the scripts, in Unicode order.
var Scripts = []Script{
	{"devanagari", "Devanagari", 0x0900, unicode.Devanagari},
	{"bengali", "Bengali", 0x0980, unicode.Bengali},
	{"gurmukhi", "Gurmukhi", 0x0A00, unicode.Gurmukhi},
	{"gujarati", "Gujarati", 0x0A80, unicode.Gujarati},
	{"odia", "Odia", 0x0B00, unicode.Oriya},
	{"tamil", "Tamil", 0x0B80, unicode.Tamil},
	{"telugu", "Telugu", 0x0C00, unicode.Telugu},
	{"kannada", "Kannada", 0x0C80, unicode.Kannada},
	{"malayalam", "Malayalam", 0x0D00, unicode.Malayalam},
}

// Romanisations are the Roman-letter schemes. Only iso15919 and iast can be read back.
var Romanisations = []string{"iso15919", "iast", "ascii"}

// languageScripts lets a language code stand for its usual script.
var languageScripts = map[string]string{
	"hi": "devanagari", "mr": "devanagari", "ne": "devanagari", "sa": "devanagari", "kok": "devanagari", "mai": "devanagari",
	"bn": "bengali", "as": "bengali", "pa": "gurmukhi", "gu": "gujarati", "or": "odia", "oriya": "odia",
	"ta": "tamil", "te": "telugu", "kn": "kannada", "ml": "malayalam",
}

// Lookup finds a script by ID, name or language code.
func Lookup(name string) (Script, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if id, ok := languageScripts[name]; ok {
		name = id
	}
	for _, script := range Scripts {
		if script.ID == name {
			return script, true
		}
	}
	return Script{}, false
}

// scriptOf returns the Indian script a rune belongs to.
func scriptOf(r rune) (Script, bool) {
	for _, script := range Scripts {
		if r >= script.base && r < script.base+0x80 {
			return script, true
		}
	}
	return Script{}, false
}

// Detect names the Indian script most of the letters are in.
func Detect(text string) (Script, bool) {
	counts := map[string]int{}
	best, bestCount := Script{}, 0
	for _, r := range text {
		if script, ok := scriptOf(r); ok {
			counts[script.ID]++
			if counts[script.ID] > bestCount {
				best, bestCount = script, counts[script.ID]
			}
		}
	}
	return best, bestCount > 0
}

// Offsets within a block, named after their Devanagari letters.
const (
	oCandrabindu = 0x01
	oAnusvara    = 0x02
	oVisarga     = 0x03
	oNukta       = 0x3C
	oVirama      = 0x4D
	oAvagraha    = 0x3D
)

func isConsonant(o rune) bool { return (o >= 0x15 && o <= 0x39) || (o >= 0x58 && o <= 0x5F) }

func isIndependentVowel(o rune) bool { return (o >= 0x04 && o <= 0x14) || o == 0x60 || o == 0x61 }

// tamilSubstitute is what a letter Tamil lacks becomes: aspirated and voiced stops
// fall back to the plain stop Tamil writes for all of them.
var tamilSubstitute = map[rune]rune{
	0x16: 0x15, 0x17: 0x15, 0x18: 0x15, // kha ga gha -> ka
	0x1B: 0x1A, 0x1D: 0x1C, // cha -> ca, jha -> ja
	0x20: 0x1F, 0x21: 0x1F, 0x22: 0x1F, // ṭha ḍa ḍha -> ṭa
	0x25: 0x24, 0x26: 0x24, 0x27: 0x24, // tha da dha -> ta
	0x2B: 0x2A, 0x2C: 0x2A, 0x2D: 0x2A, // pha ba bha -> pa
	0x0B: 0x30, // vocalic r -> ra (with an i sign, below)
}

// gurmukhi and malayalam extras that have no parallel position.
const (
	gurmukhiTippi = 0x0A70
	gurmukhiAddak = 0x0A71
)

var malayalamChillu = map[rune]rune{0x0D7A: 0x23, 0x0D7B: 0x28, 0x0D7C: 0x30, 0x0D7D: 0x32, 0x0D7E: 0x33, 0x0D7F: 0x15}

// chilluOf is the chillu letter for a consonant offset, the other way round; ൿ is left
// out as modern Malayalam rarely uses it.
var chilluOf = map[rune]rune{0x23: 0x0D7A, 0x28: 0x0D7B, 0x30: 0x0D7C, 0x32: 0x0D7D, 0x33: 0x0D7E}

// endsWord reports whether no letter follows position i in the same word.
func endsWord(units []unit, i int) bool {
	return i >= len(units) || units[i].offset < 0
}

// ErrNotSupported means a conversion this package does not offer.
var ErrNotSupported = errors.New("conversion not supported")

// letters turns Indian-script text into a sequence of block offsets with the script
// stripped, expanding the few letters that have no parallel position. Runes outside the
// Indian blocks are carried through as they are, marked with a negative offset.
type unit struct {
	offset rune   // within the block, or -1 for a rune passed through
	r      rune   // the original rune, for pass-through
	script string // the script the letter came from
}

func toUnits(text string) []unit {
	var units []unit
	runes := []rune(norm.NFC.String(text))
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == gurmukhiTippi:
			units = append(units, unit{offset: oAnusvara, script: "gurmukhi"})
		case r == gurmukhiAddak:
			// The addak doubles the next consonant.
			if i+1 < len(runes) {
				if script, ok := scriptOf(runes[i+1]); ok && isConsonant(runes[i+1]-script.base) {
					units = append(units, unit{offset: runes[i+1] - script.base, script: script.ID}, unit{offset: oVirama, script: script.ID})
				}
			}
		case malayalamChillu[r] != 0:
			units = append(units, unit{offset: malayalamChillu[r], script: "malayalam"}, unit{offset: oVirama, script: "malayalam"})
		default:
			if script, ok := scriptOf(r); ok {
				units = append(units, unit{offset: r - script.base, script: script.ID})
			} else {
				units = append(units, unit{offset: -1, r: r})
			}
		}
	}
	return units
}

// ToScript writes text from any Indian script in another one. Letters the target script
// lacks become their closest equivalent; anything else passes through.
func ToScript(text string, target Script) string {
	var out strings.Builder
	units := toUnits(text)
	for i, u := range units {
		if u.offset < 0 {
			out.WriteRune(u.r)
			continue
		}
		o := u.offset
		if target.ID == "tamil" {
			if o == oNukta {
				continue
			}
			if substitute, ok := tamilSubstitute[o]; ok {
				if o == 0x0B { // vocalic r, written ரு in Tamil
					out.WriteRune(target.base + 0x30)
					out.WriteRune(target.base + 0x41)
					continue
				}
				o = substitute
			}
			if o >= 0x58 && o <= 0x5F { // nukta consonants
				o = map[rune]rune{0x58: 0x15, 0x59: 0x15, 0x5A: 0x15, 0x5B: 0x1C, 0x5C: 0x1F, 0x5D: 0x1F, 0x5E: 0x2A, 0x5F: 0x2F}[o]
			}
			if o == oAnusvara {
				// Tamil writes a nasal before a consonant as ம்.
				out.WriteRune(target.base + 0x2E)
				out.WriteRune(target.base + oVirama)
				continue
			}
			if o == oCandrabindu {
				continue
			}
			if o == 0x43 || o == 0x44 { // vocalic r and rr signs: ்ரு and ்ரூ
				out.WriteRune(target.base + oVirama)
				out.WriteRune(target.base + 0x30)
				out.WriteRune(target.base + map[rune]rune{0x43: 0x41, 0x44: 0x42}[o])
				continue
			}
		}
		if target.ID == "malayalam" && o == oVirama && i > 0 && endsWord(units, i+1) {
			// Modern Malayalam ends a word in m with ം and in n, ṇ, r, l, ḷ with a chillu.
			if previous := units[i-1].offset; previous == 0x2E || chilluOf[previous] != 0 {
				rewritten := []rune(out.String())
				rewritten = rewritten[:len(rewritten)-1]
				out.Reset()
				out.WriteString(string(rewritten))
				if previous == 0x2E {
					out.WriteRune(target.base + oAnusvara)
				} else {
					out.WriteRune(chilluOf[previous])
				}
				continue
			}
		}
		if o == oNukta && target.ID != "devanagari" && !unicode.Is(target.table, target.base+o) {
			continue
		}
		converted := target.base + o
		if !unicode.Is(target.table, converted) {
			// No parallel letter: fall back through romanisation so nothing is lost.
			out.WriteString(romanise([]unit{u}, "iso15919"))
			continue
		}
		out.WriteRune(converted)
	}
	return norm.NFC.String(out.String())
}
