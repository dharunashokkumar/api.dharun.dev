package api

// Census of India 2011, Primary Census Abstract (dataset census): India, states,
// districts, sub-districts and towns, each with its figures as a total and split into
// rural and urban, and the rates worked out from them.

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

func (s *Server) registerCensus() {
	s.handle("census", costLight, "GET /v1/census", s.handleCensusSearch)
	s.handle("census", costLight, "GET /v1/census/{level}/{code}", s.handleCensusPlace)
}

type censusFigures map[string]int64

type censusRates struct {
	Literacy      *float64 `json:"literacy_rate"`        // per cent of people aged 7 and over
	MaleLiteracy  *float64 `json:"male_literacy_rate"`   // per cent
	FemaleLit     *float64 `json:"female_literacy_rate"` // per cent
	SexRatio      *int     `json:"sex_ratio"`            // females per 1,000 males
	ChildSexRatio *int     `json:"child_sex_ratio"`      // girls per 1,000 boys aged 0 to 6
	WorkRate      *float64 `json:"work_participation_rate"`
	Urban         *float64 `json:"urban_share,omitempty"` // per cent of people living in towns
}

type censusPlace struct {
	Level       string                   `json:"level"`
	Code        string                   `json:"code"`
	Name        string                   `json:"name"`
	State       *censusRef               `json:"state,omitempty"`
	District    *censusRef               `json:"district,omitempty"`
	Subdistrict *censusRef               `json:"subdistrict,omitempty"`
	Figures     map[string]censusFigures `json:"figures,omitempty"` // total, rural, urban
	Rates       *censusRates             `json:"rates,omitempty"`
	Population  *int64                   `json:"population,omitempty"` // in lists
	Children    []censusChild            `json:"children,omitempty"`
}

type censusRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type censusChild struct {
	Level      string `json:"level"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Population int64  `json:"population"`
}

const censusNote = "Census of India 2011 (Primary Census Abstract), the latest published count. Codes are the 2011 census's: states 2 digits, districts 3, sub-districts 5, towns 6 (see /v1/lgd for today's codes). Literacy is of people aged 7 and over. Boundaries are those of 2011: districts made since are not listed."

var censusCodeShape = map[string]*regexp.Regexp{
	"india": regexp.MustCompile(`^0$`), "state": regexp.MustCompile(`^\d{1,2}$`), "district": regexp.MustCompile(`^\d{1,3}$`),
	"subdistrict": regexp.MustCompile(`^\d{1,5}$`), "town": regexp.MustCompile(`^\d{6}$`),
}

var censusWidth = map[string]int{"state": 2, "district": 3, "subdistrict": 5, "town": 6, "india": 1}

func censusPad(level, code string) string {
	for len(code) < censusWidth[level] {
		code = "0" + code
	}
	return code
}

func (s *Server) handleCensusPlace(w http.ResponseWriter, r *http.Request) {
	level := strings.ToLower(r.PathValue("level"))
	code := r.PathValue("code")
	shape, ok := censusCodeShape[level]
	if !ok {
		s.notFound(w, r, "level is india, state, district, subdistrict or town.")
		return
	}
	if !shape.MatchString(code) {
		s.badRequest(w, r, "code is the 2011 census code of the "+level+" (find it with /v1/census?q=name).")
		return
	}
	code = censusPad(level, code)
	info(r).subject = level + " " + code
	set := s.need(w, r, "census")
	if set == nil {
		return
	}
	place, found, err := s.censusPlace(set, level, code)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	if !found {
		s.notFound(w, r, "No "+level+" with census code "+code+" in the 2011 census.")
		return
	}
	s.ok(w, r, place, meta{Source: s.source("census"), Note: censusNote}, cacheAPI)
}

func (s *Server) censusPlace(set *handle, level, code string) (censusPlace, bool, error) {
	p := censusPlace{Level: level, Code: code}
	var state, district, subdistrict string
	err := set.row(`SELECT name, state, district, subdistrict FROM places WHERE level = ? AND code = ?`, level, code).Scan(&p.Name, &state, &district, &subdistrict)
	if err == sql.ErrNoRows {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	ref := func(l, c string) *censusRef {
		var name string
		if set.row(`SELECT name FROM places WHERE level = ? AND code = ?`, l, c).Scan(&name) != nil {
			return nil
		}
		return &censusRef{Code: c, Name: name}
	}
	if level != "india" && level != "state" {
		p.State = ref("state", state)
	}
	if level == "subdistrict" || level == "town" {
		p.District = ref("district", district)
	}
	if level == "town" {
		p.Subdistrict = ref("subdistrict", subdistrict)
	}
	rows, err := set.rows(`SELECT area, `+strings.Join(dataset.CensusFigures, ", ")+` FROM figures WHERE level = ? AND code = ?`, level, code)
	if err != nil {
		return p, false, err
	}
	defer rows.Close()
	p.Figures = map[string]censusFigures{}
	for rows.Next() {
		var area string
		values := make([]int64, len(dataset.CensusFigures))
		dest := []any{&area}
		for i := range values {
			dest = append(dest, &values[i])
		}
		if rows.Scan(dest...) != nil {
			continue
		}
		f := censusFigures{}
		for i, name := range dataset.CensusFigures {
			f[name] = values[i]
		}
		p.Figures[area] = f
	}
	if total, ok := p.Figures["total"]; ok {
		p.Rates = censusRatesOf(total, p.Figures["urban"])
	} else if urban, ok := p.Figures["urban"]; ok {
		// A town is all urban.
		p.Rates = censusRatesOf(urban, nil)
	}
	// What lies one level down.
	var childLevel, where string
	switch level {
	case "india":
		childLevel, where = "state", "1 = 1"
	case "state":
		childLevel, where = "district", "p.state = ?"
	case "district":
		childLevel, where = "subdistrict", "p.district = ?"
	case "subdistrict":
		childLevel, where = "town", "p.subdistrict = ?"
	}
	if childLevel != "" {
		args := []any{childLevel}
		if where != "1 = 1" {
			args = append(args, code)
		}
		children, err := set.rows(`SELECT p.code, p.name, f.persons FROM places p JOIN figures f ON f.level = p.level AND f.code = p.code
AND f.area = CASE WHEN p.level = 'town' THEN 'urban' ELSE 'total' END WHERE p.level = ? AND `+where+` ORDER BY p.name`, args...)
		if err != nil {
			return p, false, err
		}
		defer children.Close()
		for children.Next() {
			c := censusChild{Level: childLevel}
			if children.Scan(&c.Code, &c.Name, &c.Population) == nil {
				p.Children = append(p.Children, c)
			}
		}
	}
	return p, true, nil
}

func censusRatesOf(f, urban censusFigures) *censusRates {
	per := func(a, b int64, scale float64) *float64 {
		if b <= 0 {
			return nil
		}
		v := float64(int64(float64(a)/float64(b)*scale*100+0.5)) / 100
		return &v
	}
	ratio := func(a, b int64) *int {
		if b <= 0 {
			return nil
		}
		v := int(float64(a)/float64(b)*1000 + 0.5)
		return &v
	}
	rates := &censusRates{
		Literacy:      per(f["literate"], f["persons"]-f["children"], 100),
		MaleLiteracy:  per(f["literate_males"], f["males"]-f["boys"], 100),
		FemaleLit:     per(f["literate_females"], f["females"]-f["girls"], 100),
		SexRatio:      ratio(f["females"], f["males"]),
		ChildSexRatio: ratio(f["girls"], f["boys"]),
		WorkRate:      per(f["workers"], f["persons"], 100),
	}
	if urban != nil {
		rates.Urban = per(urban["persons"], f["persons"], 100)
	}
	return rates
}

func (s *Server) handleCensusSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if len(q) < 3 || len(q) > 60 {
		s.badRequest(w, r, "q is 3 to 60 letters of a place's name, such as coimbatore. Or ask for a place by code: /v1/census/state/33.")
		return
	}
	if _, ok := censusCodeShape[level]; level != "" && (!ok || level == "india") {
		s.badRequest(w, r, "level is state, district, subdistrict or town.")
		return
	}
	info(r).subject = q
	set := s.need(w, r, "census")
	if set == nil {
		return
	}
	pattern := "%" + strings.NewReplacer("%", "", "_", "").Replace(q) + "%"
	query := `SELECT p.level, p.code, p.name, p.state, f.persons FROM places p JOIN figures f ON f.level = p.level AND f.code = p.code
AND f.area = CASE WHEN p.level = 'town' THEN 'urban' ELSE 'total' END WHERE p.level != 'india' AND p.search LIKE ?`
	args := []any{pattern}
	if level != "" {
		query += ` AND p.level = ?`
		args = append(args, level)
	}
	// Exact names first, then the largest places.
	query += ` ORDER BY (p.search = ?) DESC, f.persons DESC LIMIT 50`
	args = append(args, q)
	rows, err := set.rows(query, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	out := []censusPlace{}
	for rows.Next() {
		var p censusPlace
		var state string
		var population int64
		if rows.Scan(&p.Level, &p.Code, &p.Name, &state, &population) == nil {
			p.Population = &population
			if p.Level != "state" {
				var name string
				if set.row(`SELECT name FROM places WHERE level = 'state' AND code = ?`, state).Scan(&name) == nil {
					p.State = &censusRef{Code: state, Name: name}
				}
			}
			out = append(out, p)
		}
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("census"), Note: "Places whose names contain the words, at most 50, largest first. " + censusNote}, cacheAPI)
}
