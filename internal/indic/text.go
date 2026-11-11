// Package indic holds text tools for Indian scripts: splitting into aksharas (the
// letters a reader counts), dictionary order, normalising look-alike sequences,
// Bharati braille and keyboard layouts.
package indic

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

func isVirama(r rune) bool {
	switch r {
	case 0x094D, 0x09CD, 0x0A4D, 0x0ACD, 0x0B4D, 0x0BCD, 0x0C4D, 0x0CCD, 0x0D4D:
		return true
	}
	return false
}

func isTamil(r rune) bool { return r >= 0x0B80 && r <= 0x0BFF }

const (
	zwj  = '\u200D'
	zwnj = '\u200C'
)

// Aksharas splits text into the units a reader of an Indian script counts as one
// letter: a consonant with its vowel sign and marks, and in most scripts the
// consonants joined to it by a virama (क्ष is one). In Tamil a consonant with pulli
// (க்) is a letter of its own, as Tamil grammar counts it. Spaces and punctuation are
// left out.
func Aksharas(text string) []string {
	text = norm.NFC.String(text)
	var out []string
	var current []rune
	joinNext := false
	flush := func() {
		if len(current) > 0 {
			out = append(out, string(current))
			current = nil
		}
	}
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || r == zwj || r == zwnj:
			if len(current) == 0 {
				continue // a sign with nothing to sit on
			}
			current = append(current, r)
			joinNext = isVirama(r) && !isTamil(r)
			if r == zwj {
				joinNext = true
			}
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if !joinNext {
				flush()
			}
			current = append(current, r)
			joinNext = false
		default:
			flush()
			joinNext = false
		}
	}
	flush()
	return out
}

// Count is how a text's letters are counted.
type Count struct {
	Aksharas   int `json:"aksharas"`
	CodePoints int `json:"codePoints"`
	Words      int `json:"words"`
	Bytes      int `json:"bytesUTF8"`
}

// CountText counts a text the ways people and programs count it.
func CountText(text string) Count {
	return Count{Aksharas: len(Aksharas(text)), CodePoints: len([]rune(text)), Words: len(strings.Fields(text)), Bytes: len(text)}
}

// Sort puts words in the dictionary order of a language (hi, ta, bn, te, kn, ml, mr,
// gu, pa, or, en), as Unicode's collation rules (CLDR) give it.
func Sort(words []string, lang string) ([]string, error) {
	tag, err := language.Parse(lang)
	if err != nil {
		return nil, err
	}
	c := collate.New(tag)
	out := append([]string(nil), words...)
	sort.SliceStable(out, func(i, j int) bool { return c.CompareString(out[i], out[j]) < 0 })
	return out, nil
}

// Fix is one change Normalise made.
type Fix struct {
	From string `json:"from"`
	To   string `json:"to"`
	Why  string `json:"why"`
}

// lookAlikes are sequences that look like a letter but are not it, as typed with old
// keyboards and fonts or pasted from PDFs.
var lookAlikes = []struct{ from, to, why string }{
	{"अा", "आ", "a + aa sign drawn as the letter aa"},
	{"अो", "ओ", "a + o sign drawn as the letter o"},
	{"अौ", "औ", "a + au sign drawn as the letter au"},
	{"एे", "ऐ", "e + e sign drawn as the letter ai"},
	{"\u0BC6\u0BBE", "\u0BCA", "Tamil e sign + aa sign typed for the o sign"},
	{"\u0BC7\u0BBE", "\u0BCB", "Tamil ee sign + aa sign typed for the oo sign"},
	{"ന്\u200D", "ൻ", "old chillu n (with ZWJ)"},
	{"ണ്\u200D", "ൺ", "old chillu nn"},
	{"ര്\u200D", "ർ", "old chillu r"},
	{"ല്\u200D", "ൽ", "old chillu l"},
	{"ള്\u200D", "ൾ", "old chillu ll"},
	{"்்", "்", "pulli typed twice"},
	{"््", "्", "virama typed twice"},
}

// Normalise composes a text (Unicode NFC), replaces look-alike sequences with the
// letters they imitate, and with strip removes zero-width joiners and spaces.
func Normalise(text string, strip bool) (string, []Fix) {
	fixes := []Fix{}
	composed := norm.NFC.String(text)
	if composed != text {
		fixes = append(fixes, Fix{From: "decomposed letters", To: "composed (NFC)", Why: "the same letters stored two ways compare unequal"})
	}
	for _, l := range lookAlikes {
		if strings.Contains(composed, l.from) {
			composed = strings.ReplaceAll(composed, l.from, l.to)
			fixes = append(fixes, Fix{From: l.from, To: l.to, Why: l.why})
		}
	}
	if strip {
		before := composed
		composed = strings.Map(func(r rune) rune {
			switch r {
			case zwj, zwnj, '\u200B', '\uFEFF', '\u00AD':
				return -1
			}
			return r
		}, composed)
		if before != composed {
			fixes = append(fixes, Fix{From: "zero-width characters", To: "", Why: "invisible joiners, spaces and soft hyphens removed"})
		}
	}
	return composed, fixes
}
