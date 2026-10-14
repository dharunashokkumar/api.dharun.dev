package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/regextest"
)

// The regex tester. Patterns and texts can be anything a caller is working on, so a
// request is private like the text tools.

type regexView struct {
	Flavor  string `json:"flavor"`
	Pattern string `json:"pattern"`
	Flags   string `json:"flags"`
	regextest.Result
}

const regexMatchLimit = 1000

// regexTimeout bounds a JavaScript-flavour run; RE2 needs no limit.
const regexTimeout = time.Second

func (s *Server) handleRegex(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	pattern := in.option(r, "pattern")
	if pattern == "" {
		s.badRequest(w, r, "Give the regular expression as pattern, and the text to test it on as text.")
		return
	}
	if len(pattern) > 4096 {
		s.badRequest(w, r, "The pattern is longer than 4096 bytes.")
		return
	}
	flavor := strings.ToLower(strings.TrimSpace(in.option(r, "flavor")))
	if flavor == "" {
		flavor = string(regextest.JavaScript)
	}
	if !contains(regextest.Flavors(), flavor) {
		s.badRequest(w, r, "flavor must be js (JavaScript) or re2 (Go, RE2).")
		return
	}
	flags := in.option(r, "flags")
	opts := regextest.Options{Flavor: regextest.Flavor(flavor), Timeout: regexTimeout}
	for _, flag := range flags {
		switch flag {
		case 'i':
			opts.IgnoreCase = true
		case 'm':
			opts.Multiline = true
		case 's':
			opts.DotAll = true
		case 'g', 'u':
			// Every match is always listed, and patterns are always read as Unicode.
		default:
			s.badRequest(w, r, "flags is any of i (ignore case), m (multi-line) and s (dot matches line breaks).")
			return
		}
	}
	limit, err := textOption(r, in, "limit", 100, 1, regexMatchLimit)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	opts.Limit = limit
	if replace, given := in.options["replace"]; given {
		opts.Replace = &replace
	} else if values, given := r.URL.Query()["replace"]; given {
		opts.Replace = &values[0]
	}

	result, err := regextest.Run(pattern, in.Text, opts)
	var patternError *regextest.PatternError
	switch {
	case errors.As(err, &patternError):
		s.fail(w, r, http.StatusUnprocessableEntity, "invalid_pattern", "The pattern is not valid "+flavorName(flavor)+": "+patternError.Message)
		return
	case errors.Is(err, regextest.ErrTimeout):
		s.fail(w, r, http.StatusUnprocessableEntity, "timeout",
			"The pattern ran for more than a second on this text and was stopped. Nested quantifiers such as (a+)+ can take exponentially long when a match fails; the re2 flavour never does.")
		return
	case err != nil:
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, regexView{Flavor: flavor, Pattern: pattern, Flags: flags, Result: result}, meta{Note: regexNotes[flavor]}, "private, no-store")
}

func flavorName(flavor string) string {
	if flavor == string(regextest.RE2) {
		return "RE2"
	}
	return "JavaScript"
}

var regexNotes = map[string]string{
	"js": "JavaScript syntax, run by regexp2 in ECMAScript mode. It agrees with browsers and Node.js in nearly every case; " +
		"a backreference such as \\1 in a pattern that mixes named and unnamed groups can count groups differently. " +
		"Offsets count characters (Unicode code points), not UTF-16 units as JavaScript's own index does.",
	"re2": "RE2 syntax, as Go's regexp package reads it: no lookaround and no backreferences, and matching time is linear in the text. " +
		"Offsets count characters (Unicode code points).",
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "test_regex",
		Description: "Run a regular expression (JavaScript or RE2/Go syntax) against a text: every match with its groups and character offsets, an optional replacement, and a token by token explanation of the pattern.",
		InputSchema: schema([]string{"pattern", "text"}, map[string]any{
			"pattern": text("The regular expression, without slashes."),
			"text":    text("The text to test it on."),
			"flavor":  oneOf("Default js.", "js", "re2"),
			"flags":   text("Any of i, m, s."),
			"replace": text("Replacement for each match: $1 and $<name> in js, ${1} and ${name} in re2."),
		}),
		path: func(args map[string]any) (string, error) {
			if _, err := required(args, "pattern"); err != nil {
				return "", err
			}
			_, err := required(args, "text")
			return "/v1/regex" + query(args, map[string]string{"pattern": "pattern", "text": "text", "flavor": "flavor", "flags": "flags", "replace": "replace"}), err
		},
	})
}
