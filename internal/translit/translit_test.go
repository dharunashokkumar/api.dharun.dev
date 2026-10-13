package translit

import "testing"

func script(t *testing.T, name string) Script {
	t.Helper()
	s, ok := Lookup(name)
	if !ok {
		t.Fatalf("no script %q", name)
	}
	return s
}

func TestRomanise(t *testing.T) {
	cases := []struct{ text, scheme, want string }{
		{"नमस्ते", "iso15919", "namastē"},
		{"नमस्ते", "iast", "namaste"},
		{"नमस्ते", "ascii", "namaste"},
		{"वணக்கம்", "iso15919", "vaṇakkam"},
		{"வணக்கம்", "ascii", "vanakkam"},
		{"தமிழ்", "iso15919", "tamiḻ"},
		{"தமிழ்", "ascii", "tamizh"},
		{"भारत", "ascii", "bhaarat"},
		{"भारत", "iast", "bhārata"},
		{"संस्कृतम्", "iast", "saṃskṛtam"},
		{"संस्कृतम्", "iso15919", "saṁskr̥tam"},
		{"ज़िंदगी", "iso15919", "ziṁdagī"},
		{"ज़िंदगी", "ascii", "zindagii"},
		{"गंगा", "ascii", "gangaa"},
		{"संपूर्ण", "ascii", "sampuurn"},
		{"ਪੰਜਾਬ", "iso15919", "paṁjāba"},
		{"ਪੰਜਾਬ", "ascii", "panjaab"},
		{"ਪੱਕਾ", "iso15919", "pakkā"},
		{"മലയാളം", "iso15919", "malayāḷaṁ"},
		{"അവൻ", "iso15919", "avan"},
		{"ಕನ್ನಡ", "ascii", "kannada"},
		{"తెలుగు", "iso15919", "telugu"},
		{"বাংলা", "iso15919", "bāṁlā"},
		{"ଓଡ଼ିଆ", "iso15919", "ōṛiā"},
		{"ગુજરાતી", "iso15919", "gujarātī"},
		{"ராமன் 2026, ok!", "ascii", "raaman 2026, ok!"},
		{"१२३ रुपये", "iso15919", "123 rupayē"},
	}
	for _, c := range cases {
		got, err := Romanise(c.text, c.scheme)
		if err != nil || got != c.want {
			t.Errorf("Romanise(%q, %s) = %q, want %q (%v)", c.text, c.scheme, got, c.want, err)
		}
	}
	if _, err := Romanise("x", "itrans"); err != ErrNotSupported {
		t.Error("unknown scheme should be refused")
	}
}

func TestScriptToScript(t *testing.T) {
	cases := []struct{ text, to, want string }{
		{"भारत", "tamil", "பாரத"},
		{"தமிழ்", "devanagari", "तमिऴ्"},
		{"हिन्दी", "kannada", "ಹಿನ್ದೀ"},
		{"नमस्ते", "telugu", "నమస్తే"},
		{"गंगा", "tamil", "கம்கா"},
		{"कृष्ण", "tamil", "க்ருஷ்ண"},
		{"வணக்கம்", "malayalam", "വണക്കം"},
		{"ਪੱਕਾ", "devanagari", "पक्का"},
		{"അവൻ", "devanagari", "अवन्"},
		{"Hello भारत!", "bengali", "Hello ভারত!"},
	}
	for _, c := range cases {
		if got := ToScript(c.text, script(t, c.to)); got != c.want {
			t.Errorf("ToScript(%q, %s) = %q, want %q", c.text, c.to, got, c.want)
		}
	}
}

func TestFromRoman(t *testing.T) {
	cases := []struct{ text, scheme, to, want string }{
		{"vaṇakkam", "iso15919", "tamil", "வணக்கம்"},
		{"tamiḻ", "iso15919", "tamil", "தமிழ்"},
		{"saṃskṛtam", "iast", "devanagari", "संस्कृतम्"},
		{"namastē", "iso15919", "devanagari", "नमस्ते"},
		{"namaste", "iast", "devanagari", "नमस्ते"},
		{"Bhārata 2026", "iast", "devanagari", "भारत 2026"},
		{"kannaḍa", "iso15919", "kannada", "ಕನ್ನಡ"},
		{"gaṅgā", "iast", "tamil", "கங்கா"},
	}
	for _, c := range cases {
		got, err := FromRoman(c.text, c.scheme, script(t, c.to))
		if err != nil || got != c.want {
			t.Errorf("FromRoman(%q, %s, %s) = %q, want %q (%v)", c.text, c.scheme, c.to, got, c.want, err)
		}
	}
	// Round trip through ISO 15919 is exact for Devanagari.
	for _, word := range []string{"संस्कृतम्", "विद्यालय", "क्षत्रिय", "ऋषि", "दुःख"} {
		roman, _ := Romanise(word, "iso15919")
		back, _ := FromRoman(roman, "iso15919", script(t, "devanagari"))
		if back != word {
			t.Errorf("round trip %q -> %q -> %q", word, roman, back)
		}
	}
	if _, err := FromRoman("x", "ascii", script(t, "tamil")); err != ErrNotSupported {
		t.Error("ascii is not readable back")
	}
}

func TestDetectAndLookup(t *testing.T) {
	if s, ok := Detect("Hello வணக்கம் world"); !ok || s.ID != "tamil" {
		t.Errorf("detect: %v", s)
	}
	if _, ok := Detect("plain English"); ok {
		t.Error("no Indian script in plain English")
	}
	for name, want := range map[string]string{"hi": "devanagari", "Tamil": "tamil", "or": "odia", "pa": "gurmukhi"} {
		if s, ok := Lookup(name); !ok || s.ID != want {
			t.Errorf("Lookup(%q) = %v", name, s.ID)
		}
	}
}
