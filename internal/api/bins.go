package api

// Card number prefixes (server-only dataset bins): which network, kind of card and bank
// a card number's first six to eight digits belong to.

import (
	"net/http"
	"strings"
)

func (s *Server) registerBins() {
	s.handle("bin", costLight, "GET /v1/bin/{prefix}", s.handleBin)
}

type binView struct {
	BIN         string `json:"bin"`
	Network     string `json:"network"`
	Type        string `json:"type"`
	Category    string `json:"category"`
	Issuer      string `json:"issuer"`
	IssuerPhone string `json:"issuer_phone"`
	IssuerURL   string `json:"issuer_url"`
	Country     string `json:"country"`
	CountryName string `json:"country_name"`
}

const binNote = "From an open BIN list (CC BY 4.0) that is not the card networks' own: banks move prefixes between products, so treat the answer as a hint. Send only the first six to eight digits, never a whole card number."

func (s *Server) handleBin(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	raw := strings.NewReplacer(" ", "", "-", "").Replace(r.PathValue("prefix"))
	if len(raw) < 6 || strings.Trim(raw, "0123456789") != "" {
		s.badRequest(w, r, "prefix is the first 6 to 8 digits of a card number.")
		return
	}
	if len(raw) > 8 {
		// Never look up, log or echo more than the prefix.
		s.badRequest(w, r, "Send only the first 6 to 8 digits of a card, not the whole number.")
		return
	}
	set := s.need(w, r, "bins")
	if set == nil {
		return
	}
	// Some lists carry 8-digit ranges: the longest prefix that is known wins.
	for n := len(raw); n >= 6; n-- {
		var v binView
		err := set.row(`SELECT bin, brand, type, category, issuer, issuer_phone, issuer_url, country, country_name FROM bins WHERE bin = ?`, raw[:n]).Scan(
			&v.BIN, &v.Network, &v.Type, &v.Category, &v.Issuer, &v.IssuerPhone, &v.IssuerURL, &v.Country, &v.CountryName)
		if err == nil {
			s.ok(w, r, v, meta{Source: s.source("bins"), Note: binNote}, cacheAPI)
			return
		}
	}
	s.notFound(w, r, "The prefix "+raw[:6]+" is not in the BIN list.")
}
