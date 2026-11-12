package api

// STD codes (dataset std-codes): the landline trunk code of each Short Distance
// Charging Area, and which area a landline number belongs to.

import (
	"net/http"
	"strings"
)

func (s *Server) registerSTDCodes() {
	s.handle("std-codes", costLight, "GET /v1/std-codes", s.handleSTDSearch)
	s.handle("std-codes", costLight, "GET /v1/std-codes/{code}", s.handleSTDCode)
}

type stdArea struct {
	Code       string `json:"code"`
	Dial       string `json:"dial"` // with the trunk prefix 0
	Area       string `json:"area"` // the SDCA
	LDCA       string `json:"long_distance_area"`
	Circle     string `json:"circle"`
	CircleName string `json:"circle_name"`
}

const stdNote = "Landline STD codes from the Department of Telecommunications' National Numbering Plan (2003). Area is the Short Distance Charging Area; circle is the telecom service area, not always the state (Andhra Pradesh includes Telangana, North East is seven states). A landline number is the code and the subscriber number, ten digits in all; mobile numbers have no STD code."

const stdSelect = `SELECT code, sdca, ldca, circle, circle_name FROM sdca WHERE `

func (s *Server) stdRows(w http.ResponseWriter, r *http.Request, set *handle, where string, args ...any) ([]stdArea, bool) {
	rows, err := set.rows(stdSelect+where, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return nil, false
	}
	defer rows.Close()
	out := []stdArea{}
	for rows.Next() {
		var a stdArea
		if rows.Scan(&a.Code, &a.Area, &a.LDCA, &a.Circle, &a.CircleName) == nil {
			a.Dial = "0" + a.Code
			out = append(out, a)
		}
	}
	return out, true
}

// handleSTDCode answers for a code, or for a landline number: the longest code that
// starts the number is its area (codes are a prefix code: no code starts another).
func (s *Server) handleSTDCode(w http.ResponseWriter, r *http.Request) {
	raw := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(r.PathValue("code"))
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "+91"), "0")
	if raw == "" || len(raw) > 10 || strings.Trim(raw, "0123456789") != "" {
		s.badRequest(w, r, "code is an STD code such as 44 or 0422, or a landline number such as 0422 2345678.")
		return
	}
	info(r).subject = raw[:min(len(raw), 5)]
	if len(raw) > 5 {
		info(r).private = true // a whole number may be someone's
	}
	set := s.need(w, r, "std-codes")
	if set == nil {
		return
	}
	// A code on its own must match whole; a number is matched by its longest prefix.
	shortest := 2
	if len(raw) <= 5 {
		shortest = len(raw)
	}
	var out []stdArea
	for n := min(len(raw), 5); n >= shortest && len(out) == 0; n-- {
		var ok bool
		if out, ok = s.stdRows(w, r, set, `code = ?`, raw[:n]); !ok {
			return
		}
	}
	if len(out) == 0 {
		s.notFound(w, r, "No area has the STD code "+raw+".")
		return
	}
	s.ok(w, r, out[0], meta{Source: s.source("std-codes"), Note: stdNote}, cacheAPI)
}

func (s *Server) handleSTDSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	circle := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("circle")))
	if (len(q) < 3 || len(q) > 60) && circle == "" {
		s.badRequest(w, r, "q is 3 to 60 letters of a town's name, such as coimbatore; or circle, such as TN.")
		return
	}
	info(r).subject = q + circle
	set := s.need(w, r, "std-codes")
	if set == nil {
		return
	}
	where, args := "1 = 1", []any{}
	if q != "" {
		where += ` AND search LIKE ?`
		args = append(args, "%"+strings.NewReplacer("%", "", "_", "").Replace(q)+"%")
	}
	if circle != "" {
		where += ` AND circle = ?`
		args = append(args, circle)
	}
	out, ok := s.stdRows(w, r, set, where+` ORDER BY (lower(sdca) = ?) DESC, length(code), sdca LIMIT 500`, append(args, q)...)
	if !ok {
		return
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("std-codes"), Note: stdNote}, cacheAPI)
}
