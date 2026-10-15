package textkit

import (
	"testing"
	"unicode/utf8"
)

// Chunks keep to the size asked for, and each one's offsets point at its own text.
func FuzzChunkText(f *testing.F) {
	f.Add("# Title\n\nFirst sentence. Second one!\n\n## Part\n\n```\ncode\n```\nतीसरा वाक्य। चौथा।", 40, 10, true)
	f.Add("no breaks at all in this long line of text", 5, 2, false)
	f.Fuzz(func(t *testing.T, text string, size, overlap int, markdown bool) {
		if !utf8.ValidString(text) || size < 1 || size > 5000 || overlap < 0 || overlap >= size {
			return // the API refuses these before chunking
		}
		runes := []rune(text)
		for _, c := range ChunkText(text, ChunkOptions{Size: size, Overlap: overlap, Markdown: markdown}) {
			if c.Chars > size || c.Chars != utf8.RuneCountInString(c.Text) {
				t.Fatalf("chunk %d has %d characters (%d counted) for size %d", c.Index, utf8.RuneCountInString(c.Text), c.Chars, size)
			}
			if c.Start < 0 || c.End > len(runes) || c.Start >= c.End || string(runes[c.Start:c.End]) != c.Text {
				t.Fatalf("chunk %d at %d..%d does not match its text %q", c.Index, c.Start, c.End, c.Text)
			}
		}
	})
}

// Redaction finds identifiers in any text without panicking, keeps valid text valid,
// and reports positions inside the input.
func FuzzRedact(f *testing.F) {
	f.Add("PAN ABCPE1234F, Aadhaar 2341 2341 2346, call +91 98765 43210 or mail a@b.in", false)
	f.Add("card 4111 1111 1111 1111, IFSC SBIN0000001, GSTIN 27AAPFU0939F1ZV", true)
	f.Fuzz(func(t *testing.T, text string, partial bool) {
		if !utf8.ValidString(text) {
			return // readText refuses invalid UTF-8
		}
		r := Redact(text, nil, partial)
		if !utf8.ValidString(r.Text) {
			t.Fatalf("Redact(%q) gave invalid UTF-8 %q", text, r.Text)
		}
		n := utf8.RuneCountInString(text)
		for _, finding := range r.Findings {
			if finding.Start < 0 || finding.End > n || finding.Start >= finding.End {
				t.Fatalf("Redact(%q): finding at %d..%d outside the text", text, finding.Start, finding.End)
			}
		}
		Detect(text)
		LooksLikeMarkdown(text)
	})
}
