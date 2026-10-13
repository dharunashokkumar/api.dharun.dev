package words

import "testing"

func TestEnglish(t *testing.T) {
	cases := map[int64]string{
		0:               "zero",
		7:               "seven",
		19:              "nineteen",
		40:              "forty",
		99:              "ninety-nine",
		100:             "one hundred",
		101:             "one hundred one",
		1000:            "one thousand",
		1100:            "one thousand one hundred",
		99999:           "ninety-nine thousand nine hundred ninety-nine",
		100000:          "one lakh",
		123456:          "one lakh twenty-three thousand four hundred fifty-six",
		10000000:        "one crore",
		12345678:        "one crore twenty-three lakh forty-five thousand six hundred seventy-eight",
		1000000000:      "one hundred crore",
		1000000000000:   "one lakh crore",
		100000000000000: "one crore crore",
	}
	for n, want := range cases {
		got, err := Number("en", false, n, "")
		if err != nil || got != want {
			t.Errorf("en %d = %q (%v), want %q", n, got, err, want)
		}
	}
}

func TestHindi(t *testing.T) {
	cases := map[int64]string{
		0:        "शून्य",
		5:        "पाँच",
		21:       "इक्कीस",
		79:       "उनासी",
		99:       "निन्यानवे",
		100:      "एक सौ",
		150:      "एक सौ पचास",
		1000:     "एक हज़ार",
		100000:   "एक लाख",
		250000:   "दो लाख पचास हज़ार",
		12345678: "एक करोड़ तेईस लाख पैंतालीस हज़ार छह सौ अठहत्तर",
	}
	for n, want := range cases {
		got, err := Number("hi", false, n, "")
		if err != nil || got != want {
			t.Errorf("hi %d = %q (%v), want %q", n, got, err, want)
		}
	}
	if len(hiBelowHundred) != 100 {
		t.Fatalf("Hindi table has %d entries, want 100", len(hiBelowHundred))
	}
	seen := map[string]int{}
	for i, word := range hiBelowHundred[1:] {
		if previous, dup := seen[word]; dup {
			t.Errorf("Hindi word %q used for both %d and %d", word, previous, i+1)
		}
		seen[word] = i + 1
	}
}

func TestTamil(t *testing.T) {
	cases := map[int64]string{
		0:        "பூஜ்ஜியம்",
		1:        "ஒன்று",
		10:       "பத்து",
		11:       "பதினொன்று",
		19:       "பத்தொன்பது",
		20:       "இருபது",
		21:       "இருபத்து ஒன்று",
		90:       "தொண்ணூறு",
		99:       "தொண்ணூற்று ஒன்பது",
		100:      "நூறு",
		105:      "நூற்று ஐந்து",
		900:      "தொள்ளாயிரம்",
		999:      "தொள்ளாயிரத்து தொண்ணூற்று ஒன்பது",
		1000:     "ஆயிரம்",
		1100:     "ஆயிரத்து நூறு",
		2000:     "இரண்டாயிரம்",
		2500:     "இரண்டாயிரத்து ஐநூறு",
		10000:    "பத்தாயிரம்",
		25000:    "இருபத்து ஐந்து ஆயிரம்",
		25001:    "இருபத்து ஐந்து ஆயிரத்து ஒன்று",
		100000:   "ஒரு லட்சம்",
		150000:   "ஒரு லட்சத்து ஐம்பது ஆயிரம்",
		200000:   "இரண்டு லட்சம்",
		10000000: "ஒரு கோடி",
		12500000: "ஒரு கோடியே இருபத்து ஐந்து லட்சம்",
	}
	for n, want := range cases {
		got, err := Number("ta", false, n, "")
		if err != nil || got != want {
			t.Errorf("ta %d = %q (%v), want %q", n, got, err, want)
		}
	}
}

func TestDecimalsAndSign(t *testing.T) {
	if got, _ := Number("en", false, 12, "34"); got != "twelve point three four" {
		t.Errorf("got %q", got)
	}
	if got, _ := Number("en", true, 5, ""); got != "minus five" {
		t.Errorf("got %q", got)
	}
	if got, _ := Number("en", true, 0, "00"); got != "zero point zero zero" {
		t.Errorf("negative zero should not say minus, got %q", got)
	}
	if got, _ := Number("hi", false, 3, "5"); got != "तीन दशमलव पाँच" {
		t.Errorf("got %q", got)
	}
}

func TestCurrency(t *testing.T) {
	cases := []struct {
		lang   string
		rupees int64
		paise  int
		want   string
	}{
		{"en", 100000, 0, "one lakh rupees only"},
		{"en", 1, 0, "one rupee only"},
		{"en", 1250, 50, "one thousand two hundred fifty rupees and fifty paise only"},
		{"en", 0, 1, "one paisa only"},
		{"en", 0, 0, "zero rupees only"},
		{"hi", 1500, 0, "एक हज़ार पाँच सौ रुपये मात्र"},
		{"hi", 1, 0, "एक रुपया मात्र"},
		{"ta", 1500, 0, "ஆயிரத்து ஐநூறு ரூபாய் மட்டும்"},
		{"ta", 1, 0, "ஒரு ரூபாய் மட்டும்"},
	}
	for _, c := range cases {
		got, err := Currency(c.lang, c.rupees, c.paise)
		if err != nil || got != c.want {
			t.Errorf("%s %d.%02d = %q (%v), want %q", c.lang, c.rupees, c.paise, got, err, c.want)
		}
	}
}

func TestLimits(t *testing.T) {
	if _, err := Number("en", false, MaxValue, ""); err != nil {
		t.Errorf("MaxValue should be supported: %v", err)
	}
	if _, err := Number("en", false, MaxValue+1, ""); err != ErrOutOfRange {
		t.Errorf("want ErrOutOfRange, got %v", err)
	}
	if _, err := Number("fr", false, 1, ""); err != ErrUnsupportedLanguage {
		t.Errorf("want ErrUnsupportedLanguage, got %v", err)
	}
	if _, err := Currency("en", 1, 100); err != ErrOutOfRange {
		t.Errorf("want ErrOutOfRange for 100 paise, got %v", err)
	}
	// Every value up to ten lakh must spell without an empty word slipping in.
	for _, lang := range Languages() {
		for n := int64(1); n <= 1_000_000; n += 7 {
			got, _ := Number(lang, false, n, "")
			if got == "" || got[0] == ' ' || got[len(got)-1] == ' ' || containsDoubleSpace(got) {
				t.Fatalf("%s %d spelled badly: %q", lang, n, got)
			}
		}
	}
}

func containsDoubleSpace(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] == ' ' && s[i-1] == ' ' {
			return true
		}
	}
	return false
}
