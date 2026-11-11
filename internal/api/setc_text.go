package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/indic"
)

func (s *Server) registerSetCText() {
	both := func(route, path string, handler http.HandlerFunc) {
		s.handle(route, costLight, "GET "+path, handler)
		s.handle(route, costLight, "POST "+path, handler)
	}
	both("braille", "/v1/braille", s.handleBraille)
	both("keyboard", "/v1/keyboard/convert", s.handleKeyboard)
	both("indic", "/v1/indic/aksharas", s.handleAksharas)
	both("indic", "/v1/indic/normalise", s.handleNormalise)
	both("indic", "/v1/indic/sort", s.handleIndicSort)
	s.handle("helplines", costLight, "GET /v1/helplines", s.handleHelplines)
}

func (s *Server) handleBraille(w http.ResponseWriter, r *http.Request) {
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	if len([]rune(in.Text)) > 5000 {
		s.badRequest(w, r, "At most 5,000 characters.")
		return
	}
	s.ok(w, r, indic.ToBraille(in.Text), meta{Note: "Bharati braille, Grade 1, as Unicode braille cells. Other Indian scripts are written by sound through Devanagari, as Bharati braille intends. Check with a braille reader before printing material for others."}, cacheLong)
}

func (s *Server) handleKeyboard(w http.ResponseWriter, r *http.Request) {
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	layout := strings.ToLower(in.option(r, "layout"))
	if layout == "" {
		layout = "inscript-hindi"
	}
	direction := strings.ToLower(in.option(r, "direction"))
	switch direction {
	case "", "type":
		out, err := indic.Type(in.Text, layout)
		if err != nil {
			s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
			return
		}
		s.ok(w, r, map[string]any{"layout": layout, "keys": in.Text, "text": out}, meta{Note: "What typing those QWERTY keys gives with the layout switched on: the fix for text typed with the wrong keyboard selected."}, cacheLong)
	case "keys":
		keys, missing, err := indic.Keys(in.Text, layout)
		if err != nil {
			s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
			return
		}
		s.ok(w, r, map[string]any{"layout": layout, "text": in.Text, "keys": keys, "notOnLayout": missing}, meta{Note: "The QWERTY keys to press on the layout to type the text."}, cacheLong)
	default:
		s.badRequest(w, r, "direction is type (keys to text) or keys (text to keys).")
	}
}

func (s *Server) handleAksharas(w http.ResponseWriter, r *http.Request) {
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	if len([]rune(in.Text)) > 20000 {
		s.badRequest(w, r, "At most 20,000 characters.")
		return
	}
	s.ok(w, r, map[string]any{"aksharas": indic.Aksharas(in.Text), "count": indic.CountText(in.Text)},
		meta{Note: "Aksharas are the letters a reader counts: a consonant with its vowel sign, and conjuncts as one (क्ष). In Tamil a consonant with pulli (க்) counts as its own letter."}, cacheLong)
}

func (s *Server) handleNormalise(w http.ResponseWriter, r *http.Request) {
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	text, fixes := indic.Normalise(in.Text, in.option(r, "strip") == "true")
	s.ok(w, r, map[string]any{"text": text, "changed": text != in.Text, "fixes": fixes},
		meta{Note: "Unicode NFC, then sequences that only look like a letter (अ + ा for आ, old Malayalam chillu forms, a doubled virama) replaced by the letter. strip=true also removes zero-width joiners and spaces."}, cacheLong)
}

func (s *Server) handleIndicSort(w http.ResponseWriter, r *http.Request) {
	var words []string
	lang := r.URL.Query().Get("lang")
	if r.Method == http.MethodPost {
		raw, ok := s.readJSONBody(w, r)
		if !ok {
			return
		}
		var body struct {
			Words []string `json:"words"`
			Lang  string   `json:"lang"`
		}
		if json.Unmarshal(raw, &body) != nil {
			s.badRequest(w, r, `Send JSON {"words": [...], "lang": "ta"}.`)
			return
		}
		words = body.Words
		if body.Lang != "" {
			lang = body.Lang
		}
	} else if raw := r.URL.Query().Get("words"); raw != "" {
		words = strings.Split(raw, ",")
	}
	if len(words) == 0 || len(words) > 10000 {
		s.badRequest(w, r, "Give 1 to 10,000 words: words=a,b,c, or POST {\"words\": [...], \"lang\": \"ta\"}.")
		return
	}
	if lang == "" {
		lang = "hi"
	}
	sorted, err := indic.Sort(words, lang)
	if err != nil {
		s.badRequest(w, r, "lang is a language code such as hi, ta, bn, te, kn, ml, mr, gu, pa or en.")
		return
	}
	s.ok(w, r, map[string]any{"lang": lang, "words": sorted}, meta{Note: "Dictionary order by Unicode's collation rules (CLDR) for the language."}, cacheLong)
}

// ---- helplines ----

type helpline struct {
	Number string `json:"number"`
	Name   string `json:"name"`
	For    string `json:"for"`
	Free   bool   `json:"tollFree"`
	Hours  string `json:"hours"`
	RunBy  string `json:"runBy"`
}

// helplines are national numbers, typed from the ministries' own pages. State
// helplines differ and are not listed.
var helplines = []helpline{
	{"112", "Emergency Response Support System", "Any emergency: police, fire, ambulance", true, "24x7", "Ministry of Home Affairs"},
	{"100", "Police", "Police", true, "24x7", "State police"},
	{"101", "Fire", "Fire service", true, "24x7", "State fire service"},
	{"108", "Emergency ambulance", "Medical emergency ambulance (most states)", true, "24x7", "State health departments"},
	{"102", "Ambulance for mothers and children", "Pregnant women and infants", true, "24x7", "National Health Mission"},
	{"1091", "Women in distress", "Women's safety (police)", true, "24x7", "State police"},
	{"181", "Women helpline", "Women affected by violence", true, "24x7", "Ministry of Women and Child Development"},
	{"1098", "Childline", "Children in need of care and protection", true, "24x7", "Ministry of Women and Child Development"},
	{"14567", "Elderline", "Senior citizens", true, "8 am to 8 pm", "Ministry of Social Justice and Empowerment"},
	{"1930", "Cyber crime helpline", "Report online financial fraud at once (also cybercrime.gov.in)", true, "24x7", "Indian Cyber Crime Coordination Centre"},
	{"14416", "Tele-MANAS", "Mental health support", true, "24x7", "Ministry of Health and Family Welfare"},
	{"104", "Health helpline", "Health information and advice (most states)", true, "24x7", "State health departments"},
	{"1097", "National AIDS helpline", "HIV and AIDS information", true, "24x7", "National AIDS Control Organisation"},
	{"1078", "Disaster management", "Disasters and relief", true, "24x7", "National Disaster Management Authority"},
	{"1033", "National Highways helpline", "Accidents and help on national highways", true, "24x7", "NHAI"},
	{"139", "Rail Madad", "Railway enquiries, complaints and help on trains", true, "24x7", "Indian Railways"},
	{"182", "Railway Protection Force", "Security on trains and stations", true, "24x7", "RPF"},
	{"1906", "LPG emergency", "Gas leaks", true, "24x7", "Ministry of Petroleum and Natural Gas"},
	{"1912", "Electricity complaints", "Power cuts and faults (many states)", true, "24x7", "State power utilities"},
	{"1915", "National Consumer Helpline", "Consumer complaints", true, "8 am to 8 pm", "Department of Consumer Affairs"},
	{"1950", "Voter helpline", "Electoral rolls and voter ID", true, "Office hours", "Election Commission of India"},
	{"15100", "Legal aid", "Free legal services", true, "Office hours", "National Legal Services Authority"},
	{"1800-180-1551", "Kisan Call Centre", "Farming questions, in local languages", true, "6 am to 10 pm", "Ministry of Agriculture"},
	{"1363", "Tourist helpline", "Tourists in India, in many languages", true, "24x7", "Ministry of Tourism"},
}

func (s *Server) handleHelplines(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	out := []helpline{}
	for _, h := range helplines {
		if q == "" || strings.Contains(strings.ToLower(h.Name+" "+h.For+" "+h.Number+" "+h.RunBy), q) {
			out = append(out, h)
		}
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Note: "National numbers, free from any phone. In an emergency call 112. State helplines are not listed."}, cacheLong)
}
