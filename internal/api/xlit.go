package api

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
)

// "Type English, get Tamil": IndicXlit (AI4Bharat) guesses how a word typed in Roman
// letters is written in an Indian script, as phone keyboards do. Unlike /v1/transliterate,
// which follows fixed rules between scripts, this is a model trained on how people
// actually type, so it returns several guesses, best first. Each word is guessed on its
// own; anything that is not a run of Roman letters is kept as it is.

const (
	xlitMaxWords = 60
	xlitMaxChars = 40 // one word
)

// xlitLanguages are IndicXlit's target languages and their names.
var xlitLanguages = map[string]string{
	"as": "Assamese", "bn": "Bengali", "brx": "Bodo", "gom": "Konkani", "gu": "Gujarati", "hi": "Hindi",
	"kn": "Kannada", "ks": "Kashmiri", "mai": "Maithili", "ml": "Malayalam", "mni": "Manipuri", "mr": "Marathi",
	"ne": "Nepali", "or": "Odia", "pa": "Punjabi", "sa": "Sanskrit", "sd": "Sindhi", "si": "Sinhala",
	"ta": "Tamil", "te": "Telugu", "ur": "Urdu",
}

type xlitVocabulary struct {
	Source []string `json:"source"`
	Target []string `json:"target"`
	Start  int64    `json:"start"`
	End    int64    `json:"end"`
	source map[string]int64
}

func readXlitVocabulary(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v xlitVocabulary
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	v.source = map[string]int64{}
	for i, symbol := range v.Source {
		v.source[symbol] = int64(i)
	}
	return &v, nil
}

type xlitGuess struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

type xlitWord struct {
	Input       string      `json:"input"`
	Suggestions []xlitGuess `json:"suggestions"`
}

type xlitView struct {
	Language string     `json:"language"`
	Name     string     `json:"name"`
	Text     string     `json:"text"`
	Words    []xlitWord `json:"words"`
}

func (s *Server) handleXlit(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	lang := strings.ToLower(strings.TrimSpace(in.option(r, "lang")))
	name, known := xlitLanguages[lang]
	if !known {
		s.badRequest(w, r, "lang is required: as, bn, brx, gom, gu, hi, kn, ks, mai, ml, mni, mr, ne, or, pa, sa, sd, si, ta, te or ur.")
		return
	}
	count, err := textOption(r, in, "count", 4, 1, 8)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	// The text in pieces: runs of Roman letters to guess, and everything between.
	type piece struct {
		text  string
		guess bool
	}
	var pieces []piece
	words := 0
	for _, run := range splitRuns(in.Text, func(r rune) bool { return r < 128 && unicode.IsLetter(r) }) {
		guess := run[0] < 128 && unicode.IsLetter(rune(run[0]))
		if guess {
			words++
			if len(run) > xlitMaxChars {
				s.badRequest(w, r, "A word is longer than 40 letters.")
				return
			}
		}
		pieces = append(pieces, piece{run, guess})
	}
	if words == 0 {
		s.badRequest(w, r, "The text has no words in Roman letters to transliterate.")
		return
	}
	if words > xlitMaxWords {
		s.badRequest(w, r, "At most 60 words a request.")
		return
	}
	set := s.need(w, r, "xlit")
	if set == nil {
		return
	}
	loaded, err := s.extras.text.load("xlit", set.attachment("vocabulary"), readXlitVocabulary)
	if err != nil {
		s.modelFailed(w, r, err, "read the vocabulary")
		return
	}
	vocab := loaded.(*xlitVocabulary)
	tag, ok := vocab.source["__"+lang+"__"]
	if !ok {
		s.modelFailed(w, r, os.ErrNotExist, "transliterate into that language")
		return
	}
	encoder, decoder := set.attachment("encoder"), set.attachment("decoder")
	view := xlitView{Language: lang, Name: name, Words: []xlitWord{}}
	guesses := map[string][]xlitGuess{}
	err = s.extras.models.use(r.Context(), xlitKind, func(c *onnxwire.Conn) error {
		for _, p := range pieces {
			word := strings.ToLower(p.text)
			if !p.guess || guesses[word] != nil {
				continue
			}
			ids := []int64{tag}
			for _, ch := range word {
				if id, ok := vocab.source[string(ch)]; ok {
					ids = append(ids, id)
				}
			}
			ids = append(ids, vocab.End)
			found, err := c.Generate(onnxwire.GenerateRequest{Encoder: encoder, Decoder: decoder, Input: ids,
				Start: vocab.Start, End: vocab.End, MaxLength: 3*len(word) + 8, Beams: max(count, 4), Best: count,
				Banned: []int64{0, 1, 3}})
			if err != nil {
				return err
			}
			list := []xlitGuess{}
			for _, seq := range found {
				var b strings.Builder
				for _, id := range seq.IDs {
					if id >= 0 && int(id) < len(vocab.Target) && !strings.HasPrefix(vocab.Target[id], "__") {
						b.WriteString(vocab.Target[id])
					}
				}
				text := b.String()
				if lang == "mr" {
					text = strings.ReplaceAll(text, "अॅ", "ॲ")
				}
				list = append(list, xlitGuess{Text: text, Score: math.Round(math.Exp(float64(seq.Score))*1e4) / 1e4})
			}
			guesses[word] = list
		}
		return nil
	})
	if err != nil {
		s.modelFailed(w, r, err, "transliterate")
		return
	}
	var out strings.Builder
	for _, p := range pieces {
		if !p.guess {
			out.WriteString(p.text)
			continue
		}
		list := guesses[strings.ToLower(p.text)]
		view.Words = append(view.Words, xlitWord{Input: p.text, Suggestions: list})
		if len(list) > 0 {
			out.WriteString(list[0].Text)
		} else {
			out.WriteString(p.text)
		}
	}
	view.Text = out.String()
	s.ok(w, r, view, meta{Source: s.source("xlit"),
		Note: "Guesses from a model, best first; text uses the first guess for every word. score is the model's average confidence per letter, from 0 to 1."}, "private, no-store")
}

// splitRuns cuts text into maximal runs of characters for which in is true, and the runs
// between them, in order.
func splitRuns(text string, in func(rune) bool) []string {
	var out []string
	start, inside := 0, false
	for i, r := range text {
		now := in(r)
		if i > 0 && now != inside {
			out = append(out, text[start:i])
			start = i
		}
		inside = now
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}
