package api

import (
	"net/http"
	"strconv"
	"strings"
)

// Gold, silver and platinum rates from IBJA (the server-only bullion dataset).

func (s *Server) registerBullion() {
	s.handle("bullion", costLight, "GET /v1/bullion", s.handleBullion)
	s.handle("bullion", costLight, "GET /v1/bullion/history", s.handleBullionHistory)
}

var karats = map[string]string{"999": "24K", "995": "23.9K", "916": "22K", "750": "18K", "585": "14K"}

type bullionView struct {
	Metal    string   `json:"metal"`
	Purity   string   `json:"purity"`
	Karat    string   `json:"karat,omitempty"`
	Session  string   `json:"session"`
	Rupees   float64  `json:"rupees"`
	Unit     string   `json:"unit"`
	PerGram  float64  `json:"perGram"`
	Change   *float64 `json:"change,omitempty"` // from the session before
	Previous string   `json:"previousDate,omitempty"`
}

const bullionNote = "IBJA's benchmark rates, before GST (3%) and making charges; jewellers' prices differ. AM and PM are the day's two sessions."

func gramsIn(unit string) float64 {
	if unit == "1 kg" {
		return 1000
	}
	return 10
}

func (s *Server) handleBullion(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "bullion")
	if set == nil {
		return
	}
	var latest string
	if err := set.row(`SELECT max(date) FROM rates`).Scan(&latest); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.rows(`SELECT r.metal, r.purity, r.session, r.rupees, r.unit,
  (SELECT p.rupees FROM rates p WHERE p.metal = r.metal AND p.purity = r.purity AND (p.date < r.date OR (p.date = r.date AND p.session < r.session)) ORDER BY p.date DESC, p.session DESC LIMIT 1),
  (SELECT p.date FROM rates p WHERE p.metal = r.metal AND p.purity = r.purity AND (p.date < r.date OR (p.date = r.date AND p.session < r.session)) ORDER BY p.date DESC, p.session DESC LIMIT 1)
FROM rates r WHERE r.date = ? AND r.session = (SELECT max(session) FROM rates WHERE date = r.date AND metal = r.metal AND purity = r.purity)
ORDER BY CASE r.metal WHEN 'gold' THEN 0 WHEN 'silver' THEN 1 ELSE 2 END, r.purity DESC`, latest)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	out := []bullionView{}
	for rows.Next() {
		var v bullionView
		var previous *float64
		var previousDate *string
		if rows.Scan(&v.Metal, &v.Purity, &v.Session, &v.Rupees, &v.Unit, &previous, &previousDate) != nil {
			continue
		}
		if v.Metal == "gold" {
			v.Karat = karats[v.Purity]
		}
		v.PerGram = roundCents(v.Rupees / gramsIn(v.Unit))
		if previous != nil {
			change := roundCents(v.Rupees - *previous)
			v.Change = &change
			v.Previous = *previousDate
		}
		out = append(out, v)
	}
	s.ok(w, r, map[string]any{"date": latest, "rates": out}, meta{Source: s.source("bullion"), Note: bullionNote}, "public, max-age=600")
}

func (s *Server) handleBullionHistory(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "bullion")
	if set == nil {
		return
	}
	query := r.URL.Query()
	metal := strings.ToLower(query.Get("metal"))
	if metal == "" {
		metal = "gold"
	}
	purity := query.Get("purity")
	if purity == "" {
		purity = "999"
	}
	days := 30
	if raw := query.Get("days"); raw != "" {
		var err error
		if days, err = strconv.Atoi(raw); err != nil || days < 1 || days > 3650 {
			s.badRequest(w, r, "days is 1 to 3650.")
			return
		}
	}
	rows, err := set.rows(`SELECT date, session, rupees, unit FROM rates WHERE metal = ? AND purity = ? AND date >= date((SELECT max(date) FROM rates), ?) ORDER BY date, session`,
		metal, purity, "-"+strconv.Itoa(days)+" day")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	type point struct {
		Date    string  `json:"date"`
		Session string  `json:"session"`
		Rupees  float64 `json:"rupees"`
		PerGram float64 `json:"perGram"`
	}
	out := []point{}
	unit := ""
	for rows.Next() {
		var p point
		if rows.Scan(&p.Date, &p.Session, &p.Rupees, &unit) == nil {
			p.PerGram = roundCents(p.Rupees / gramsIn(unit))
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		s.notFound(w, r, "No rates for "+metal+" "+purity+". Metals: gold (999, 995, 916, 750, 585), silver 999, platinum 999.")
		return
	}
	total := len(out)
	s.ok(w, r, map[string]any{"metal": metal, "purity": purity, "unit": unit, "rates": out}, meta{Total: &total, Source: s.source("bullion"), Note: bullionNote + " History starts on the day this server began recording."}, "public, max-age=600")
}
