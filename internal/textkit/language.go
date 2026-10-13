package textkit

import (
	"math"
	"strings"
	"unicode"

	"github.com/abadojack/whatlanggo"
)

// Language is what Detect found.
type Language struct {
	Code       string  `json:"code"`   // ISO 639-3, such as tam
	Code2      string  `json:"code2"`  // ISO 639-1 where one exists, such as ta
	Name       string  `json:"name"`   // English name
	Script     string  `json:"script"` // Unicode script name, such as Tamil
	Confidence float64 `json:"confidence"`
	Reliable   bool    `json:"reliable"`
	// Romanised is set when Indian-language text appears to be written in Latin letters
	// (Tanglish, Hinglish); trigram models cannot tell which language that is.
	Romanised bool `json:"romanised,omitempty"`
}

// Detect names the language of a text from its script and letter trigrams. Short texts
// and Romanised Indian languages are hard; the confidence says how sure it is.
func Detect(text string) (Language, bool) {
	letters := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if letters == 0 {
		return Language{}, false
	}
	info := whatlanggo.Detect(text)
	language := Language{
		Code: info.Lang.Iso6393(), Code2: info.Lang.Iso6391(), Name: info.Lang.String(),
		Confidence: math.Round(info.Confidence*100) / 100, Reliable: info.IsReliable(),
	}
	if info.Script != nil {
		language.Script = whatlanggo.Scripts[info.Script]
	}
	if language.Code == "" {
		return Language{}, false
	}
	if language.Script == "Latin" && looksRomanisedIndian(text) {
		language.Romanised, language.Reliable = true, false
	}
	return language, true
}

// Common words of Romanised Hindi and Tamil that are rare in English.
var romanisedWords = setOfWords("hai", "nahi", "nahin", "kya", "kyun", "aap", "tum", "mein", "hum", "acha", "accha", "theek",
	"bhai", "yaar", "kaise", "kaisa", "kar", "karo", "raha", "rahi", "hoga", "vanakkam", "enna", "epdi", "eppadi", "illa",
	"irukku", "romba", "nalla", "seri", "sapdu", "pannu", "panna", "da", "machan", "inga", "anga", "unga", "naan", "nee")

func setOfWords(words ...string) map[string]bool {
	set := map[string]bool{}
	for _, word := range words {
		set[word] = true
	}
	return set
}

func looksRomanisedIndian(text string) bool {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) })
	hits := 0
	for _, word := range words {
		if romanisedWords[word] {
			hits++
		}
	}
	return hits >= 2 || (len(words) > 0 && hits*4 >= len(words))
}
