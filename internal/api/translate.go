package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/indictrans"
	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
	"github.com/dharunashokkumar/api.dharun.dev/internal/subword"
)

// Translation between English and 22 Indian languages with IndicTrans2 (AI4Bharat), the
// distilled 200-million-parameter models, one for English to Indian languages and one
// for the way back. Between two Indian languages the text goes through English, which
// loses some meaning; the answer says so. Text is translated a sentence at a time.

const (
	translateMaxChars     = 2000
	translateMaxSentences = 30
	translateMaxTokens    = 200 // one sentence, source side
)

type translateView struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	Sentences   []string `json:"sentences"`
	ViaEnglish  bool     `json:"via_english"`
	Truncated   []int    `json:"truncated"`
	Translation string   `json:"translation"`
}

func (s *Server) handleTranslate(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	if utf8.RuneCountInString(in.Text) > translateMaxChars {
		s.badRequest(w, r, "The text is longer than 2,000 characters; send it in parts.")
		return
	}
	from, okFrom := indictrans.Find(strings.TrimSpace(in.option(r, "from")))
	to, okTo := indictrans.Find(strings.TrimSpace(in.option(r, "to")))
	if !okFrom || !okTo {
		s.badRequest(w, r, "from and to are required: en, or an Indian language (as, bn, brx, doi, gom, gu, hi, kn, ks, mai, ml, mr, mni, ne, or, pa, sa, sat, sd, ta, te, ur). See /v1/translate/languages.")
		return
	}
	if from.Code == to.Code {
		s.badRequest(w, r, "from and to are the same language.")
		return
	}
	sentences := indictrans.SplitSentences(in.Text)
	if len(sentences) > translateMaxSentences {
		s.badRequest(w, r, "At most 30 sentences a request.")
		return
	}
	set := s.need(w, r, "translation")
	if set == nil {
		return
	}
	view := translateView{From: from.Code, To: to.Code, Sentences: []string{}, Truncated: []int{}}
	steps := [][2]indictrans.Language{{from, to}}
	if !from.English() && !to.English() {
		english, _ := indictrans.Find("en")
		steps = [][2]indictrans.Language{{from, english}, {english, to}}
		view.ViaEnglish = true
	}
	current := sentences
	for stepIndex, step := range steps {
		model := "en-indic"
		if !step[0].English() {
			model = "indic-en"
		}
		source, err := s.extras.text.tokenizer("translation", set.attachment(model+"-source"))
		if err != nil {
			s.modelFailed(w, r, err, "read the tokenizer")
			return
		}
		target, err := s.extras.text.tokenizer("translation", set.attachment(model+"-target"))
		if err != nil {
			s.modelFailed(w, r, err, "read the tokenizer")
			return
		}
		vocabulary := s.translationVocabulary(target, set.attachment(model+"-target"))
		next := make([]string, len(current))
		err = s.extras.models.use(r.Context(), translationKind, func(c *onnxwire.Conn) error {
			for i, sentence := range current {
				prepared, holders := indictrans.Prepare(sentence, step[0], step[1])
				ids, cut := translationInput(source, prepared, step[0].Code, step[1].Code, translationSizes[model][0])
				if cut && stepIndex == 0 {
					view.Truncated = append(view.Truncated, i)
				}
				found, err := c.Generate(onnxwire.GenerateRequest{
					Encoder: set.attachment(model + "-encoder"), Decoder: set.attachment(model + "-decoder"),
					DecoderPast: set.attachment(model + "-decoder-past"), Input: ids,
					Start: 2, End: 2, MaxLength: min(2*len(ids)+16, 256), Beams: 1, Best: 1, VocabLimit: translationSizes[model][1],
					Banned: []int64{0, 1, 3},
				})
				if err != nil {
					return err
				}
				var pieces []string
				for _, id := range found[0].IDs {
					if id >= 0 && int(id) < len(vocabulary) {
						pieces = append(pieces, vocabulary[id])
					}
				}
				raw := strings.TrimSpace(strings.ReplaceAll(strings.Join(pieces, ""), "▁", " "))
				next[i] = indictrans.Finish(raw, step[1], holders)
			}
			return nil
		})
		if err != nil {
			s.modelFailed(w, r, err, "translate")
			return
		}
		current = next
	}
	view.Sentences = current
	view.Translation = strings.Join(current, " ")
	note := "Machine translation by IndicTrans2; names, numbers and technical terms deserve a check."
	if view.ViaEnglish {
		note += " Translated through English, as both languages are Indian; some meaning may be lost."
	}
	s.ok(w, r, view, meta{Source: s.source("translation"), Note: note}, "private, no-store")
}

// translationSizes are each model's source and target vocabulary sizes
// (tokenizer_meta.json): its tokenizers know more tokens than the model was trained with,
// and those are read as unknown.
var translationSizes = map[string][2]int64{"en-indic": {32322, 122672}, "indic-en": {122706, 32296}}

// translationInput is the language tags and the sentence's tokens, ending in </s>.
func translationInput(tok *subword.Tokenizer, prepared, from, to string, size int64) ([]int64, bool) {
	text := strings.TrimPrefix(prepared, from+" "+to+" ")
	var ids []int64
	if id, ok := tok.ID(from); ok {
		ids = append(ids, id)
	}
	if id, ok := tok.ID(to); ok {
		ids = append(ids, id)
	}
	body := tok.IDs(text)
	for i, id := range body {
		if id >= size {
			body[i] = 3
		}
	}
	cut := false
	if len(body) > translateMaxTokens {
		body, cut = body[:translateMaxTokens], true
	}
	return append(append(ids, body...), 2), cut
}

// translationVocabulary returns the target tokenizer's tokens by ID, built once per file.
func (s *Server) translationVocabulary(tok *subword.Tokenizer, path string) []string {
	value, _ := s.extras.text.load("translation", path+"#vocabulary", func(string) (any, error) {
		return tok.Vocabulary(), nil
	})
	vocabulary, _ := value.([]string)
	return vocabulary
}

type translateLanguage struct {
	Code  string `json:"code"`
	Short string `json:"short"`
	Name  string `json:"name"`
}

func (s *Server) handleTranslateLanguages(w http.ResponseWriter, r *http.Request) {
	var list []translateLanguage
	for _, l := range indictrans.Languages {
		list = append(list, translateLanguage{Code: l.Code, Short: l.Short, Name: l.Name})
	}
	total := len(list)
	s.ok(w, r, list, meta{Total: &total, Note: "Use short or code as from and to. Between two Indian languages the text is translated through English."}, "public, max-age=86400")
}
