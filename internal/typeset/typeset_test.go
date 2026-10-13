package typeset

import (
	"strings"
	"testing"
)

func TestEveryFontLoads(t *testing.T) {
	if err := check(); err != nil {
		t.Fatal(err)
	}
}

// Each script is drawn with its own font, with no missing glyphs.
func TestScriptsAreCovered(t *testing.T) {
	samples := map[string]string{
		"Latin":      "Free APIs for Indian developers, ₹250 — “quoted”…",
		"Devanagari": "नमस्ते भारत। क्षत्रिय श्री",
		"Bengali":    "বাংলা ভাষা ক্ষ",
		"Gurmukhi":   "ਪੰਜਾਬੀ ਭਾਸ਼ਾ",
		"Gujarati":   "ગુજરાતી ભાષા",
		"Odia":       "ଓଡ଼ିଆ ଭାଷା",
		"Tamil":      "தமிழ் மொழி ஸ்ரீ க்ஷ",
		"Telugu":     "తెలుగు భాష",
		"Kannada":    "ಕನ್ನಡ ಭಾಷೆ",
		"Malayalam":  "മലയാളം ഭാഷ",
	}
	setter := New()
	for _, name := range Scripts() {
		sample, ok := samples[name]
		if !ok {
			t.Errorf("no sample for %s", name)
			continue
		}
		for _, style := range []Style{{}, {Bold: true}, {Mono: true}} {
			line := setter.Shape(sample, style)
			if line.Width <= 0 || len(line.Glyphs) == 0 {
				t.Errorf("%s %+v: nothing shaped", name, style)
			}
			if !setter.Covered(sample, style) {
				t.Errorf("%s %+v: a glyph is missing", name, style)
			}
			if line.Indic != (name != "Latin") {
				t.Errorf("%s: Indic = %v", name, line.Indic)
			}
		}
	}
	if setter.Covered("漢字", Style{}) {
		t.Error("Chinese is not covered, and Covered should say so")
	}
}

func TestShapingJoinsConjuncts(t *testing.T) {
	setter := New()
	// क + ् + ष is one conjunct glyph in Devanagari, not three.
	if glyphs := setter.Shape("क्ष", Style{}).Glyphs; len(glyphs) != 1 {
		t.Errorf("क्ष shaped into %d glyphs", len(glyphs))
	}
	// The vowel sign of கொ is drawn on both sides of the consonant.
	if glyphs := setter.Shape("கொ", Style{}).Glyphs; len(glyphs) != 3 || glyphs[0].X >= glyphs[1].X {
		t.Errorf("கொ shaped into %d glyphs", len(glyphs))
	}
}

func TestMixedScriptsUseTheirOwnFonts(t *testing.T) {
	setter := New()
	line := setter.Shape("API தமிழ் 2026 हिंदी", Style{Bold: true})
	fonts := map[int]bool{}
	previous := -1
	for _, glyph := range line.Glyphs {
		if glyph.Font%2 != 1 {
			t.Fatal("a bold text uses bold fonts throughout")
		}
		fonts[glyph.Font.family()] = true
		if glyph.pen < previous {
			t.Fatal("glyphs run left to right")
		}
		previous = glyph.pen
	}
	if !fonts[sans] || !fonts[6] || !fonts[1] || len(fonts) != 3 {
		t.Errorf("families used: %v", fonts)
	}
	// Digits after Tamil letters stay in the Tamil font, so the line does not change face
	// for a number.
	runs := setter.itemise([]rune("தமிழ் 2026"), Style{})
	if len(runs) != 1 || runs[0].family != 6 {
		t.Errorf("runs %+v", runs)
	}
}

func TestMonoIsAGrid(t *testing.T) {
	setter := New()
	for _, text := range []string{"i", "W", "func main() {", "    return x <= 10 && y != nil"} {
		if width, want := setter.Width(text, Style{Mono: true}), 600*len(text); width != want {
			t.Errorf("%q is %d wide, want %d", text, width, want)
		}
	}
	if regular, bold := setter.Width("keyword", Style{Mono: true}), setter.Width("keyword", Style{Mono: true, Bold: true}); regular != bold {
		t.Errorf("bold code is as wide as regular: %d and %d", regular, bold)
	}
}

func TestWrap(t *testing.T) {
	setter := New()
	text := "Free and open APIs for Indian developers: pincodes, IFSC, holidays and more"
	lines := setter.Wrap(text, Style{}, 12*Em)
	if len(lines) < 3 {
		t.Fatalf("lines %q", lines)
	}
	for _, line := range lines {
		if setter.Width(line, Style{}) > 12*Em {
			t.Errorf("%q is wider than the limit", line)
		}
	}
	if joined := strings.Join(lines, " "); joined != text {
		t.Errorf("words were lost: %q", joined)
	}
	if setter.Wrap("  ", Style{}, Em) != nil {
		t.Error("blank text has no lines")
	}

	// A word wider than the line is cut into pieces that fit.
	long := strings.Repeat("W", 60)
	pieces := setter.Wrap(long, Style{Bold: true}, 10*Em)
	if len(pieces) < 4 || strings.Join(pieces, "") != long {
		t.Errorf("pieces %q", pieces)
	}
	for _, piece := range pieces {
		if setter.Width(piece, Style{Bold: true}) > 10*Em {
			t.Errorf("%q is wider than the limit", piece)
		}
	}

	// A cut never lands inside a syllable: no piece starts with a vowel sign or virama.
	tamil := strings.Repeat("வணக்கம்", 12)
	for _, piece := range setter.Wrap(tamil, Style{}, 8*Em) {
		if first := []rune(piece)[0]; first >= 0x0BBE && first <= 0x0BCD {
			t.Errorf("%q starts inside a syllable", piece)
		}
	}
}

func TestClamp(t *testing.T) {
	setter := New()
	lines := []string{"first line", "second line,", "third"}
	if got := setter.Clamp(lines[:2], 3, Style{}, 10*Em); len(got) != 2 {
		t.Errorf("short text is kept: %q", got)
	}
	if got := setter.Clamp(lines, 2, Style{}, 10*Em); len(got) != 2 || got[1] != "second line…" {
		t.Errorf("clamped %q", got)
	}
	// With no room to spare, letters make way for the ellipsis.
	tight := setter.Width("second line", Style{})
	got := setter.Clamp(lines, 2, Style{}, tight)
	if !strings.HasSuffix(got[1], "…") || setter.Width(got[1], Style{}) > tight || got[1] == "second line…" {
		t.Errorf("clamped %q", got)
	}
}

func TestFit(t *testing.T) {
	setter := New()
	if got := setter.Fit("short", Style{}, 10*Em); got != "short" {
		t.Errorf("a text that fits is kept: %q", got)
	}
	got := setter.Fit("a much longer piece of text", Style{}, 5*Em)
	if !strings.HasSuffix(got, "…") || setter.Width(got, Style{}) > 5*Em || len(got) < 6 {
		t.Errorf("fitted %q", got)
	}
	if got := setter.Fit("wide", Style{}, 1); got != "…" {
		t.Errorf("with no room at all: %q", got)
	}
}

func TestOutline(t *testing.T) {
	setter := New()
	line := setter.Shape("A g", Style{})
	if len(line.Glyphs) != 3 {
		t.Fatalf("%d glyphs", len(line.Glyphs))
	}
	a := setter.Outline(line.Glyphs[0].Font, line.Glyphs[0].ID)
	if !strings.HasPrefix(a, "M") || !strings.HasSuffix(a, "Z") {
		t.Errorf("outline %q", a)
	}
	// "A" stands on the baseline: with Y growing downwards, nothing reaches far below 0.
	if strings.Contains(a, " 2") && strings.Contains(a, " 300") {
		t.Errorf("A reaches below the baseline: %q", a)
	}
	if space := setter.Outline(line.Glyphs[1].Font, line.Glyphs[1].ID); space != "" {
		t.Errorf("a space has no outline, got %q", space)
	}
}

func BenchmarkCard(b *testing.B) {
	for b.Loop() {
		setter := New()
		lines := setter.Wrap("இந்திய டெவலப்பர்களுக்கான இலவச API and नमस्ते भारत", Style{Bold: true}, 12*Em)
		for _, line := range lines {
			for _, glyph := range setter.Shape(line, Style{Bold: true}).Glyphs {
				setter.Outline(glyph.Font, glyph.ID)
			}
		}
	}
}
