package textkit

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pkoukk/tiktoken-go"
	tiktoken_loader "github.com/pkoukk/tiktoken-go-loader"
)

const guide = `# Setup

Install Go. Then clone the repository and build it with go build. It takes a minute.

## Database

Run apidata build to make the seed files. They go into seed/ by default.

` + "```" + `
go run ./cmd/apidata build

# not a heading: this is inside a code fence
` + "```" + `

# Deploy

Push to main. The server installs the release within ten minutes.
`

// Every chunk must be an exact slice of the input at its stated character offsets.
func checkSlices(t *testing.T, text string, chunks []Chunk, size int) {
	t.Helper()
	all := []rune(text)
	for i, chunk := range chunks {
		if chunk.Index != i {
			t.Errorf("chunk %d has index %d", i, chunk.Index)
		}
		if string(all[chunk.Start:chunk.End]) != chunk.Text {
			t.Errorf("chunk %d is not text[%d:%d]", i, chunk.Start, chunk.End)
		}
		if chunk.Chars != utf8.RuneCountInString(chunk.Text) || chunk.Chars > size {
			t.Errorf("chunk %d has %d characters, limit %d", i, chunk.Chars, size)
		}
		if strings.TrimSpace(chunk.Text) != chunk.Text || chunk.Text == "" {
			t.Errorf("chunk %d is not trimmed: %q", i, chunk.Text)
		}
	}
}

func TestChunkMarkdown(t *testing.T) {
	chunks := ChunkText(guide, ChunkOptions{Size: 400, Overlap: 0, Markdown: true})
	checkSlices(t, guide, chunks, 400)
	var headings []string
	for _, chunk := range chunks {
		headings = append(headings, chunk.Heading)
	}
	if strings.Join(headings, " | ") != "Setup | Setup > Database | Deploy" {
		t.Errorf("headings: %q", headings)
	}
	// The code fence stays whole and its comment is not taken for a heading.
	if !strings.Contains(chunks[1].Text, "# not a heading") || !strings.HasSuffix(chunks[1].Text, "```") {
		t.Errorf("code fence split: %q", chunks[1].Text)
	}
	if !LooksLikeMarkdown(guide) || LooksLikeMarkdown("plain text\nwith lines") {
		t.Error("Markdown detection")
	}
}

func TestChunkSizesAndOverlap(t *testing.T) {
	var text strings.Builder
	for i := 0; i < 60; i++ {
		text.WriteString("இது ஒரு சோதனை வாக்கியம். This sentence is number ")
		text.WriteString(strings.Repeat("x", i%7))
		text.WriteString(". ")
		if i%10 == 9 {
			text.WriteString("\n\n")
		}
	}
	input := text.String()
	chunks := ChunkText(input, ChunkOptions{Size: 300, Overlap: 60})
	if len(chunks) < 5 {
		t.Fatalf("only %d chunks", len(chunks))
	}
	checkSlices(t, input, chunks, 300)
	for i := 1; i < len(chunks); i++ {
		if chunks[i].Start >= chunks[i-1].End {
			t.Errorf("chunk %d does not overlap the one before (%d >= %d)", i, chunks[i].Start, chunks[i-1].End)
		}
		if chunks[i-1].End-chunks[i].Start > 60 {
			t.Errorf("chunk %d overlaps by %d, more than 60", i, chunks[i-1].End-chunks[i].Start)
		}
		// Overlap starts at a word, never inside one.
		before := []rune(input)[chunks[i].Start-1]
		if before != ' ' && before != '\n' {
			t.Errorf("chunk %d starts mid-word after %q", i, before)
		}
	}
	// Without overlap the chunks cover the text end to end (whitespace aside).
	plain := ChunkText(input, ChunkOptions{Size: 300})
	var joined strings.Builder
	for _, chunk := range plain {
		joined.WriteString(chunk.Text)
	}
	if strings.Join(strings.Fields(joined.String()), "") != strings.Join(strings.Fields(input), "") {
		t.Error("chunks without overlap lose or repeat text")
	}
}

func TestChunkHardCuts(t *testing.T) {
	long := strings.Repeat("a", 1000) // no boundary at all
	chunks := ChunkText(long, ChunkOptions{Size: 300})
	checkSlices(t, long, chunks, 300)
	if len(chunks) != 4 {
		t.Errorf("%d chunks", len(chunks))
	}
	if ChunkText("   ", ChunkOptions{Size: 100}) != nil {
		t.Error("blank text has no chunks")
	}
}

func TestRedact(t *testing.T) {
	text := "Ravi (PAN ABCPK1234F, GSTIN 27AAPFU0939F1ZV) paid with 4111 1111 1111 1111. " +
		"Aadhaar 2341 2341 2346, phone +91 98765 43210 or 09876543210, mail ravi.k@example.co.in, " +
		"UPI ravi@okaxis, car TN 28 AB 1234, voter ID ABC1234567, passport K1234567, from 49.204.0.1. " +
		"Order 123412341234 and invoice 4111111111111112 are not personal."
	result := Redact(text, nil, false)
	want := map[string]int{"pan": 1, "gstin": 1, "card": 1, "aadhaar": 1, "phone": 2, "email": 1, "upi": 1, "vehicle": 1, "voter_id": 1, "passport": 1, "ip": 1}
	for kind, count := range want {
		if result.Counts[kind] != count {
			t.Errorf("%s: found %d, want %d (%v)", kind, result.Counts[kind], count, result.Counts)
		}
	}
	for _, leaked := range []string{"ABCPK1234F", "27AAPFU0939F1ZV", "4111 1111", "2341 2341", "98765", "ravi.k@", "ravi@okaxis", "TN 28", "ABC1234567", "K1234567", "49.204"} {
		if strings.Contains(result.Text, leaked) {
			t.Errorf("%q survived redaction", leaked)
		}
	}
	// Numbers that fail their check digit are left alone.
	if !strings.Contains(result.Text, "123412341234") || !strings.Contains(result.Text, "4111111111111112") {
		t.Error("a number with a wrong check digit was redacted")
	}
	// The GSTIN contains a PAN; it is reported once, as a GSTIN.
	runes := []rune(text)
	for _, finding := range result.Findings {
		value := string(runes[finding.Start:finding.End])
		if finding.Type == "pan" && value != "ABCPK1234F" {
			t.Errorf("PAN finding points at %q", value)
		}
	}

	partial := Redact("card 4111 1111 1111 1111, Aadhaar 2341-2341-2346, PAN ABCPK1234F", nil, true).Text
	if partial != "card XXXX XXXX XXXX 1111, Aadhaar XXXX-XXXX-2346, PAN XXXXXXXX4F" {
		t.Errorf("partial masking: %q", partial)
	}
	only := Redact(text, []string{"email"}, false)
	if len(only.Counts) != 1 || only.Counts["email"] != 1 || !strings.Contains(only.Text, "ABCPK1234F") {
		t.Errorf("types filter: %v", only.Counts)
	}
	if clean := Redact("nothing here", nil, false); clean.Text != "nothing here" || len(clean.Findings) != 0 {
		t.Errorf("clean text: %+v", clean)
	}
}

// The counter must agree with tiktoken-go, which follows OpenAI's own implementation.
func TestTokenCountsMatchTiktoken(t *testing.T) {
	tiktoken.SetBpeLoader(tiktoken_loader.NewOfflineLoader())
	samples := []string{
		"hello world",
		"Hello, World! It's a test; we're testing tokens' counts.",
		"வணக்கம், இது ஒரு சோதனை. नमस्ते दुनिया। ನಮಸ್ಕಾರ",
		"func main() {\n\tfmt.Println(\"x\") // comment\n}\n\n\n   indented\r\n",
		"Numbers 1234567 and 3.14159, emoji 🙂🇮🇳, URL https://api.dharun.dev/v1/pincode/637001?x=1",
		"<|endoftext|> special tokens are ordinary text here",
		strings.Repeat("abc ", 300) + strings.Repeat("ஆ", 100),
		"   leading and trailing spaces   ",
		"",
	}
	counter := NewTokenizer()
	for _, name := range []string{"o200k_base", "cl100k_base", "p50k_base"} {
		reference, err := tiktoken.GetEncoding(name)
		if err != nil {
			t.Fatal(err)
		}
		encoding, _ := EncodingFor(name)
		for _, sample := range samples {
			want := len(reference.EncodeOrdinary(sample))
			got, err := counter.Count(encoding, sample)
			if err != nil || got != want {
				t.Errorf("%s: %q counts %d, tiktoken %d (%v)", name, clip(sample), got, want, err)
			}
		}
	}
	if encoding, ok := EncodingFor("GPT-4o"); !ok || encoding.Name != "o200k_base" {
		t.Errorf("model lookup: %v %v", encoding.Name, ok)
	}
	if _, ok := EncodingFor("claude"); ok {
		t.Error("an unknown model must not map to an encoding")
	}
	if len(counter.Loaded()) != 3 {
		t.Errorf("loaded: %v", counter.Loaded())
	}
	if dropped := counter.DropIdle(0); len(dropped) != 3 || len(counter.Loaded()) != 0 {
		t.Errorf("drop idle: %v", dropped)
	}
	// A pathologically long word still finishes, close to the exact count.
	long := strings.Repeat("a", 20000)
	o200k, _ := EncodingFor("o200k_base")
	got, _ := counter.Count(o200k, long)
	reference, _ := tiktoken.GetEncoding("o200k_base")
	want := len(reference.EncodeOrdinary(long))
	if got < want || got > want+10 {
		t.Errorf("long word: %d, exact %d", got, want)
	}
}

func clip(value string) string {
	if len(value) > 40 {
		return value[:40]
	}
	return value
}

func TestDetectLanguage(t *testing.T) {
	cases := []struct{ text, code, script string }{
		{"அகர முதல எழுத்தெல்லாம் ஆதி பகவன் முதற்றே உலகு", "tam", "Tamil"},
		{"भारत एक विशाल देश है जहाँ अनेक भाषाएँ बोली जाती हैं", "hin", "Devanagari"},
		{"The quick brown fox jumps over the lazy dog near the river bank", "eng", "Latin"},
		{"ಕರ್ನಾಟಕ ರಾಜ್ಯದ ರಾಜಧಾನಿ ಬೆಂಗಳೂರು ನಗರವಾಗಿದೆ", "kan", "Kannada"},
		{"বাংলা ভাষা পৃথিবীর অন্যতম প্রধান ভাষা", "ben", "Bengali"},
	}
	for _, c := range cases {
		language, ok := Detect(c.text)
		if !ok || language.Code != c.code || language.Script != c.script {
			t.Errorf("%q: %+v", c.text, language)
		}
	}
	if tanglish, _ := Detect("enna da machan epdi irukka, romba nalla irukku"); !tanglish.Romanised || tanglish.Reliable {
		t.Errorf("Tanglish should be flagged as Romanised and unreliable: %+v", tanglish)
	}
	if _, ok := Detect("12345 !!! ..."); ok {
		t.Error("text without letters has no language")
	}
}
