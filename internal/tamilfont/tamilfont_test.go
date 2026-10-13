package tamilfont

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tscii(t *testing.T) Encoding {
	t.Helper()
	encoding, ok := Lookup(" TSCII ")
	if !ok {
		t.Fatal("no TSCII")
	}
	return encoding
}

func TestTSCIIToUnicode(t *testing.T) {
	encoding := tscii(t)
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"plain letters and pulli", []byte{0xBE, 0xC1, 0xA2, 0xFA}, "தமிழ்"},
		{"ASCII passes through", []byte("TSCII 1.7 "), "TSCII 1.7 "},
		{"sign to the left moves after its consonant", []byte{0xA6, 0xB8}, "கெ"},
		{"ee and ai", []byte{0xA7, 0xBE, 0xA8, 0xC5}, "தேவை"},
		{"o is e, consonant, aa", []byte{0xA6, 0xB8, 0xA1}, "கொ"},
		{"oo is ee, consonant, aa", []byte{0xA7, 0xB8, 0xA1}, "கோ"},
		{"au is e, consonant, length mark", []byte{0xA6, 0xB8, 0xAA}, "கௌ"},
		{"au as the GNU C library writes it", []byte{0xA7, 0xB8, 0xAA}, "கௌ"},
		{"ai does not join aa", []byte{0xA8, 0xB8, 0xA1}, "கைா"},
		{"u and uu ligatures", []byte{0xCC, 0xE2, 0x99, 0x9C}, "குபூஙுஞூ"},
		{"ti and tii", []byte{0xCA, 0xCB}, "டிடீ"},
		{"Grantha with a sign", []byte{0x83, 0xA4, 0xA7, 0x86, 0xA1}, "ஜுஹோ"},
		{"ksha takes the sign as one consonant", []byte{0xA6, 0x87, 0xA1, 0x8C}, "க்ஷொக்ஷ்"},
		{"sri", []byte{0x82}, "ஸ்ரீ"},
		{"vowels, with i in both places", []byte{0xAB, 0xAC, 0xFE, 0xAD, 0xAE, 0xB6, 0xB7}, "அஆஇஇஈஔஃ"},
		{"digits and numbers", []byte{0x80, 0x81, 0x8D, 0x98, 0x9D, 0x9E, 0x9F}, "௦௧௨௯௰௱௲"},
		{"quotes", []byte{0x91, 0x92, 0x93, 0x94, 0xA9}, "‘’“”©"},
		{"a sign with nothing to belong to stays", []byte{0xA6, ' ', 0xA7}, "ெ ே"},
		{"vanakkam", []byte{0xC5, 0xBD, 0xEC, 0xB8, 0xF5}, "வணக்கம்"},
	}
	for _, c := range cases {
		if got, undefined := encoding.ToUnicode(c.in); got != c.want || undefined != 0 {
			t.Errorf("%s: got %q (%d undefined), want %q", c.name, got, undefined, c.want)
		}
	}
	if got, undefined := encoding.ToUnicode([]byte{0xB8, 0xFF}); got != "க�" || undefined != 1 {
		t.Errorf("an unassigned byte: %q, %d", got, undefined)
	}
}

func TestUnicodeToTSCII(t *testing.T) {
	encoding := tscii(t)
	cases := []struct {
		in   string
		want []byte
	}{
		{"தமிழ்", []byte{0xBE, 0xC1, 0xA2, 0xFA}},
		{"கொ கோ கௌ", []byte{0xA6, 0xB8, 0xA1, ' ', 0xA7, 0xB8, 0xA1, ' ', 0xA6, 0xB8, 0xAA}},
		{"கொ", []byte{0xA6, 0xB8, 0xA1}}, // o spelt as its two parts
		{"கௌ", []byte{0xA6, 0xB8, 0xAA}}, // au spelt as its two parts
		{"தேவை", []byte{0xA7, 0xBE, 0xA8, 0xC5}},
		{"குபூ", []byte{0xCC, 0xE2}},
		{"ஜு", []byte{0x83, 0xA4}}, // no ligature for Grantha: letter and sign
		{"டிகி", []byte{0xCA, 0xB8, 0xA2}},
		{"ஸ்ரீ க்ஷ க்ஷ் க்ஷே", []byte{0x82, ' ', 0x87, ' ', 0x8C, ' ', 0xA7, 0x87}},
		{"இ", []byte{0xFE}},
		{"அ, ஆ!", []byte{0xAB, ',', ' ', 0xAC, '!'}},
		{"கா", []byte{0xB8, 0xA1}},
	}
	for _, c := range cases {
		got, unmapped := encoding.FromUnicode(c.in)
		if !bytes.Equal(got, c.want) || len(unmapped) != 0 {
			t.Errorf("%q: got % X (unmapped %q), want % X", c.in, got, unmapped, c.want)
		}
	}
	got, unmapped := encoding.FromUnicode("க ₹ हि ₹")
	if string(got) != "\xB8 ? ?? ?" || strings.Join(unmapped, "") != "₹हि" {
		t.Errorf("unmapped characters: % X, %q", got, unmapped)
	}
}

// Every byte TSCII assigns comes back as itself after a trip through Unicode, apart from
// the old place of the letter i.
func TestTSCIIBytesRoundTrip(t *testing.T) {
	encoding := tscii(t)
	for b := 0x80; b <= 0xFE; b++ {
		if b == 0xAD || b == tsciiE || b == tsciiEE || b == tsciiAI {
			continue // the old i, and signs that only mean something before a consonant
		}
		text, undefined := encoding.ToUnicode([]byte{byte(b)})
		if undefined != 0 {
			t.Errorf("0x%X is not assigned", b)
			continue
		}
		back, unmapped := encoding.FromUnicode(text)
		if len(back) != 1 || back[0] != byte(b) || len(unmapped) != 0 {
			t.Errorf("0x%X became %q and came back as % X", b, text, back)
		}
	}
}

// The whole Thirukkural, 1,330 couplets, survives a trip to TSCII and back unchanged.
func TestThirukkuralRoundTrip(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "data", "thirukkural", "kurals.json"))
	if err != nil {
		t.Skip("no Thirukkural data here: ", err)
	}
	var texts []string
	collect(t, raw, &texts)
	if len(texts) < 1330 {
		t.Fatalf("only %d Tamil texts found", len(texts))
	}
	encoding := tscii(t)
	for _, text := range texts {
		data, unmapped := encoding.FromUnicode(text)
		if len(unmapped) != 0 {
			t.Fatalf("%q has characters TSCII lacks: %q", text, unmapped)
		}
		if back, undefined := encoding.ToUnicode(data); back != text || undefined != 0 {
			t.Fatalf("round trip changed the text:\n%q\n%q", text, back)
		}
		// And through the form it is usually pasted in.
		viewed, err := Bytes(View(data))
		if err != nil || !bytes.Equal(viewed, data) {
			t.Fatalf("the Windows-1252 view does not come back: %v", err)
		}
	}
}

// collect gathers every string holding Tamil from a JSON document.
func collect(t *testing.T, raw []byte, into *[]string) {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case string:
			if HasTamil(typed) {
				*into = append(*into, typed)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
}

func TestView(t *testing.T) {
	// தமிழ் in TSCII, as it looks without a TSCII font.
	if got := View([]byte{0xBE, 0xC1, 0xA2, 0xFA}); got != "¾Á¢ú" {
		t.Errorf("view %q", got)
	}
	data, err := Bytes("¾Á¢ú")
	if err != nil || !bytes.Equal(data, []byte{0xBE, 0xC1, 0xA2, 0xFA}) {
		t.Errorf("bytes % X, %v", data, err)
	}
	// Every byte has a view that comes back, including the five Windows-1252 leaves out.
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	if back, err := Bytes(View(all)); err != nil || !bytes.Equal(back, all) {
		t.Errorf("the view of all bytes does not come back: %v", err)
	}
	if _, err := Bytes("தமிழ்"); err == nil {
		t.Error("Unicode Tamil is not text in an 8-bit encoding")
	}
	if !HasTamil("a தமிழ்") || HasTamil("¾Á¢ú") {
		t.Error("HasTamil")
	}
}
