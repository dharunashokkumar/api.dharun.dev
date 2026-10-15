package api

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
	"github.com/dharunashokkumar/api.dharun.dev/internal/subword"
)

// Moderation: three classifiers read the text and each says how likely it is to be
// abusive in what it was trained on. The general model (Detoxify multilingual, Unitary)
// knows English and several European languages, and Hindi through its multilingual base;
// the Hinglish model knows Roman-script Hindi mixed with English, by kind of abuse; the
// Tanglish model knows Roman-script Tamil mixed with English. None of them decides
// anything: the answer is scores, and flagged only says whether any passed the threshold.

const (
	moderateMaxChars  = 4000
	moderateMaxTokens = 256
)

type moderationModel struct {
	ID, Covers   string
	attachment   string
	labels       []string
	sigmoid      bool // labels scored independently; otherwise softmax over them
	pad          int64
	bert         bool // [CLS] text [SEP] with BERT's tokens; otherwise <s> text </s>
	reportLabels []string
}

var moderationModels = []moderationModel{
	{ID: "general", Covers: "English and other languages the multilingual model saw, Hindi among them; weak on Roman-script Indian languages",
		attachment: "general", labels: []string{"toxic"}, sigmoid: true, pad: xlmrPad},
	{ID: "hinglish", Covers: "Hindi written in Roman letters, mixed with English",
		attachment: "hinglish", labels: []string{"profanity", "harassment", "hate_speech", "caste", "communal", "regional", "misogyny"}, sigmoid: true, pad: xlmrPad},
	{ID: "tanglish", Covers: "Tamil written in Roman letters, mixed with English",
		attachment: "tanglish", labels: []string{"offensive"}, sigmoid: true, pad: 0, bert: true},
}

type moderationResult struct {
	Model     string             `json:"model"`
	Covers    string             `json:"covers"`
	Scores    map[string]float64 `json:"scores"`
	Truncated bool               `json:"truncated"`
}

type moderationHighest struct {
	Model string  `json:"model"`
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type moderationView struct {
	Flagged   bool               `json:"flagged"`
	Threshold float64            `json:"threshold"`
	Highest   moderationHighest  `json:"highest"`
	Results   []moderationResult `json:"results"`
}

func (s *Server) handleModerate(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	if utf8.RuneCountInString(in.Text) > moderateMaxChars {
		s.badRequest(w, r, "The text is longer than 4,000 characters; send a comment or message at a time.")
		return
	}
	threshold := 0.5
	if raw := strings.TrimSpace(in.option(r, "threshold")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value <= 0 || value >= 1 {
			s.badRequest(w, r, "threshold must be between 0 and 1.")
			return
		}
		threshold = value
	}
	chosen := moderationModels
	if raw := strings.TrimSpace(in.option(r, "models")); raw != "" {
		chosen = nil
		for _, id := range strings.Split(strings.ToLower(raw), ",") {
			found := false
			for _, m := range moderationModels {
				if m.ID == strings.TrimSpace(id) {
					chosen, found = append(chosen, m), true
				}
			}
			if !found {
				s.badRequest(w, r, "models is a comma-separated list of: general, hinglish, tanglish.")
				return
			}
		}
	}
	set := s.need(w, r, "moderation")
	if set == nil {
		return
	}
	type prepared struct {
		model moderationModel
		ids   []int64
		cut   bool
	}
	var jobs []prepared
	for _, m := range chosen {
		tok, err := s.extras.text.tokenizer("moderation", set.attachment(m.attachment+"-tokenizer"))
		if err != nil {
			s.modelFailed(w, r, err, "read the tokenizer")
			return
		}
		ids, cut := classifierSequence(tok, in.Text, m.bert, moderateMaxTokens)
		jobs = append(jobs, prepared{m, ids, cut})
	}
	view := moderationView{Threshold: threshold, Results: []moderationResult{}}
	err := s.extras.models.use(r.Context(), moderationKind, func(c *onnxwire.Conn) error {
		for _, job := range jobs {
			var logits []float32
			if err := scoreSequences(c, set.attachment(job.model.attachment), [][]int64{job.ids}, job.model.pad, func(row []float32, labels int) {
				logits = row[:labels]
			}); err != nil {
				return err
			}
			result := moderationResult{Model: job.model.ID, Covers: job.model.Covers, Scores: map[string]float64{}, Truncated: job.cut}
			probabilities := make([]float64, len(logits))
			if job.model.sigmoid {
				for i, v := range logits {
					probabilities[i] = sigmoid(float64(v))
				}
			} else {
				var sum float64
				top := float64(logits[0])
				for _, v := range logits {
					top = math.Max(top, float64(v))
				}
				for i, v := range logits {
					probabilities[i] = math.Exp(float64(v) - top)
					sum += probabilities[i]
				}
				for i := range probabilities {
					probabilities[i] /= sum
				}
			}
			for i, label := range job.model.labels {
				if i >= len(probabilities) {
					break
				}
				if job.model.reportLabels != nil && !contains(job.model.reportLabels, label) {
					continue
				}
				score := math.Round(probabilities[i]*1e4) / 1e4
				result.Scores[label] = score
				if score > view.Highest.Score {
					view.Highest = moderationHighest{Model: job.model.ID, Label: label, Score: score}
				}
			}
			view.Results = append(view.Results, result)
		}
		return nil
	})
	if err != nil {
		s.modelFailed(w, r, err, "moderate the text")
		return
	}
	view.Flagged = view.Highest.Score >= threshold
	s.ok(w, r, view, meta{Source: s.source("moderation"),
		Note: "Scores from 0 to 1 by each model, for the languages it covers; a model reading a language it does not cover guesses. Use the scores to sort or to ask a person, not to decide alone."}, "private, no-store")
}

// classifierSequence is [CLS] text [SEP] for a BERT model, <s> text </s> otherwise, cut
// to most tokens.
func classifierSequence(tok *subword.Tokenizer, text string, bert bool, most int) ([]int64, bool) {
	if !bert {
		return xlmrSequence(tok, text, most)
	}
	ids := tok.IDs(text)
	cut := false
	if len(ids) > most-2 {
		ids, cut = ids[:most-2], true
	}
	cls, _ := tok.ID("[CLS]")
	sep, _ := tok.ID("[SEP]")
	return append(append([]int64{cls}, ids...), sep), cut
}
