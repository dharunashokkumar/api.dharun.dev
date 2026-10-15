package indictrans

import (
	"strings"
	"testing"
	"time"
)

func TestMosesTokenize(t *testing.T) {
	for in, want := range map[string]string{
		"Hello, how are you?":                 "Hello , how are you ?",
		"It costs Rs. 1,00,000 today.":        "It costs Rs. 1,00,000 today .",
		"Dr. A. P. J. Abdul Kalam didn't go.": "Dr. A. P. J. Abdul Kalam didn 't go .",
		"Visit <ID1> now (please)!":           "Visit <ID1> now ( please ) !",
		"a,b,c,,d":                            "a , b , c , , d",
	} {
		done := make(chan string, 1)
		go func() { done <- mosesTokenize(in) }()
		select {
		case got := <-done:
			if got != want {
				t.Errorf("mosesTokenize(%q) = %q, want %q", in, got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("mosesTokenize(%q) did not finish", in)
		}
	}
}

func TestDetokenize(t *testing.T) {
	if got := mosesDetokenize(`He said " hello , world " ( twice ) .`); got != `He said "hello, world" (twice).` {
		t.Errorf("English: %q", got)
	}
	if got := indicDetokenize("नमस्ते , दुनिया ।"); got != "नमस्ते, दुनिया।" {
		t.Errorf("Indic: %q", got)
	}
}

func TestPrepareAndFinish(t *testing.T) {
	en, _ := Find("en")
	ta, _ := Find("ta")
	hi, _ := Find("hin_Deva")
	prepared, holders := Prepare("Write to help@example.com or call 1800-180-1111, please.", en, ta)
	if !strings.HasPrefix(prepared, "eng_Latn tam_Taml ") || strings.Contains(prepared, "example.com") || strings.Contains(prepared, "1800") {
		t.Errorf("prepared %q", prepared)
	}
	if got := Finish("<ID1> அல்லது <ID2>", ta, holders); !strings.Contains(got, "help@example.com") || !strings.Contains(got, "1800-180-1111") {
		t.Errorf("placeholders not restored: %q (%v)", got, holders)
	}
	// Tamil goes into Devanagari for the model, and comes back.
	prepared, _ = Prepare("வணக்கம்", ta, en)
	if !strings.Contains(prepared, "वणक्कम्") {
		t.Errorf("Tamil not moved to Devanagari: %q", prepared)
	}
	if got := Finish("वणक्कम्", ta, Placeholders{}); got != "வணக்கம்" {
		t.Errorf("Devanagari not moved back to Tamil: %q", got)
	}
	if got := Finish("नमस्ते ।", hi, Placeholders{}); got != "नमस्ते।" {
		t.Errorf("Hindi: %q", got)
	}
}

func TestSplitSentences(t *testing.T) {
	got := SplitSentences("Dr. A. P. J. Abdul Kalam was born in 1931. He died in 2015! मेरा नाम राहुल है। ठीक है?")
	want := []string{"Dr. A. P. J. Abdul Kalam was born in 1931.", "He died in 2015!", "मेरा नाम राहुल है।", "ठीक है?"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
}
