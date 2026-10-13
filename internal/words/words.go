// Package words spells out numbers the Indian way: thousand, lakh, crore.
// 1,23,45,678 is "one crore twenty-three lakh forty-five thousand six hundred seventy-eight".
package words

import (
	"errors"
	"strings"
)

// MaxValue is the largest whole number the package spells out (99,999 crore crore is
// far beyond any practical amount; this keeps arithmetic inside int64).
const MaxValue = 999_999_999_999_999

// ErrUnsupportedLanguage is returned for a language code other than en, hi or ta.
var ErrUnsupportedLanguage = errors.New("unsupported language")

// ErrOutOfRange is returned for negative numbers below -MaxValue or above MaxValue.
var ErrOutOfRange = errors.New("number out of range")

type language struct {
	zero, minus, point, and string
	rupee, rupees           string
	paisa, paise            string
	only                    string
	digits                  [10]string
	whole                   func(n int64) string
}

var languages = map[string]language{
	"en": {
		zero: "zero", minus: "minus", point: "point", and: "and",
		rupee: "rupee", rupees: "rupees", paisa: "paisa", paise: "paise", only: "only",
		digits: [10]string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"},
		whole:  english,
	},
	"hi": {
		zero: "शून्य", minus: "ऋण", point: "दशमलव", and: "और",
		rupee: "रुपया", rupees: "रुपये", paisa: "पैसा", paise: "पैसे", only: "मात्र",
		digits: [10]string{"शून्य", "एक", "दो", "तीन", "चार", "पाँच", "छह", "सात", "आठ", "नौ"},
		whole:  hindi,
	},
	"ta": {
		zero: "பூஜ்ஜியம்", minus: "கழித்தல்", point: "புள்ளி", and: "மற்றும்",
		rupee: "ரூபாய்", rupees: "ரூபாய்", paisa: "பைசா", paise: "பைசா", only: "மட்டும்",
		digits: [10]string{"பூஜ்ஜியம்", "ஒன்று", "இரண்டு", "மூன்று", "நான்கு", "ஐந்து", "ஆறு", "ஏழு", "எட்டு", "ஒன்பது"},
		whole:  tamil,
	},
}

// Languages lists the supported language codes.
func Languages() []string { return []string{"en", "hi", "ta"} }

// Number spells a whole number plus optional decimal digits ("12", "34" gives
// "twelve point three four"). fraction holds the digits after the decimal point.
func Number(lang string, negative bool, whole int64, fraction string) (string, error) {
	l, ok := languages[lang]
	if !ok {
		return "", ErrUnsupportedLanguage
	}
	if whole < 0 || whole > MaxValue {
		return "", ErrOutOfRange
	}
	parts := []string{}
	if negative && (whole != 0 || strings.Trim(fraction, "0") != "") {
		parts = append(parts, l.minus)
	}
	parts = append(parts, spell(l, whole))
	if fraction != "" {
		parts = append(parts, l.point)
		for _, digit := range fraction {
			parts = append(parts, l.digits[digit-'0'])
		}
	}
	return strings.Join(parts, " "), nil
}

// Currency spells an amount in rupees and paise, the way it is written on a cheque:
// "one lakh rupees and fifty paise only".
func Currency(lang string, rupees int64, paise int) (string, error) {
	l, ok := languages[lang]
	if !ok {
		return "", ErrUnsupportedLanguage
	}
	if rupees < 0 || rupees > MaxValue || paise < 0 || paise > 99 {
		return "", ErrOutOfRange
	}

	unit := func(n int64, singular, plural string) string {
		if n == 1 {
			return singular
		}
		return plural
	}

	var parts []string
	if rupees > 0 || paise == 0 {
		parts = append(parts, currencyCount(lang, l, rupees), unit(rupees, l.rupee, l.rupees))
	}
	if paise > 0 {
		if len(parts) > 0 {
			parts = append(parts, l.and)
		}
		parts = append(parts, currencyCount(lang, l, int64(paise)), unit(int64(paise), l.paisa, l.paise))
	}
	parts = append(parts, l.only)
	return strings.Join(parts, " "), nil
}

// Before a counted noun Tamil uses ஒரு rather than ஒன்று: "ஒரு ரூபாய்".
func currencyCount(lang string, l language, n int64) string {
	if lang == "ta" && n == 1 {
		return "ஒரு"
	}
	return spell(l, n)
}

func spell(l language, n int64) string {
	if n == 0 {
		return l.zero
	}
	return l.whole(n)
}

// ---- English ----

var (
	enOnes = []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
		"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	enTens = []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
)

func enBelowHundred(n int64) string {
	if n < 20 {
		return enOnes[n]
	}
	if n%10 == 0 {
		return enTens[n/10]
	}
	return enTens[n/10] + "-" + enOnes[n%10]
}

func english(n int64) string {
	var parts []string
	if crore := n / 10_000_000; crore > 0 {
		// Above 99 crore the count of crores is itself spelled Indian-style: "one lakh crore".
		parts = append(parts, english(crore), "crore")
		n %= 10_000_000
	}
	if lakh := n / 100_000; lakh > 0 {
		parts = append(parts, enBelowHundred(lakh), "lakh")
		n %= 100_000
	}
	if thousand := n / 1000; thousand > 0 {
		parts = append(parts, enBelowHundred(thousand), "thousand")
		n %= 1000
	}
	if hundred := n / 100; hundred > 0 {
		parts = append(parts, enOnes[hundred], "hundred")
		n %= 100
	}
	if n > 0 {
		parts = append(parts, enBelowHundred(n))
	}
	return strings.Join(parts, " ")
}

// ---- Hindi ----

// Hindi has a distinct word for every number up to 99.
var hiBelowHundred = []string{"",
	"एक", "दो", "तीन", "चार", "पाँच", "छह", "सात", "आठ", "नौ", "दस",
	"ग्यारह", "बारह", "तेरह", "चौदह", "पंद्रह", "सोलह", "सत्रह", "अठारह", "उन्नीस", "बीस",
	"इक्कीस", "बाईस", "तेईस", "चौबीस", "पच्चीस", "छब्बीस", "सत्ताईस", "अट्ठाईस", "उनतीस", "तीस",
	"इकतीस", "बत्तीस", "तैंतीस", "चौंतीस", "पैंतीस", "छत्तीस", "सैंतीस", "अड़तीस", "उनतालीस", "चालीस",
	"इकतालीस", "बयालीस", "तैंतालीस", "चौवालीस", "पैंतालीस", "छियालीस", "सैंतालीस", "अड़तालीस", "उनचास", "पचास",
	"इक्यावन", "बावन", "तिरेपन", "चौवन", "पचपन", "छप्पन", "सत्तावन", "अट्ठावन", "उनसठ", "साठ",
	"इकसठ", "बासठ", "तिरेसठ", "चौंसठ", "पैंसठ", "छियासठ", "सड़सठ", "अड़सठ", "उनहत्तर", "सत्तर",
	"इकहत्तर", "बहत्तर", "तिहत्तर", "चौहत्तर", "पचहत्तर", "छिहत्तर", "सतहत्तर", "अठहत्तर", "उनासी", "अस्सी",
	"इक्यासी", "बयासी", "तिरासी", "चौरासी", "पचासी", "छियासी", "सत्तासी", "अट्ठासी", "नवासी", "नब्बे",
	"इक्यानवे", "बानवे", "तिरानवे", "चौरानवे", "पंचानवे", "छियानवे", "सत्तानवे", "अट्ठानवे", "निन्यानवे",
}

func hindi(n int64) string {
	var parts []string
	if crore := n / 10_000_000; crore > 0 {
		parts = append(parts, hindi(crore), "करोड़")
		n %= 10_000_000
	}
	if lakh := n / 100_000; lakh > 0 {
		parts = append(parts, hiBelowHundred[lakh], "लाख")
		n %= 100_000
	}
	if thousand := n / 1000; thousand > 0 {
		parts = append(parts, hiBelowHundred[thousand], "हज़ार")
		n %= 1000
	}
	if hundred := n / 100; hundred > 0 {
		parts = append(parts, hiBelowHundred[hundred], "सौ")
		n %= 100
	}
	if n > 0 {
		parts = append(parts, hiBelowHundred[n])
	}
	return strings.Join(parts, " ")
}

// ---- Tamil ----
//
// Tamil changes the form of a number word when more follows it: நூறு (100) but
// நூற்று ஐந்து (105); ஆயிரம் (1000) but ஆயிரத்து நூறு (1100). Each table below has the
// standalone form and the joining form.

var (
	taOnes  = []string{"", "ஒன்று", "இரண்டு", "மூன்று", "நான்கு", "ஐந்து", "ஆறு", "ஏழு", "எட்டு", "ஒன்பது"}
	taTeens = []string{"பத்து", "பதினொன்று", "பன்னிரண்டு", "பதின்மூன்று", "பதினான்கு", "பதினைந்து", "பதினாறு", "பதினேழு", "பதினெட்டு", "பத்தொன்பது"}

	taTens     = []string{"", "", "இருபது", "முப்பது", "நாற்பது", "ஐம்பது", "அறுபது", "எழுபது", "எண்பது", "தொண்ணூறு"}
	taTensJoin = []string{"", "", "இருபத்து", "முப்பத்து", "நாற்பத்து", "ஐம்பத்து", "அறுபத்து", "எழுபத்து", "எண்பத்து", "தொண்ணூற்று"}

	taHundreds     = []string{"", "நூறு", "இருநூறு", "முந்நூறு", "நானூறு", "ஐநூறு", "அறுநூறு", "எழுநூறு", "எண்ணூறு", "தொள்ளாயிரம்"}
	taHundredsJoin = []string{"", "நூற்று", "இருநூற்று", "முந்நூற்று", "நானூற்று", "ஐநூற்று", "அறுநூற்று", "எழுநூற்று", "எண்ணூற்று", "தொள்ளாயிரத்து"}

	// One to ten thousand are single fused words.
	taThousands = []string{"", "ஆயிரம்", "இரண்டாயிரம்", "மூவாயிரம்", "நான்காயிரம்", "ஐயாயிரம்", "ஆறாயிரம்", "ஏழாயிரம்", "எட்டாயிரம்", "ஒன்பதாயிரம்", "பத்தாயிரம்"}
)

func taBelowHundred(n int64) string {
	switch {
	case n < 10:
		return taOnes[n]
	case n < 20:
		return taTeens[n-10]
	case n%10 == 0:
		return taTens[n/10]
	default:
		return taTensJoin[n/10] + " " + taOnes[n%10]
	}
}

// taCount is the number as used before லட்சம் or கோடி, where one is ஒரு.
func taCount(n int64) string {
	if n == 1 {
		return "ஒரு"
	}
	return tamil(n)
}

func tamil(n int64) string {
	var parts []string

	if crore := n / 10_000_000; crore > 0 {
		n %= 10_000_000
		unit := "கோடி"
		if n > 0 {
			unit = "கோடியே"
		}
		parts = append(parts, taCount(crore), unit)
	}
	if lakh := n / 100_000; lakh > 0 {
		n %= 100_000
		unit := "லட்சம்"
		if n > 0 {
			unit = "லட்சத்து"
		}
		parts = append(parts, taCount(lakh), unit)
	}
	if thousand := n / 1000; thousand > 0 {
		n %= 1000
		var word string
		if thousand <= 10 {
			word = taThousands[thousand]
		} else {
			word = taBelowHundred(thousand) + " ஆயிரம்"
		}
		if n > 0 {
			word = strings.TrimSuffix(word, "ம்") + "த்து"
		}
		parts = append(parts, word)
	}
	if hundred := n / 100; hundred > 0 {
		n %= 100
		if n > 0 {
			parts = append(parts, taHundredsJoin[hundred])
		} else {
			parts = append(parts, taHundreds[hundred])
		}
	}
	if n > 0 {
		parts = append(parts, taBelowHundred(n))
	}
	return strings.Join(parts, " ")
}
