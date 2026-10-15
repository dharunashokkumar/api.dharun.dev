package indictrans

import (
	"testing"
	"time"
)

// Pre- and post-processing take any text in any language pair and always finish: the
// tokenizer's loops run on caller input before the model sees it.
func FuzzPrepareAndFinish(f *testing.F) {
	f.Add("Write to help@example.com or call 1800-180-1111, please. Dr. A. P. J. Kalam didn't go!", 0, 5)
	f.Add("नमस्ते , दुनिया । <ID1> ( twice ) \" quoted \"", 3, 0)
	f.Fuzz(func(t *testing.T, text string, from, to int) {
		if len(Languages) == 0 {
			t.Skip("no languages")
		}
		source := Languages[uint(from)%uint(len(Languages))]
		target := Languages[uint(to)%uint(len(Languages))]
		done := make(chan struct{})
		go func() {
			defer close(done)
			for _, sentence := range SplitSentences(text) {
				prepared, holders := Prepare(sentence, source, target)
				Finish(prepared, target, holders)
			}
			Finish(text, target, Placeholders{})
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s to %s did not finish for %q", source.Code, target.Code, text)
		}
	})
}
