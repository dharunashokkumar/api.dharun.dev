package api

import (
	"encoding/json"
	"errors"
)

// MCP tools for the AI model endpoints: translation, transliteration of Roman typing,
// moderation, speech to text and reranking. Embeddings are in mcp_more.go.

func init() {
	for _, name := range []string{"translate_text", "transliterate_roman_typing", "moderate_text", "transcribe_audio", "rerank_documents"} {
		privateTools[name] = true
	}
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "translate_text",
			Description: "Translate text between English and 22 Indian languages (Hindi, Tamil, Telugu, Kannada, Malayalam, Bengali, Marathi, Gujarati, Punjabi, Odia, Assamese, Urdu and more) with AI4Bharat's IndicTrans2. Between two Indian languages it goes through English. Up to 2,000 characters.",
			InputSchema: schema([]string{"text", "from", "to"}, map[string]any{
				"text": text("The text, Indian languages in their own script."),
				"from": text("Language of the text: en, hi, ta, te, kn, ml, bn, mr, gu, pa, or, as, ur, sa, ne, ks, sd, mai, gom, brx, doi, mni, sat."),
				"to":   text("Language to translate into, same codes."),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				words, err := required(args, "text")
				return toolRequest{path: "/v1/translate" + query(args, map[string]string{"from": "from", "to": "to"}), contentType: "text/plain", body: []byte(words)}, err
			},
		},
		mcpTool{
			Name:        "transliterate_roman_typing",
			Description: "Turn Indian-language words typed in Roman letters (vanakkam, namaste, kaise ho) into their own script, as phone keyboards do, with several guesses per word. Uses AI4Bharat's IndicXlit. Up to 60 words.",
			InputSchema: schema([]string{"text", "lang"}, map[string]any{
				"text":  text("The words in Roman letters."),
				"lang":  text("Target language: ta, hi, te, kn, ml, bn, mr, gu, pa, or, as, ur, sa, ne, si, ks, sd, mai, gom, brx, mni."),
				"count": map[string]any{"type": "integer", "minimum": 1, "maximum": 8, "description": "Guesses per word. Default 4."},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				words, err := required(args, "text")
				return toolRequest{path: "/v1/transliterate/suggest" + query(args, map[string]string{"lang": "lang", "count": "count"}), contentType: "text/plain", body: []byte(words)}, err
			},
		},
		mcpTool{
			Name:        "moderate_text",
			Description: "Score a comment or message for abuse with three classifiers: general multilingual, Hinglish (Hindi in Roman letters, by kind: profanity, harassment, hate speech, caste, communal, regional, misogyny) and Tanglish (Tamil in Roman letters). Scores 0 to 1; a model's guess, not a decision.",
			InputSchema: schema([]string{"text"}, map[string]any{
				"text":      text("The text, up to 4,000 characters."),
				"threshold": map[string]any{"type": "number", "minimum": 0.01, "maximum": 0.99, "description": "Score at which flagged becomes true. Default 0.5."},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				words, err := required(args, "text")
				return toolRequest{path: "/v1/text/moderate" + query(args, map[string]string{"threshold": "threshold"}), contentType: "text/plain", body: []byte(words)}, err
			},
		},
		mcpTool{
			Name:        "transcribe_audio",
			Description: "Speech to text with Whisper: the text of a recording (MP3, WAV, Ogg, M4A, WebM or video), with timed segments; about a hundred languages, Indian ones included. task=translate writes it in English. The first five minutes are heard.",
			InputSchema: schema([]string{"file"}, map[string]any{
				"file":     map[string]any{"type": "string", "description": "The recording, base64-encoded (a data: URL also works). Up to 5 MB."},
				"language": text("auto (default) or the language spoken: en, hi, ta, te, kn, ml, mr, bn, gu, pa, ur..."),
				"task":     oneOf("transcribe (default) or translate (into English).", "transcribe", "translate"),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				data, err := fileArgument(args, "file")
				return toolRequest{path: "/v1/stt" + query(args, map[string]string{"language": "language", "task": "task"}), contentType: "application/octet-stream", body: data}, err
			},
		},
		mcpTool{
			Name:        "rerank_documents",
			Description: "Order documents by how well each answers a query, with the multilingual bge-reranker-v2-m3 cross-encoder (Indian languages and English). Returns scores from 0 to 1, best first.",
			InputSchema: schema([]string{"query", "documents"}, map[string]any{
				"query":     text("The question."),
				"documents": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Up to 100 candidate passages."},
				"top":       map[string]any{"type": "integer", "minimum": 1, "description": "Return only the best few."},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				question, err := required(args, "query")
				if err != nil {
					return toolRequest{}, err
				}
				documents, ok := args["documents"].([]any)
				if !ok || len(documents) == 0 {
					return toolRequest{}, errors.New("documents is required: a list of passages.")
				}
				body := map[string]any{"query": question, "documents": documents}
				if top, ok := args["top"].(float64); ok {
					body["top"] = int(top)
				}
				encoded, _ := json.Marshal(body)
				return toolRequest{path: "/v1/rerank", contentType: "application/json", body: encoded}, nil
			},
		},
	)
}
