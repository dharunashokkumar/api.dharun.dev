package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
	"github.com/dharunashokkumar/api.dharun.dev/internal/words"
)

// ---- number to words ----

type wordsView struct {
	Input    string `json:"input"`
	Number   string `json:"number"`
	Language string `json:"language"`
	Style    string `json:"style"`
	Words    string `json:"words"`
}

var numberShape = regexp.MustCompile(`^(-?)([0-9]{1,15})(?:\.([0-9]{1,6}))?$`)

func (s *Server) handleWords(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	input := r.PathValue("number")
	info(r).subject = input

	language := strings.ToLower(query.Get("lang"))
	if language == "" {
		language = "en"
	}
	style := strings.ToLower(query.Get("style"))
	if style == "" {
		style = "number"
	}
	if style != "number" && style != "currency" {
		s.badRequest(w, r, "style must be number or currency.")
		return
	}

	// Accept the separators people actually type: 1,00,000 or 1 00 000 or 1_00_000.
	cleaned := strings.NewReplacer(",", "", " ", "", "_", "", "₹", "").Replace(input)
	match := numberShape.FindStringSubmatch(cleaned)
	if match == nil {
		s.badRequest(w, r, "Give a number with up to 15 digits before the decimal point and up to 6 after, for example 100000 or 1,00,000.50.")
		return
	}
	negative, fraction := match[1] == "-", match[3]
	whole, _ := strconv.ParseInt(match[2], 10, 64)

	var spelled string
	var err error
	if style == "currency" {
		if negative {
			s.badRequest(w, r, "A currency amount cannot be negative.")
			return
		}
		if len(fraction) > 2 {
			s.badRequest(w, r, "A currency amount has at most two decimal places.")
			return
		}
		paise, _ := strconv.Atoi((fraction + "00")[:2])
		spelled, err = words.Currency(language, whole, paise)
	} else {
		spelled, err = words.Number(language, negative, whole, fraction)
	}
	switch {
	case errors.Is(err, words.ErrUnsupportedLanguage):
		s.badRequest(w, r, "lang must be one of: "+strings.Join(words.Languages(), ", ")+".")
		return
	case err != nil:
		s.badRequest(w, r, "That number is out of range.")
		return
	}

	normalized := match[1] + strings.TrimLeft(match[2], "0")
	if strings.TrimLeft(match[2], "0") == "" {
		normalized = match[1] + "0"
	}
	if fraction != "" {
		normalized += "." + fraction
	}
	s.ok(w, r, wordsView{Input: input, Number: normalized, Language: language, Style: style, Words: spelled}, meta{}, cacheLong)
}

// ---- validators ----

type validationView struct {
	Kind string `json:"kind"`
	validate.Result
	// Exists is set for IFSC only: whether the code is in the branch dataset.
	Exists *bool `json:"exists,omitempty"`
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	kind := strings.ToLower(r.PathValue("kind"))
	value := r.PathValue("value")
	// A PAN identifies a person. Keep these values out of the access log entirely.
	info(r).private = true

	view := validationView{Kind: kind}
	switch kind {
	case "gstin":
		view.Result = validate.GSTIN(value)
	case "pan":
		view.Result = validate.PAN(value)
	case "ifsc":
		view.Result = validate.IFSC(value)
		if branches := s.data("ifsc"); view.Valid && branches != nil {
			var one int
			exists := branches.row(`SELECT 1 FROM ifsc WHERE code = ?`, view.Normalized).Scan(&one) == nil
			view.Exists = &exists
		}
	default:
		s.notFound(w, r, "Unknown identifier kind. Use gstin, pan or ifsc.")
		return
	}
	// The raw input is not echoed (the normalized form is enough), and the response
	// must not sit in a shared cache.
	view.Input = ""
	s.ok(w, r, view, meta{Note: "This checks the format only. It does not confirm that the identifier was ever issued."}, "private, no-store")
}

// ---- states ----

func (s *Server) handleStates(w http.ResponseWriter, r *http.Request) {
	total := len(india.States)
	s.ok(w, r, india.States, meta{Total: &total}, cacheLong)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	info(r).subject = code
	state, ok := india.Lookup(code)
	if !ok {
		s.notFound(w, r, `No state found for "`+code+`". See /v1/states.`)
		return
	}
	s.ok(w, r, state, meta{}, cacheLong)
}

// ---- health ----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	info(r).route = "health"
	states := map[string]datasetState{}
	for _, state := range s.store.states() {
		states[state.Name] = state
	}
	// The version comes first and in this exact form: the updater looks for it.
	datasets, _ := json.Marshal(states)
	body := `{"status":"ok","version":"` + s.cfg.Version + `","datasets":` + string(datasets) + "}\n"
	writeBody(w, r, http.StatusOK, []byte(body), typeJSON, cacheNone, false)
}

func weekday(date string) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	return parsed.Weekday().String()
}
