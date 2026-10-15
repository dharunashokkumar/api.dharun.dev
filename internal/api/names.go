package api

import (
	"context"
	"errors"
	"unicode"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
	"github.com/dharunashokkumar/api.dharun.dev/internal/subword"
)

// Name detection for /v1/text/redact?names=true. Text in Indian scripts goes to IndicNER
// (AI4Bharat, eleven languages); the rest to bert-base-NER, which was trained on English
// news and misses many Indian names written in Roman letters. Both label each token as
// part of a person, place or organisation; only people are kept.

const (
	nerWindow           = 256 // tokens a model reads at once
	redactNamesMaxChars = 20000
)

var errUnexpectedShape = errors.New("unexpected output shape")

// isIndicLetter reports a letter of the Indian scripts IndicNER reads (Devanagari to
// Malayalam, U+0900 to U+0D7F).
func isIndicLetter(r rune) bool { return r >= 0x0900 && r <= 0x0D7F }

// scriptRuns cuts text into byte ranges of Indian-script and other words; spaces and
// punctuation join the run before them.
func scriptRuns(text string) (runs [][2]int, indic []bool) {
	start, current, decided := 0, false, false
	for i, r := range text {
		if !unicode.IsLetter(r) && !unicode.Is(unicode.Mn, r) && !unicode.Is(unicode.Mc, r) {
			continue
		}
		this := isIndicLetter(r)
		if !decided {
			current, decided = this, true
			continue
		}
		if this != current {
			// Cut at the start of this word.
			cut := i
			for cut > start {
				prev, size := utf8.DecodeLastRuneInString(text[:cut])
				if unicode.IsSpace(prev) {
					break
				}
				cut -= size
			}
			if cut == start {
				cut = i
			}
			runs, indic = append(runs, [2]int{start, cut}), append(indic, current)
			start, current = cut, this
		}
	}
	if start < len(text) {
		runs, indic = append(runs, [2]int{start, len(text)}), append(indic, current)
	}
	return runs, indic
}

type nerModel struct {
	attachment string
	labels     []string // by output index
}

var (
	indicNER = nerModel{"indic", []string{"B-LOC", "B-ORG", "B-PER", "I-LOC", "I-ORG", "I-PER", "O"}}
	bertNER  = nerModel{"english", []string{"O", "B-MISC", "I-MISC", "B-PER", "I-PER", "B-ORG", "I-ORG", "B-LOC", "I-LOC"}}
)

// findNames returns the byte ranges of people's names in text.
func (s *Server) findNames(ctx context.Context, set *handle, text string) ([][2]int, error) {
	runs, indic := scriptRuns(text)
	tokenizers := map[string]*subword.Tokenizer{}
	for _, model := range []nerModel{indicNER, bertNER} {
		tok, err := s.extras.text.tokenizer("names", set.attachment(model.attachment+"-tokenizer"))
		if err != nil {
			return nil, err
		}
		tokenizers[model.attachment] = tok
	}
	var names [][2]int
	err := s.extras.models.use(ctx, namesKind, func(c *onnxwire.Conn) error {
		for i, run := range runs {
			model := bertNER
			if indic[i] {
				model = indicNER
			}
			tok := tokenizers[model.attachment]
			cls, _ := tok.ID("[CLS]")
			sep, _ := tok.ID("[SEP]")
			tokens := tok.Encode(text[run[0]:run[1]])
			for from := 0; from < len(tokens); from += nerWindow {
				window := tokens[from:min(from+nerWindow, len(tokens))]
				ids := []int64{cls}
				for _, t := range window {
					ids = append(ids, t.ID)
				}
				ids = append(ids, sep)
				b := newBatch([][]int64{ids}, 0)
				out, err := c.Run(onnxwire.RunRequest{Model: set.attachment(model.attachment), Inputs: b.inputs()})
				if err != nil {
					return err
				}
				logits := out[0]
				if len(logits.Shape) != 3 || int(logits.Shape[1]) != len(ids) {
					return errUnexpectedShape
				}
				labels := int(logits.Shape[2])
				person := make([]bool, len(window))
				for t := range window {
					row := logits.Floats[(t+1)*labels : (t+2)*labels]
					best := 0
					for k, v := range row {
						if v > row[best] {
							best = k
						}
					}
					if best < len(model.labels) {
						label := model.labels[best]
						person[t] = label == "B-PER" || label == "I-PER"
					}
				}
				for _, span := range personSpans(text[run[0]:run[1]], window, person) {
					names = append(names, [2]int{run[0] + span[0], run[0] + span[1]})
				}
			}
		}
		return nil
	})
	return names, err
}

// personSpans joins tokens marked as a person, and the pieces of a word one of them
// belongs to, into byte ranges of text.
func personSpans(text string, tokens []subword.Token, person []bool) [][2]int {
	// A word: tokens that follow each other with nothing between them.
	sameWord := func(a, b subword.Token) bool {
		if a.End != b.Start {
			return false
		}
		first, _ := utf8.DecodeRuneInString(text[b.Start:])
		last, _ := utf8.DecodeLastRuneInString(text[:a.End])
		return !unicode.IsPunct(first) && !unicode.IsPunct(last)
	}
	marked := append([]bool(nil), person...)
	for i := range tokens {
		if !person[i] {
			continue
		}
		for j := i - 1; j >= 0 && sameWord(tokens[j], tokens[j+1]); j-- {
			marked[j] = true
		}
		for j := i + 1; j < len(tokens) && sameWord(tokens[j-1], tokens[j]); j++ {
			marked[j] = true
		}
	}
	var spans [][2]int
	for i := 0; i < len(tokens); i++ {
		if !marked[i] {
			continue
		}
		start, end := tokens[i].Start, tokens[i].End
		for i+1 < len(tokens) && marked[i+1] && onlySpaceBetween(text, end, tokens[i+1].Start) {
			i++
			end = tokens[i].End
		}
		spans = append(spans, [2]int{start, end})
	}
	return spans
}

func onlySpaceBetween(text string, from, to int) bool {
	if to < from {
		return false
	}
	for _, r := range text[from:to] {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
