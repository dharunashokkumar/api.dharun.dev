package api

// The Local Government Directory (server-only dataset lgd): states, districts,
// sub-districts and villages with their LGD and Census 2011 codes, and village PIN codes.

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) registerLGD() {
	s.handle("lgd", costLight, "GET /v1/lgd/states", s.handleLGDStates)
	s.handle("lgd", costLight, "GET /v1/lgd/districts", s.handleLGDDistricts)
	s.handle("lgd", costLight, "GET /v1/lgd/subdistricts", s.handleLGDSubdistricts)
	s.handle("lgd", costLight, "GET /v1/lgd/villages", s.handleLGDVillages)
	s.handle("lgd", costLight, "GET /v1/lgd/villages/{code}", s.handleLGDVillage)
}

type lgdPlace struct {
	Code       int    `json:"code"`
	Name       string `json:"name"`
	NameLocal  string `json:"name_local,omitempty"`
	Census2011 string `json:"census2011_code,omitempty"`
	Count      *int   `json:"count,omitempty"` // districts in a state, sub-districts in a district, villages in a sub-district
}

type lgdVillage struct {
	Code        int    `json:"code"`
	Name        string `json:"name"`
	NameLocal   string `json:"name_local,omitempty"`
	Census2011  string `json:"census2011_code,omitempty"`
	Pincode     string `json:"pincode,omitempty"`
	Subdistrict string `json:"subdistrict"`
	SubCode     int    `json:"subdistrict_code"`
	District    string `json:"district"`
	DistCode    int    `json:"district_code"`
	State       string `json:"state"`
	StateCode   int    `json:"state_code"`
}

const lgdNote = "Codes are the Local Government Directory's (LGD), which government systems use; census2011_code is the Census of 2011's. LGD updates the lists monthly."

func (s *Server) lgdList(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	set := s.need(w, r, "lgd")
	if set == nil {
		return
	}
	rows, err := set.rows(query, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	out := []lgdPlace{}
	for rows.Next() {
		var p lgdPlace
		var count int
		if rows.Scan(&p.Code, &p.Name, &p.NameLocal, &p.Census2011, &count) == nil {
			p.Count = &count
			out = append(out, p)
		}
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("lgd"), Note: lgdNote}, cacheAPI)
}

func (s *Server) handleLGDStates(w http.ResponseWriter, r *http.Request) {
	s.lgdList(w, r, `SELECT state_code, state, max(state_local), '', count(*) FROM districts GROUP BY state_code ORDER BY state`)
}

// lgdCodeParam reads a numeric code from the query, answering the request when it is
// missing or wrong.
func (s *Server) lgdCodeParam(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	code, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(name)))
	if err != nil || code <= 0 {
		s.badRequest(w, r, name+" is an LGD code, a number (see the list one level up).")
		return 0, false
	}
	return code, true
}

func (s *Server) handleLGDDistricts(w http.ResponseWriter, r *http.Request) {
	state, ok := s.lgdCodeParam(w, r, "state")
	if !ok {
		return
	}
	s.lgdList(w, r, `SELECT d.code, d.name, d.name_local, d.census2011, (SELECT count(*) FROM subdistricts s WHERE s.district_code = d.code)
FROM districts d WHERE d.state_code = ? ORDER BY d.name`, state)
}

func (s *Server) handleLGDSubdistricts(w http.ResponseWriter, r *http.Request) {
	district, ok := s.lgdCodeParam(w, r, "district")
	if !ok {
		return
	}
	s.lgdList(w, r, `SELECT s.code, s.name, s.name_local, s.census2011, (SELECT count(*) FROM villages v WHERE v.subdistrict_code = s.code)
FROM subdistricts s WHERE s.district_code = ? ORDER BY s.name`, district)
}

const lgdVillageSelect = `SELECT v.code, v.name, v.name_local, v.census2011, coalesce(v.pincode, ''), s.name, s.code, d.name, d.code, d.state, d.state_code
FROM villages v JOIN subdistricts s ON s.code = v.subdistrict_code JOIN districts d ON d.code = s.district_code WHERE `

func (s *Server) lgdVillages(w http.ResponseWriter, r *http.Request, set *handle, where string, args ...any) ([]lgdVillage, bool) {
	rows, err := set.rows(lgdVillageSelect+where, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return nil, false
	}
	defer rows.Close()
	out := []lgdVillage{}
	for rows.Next() {
		var v lgdVillage
		if rows.Scan(&v.Code, &v.Name, &v.NameLocal, &v.Census2011, &v.Pincode, &v.Subdistrict, &v.SubCode, &v.District, &v.DistCode, &v.State, &v.StateCode) == nil {
			out = append(out, v)
		}
	}
	return out, true
}

func (s *Server) handleLGDVillages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var where string
	var args []any
	switch {
	case query.Get("subdistrict") != "":
		code, ok := s.lgdCodeParam(w, r, "subdistrict")
		if !ok {
			return
		}
		where, args = `v.subdistrict_code = ? ORDER BY v.name LIMIT 5000`, []any{code}
	case query.Get("pincode") != "":
		pin := strings.TrimSpace(query.Get("pincode"))
		if len(pin) != 6 || strings.Trim(pin, "0123456789") != "" {
			s.badRequest(w, r, "pincode is six digits.")
			return
		}
		info(r).subject = pin
		where, args = `v.pincode = ? ORDER BY v.name LIMIT 1000`, []any{pin}
	case query.Get("q") != "":
		q := strings.TrimSpace(query.Get("q"))
		if len(q) < 3 || len(q) > 80 {
			s.badRequest(w, r, "q is 3 to 80 characters of a village's name.")
			return
		}
		info(r).subject = q
		where, args = `v.name LIKE ? COLLATE NOCASE`, []any{strings.NewReplacer("%", "", "_", "").Replace(q) + "%"}
		if state := strings.TrimSpace(query.Get("state")); state != "" {
			code, err := strconv.Atoi(state)
			if err != nil {
				s.badRequest(w, r, "state is an LGD state code (see /v1/lgd/states).")
				return
			}
			where, args = where+` AND d.state_code = ?`, append(args, code)
		}
		where += ` ORDER BY v.name, d.state LIMIT 100`
	default:
		s.badRequest(w, r, "Give subdistrict (an LGD code), pincode, or q (the start of a village's name, with state to narrow it).")
		return
	}
	set := s.need(w, r, "lgd")
	if set == nil {
		return
	}
	out, ok := s.lgdVillages(w, r, set, where, args...)
	if !ok {
		return
	}
	total := len(out)
	note := lgdNote
	if query.Get("pincode") != "" {
		note = "Villages LGD lists under this PIN code; towns and city wards are not villages and are not here. " + lgdNote
	}
	s.ok(w, r, out, meta{Total: &total, Source: s.source("lgd"), Note: note}, cacheAPI)
}

func (s *Server) handleLGDVillage(w http.ResponseWriter, r *http.Request) {
	code, err := strconv.Atoi(r.PathValue("code"))
	if err != nil || code <= 0 {
		s.badRequest(w, r, "The village code is a number.")
		return
	}
	set := s.need(w, r, "lgd")
	if set == nil {
		return
	}
	out, ok := s.lgdVillages(w, r, set, `v.code = ?`, code)
	if !ok {
		return
	}
	if len(out) == 0 {
		s.notFound(w, r, "No village has the LGD code "+strconv.Itoa(code)+".")
		return
	}
	s.ok(w, r, out[0], meta{Source: s.source("lgd"), Note: lgdNote}, cacheAPI)
}
