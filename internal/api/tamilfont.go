package api

import (
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/tamilfont"
)

// Tamil text in the 8-bit encodings that came before Unicode ("legacy fonts"), to
// Unicode and back.

type tamilEncodingView struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	About string   `json:"about"`
	Fonts []string `json:"fonts"`
}

func (s *Server) handleTamilEncodings(w http.ResponseWriter, r *http.Request) {
	views := make([]tamilEncodingView, len(tamilfont.Encodings))
	for i, encoding := range tamilfont.Encodings {
		views[i] = tamilEncodingView{encoding.ID, encoding.Name, encoding.About, encoding.Fonts}
	}
	s.ok(w, r, views, meta{Note: "Convert with /v1/tamil-font/convert, from or to any of these and unicode."}, cacheLong)
}

type tamilConversionView struct {
	From string `json:"from"`
	To   string `json:"to"`
	Text string `json:"text"`
	// Base64 holds the converted bytes when converting to an 8-bit encoding; Text is then
	// those bytes as Windows-1252 characters, which is how such text is usually stored.
	Base64 string `json:"base64,omitempty"`
	// Undefined counts input bytes the encoding does not assign (each becomes U+FFFD).
	Undefined int `json:"undefined,omitempty"`
	// Unmapped lists characters the encoding has no code for (each becomes "?").
	Unmapped []string `json:"unmapped,omitempty"`
}

func tamilEncodingIDs() []string {
	ids := make([]string, len(tamilfont.Encodings))
	for i, encoding := range tamilfont.Encodings {
		ids[i] = encoding.ID
	}
	return ids
}

func (s *Server) handleTamilConvert(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	// Bytes exactly as they are in a file, for text that was never turned into characters.
	var raw []byte
	kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	binary := r.Method == http.MethodPost && kind == "application/octet-stream"
	var in textInput
	if binary {
		var err error
		raw, err = io.ReadAll(http.MaxBytesReader(w, r.Body, int64(s.cfg.MaxBodyBytes)))
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The body is larger than "+byteSize(s.cfg.MaxBodyBytes)+".")
			return
		case err != nil || len(raw) == 0:
			s.badRequest(w, r, "The request body is empty or could not be read.")
			return
		}
		in = textInput{options: map[string]string{}}
	} else {
		var ok bool
		if in, ok = s.readText(w, r); !ok {
			return
		}
	}

	from := strings.ToLower(strings.TrimSpace(in.option(r, "from")))
	to := strings.ToLower(strings.TrimSpace(in.option(r, "to")))
	known := "unicode, " + strings.Join(tamilEncodingIDs(), ", ")
	if from == "" {
		s.badRequest(w, r, "from is required: one of "+known+".")
		return
	}
	if to == "" && from != "unicode" {
		to = "unicode"
	}
	switch {
	case from == "unicode" && to != "unicode":
		encoding, ok := tamilfont.Lookup(to)
		if !ok {
			s.badRequest(w, r, "to must be one of "+strings.Join(tamilEncodingIDs(), ", ")+" when from is unicode.")
			return
		}
		if binary {
			s.badRequest(w, r, "Unicode text is sent as text/plain or JSON, not as application/octet-stream.")
			return
		}
		data, unmapped := encoding.FromUnicode(in.Text)
		s.ok(w, r, tamilConversionView{From: "unicode", To: encoding.ID, Text: tamilfont.View(data),
			Base64: base64.StdEncoding.EncodeToString(data), Unmapped: unmapped},
			meta{Note: "text is the converted bytes shown as Windows-1252 characters, the way this encoding is usually stored and pasted; base64 is the bytes themselves."}, "private, no-store")
	case to == "unicode" && from != "unicode":
		encoding, ok := tamilfont.Lookup(from)
		if !ok {
			s.badRequest(w, r, "from must be one of "+known+".")
			return
		}
		if !binary {
			var err error
			if raw, err = tamilfont.Bytes(in.Text); err != nil {
				message := "The text is not in an 8-bit encoding: " + err.Error() + "."
				if tamilfont.HasTamil(in.Text) {
					message = "The text is already Unicode Tamil. To convert the other way, use from=unicode&to=" + encoding.ID + "."
				}
				s.badRequest(w, r, message+" Raw bytes can be sent as application/octet-stream.")
				return
			}
		}
		text, undefined := encoding.ToUnicode(raw)
		s.ok(w, r, tamilConversionView{From: encoding.ID, To: "unicode", Text: text, Undefined: undefined}, meta{}, "private, no-store")
	default:
		s.badRequest(w, r, "One of from and to must be unicode, the other one of "+strings.Join(tamilEncodingIDs(), ", ")+".")
	}
}
