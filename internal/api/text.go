package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/textkit"
)

// Text tools: everything posted here can be anyone's document, so every request is
// private (not logged, not shared-cached) and nothing is kept after the response.

func textOption(r *http.Request, in textInput, name string, fallback, min, max int) (int, error) {
	raw := strings.TrimSpace(in.option(r, name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, errors.New(name + " must be an integer from " + strconv.Itoa(min) + " to " + strconv.Itoa(max) + ".")
	}
	return value, nil
}

type chunkView struct {
	Format string          `json:"format"`
	Size   int             `json:"size"`
	Chunks []textkit.Chunk `json:"chunks"`
}

func (s *Server) handleChunk(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	size, err := textOption(r, in, "size", 1000, 100, 20000)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	overlap, err := textOption(r, in, "overlap", size/10, 0, size/2)
	if err != nil {
		s.badRequest(w, r, err.Error()+" At most half the size.")
		return
	}
	format := strings.ToLower(strings.TrimSpace(in.option(r, "format")))
	switch format {
	case "", "auto":
		format = "text"
		if textkit.LooksLikeMarkdown(in.Text) {
			format = "markdown"
		}
	case "markdown", "text":
	default:
		s.badRequest(w, r, "format must be auto, markdown or text.")
		return
	}
	chunks := textkit.ChunkText(in.Text, textkit.ChunkOptions{Size: size, Overlap: overlap, Markdown: format == "markdown"})
	if chunks == nil {
		chunks = []textkit.Chunk{}
	}
	total := len(chunks)
	s.ok(w, r, chunkView{Format: format, Size: size, Chunks: chunks}, meta{Total: &total,
		Note: "Sizes and offsets count characters (Unicode code points). Each chunk is exactly text[start:end]."}, "private, no-store")
}

func (s *Server) handleRedact(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	var kinds []string
	if raw := strings.TrimSpace(in.option(r, "types")); raw != "" {
		for _, kind := range strings.Split(strings.ToLower(raw), ",") {
			kind = strings.TrimSpace(kind)
			if !contains(textkit.KindIDs(), kind) {
				s.badRequest(w, r, "types is a comma-separated list of: "+strings.Join(textkit.KindIDs(), ", ")+".")
				return
			}
			kinds = append(kinds, kind)
		}
	}
	style := strings.ToLower(strings.TrimSpace(in.option(r, "style")))
	if style != "" && style != "label" && style != "partial" {
		s.badRequest(w, r, "style must be label or partial.")
		return
	}
	result := textkit.Redact(in.Text, kinds, style == "partial")
	s.ok(w, r, result, meta{Note: "Finds identifiers by their structure and check digits. Names and street addresses are not detected."}, "private, no-store")
}

type tokensView struct {
	Tokens     int      `json:"tokens"`
	Encoding   string   `json:"encoding"`
	Models     []string `json:"models"`
	Characters int      `json:"characters"`
	Words      int      `json:"words"`
	Bytes      int      `json:"bytes"`
}

func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	name := in.option(r, "model")
	if name == "" {
		name = in.option(r, "encoding")
	}
	if name == "" {
		name = "o200k_base"
	}
	encoding, ok := textkit.EncodingFor(name)
	if !ok {
		s.badRequest(w, r, "Unknown model or encoding. Encodings: "+strings.Join(textkit.EncodingNames(), ", ")+
			". Only OpenAI's tokenizers are openly available; other models' token counts differ.")
		return
	}
	count, err := s.extras.tokens.Count(encoding, in.Text)
	if err != nil {
		s.badRequest(w, r, "The text could not be split into tokens.")
		return
	}
	s.ok(w, r, tokensView{
		Tokens: count, Encoding: encoding.Name, Models: encoding.Models,
		Characters: utf8.RuneCountInString(in.Text), Words: len(strings.FieldsFunc(in.Text, unicode.IsSpace)), Bytes: len(in.Text),
	}, meta{Note: "Counted with OpenAI's vocabulary for the encoding, treating special tokens as text. Chat requests add a few tokens per message on top."}, "private, no-store")
}

func (s *Server) handleLanguage(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	language, found := textkit.Detect(in.Text)
	if !found {
		s.fail(w, r, http.StatusUnprocessableEntity, "no_language", "The text has no letters to judge a language by.")
		return
	}
	s.ok(w, r, language, meta{Note: "About 80 languages from their letters. Short texts are uncertain; Romanised Indian languages are flagged but not identified."}, "private, no-store")
}

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "count_tokens",
			Description: "Count the tokens a text uses for an OpenAI model or encoding (o200k_base for GPT-4o and later, cl100k_base for GPT-4).",
			InputSchema: schema([]string{"text"}, map[string]any{"text": text("The text."), "model": text("Model or encoding name. Default o200k_base.")}),
			path: func(args map[string]any) (string, error) {
				_, err := required(args, "text")
				return "/v1/text/tokens" + query(args, map[string]string{"text": "text", "model": "model"}), err
			},
		},
		mcpTool{
			Name:        "redact_indian_identifiers",
			Description: "Mask Aadhaar, PAN, GSTIN, card, phone, email, UPI ID, vehicle, voter ID, passport and IP numbers in a text, checking check digits where they exist.",
			InputSchema: schema([]string{"text"}, map[string]any{
				"text": text("The text."), "style": oneOf("label replaces with [PAN]; partial keeps the last digits.", "label", "partial"),
			}),
			path: func(args map[string]any) (string, error) {
				_, err := required(args, "text")
				return "/v1/text/redact" + query(args, map[string]string{"text": "text", "style": "style"}), err
			},
		},
		mcpTool{
			Name:        "detect_language",
			Description: "Detect the language and script of a text, including Indian languages.",
			InputSchema: schema([]string{"text"}, map[string]any{"text": text("The text.")}),
			path: func(args map[string]any) (string, error) {
				_, err := required(args, "text")
				return "/v1/text/language" + query(args, map[string]string{"text": "text"}), err
			},
		},
	)
}
