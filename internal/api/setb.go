package api

// Set B of the idea catalogue (growth plan, 05): markets and company data.

import (
	"database/sql"
	"net/http"
	"strings"
)

func (s *Server) registerSetB() {
	s.handle("indices", costLight, "GET /v1/indices", s.handleIndices)
	s.handle("indices", costLight, "GET /v1/indices/{id}", s.handleMarketIndex)
}

type indexView struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Count   int           `json:"count"`
	Members []indexMember `json:"members,omitempty"`
}

type indexMember struct {
	Symbol   string  `json:"symbol"`
	Company  string  `json:"company"`
	Industry *string `json:"industry"`
	Series   *string `json:"series"`
	ISIN     *string `json:"isin"`
	Logo     string  `json:"logo"`
}

func (s *Server) handleIndices(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "indices")
	if set == nil {
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("symbol")))
	query, args := `SELECT id, name, members FROM indices ORDER BY members DESC, name`, []any{}
	if symbol != "" {
		info(r).subject = symbol
		query, args = `SELECT i.id, i.name, i.members FROM indices i JOIN members m ON m.index_id = i.id WHERE m.symbol = ? OR m.isin = ? ORDER BY i.members DESC, i.name`, []any{symbol, symbol}
	}
	rows, err := set.rows(query, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	out := []indexView{}
	for rows.Next() {
		var v indexView
		if rows.Scan(&v.ID, &v.Name, &v.Count) == nil {
			out = append(out, v)
		}
	}
	if symbol != "" && len(out) == 0 {
		s.notFound(w, r, symbol+" is in none of the NSE indices listed here.")
		return
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("indices")}, cacheAPI)
}

func (s *Server) handleMarketIndex(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "indices")
	if set == nil {
		return
	}
	id := strings.ToLower(strings.TrimSpace(r.PathValue("id")))
	id = strings.NewReplacer(" ", "-", "_", "-", "&", "and").Replace(id)
	info(r).subject = id
	var v indexView
	if err := set.row(`SELECT id, name, members FROM indices WHERE id = ? OR replace(id, '-', '') = replace(?, '-', '')`, id, id).Scan(&v.ID, &v.Name, &v.Count); err != nil {
		if err == sql.ErrNoRows {
			s.notFound(w, r, "No index "+id+". /v1/indices lists them.")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	industry := r.URL.Query().Get("industry")
	rows, err := set.rows(`SELECT symbol, company, industry, series, isin FROM members WHERE index_id = ? AND (? = '' OR industry = ?) ORDER BY company`, v.ID, industry, industry)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	origin := s.origin(r)
	for rows.Next() {
		var m indexMember
		var industry, series, isin sql.NullString
		if rows.Scan(&m.Symbol, &m.Company, &industry, &series, &isin) != nil {
			continue
		}
		m.Industry, m.Series, m.ISIN = str(industry), str(series), str(isin)
		m.Logo = origin + "/v1/logos/" + m.Symbol
		v.Members = append(v.Members, m)
	}
	s.ok(w, r, v, meta{Source: s.source("indices"), Note: "Constituents as NSE Indices publishes them, without weights. Each member links to its logo."}, cacheAPI)
}
