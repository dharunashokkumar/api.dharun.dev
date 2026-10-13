package api

import (
	"net/http"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/translit"
)

type transliterationView struct {
	Text string `json:"text"`
	From string `json:"from"`
	To   string `json:"to"`
}

func transliterationTargets() []string {
	targets := append([]string{}, translit.Romanisations...)
	for _, script := range translit.Scripts {
		targets = append(targets, script.ID)
	}
	return targets
}

func (s *Server) handleTransliterate(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	to := strings.ToLower(strings.TrimSpace(in.option(r, "to")))
	from := strings.ToLower(strings.TrimSpace(in.option(r, "from")))
	if to == "" {
		s.badRequest(w, r, "to is required: a script ("+strings.Join(transliterationTargets()[3:], ", ")+", or a language code such as ta or hi) or a romanisation (iso15919, iast, ascii).")
		return
	}
	view := transliterationView{To: to}
	romanTarget := contains(translit.Romanisations, to)
	target, scriptTarget := translit.Lookup(to)
	if !romanTarget && !scriptTarget {
		s.badRequest(w, r, "to must be a script, a language code or one of iso15919, iast, ascii.")
		return
	}
	if scriptTarget {
		view.To = target.ID
	}

	switch from {
	case "iso15919", "iast":
		if !scriptTarget {
			s.badRequest(w, r, "From a romanisation, to must be a script.")
			return
		}
		converted, err := translit.FromRoman(in.Text, from, target)
		if err != nil {
			s.badRequest(w, r, "That conversion is not offered.")
			return
		}
		view.Text, view.From = converted, from
	case "ascii":
		s.badRequest(w, r, "Casual Roman spelling (such as vanakkam) cannot be read back by rule: it does not say which n or which t is meant. Give text in ISO 15919 or IAST, such as vaṇakkam.")
		return
	case "", "auto":
		source, found := translit.Detect(in.Text)
		if !found {
			s.badRequest(w, r, "The text has no letters of an Indian script. To convert Roman letters, give from=iso15919 or from=iast.")
			return
		}
		view.From = source.ID
		view.Text = convertFromScript(in.Text, to, target, romanTarget)
	default:
		source, found := translit.Lookup(from)
		if !found {
			s.badRequest(w, r, "from must be auto, a script, a language code, iso15919 or iast.")
			return
		}
		view.From = source.ID
		view.Text = convertFromScript(in.Text, to, target, romanTarget)
	}
	s.ok(w, r, view, meta{Note: "Converted by rule, letter by letter: the spelling changes, not the language. Letters a script lacks become the nearest one it has."}, "private, no-store")
}

func convertFromScript(text, to string, target translit.Script, romanTarget bool) string {
	if romanTarget {
		converted, _ := translit.Romanise(text, to)
		return converted
	}
	return translit.ToScript(text, target)
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "transliterate_indian_text",
		Description: "Rewrite text from one Indian script in another (Devanagari, Bengali, Gurmukhi, Gujarati, Odia, Tamil, Telugu, Kannada, Malayalam), or in Roman letters (ISO 15919, IAST, plain ASCII), or from ISO 15919 / IAST into a script. Spelling only, not translation.",
		InputSchema: schema([]string{"text", "to"}, map[string]any{
			"text": text("The text."),
			"to":   text("Target: a script name, a language code such as ta or hi, or iso15919, iast, ascii."),
			"from": text("Source: auto (default), a script, or iso15919 / iast for romanised input."),
		}),
		path: func(args map[string]any) (string, error) {
			_, err := required(args, "text")
			if err == nil {
				_, err = required(args, "to")
			}
			return "/v1/transliterate" + query(args, map[string]string{"text": "text", "to": "to", "from": "from"}), err
		},
	})
}
