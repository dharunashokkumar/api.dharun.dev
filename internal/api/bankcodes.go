package api

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// MICR and SWIFT codes, looked up in the IFSC data, which carries both for each branch.

var (
	micrShape  = regexp.MustCompile(`^\d{9}$`)
	swiftShape = regexp.MustCompile(`^[A-Z]{4}IN[A-Z0-9]{2}([A-Z0-9]{3})?$`)
)

func (s *Server) registerBankCodes() {
	s.handle("micr", costLight, "GET /v1/micr/{code}", s.handleBankCode)
	s.handle("swift", costLight, "GET /v1/swift/{code}", s.handleBankCode)
}

func (s *Server) handleBankCode(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "ifsc")
	if set == nil {
		return
	}
	code := strings.ToUpper(strings.Join(strings.Fields(r.PathValue("code")), ""))
	info(r).subject = code
	column, where := "micr", "i.micr = ?"
	args := []any{code}
	if strings.HasPrefix(r.URL.Path, "/v1/swift/") {
		if !swiftShape.MatchString(code) {
			s.badRequest(w, r, "A SWIFT (BIC) code of an Indian bank is 8 or 11 characters: four letters, IN, two letters or digits, and an optional branch part.")
			return
		}
		column, where = "swift", "(i.swift = ? OR i.swift = ? OR substr(i.swift, 1, 8) = ?)"
		args = []any{code, code + "XXX", code}
	} else if !micrShape.MatchString(code) {
		s.badRequest(w, r, "A MICR code is nine digits: city (3), bank (3), branch (3), as printed at the foot of a cheque.")
		return
	}
	rows, err := set.rows(`SELECT `+branchColumns+` FROM ifsc i JOIN banks b ON b.code = i.bank_code WHERE `+where+` ORDER BY i.code LIMIT 50`, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	branches := []branchView{}
	for rows.Next() {
		view, err := scanBranch(rows.Scan)
		if err == nil {
			branches = append(branches, view)
		}
	}
	if len(branches) == 0 {
		s.notFound(w, r, "No branch with "+strings.ToUpper(column)+" "+code+" in the IFSC data.")
		return
	}
	total := len(branches)
	note := "The branches whose IFSC record carries this MICR code."
	if column == "swift" {
		note = "The branches whose IFSC record carries this SWIFT code (an 8-character code matches every branch code under it). Many branches have none; foreign transfers often go through the bank's head office code."
	}
	s.ok(w, r, branches, meta{Total: &total, Source: s.source("ifsc"), Note: note}, cacheLong)
}

// ---- sectors ----

type sectorView struct {
	Sector     string           `json:"sector"`
	Companies  int              `json:"companies"`
	Industries []industryView   `json:"industries,omitempty"`
	Largest    []map[string]any `json:"largest,omitempty"`
}

type industryView struct {
	Industry  string `json:"industry"`
	Companies int    `json:"companies"`
}

func (s *Server) handleSectors(w http.ResponseWriter, r *http.Request) {
	index := s.companies.Load()
	if index == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "dataset_unavailable", "The company list is not loaded yet.")
		return
	}
	want := strings.TrimSpace(r.URL.Query().Get("sector"))
	info(r).subject = want
	bySector := map[string]*sectorView{}
	industries := map[string]map[string]int{}
	members := map[string][]*company{}
	for _, c := range index.all {
		if !c.Listed || c.Sector == "" {
			continue
		}
		v := bySector[c.Sector]
		if v == nil {
			v = &sectorView{Sector: c.Sector}
			bySector[c.Sector], industries[c.Sector] = v, map[string]int{}
		}
		v.Companies++
		if c.Industry != "" {
			industries[c.Sector][c.Industry]++
		}
		members[c.Sector] = append(members[c.Sector], c)
	}
	out := []sectorView{}
	for name, v := range bySector {
		if want != "" && !strings.EqualFold(want, name) {
			continue
		}
		if want != "" {
			for industry, n := range industries[name] {
				v.Industries = append(v.Industries, industryView{industry, n})
			}
			sort.Slice(v.Industries, func(i, j int) bool { return v.Industries[i].Companies > v.Industries[j].Companies })
			list := members[name]
			sort.Slice(list, func(i, j int) bool { return list[i].marketCap > list[j].marketCap })
			for _, c := range list[:min(10, len(list))] {
				v.Largest = append(v.Largest, map[string]any{"exchange": c.Exchange, "ticker": c.Ticker, "company": c.Company, "industry": c.Industry, "marketCap": c.MarketCap})
			}
		}
		out = append(out, *v)
	}
	if want != "" && len(out) == 0 {
		s.notFound(w, r, "No sector "+want+". /v1/sectors lists them.")
		return
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Companies > out[j].Companies })
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("logos"), Note: "Listed companies on NSE and BSE (a company on both counts twice), by the sector and industry in the company list."}, cacheAPI)
}
