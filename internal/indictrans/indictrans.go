// Package indictrans prepares text for IndicTrans2 and finishes what it returns, as
// AI4Bharat's IndicProcessor (IndicTransToolkit, MIT) does: punctuation and digits are
// normalised; e-mail addresses, web addresses, long numbers and tags are swapped for
// placeholders the model copies through; English is split into tokens the way the Moses
// tokenizer does; Indian scripts other than Perso-Arabic, Ol Chiki and Meitei are moved
// into Devanagari, which IndicTrans2's vocabulary is built on, and back afterwards.
//
// The Moses and Indic NLP steps are reimplemented here for the cases that matter to
// translation quality, not character for character.
package indictrans

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Language is one of IndicTrans2's languages.
type Language struct {
	Code   string // FLORES-200 code, such as hin_Deva
	Short  string // ISO 639 code callers use, such as hi
	Name   string
	script string // the script part of Code
	iso    string // Indic NLP's code, which picks the script block
}

// Languages are those IndicTrans2 translates to and from English.
var Languages = []Language{
	{Code: "asm_Beng", Short: "as", Name: "Assamese"},
	{Code: "ben_Beng", Short: "bn", Name: "Bengali"},
	{Code: "brx_Deva", Short: "brx", Name: "Bodo"},
	{Code: "doi_Deva", Short: "doi", Name: "Dogri"},
	{Code: "eng_Latn", Short: "en", Name: "English"},
	{Code: "gom_Deva", Short: "gom", Name: "Konkani"},
	{Code: "guj_Gujr", Short: "gu", Name: "Gujarati"},
	{Code: "hin_Deva", Short: "hi", Name: "Hindi"},
	{Code: "kan_Knda", Short: "kn", Name: "Kannada"},
	{Code: "kas_Arab", Short: "ks", Name: "Kashmiri (Perso-Arabic)"},
	{Code: "kas_Deva", Short: "ks-Deva", Name: "Kashmiri (Devanagari)"},
	{Code: "mai_Deva", Short: "mai", Name: "Maithili"},
	{Code: "mal_Mlym", Short: "ml", Name: "Malayalam"},
	{Code: "mar_Deva", Short: "mr", Name: "Marathi"},
	{Code: "mni_Beng", Short: "mni", Name: "Manipuri (Bengali script)"},
	{Code: "mni_Mtei", Short: "mni-Mtei", Name: "Manipuri (Meitei Mayek)"},
	{Code: "npi_Deva", Short: "ne", Name: "Nepali"},
	{Code: "ory_Orya", Short: "or", Name: "Odia"},
	{Code: "pan_Guru", Short: "pa", Name: "Punjabi"},
	{Code: "san_Deva", Short: "sa", Name: "Sanskrit"},
	{Code: "sat_Olck", Short: "sat", Name: "Santali (Ol Chiki)"},
	{Code: "snd_Arab", Short: "sd", Name: "Sindhi (Perso-Arabic)"},
	{Code: "snd_Deva", Short: "sd-Deva", Name: "Sindhi (Devanagari)"},
	{Code: "tam_Taml", Short: "ta", Name: "Tamil"},
	{Code: "tel_Telu", Short: "te", Name: "Telugu"},
	{Code: "urd_Arab", Short: "ur", Name: "Urdu"},
}

var floresToISO = map[string]string{
	"asm_Beng": "as", "ben_Beng": "bn", "brx_Deva": "hi", "doi_Deva": "hi", "eng_Latn": "en", "gom_Deva": "kK",
	"guj_Gujr": "gu", "hin_Deva": "hi", "kan_Knda": "kn", "kas_Arab": "ur", "kas_Deva": "hi", "mai_Deva": "hi",
	"mal_Mlym": "ml", "mar_Deva": "mr", "mni_Beng": "bn", "mni_Mtei": "hi", "npi_Deva": "ne", "ory_Orya": "or",
	"pan_Guru": "pa", "san_Deva": "hi", "sat_Olck": "or", "snd_Arab": "ur", "snd_Deva": "hi", "tam_Taml": "ta",
	"tel_Telu": "te", "urd_Arab": "ur",
}

func init() {
	for i := range Languages {
		l := &Languages[i]
		l.script = l.Code[strings.Index(l.Code, "_")+1:]
		l.iso = floresToISO[l.Code]
	}
}

// Find returns a language by its short or FLORES code.
func Find(code string) (Language, bool) {
	for _, l := range Languages {
		if strings.EqualFold(l.Short, code) || strings.EqualFold(l.Code, code) {
			return l, true
		}
	}
	return Language{}, false
}

// English reports whether the language is English.
func (l Language) English() bool { return l.Code == "eng_Latn" }

// The first code point of each Brahmic block Indic NLP moves text between.
var blockStart = map[string]rune{
	"hi": 0x0900, "mr": 0x0900, "ne": 0x0900, "kK": 0x0900, "sa": 0x0900,
	"bn": 0x0980, "as": 0x0980, "pa": 0x0A00, "gu": 0x0A80, "or": 0x0B00,
	"ta": 0x0B80, "te": 0x0C00, "kn": 0x0C80, "ml": 0x0D00,
}

// moveScript shifts every character of one Brahmic block to the same place in another,
// as Indic NLP's UnicodeIndicTransliterator does.
func moveScript(text, from, to string) string {
	a, okA := blockStart[from]
	b, okB := blockStart[to]
	if !okA || !okB || a == b {
		return text
	}
	return strings.Map(func(r rune) rune {
		offset := r - a
		if offset >= 0 && offset < 0x80 && r != 0x0964 && r != 0x0965 {
			moved := b + offset
			if unicode.Is(unicode.Mn, moved) || unicode.Is(unicode.Mc, moved) || unicode.IsLetter(moved) || unicode.IsDigit(moved) {
				return moved
			}
		}
		return r
	}, text)
}

// Placeholders swaps entities the model should copy through for <ID1>, <ID2>, ...
type Placeholders map[int]string

var (
	emailPattern   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	urlPattern     = regexp.MustCompile(`(?:(?:https?|ftp)://)?(?:[\w-]+\.)+[A-Za-z]{2,}(?:/[\w/\-?#&=%.]*)?`)
	numeralPattern = regexp.MustCompile(`~?\d+\.?\d*\s?%?\s?-?\s?~?\d+\.?\d*\s?%|~?\d+%|\d+[-/.,:']\d+[-/.,:'+]\d+(?:\.\d+)?|\d+[-/.:'+]\d+(?:\.\d+)?`)
	otherPattern   = regexp.MustCompile(`[A-Za-z0-9]*[#@]\w+`)
	restorePattern = regexp.MustCompile(`[<\[]\s*(?i:id)\s*(\d+)\s*[>\]]`)
)

func (p Placeholders) wrap(text string) string {
	serial := 1
	for _, pattern := range []*regexp.Regexp{emailPattern, urlPattern, numeralPattern, otherPattern} {
		seen := map[string]bool{}
		var found []string
		for _, match := range pattern.FindAllString(text, -1) {
			if !seen[match] {
				seen[match] = true
				found = append(found, match)
			}
		}
		// Longer first, so one match inside another is not replaced half way.
		sort.SliceStable(found, func(a, b int) bool { return len(found[a]) > len(found[b]) })
		for _, match := range found {
			switch pattern {
			case urlPattern:
				if len(strings.ReplaceAll(match, ".", "")) < 4 || !strings.Contains(text, match) {
					continue
				}
			case numeralPattern:
				if len(strings.NewReplacer(" ", "", ".", "", ":", "").Replace(match)) < 4 {
					continue
				}
			}
			if !strings.Contains(text, match) {
				continue
			}
			p[serial] = match
			text = strings.ReplaceAll(text, match, fmt.Sprintf("<ID%d>", serial))
			serial++
		}
	}
	return strings.Join(strings.Fields(text), " ")
}

func (p Placeholders) restore(text string) string {
	return restorePattern.ReplaceAllStringFunc(text, func(m string) string {
		var n int
		fmt.Sscanf(restorePattern.FindStringSubmatch(m)[1], "%d", &n)
		if value, ok := p[n]; ok {
			return value
		}
		return m
	})
}

var puncSteps = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`\r`), ""},
	{regexp.MustCompile(`\(\s*`), "("},
	{regexp.MustCompile(`\s*\)`), ")"},
	{regexp.MustCompile(`\s:\s?`), ":"},
	{regexp.MustCompile(`\s;\s?`), ";"},
	{regexp.MustCompile("[`´‘‚’]"), "'"},
	{regexp.MustCompile(`[„“”«»]`), `"`},
	{regexp.MustCompile(`[–—]`), "-"},
	{regexp.MustCompile(` %`), "%"},
	{regexp.MustCompile(` ([?!;])`), "$1"},
	{regexp.MustCompile(`[ ]{2,}`), " "},
	{regexp.MustCompile(`\) ([.!:?;,])`), ")$1"},
	{regexp.MustCompile(`(\d) %`), "$1%"},
	{regexp.MustCompile(`"([,.]+)`), `$1"`},
	{regexp.MustCompile(`(\d) (\d)`), "$1.$2"},
}

func normalisePunctuation(text string) string {
	for _, step := range puncSteps {
		text = step.pattern.ReplaceAllString(text, step.with)
	}
	return strings.TrimSpace(text)
}

// asciiDigits turns the digits of every Indian script into 0 to 9.
func asciiDigits(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) && r > 127 {
			if d := digitValue(r); d >= 0 {
				return '0' + rune(d)
			}
		}
		return r
	}, text)
}

func digitValue(r rune) int {
	for _, zero := range []rune{0x0966, 0x09E6, 0x0A66, 0x0AE6, 0x0B66, 0x0BE6, 0x0C66, 0x0CE6, 0x0D66, 0x0660, 0x06F0, 0xABF0, 0x1C50} {
		if r >= zero && r <= zero+9 {
			return int(r - zero)
		}
	}
	return -1
}

// Prepare turns one sentence into the text the model reads, with its language tags, and
// returns the placeholders to put back afterwards.
func Prepare(sentence string, from, to Language) (string, Placeholders) {
	p := Placeholders{}
	text := normalisePunctuation(sentence)
	text = asciiDigits(text)
	text = p.wrap(text)
	if from.English() {
		text = mosesTokenize(mosesNormalise(text))
	} else {
		text = strings.Join(indicTokenize(indicNormalise(text)), " ")
		switch from.script {
		case "Arab", "Aran", "Olck", "Mtei", "Latn":
		default:
			text = moveScript(text, from.iso, "hi")
			text = strings.ReplaceAll(text, " ् ", "्")
		}
	}
	return from.Code + " " + to.Code + " " + strings.TrimSpace(text), p
}

// Finish turns the model's output for one sentence into text in the target language.
func Finish(output string, to Language, p Placeholders) string {
	text := output
	switch to.script {
	case "Arab", "Aran":
		text = strings.NewReplacer(" ؟", "؟", " ۔", "۔", " ،", "،", "ٮ۪", "ؠ").Replace(text)
	}
	if to.Code == "ory_Orya" {
		text = strings.ReplaceAll(text, "ଯ଼", "ୟ")
	}
	text = p.restore(text)
	if to.English() {
		return mosesDetokenize(text)
	}
	switch to.script {
	case "Arab", "Aran", "Olck", "Mtei", "Latn":
	default:
		text = moveScript(text, "hi", to.iso)
	}
	return indicDetokenize(text)
}

// SplitSentences cuts text into sentences at ., ?, !, the danda and line breaks.
func SplitSentences(text string) []string {
	var out []string
	var current strings.Builder
	runes := []rune(text)
	flush := func() {
		if s := strings.TrimSpace(current.String()); s != "" {
			out = append(out, s)
		}
		current.Reset()
	}
	for i, r := range runes {
		if r == '\n' {
			flush()
			continue
		}
		current.WriteRune(r)
		end := r == '?' || r == '!' || r == '।' || r == '॥' || r == '؟' || r == '۔'
		if r == '.' {
			// Not inside a number, and not after a lone capital (an initial: "A. P. J.").
			next := rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			prevInitial := i >= 1 && unicode.IsUpper(runes[i-1]) && (i == 1 || !unicode.IsLetter(runes[i-2]))
			start := i
			for start > 0 && unicode.IsLetter(runes[start-1]) {
				start--
			}
			abbreviation := mosesNonBreaking[string(runes[start:i])]
			end = (next == 0 || unicode.IsSpace(next)) && !prevInitial && !abbreviation
		}
		if end && (i+1 == len(runes) || unicode.IsSpace(runes[i+1]) || runes[i+1] == '"') {
			flush()
		}
	}
	flush()
	return out
}

// Indic NLP ---------------------------------------------------------------------------

var zeroWidth = strings.NewReplacer("\u200b", "", "\u200c", "", "\u200d", "", "\ufeff", "")

// indicNormalise is the core of Indic NLP's normaliser: canonical composition and no
// zero-width characters, and the danda written as such rather than as a pipe.
func indicNormalise(text string) string {
	text = norm.NFC.String(text)
	text = zeroWidth.Replace(text)
	return strings.NewReplacer(" | ", " । ", "||", "॥").Replace(text)
}

// indicTokenize splits punctuation off words, as Indic NLP's trivial tokenizer does.
func indicTokenize(text string) []string {
	var out []string
	for _, word := range strings.Fields(text) {
		var current []rune
		for _, r := range word {
			if isTokenPunct(r) {
				if len(current) > 0 {
					out = append(out, string(current))
					current = current[:0]
				}
				out = append(out, string(r))
				continue
			}
			current = append(current, r)
		}
		if len(current) > 0 {
			out = append(out, string(current))
		}
	}
	return out
}

func isTokenPunct(r rune) bool {
	if r == '।' || r == '॥' {
		return true
	}
	return (unicode.IsPunct(r) || unicode.IsSymbol(r)) && r != '\'' && r != '-' && r != '<' && r != '>'
}

// indicDetokenize removes the spaces tokenizing put before punctuation.
func indicDetokenize(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	for _, mark := range []string{",", ".", "!", "?", ";", ":", "।", "॥", ")", "%", "؟", "۔", "،"} {
		text = strings.ReplaceAll(text, " "+mark, mark)
	}
	text = strings.ReplaceAll(text, "( ", "(")
	return strings.TrimSpace(text)
}

// Moses -------------------------------------------------------------------------------

var mosesNormSteps = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`\s+`), " "},
	{regexp.MustCompile(`\( ?`), "("},
	{regexp.MustCompile(` ?\)`), ")"},
	{regexp.MustCompile(`([a-z])"`), `$1"`},
	{regexp.MustCompile(`''`), `"`},
	{regexp.MustCompile(`([^ ])"(?: |$)`), `$1" `},
}

func mosesNormalise(text string) string {
	for _, step := range mosesNormSteps {
		text = step.pattern.ReplaceAllString(text, step.with)
	}
	return strings.TrimSpace(text)
}

var (
	mosesSplit       = regexp.MustCompile(`([^\p{L}\p{N}\s.,'\-/<>@#&%])`)
	mosesCommaBefore = regexp.MustCompile(`([^\p{N}]),`)
	mosesCommaAfter  = regexp.MustCompile(`,([^\p{N}])`)
	mosesSlash       = regexp.MustCompile(`(\D)/(\D)`)
	mosesDash        = regexp.MustCompile(` - |^- | -$`)
	mosesApostrophes = []struct {
		pattern *regexp.Regexp
		with    string
	}{
		{regexp.MustCompile(`([^\p{L}])'([^\p{L}])`), "$1 ' $2"},
		{regexp.MustCompile(`([^\p{L}\p{N}])'(\p{L})`), "$1 ' $2"},
		{regexp.MustCompile(`(\p{L})'([^\p{L}])`), "$1 ' $2"},
		{regexp.MustCompile(`(\p{L})'(\p{L})`), "$1 '$2"},
		{regexp.MustCompile(`(\p{N})'(s)`), "$1 '$2"},
	}
	mosesNonBreaking = map[string]bool{"Mr": true, "Mrs": true, "Ms": true, "Dr": true, "Prof": true, "Sr": true, "Jr": true,
		"St": true, "vs": true, "etc": true, "e.g": true, "i.e": true, "No": true, "Rs": true, "Govt": true, "Ltd": true, "Co": true, "Inc": true}
)

// mosesTokenize splits English into tokens much as the Moses tokenizer does (no escaping).
func mosesTokenize(text string) string {
	text = " " + text + " "
	text = mosesSplit.ReplaceAllString(text, " $1 ")
	// A comma is split off unless it sits between digits, as in 1,00,000.
	text = mosesCommaBefore.ReplaceAllString(text, "$1 , ")
	text = mosesCommaAfter.ReplaceAllString(text, " , $1")
	text = mosesSlash.ReplaceAllString(text, "$1 / $2")
	for _, step := range mosesApostrophes {
		text = step.pattern.ReplaceAllString(text, step.with)
	}
	words := strings.Fields(text)
	for i, word := range words {
		// A full stop ends a word unless the word is an abbreviation or a placeholder.
		if strings.HasSuffix(word, ".") && len(word) > 1 {
			stem := strings.TrimSuffix(word, ".")
			last := i == len(words)-1
			keep := !last && (mosesNonBreaking[stem] || strings.Contains(stem, ".") || (len([]rune(stem)) == 1 && unicode.IsUpper([]rune(stem)[0])))
			if !keep {
				words[i] = stem + " ."
			}
		}
	}
	return strings.Join(strings.Fields(strings.Join(words, " ")), " ")
}

// mosesDetokenize joins English tokens back into text.
func mosesDetokenize(text string) string {
	tokens := strings.Fields(text)
	var b strings.Builder
	quoteOpen := false
	for i, tok := range tokens {
		attachLeft := false
		switch {
		case tok == "," || tok == "." || tok == "!" || tok == "?" || tok == ";" || tok == ":" || tok == ")" || tok == "]" || tok == "}" || tok == "%":
			attachLeft = true
		case strings.HasPrefix(tok, "'") && len(tok) > 1:
			attachLeft = true // 's 're n't
		case tok == "n't":
			attachLeft = true
		case tok == `"`:
			attachLeft = quoteOpen
			quoteOpen = !quoteOpen
		}
		if i > 0 && !attachLeft {
			prev := tokens[i-1]
			if prev == "(" || prev == "[" || prev == "{" || prev == "$" || prev == "₹" || (prev == `"` && quoteOpen && tok != `"`) {
				attachLeft = true
			}
		}
		if i > 0 && !attachLeft {
			b.WriteByte(' ')
		}
		b.WriteString(tok)
	}
	return strings.TrimSpace(b.String())
}
