package indic

import (
	"strings"
	"testing"
)

func TestAksharas(t *testing.T) {
	if got := strings.Join(Aksharas("क्षत्रिय नमस्ते"), "|"); got != "क्ष|त्रि|य|न|म|स्ते" {
		t.Errorf("hindi %s", got)
	}
	// Tamil counts a consonant with pulli as a letter.
	if got := strings.Join(Aksharas("தமிழ்நாடு"), "|"); got != "த|மி|ழ்|நா|டு" {
		t.Errorf("tamil %s", got)
	}
	if c := CountText("வணக்கம் இந்தியா"); c.Aksharas != 9 || c.Words != 2 {
		t.Errorf("count %+v", c)
	}
}

func TestSortAndNormalise(t *testing.T) {
	got, err := Sort([]string{"மாம்பழம்", "அம்மா", "கடல்", "ஆறு"}, "ta")
	if err != nil || strings.Join(got, ",") != "அம்மா,ஆறு,கடல்,மாம்பழம்" {
		t.Errorf("sort %v %v", got, err)
	}
	fixed, fixes := Normalise("अाम ொ\u200D", true)
	if fixed != "आम ொ" || len(fixes) < 2 {
		t.Errorf("normalise %q %v", fixed, fixes)
	}
}

func TestBraille(t *testing.T) {
	// क = 1-3, ि = 2-4 (after the consonant), त = 2-3-4-5, ा = 3-4-5, ब = 1-2.
	if got := ToBraille("किताब"); got.Braille != "⠅⠊⠞⠜⠃" || got.Cells != 5 {
		t.Errorf("hindi %+v", got)
	}
	// Tamil goes by sound to the same cells: க = 1-3, ட = 2-3-4-5-6.
	if got := ToBraille("கடல்"); got.Braille != "⠅⠾⠇⠈" || got.Script != "tamil" {
		t.Errorf("tamil %+v", got)
	}
	if got := ToBraille("१२"); got.Braille != "⠼⠁⠃" {
		t.Errorf("digits %+v", got)
	}
}

func TestKeyboards(t *testing.T) {
	if got, _ := Type("kf;", "inscript-hindi"); got != "किच" {
		t.Errorf("inscript %s", got)
	}
	if got, _ := Type("hmwhfh", "tamil99"); got != "கரீக்க" {
		t.Errorf("tamil99 %s", got)
	}
	// Tamil99 adds the pulli itself for a doubled consonant and for ந்த.
	if got, _ := Type("hhk;lq", "tamil99"); got != "க்கமந்தா" {
		t.Errorf("auto pulli %s", got)
	}
	keys, missing, _ := Keys("नमस्ते", "inscript-hindi")
	if back, _ := Type(keys, "inscript-hindi"); back != "नमस्ते" || len(missing) != 0 {
		t.Errorf("inscript round trip %s -> %s", keys, back)
	}
	keys, _, _ = Keys("வணக்கம் தமிழ்", "tamil99")
	if back, _ := Type(keys, "tamil99"); back != "வணக்கம் தமிழ்" {
		t.Errorf("tamil99 round trip %s -> %s", keys, back)
	}
}
