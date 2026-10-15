package api

import (
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
	"github.com/dharunashokkumar/api.dharun.dev/internal/subword"
)

// Embeddings and reranking. Embeddings come from multilingual-e5-small (384 numbers per
// text, about a hundred languages, the Indian ones among them); e5 was trained with
// "query: " and "passage: " in front of the text, and the type option adds the right one.
// Reranking scores how well each document answers a query with bge-reranker-v2-m3, a
// cross-encoder that reads query and document together.

const (
	embedMaxTexts   = 64
	embedMaxTokens  = 512
	rerankMaxDocs   = 100
	rerankMaxTokens = 512
	textModelChars  = 8000 // one text or document, in characters
	embedDimensions = 384
	modelBatch      = 16 // sequences sent to a model at once
)

type embedRequest struct {
	Text  string   `json:"text"`
	Texts []string `json:"texts"`
	Type  string   `json:"type"`
}

func (e *embedRequest) setText(text string) { e.Text = text }

type embedView struct {
	Model      string      `json:"model"`
	Type       string      `json:"type"`
	Dimensions int         `json:"dimensions"`
	Embeddings [][]float32 `json:"embeddings"`
	Tokens     []int       `json:"tokens"`
	Truncated  []int       `json:"truncated"`
}

// textList reads the texts of a model request: ?text= (repeatable) on GET, or a JSON body.
func textList(r *http.Request, one string, many []string) []string {
	var texts []string
	if r.Method == http.MethodGet {
		return r.URL.Query()["text"]
	}
	if one != "" {
		texts = append(texts, one)
	}
	return append(texts, many...)
}

func (s *Server) checkTexts(w http.ResponseWriter, r *http.Request, texts []string, most int, what string) bool {
	if len(texts) == 0 {
		s.badRequest(w, r, "No "+what+" given. Use ?text=... (more than once for several), or POST JSON {\"texts\": [\"...\"]}.")
		return false
	}
	if len(texts) > most {
		s.badRequest(w, r, "At most "+strconv.Itoa(most)+" "+what+" a request.")
		return false
	}
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			s.badRequest(w, r, what+" "+strconv.Itoa(i+1)+" is empty.")
			return false
		}
		if !utf8.ValidString(text) {
			s.badRequest(w, r, what+" "+strconv.Itoa(i+1)+" is not valid UTF-8.")
			return false
		}
		if utf8.RuneCountInString(text) > textModelChars {
			s.badRequest(w, r, what+" "+strconv.Itoa(i+1)+" is longer than "+strconv.Itoa(textModelChars)+" characters; split it first (/v1/text/chunk).")
			return false
		}
	}
	return true
}

func (s *Server) handleEmbed(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	var req embedRequest
	if r.Method == http.MethodPost {
		if !s.readJSON(w, r, &req) {
			return
		}
	} else {
		req.Type = r.URL.Query().Get("type")
	}
	texts := textList(r, req.Text, req.Texts)
	if !s.checkTexts(w, r, texts, embedMaxTexts, "texts") {
		return
	}
	kind := strings.ToLower(strings.TrimSpace(req.Type))
	switch kind {
	case "":
		kind = "query"
	case "query", "passage":
	default:
		s.badRequest(w, r, "type must be query (a question or search) or passage (a document to be found).")
		return
	}
	set := s.need(w, r, "embeddings")
	if set == nil {
		return
	}
	tok, err := s.extras.text.tokenizer("embeddings", set.attachment("tokenizer"))
	if err != nil {
		s.modelFailed(w, r, err, "read the tokenizer")
		return
	}
	view := embedView{Model: "multilingual-e5-small", Type: kind, Dimensions: embedDimensions, Truncated: []int{}}
	sequences := make([][]int64, len(texts))
	for i, text := range texts {
		ids, cut := xlmrSequence(tok, kind+": "+text, embedMaxTokens)
		sequences[i] = ids
		view.Tokens = append(view.Tokens, len(ids))
		if cut {
			view.Truncated = append(view.Truncated, i)
		}
	}
	model := set.attachment("model")
	err = s.extras.models.use(r.Context(), embeddingsKind, func(c *onnxwire.Conn) error {
		for from := 0; from < len(sequences); from += modelBatch {
			part := sequences[from:min(from+modelBatch, len(sequences))]
			b := newBatch(part, xlmrPad)
			out, err := c.Run(onnxwire.RunRequest{Model: model, Inputs: b.inputs(), Outputs: []string{"last_hidden_state"}})
			if err != nil {
				return err
			}
			vectors, err := meanPool(out[0], b)
			if err != nil {
				return err
			}
			view.Embeddings = append(view.Embeddings, vectors...)
		}
		return nil
	})
	if err != nil {
		s.modelFailed(w, r, err, "make embeddings")
		return
	}
	note := "Each embedding is normalised to length 1, so the dot product of two is their cosine similarity. Embed questions with type=query and the documents searched with type=passage."
	if len(view.Truncated) > 0 {
		note += " Texts listed in truncated were cut at 512 tokens."
	}
	s.ok(w, r, view, meta{Source: s.source("embeddings"), Note: note}, "private, no-store")
}

// XLM-RoBERTa's special tokens.
const (
	xlmrStart = 0
	xlmrPad   = 1
	xlmrEnd   = 2
)

// xlmrSequence is <s> text </s>, cut to fit.
func xlmrSequence(tok *subword.Tokenizer, text string, most int) ([]int64, bool) {
	ids := tok.IDs(text)
	cut := false
	if len(ids) > most-2 {
		ids, cut = ids[:most-2], true
	}
	return append(append([]int64{xlmrStart}, ids...), xlmrEnd), cut
}

// meanPool averages each sequence's token vectors over its real tokens and scales the
// result to length 1.
func meanPool(hidden onnxwire.Tensor, b batch) ([][]float32, error) {
	if len(hidden.Shape) != 3 || hidden.Shape[0] != b.rows || hidden.Shape[1] != b.width {
		return nil, errors.New("unexpected embedding shape")
	}
	size := hidden.Shape[2]
	out := make([][]float32, b.rows)
	for row := int64(0); row < b.rows; row++ {
		sum := make([]float64, size)
		var count float64
		for t := int64(0); t < b.width; t++ {
			if b.mask[row*b.width+t] == 0 {
				continue
			}
			count++
			base := (row*b.width + t) * size
			for d := int64(0); d < size; d++ {
				sum[d] += float64(hidden.Floats[base+d])
			}
		}
		var length float64
		for d := range sum {
			sum[d] /= max(count, 1)
			length += sum[d] * sum[d]
		}
		length = math.Sqrt(length)
		vector := make([]float32, size)
		for d := range sum {
			vector[d] = float32(math.Round(sum[d]/max(length, 1e-12)*1e6) / 1e6)
		}
		out[row] = vector
	}
	return out, nil
}

type rerankRequest struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	Top       int      `json:"top"`
}

type rerankResult struct {
	Index    int     `json:"index"`
	Score    float64 `json:"score"`
	Document string  `json:"document"`
}

type rerankView struct {
	Model     string         `json:"model"`
	Query     string         `json:"query"`
	Results   []rerankResult `json:"results"`
	Truncated []int          `json:"truncated"`
}

func (s *Server) handleRerank(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	var req rerankRequest
	if r.Method == http.MethodPost {
		if !s.readJSON(w, r, &req) {
			return
		}
	} else {
		q := r.URL.Query()
		req.Query, req.Documents = q.Get("query"), q["document"]
		if raw := q.Get("top"); raw != "" {
			top, err := strconv.Atoi(raw)
			if err != nil {
				s.badRequest(w, r, "top must be a whole number.")
				return
			}
			req.Top = top
		}
	}
	if strings.TrimSpace(req.Query) == "" || !utf8.ValidString(req.Query) || utf8.RuneCountInString(req.Query) > 2000 {
		s.badRequest(w, r, "query is required: the question, up to 2,000 characters.")
		return
	}
	if !s.checkTexts(w, r, req.Documents, rerankMaxDocs, "documents") {
		return
	}
	if req.Top < 0 || req.Top > len(req.Documents) {
		s.badRequest(w, r, "top must be from 1 to the number of documents (0 or left out: all).")
		return
	}
	set := s.need(w, r, "reranker")
	if set == nil {
		return
	}
	tok, err := s.extras.text.tokenizer("reranker", set.attachment("tokenizer"))
	if err != nil {
		s.modelFailed(w, r, err, "read the tokenizer")
		return
	}
	query := tok.IDs(req.Query)
	if len(query) > 128 {
		query = query[:128]
	}
	view := rerankView{Model: "bge-reranker-v2-m3", Query: req.Query, Truncated: []int{}}
	sequences := make([][]int64, len(req.Documents))
	for i, doc := range req.Documents {
		ids := tok.IDs(doc)
		room := rerankMaxTokens - len(query) - 4
		if len(ids) > room {
			ids = ids[:room]
			view.Truncated = append(view.Truncated, i)
		}
		// <s> query </s></s> document </s>
		seq := append([]int64{xlmrStart}, query...)
		seq = append(seq, xlmrEnd, xlmrEnd)
		seq = append(seq, ids...)
		sequences[i] = append(seq, xlmrEnd)
	}
	model := set.attachment("model")
	scores := make([]float64, 0, len(sequences))
	err = s.extras.models.use(r.Context(), rerankerKind, func(c *onnxwire.Conn) error {
		return scoreSequences(c, model, sequences, xlmrPad, func(logits []float32, labels int) {
			for row := 0; row < len(logits)/labels; row++ {
				scores = append(scores, sigmoid(float64(logits[row*labels])))
			}
		})
	})
	if err != nil {
		s.modelFailed(w, r, err, "rank the documents")
		return
	}
	for i, doc := range req.Documents {
		view.Results = append(view.Results, rerankResult{Index: i, Score: math.Round(scores[i]*1e6) / 1e6, Document: doc})
	}
	sort.SliceStable(view.Results, func(a, b int) bool { return view.Results[a].Score > view.Results[b].Score })
	if req.Top > 0 {
		view.Results = view.Results[:req.Top]
	}
	total := len(req.Documents)
	s.ok(w, r, view, meta{Total: &total, Source: s.source("reranker"),
		Note: "Scores run from 0 to 1, most relevant first; index is the document's place in the request. Compare scores within one request rather than against a fixed line."}, "private, no-store")
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

// scoreSequences runs a sequence classifier over sequences in batches and hands each
// batch's logits (rows of labels values) to take, in order.
func scoreSequences(c *onnxwire.Conn, model string, sequences [][]int64, pad int64, take func(logits []float32, labels int)) error {
	for from := 0; from < len(sequences); from += modelBatch {
		part := sequences[from:min(from+modelBatch, len(sequences))]
		b := newBatch(part, pad)
		out, err := c.Run(onnxwire.RunRequest{Model: model, Inputs: b.inputs()})
		if err != nil {
			return err
		}
		logits := out[0]
		if len(logits.Shape) != 2 || logits.Shape[0] != b.rows {
			return errors.New("unexpected logits shape")
		}
		take(logits.Floats, int(logits.Shape[1]))
	}
	return nil
}
