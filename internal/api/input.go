package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"
)

// textInput is what a text endpoint receives: the text, and options alongside it.
type textInput struct {
	Text    string
	options map[string]string
}

// option returns a named option from the JSON body, or else the query string.
func (in textInput) option(r *http.Request, name string) string {
	if value, ok := in.options[name]; ok {
		return value
	}
	return r.URL.Query().Get(name)
}

var errBodyTooLarge = errors.New("body too large")

// readText takes text three ways, so a caller can use whatever is at hand:
//
//	GET  ?text=...                       short inputs, easy to try in a browser
//	POST with Content-Type: text/plain   the body is the text; options go in the query
//	POST with Content-Type: application/json   {"text": "...", "option": "..."}
//
// It writes the error response itself and returns false when the input is unusable.
func (s *Server) readText(w http.ResponseWriter, r *http.Request) (textInput, bool) {
	input := textInput{options: map[string]string{}}
	if r.Method != http.MethodPost {
		input.Text = r.URL.Query().Get("text")
	} else {
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(s.cfg.MaxBodyBytes)))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The body is larger than "+byteSize(s.cfg.MaxBodyBytes)+".")
				return input, false
			}
			s.badRequest(w, r, "The request body could not be read.")
			return input, false
		}
		kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if kind == "application/json" {
			var fields map[string]any
			if err := json.Unmarshal(raw, &fields); err != nil {
				s.badRequest(w, r, `The body is not valid JSON. Send {"text": "..."}, or the text itself as text/plain.`)
				return input, false
			}
			for name, value := range fields {
				switch typed := value.(type) {
				case string:
					input.options[name] = typed
				case float64, bool:
					encoded, _ := json.Marshal(typed)
					input.options[name] = string(encoded)
				}
			}
			input.Text = input.options["text"]
			delete(input.options, "text")
		} else {
			input.Text = string(raw)
		}
	}
	if strings.TrimSpace(input.Text) == "" {
		s.badRequest(w, r, `No text given. Use ?text=..., or POST the text as text/plain or as JSON {"text": "..."}.`)
		return input, false
	}
	if !utf8.ValidString(input.Text) {
		s.badRequest(w, r, "The text is not valid UTF-8.")
		return input, false
	}
	if len(input.Text) > s.cfg.MaxBodyBytes {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The text is larger than "+byteSize(s.cfg.MaxBodyBytes)+".")
		return input, false
	}
	return input, true
}

func byteSize(bytes int) string {
	if bytes >= 1<<20 {
		return itoa(bytes>>20) + " MB"
	}
	return itoa(bytes>>10) + " KB"
}
