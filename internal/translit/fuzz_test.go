package translit

import (
	"testing"
	"unicode/utf8"
)

// Any text, in any script or none, converts without panicking, and valid text stays valid.
func FuzzTransliterate(f *testing.F) {
	for _, seed := range []string{"नमस्ते", "வணக்கம்", "ज़िंदगी", "संस्कृतम्", "namaste", "ಕನ್ನಡ ঙ্গ", "்்", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		valid := utf8.ValidString(text)
		Detect(text)
		HasIndianLetters(text)
		for _, scheme := range []string{"iso15919", "iast", "ascii"} {
			if out, err := Romanise(text, scheme); err == nil && valid && !utf8.ValidString(out) {
				t.Errorf("Romanise(%q, %s) gave invalid UTF-8 %q", text, scheme, out)
			}
			for _, target := range Scripts {
				if out, err := FromRoman(text, scheme, target); err == nil && valid && !utf8.ValidString(out) {
					t.Errorf("FromRoman(%q, %s, %s) gave invalid UTF-8 %q", text, scheme, target.Name, out)
				}
			}
		}
		for _, target := range Scripts {
			if out := ToScript(text, target); valid && !utf8.ValidString(out) {
				t.Errorf("ToScript(%q, %s) gave invalid UTF-8 %q", text, target.Name, out)
			}
		}
	})
}
