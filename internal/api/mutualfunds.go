package api

import (
	"database/sql"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type schemeView struct {
	Code         int      `json:"code"`
	Name         string   `json:"name"`
	FundHouse    string   `json:"fundHouse"`
	Type         *string  `json:"type"`
	Category     *string  `json:"category"`
	Plan         *string  `json:"plan"`
	Option       *string  `json:"option"`
	PlanLabel    *string  `json:"planLabel,omitempty"`
	OptionLabel  *string  `json:"optionLabel,omitempty"`
	ISINGrowth   *string  `json:"isinGrowth"`
	ISINReinvest *string  `json:"isinReinvestment"`
	NAV          *float64 `json:"nav"`
	NAVDate      *string  `json:"navDate"`
	Active       bool     `json:"active"`
}

const schemeColumns = `code, name, amc, scheme_type, category, plan, option, plan_label, option_label, isin_growth, isin_reinvest, nav, nav_date, active FROM schemes`

func scanScheme(scan func(...any) error) (schemeView, error) {
	var view schemeView
	var kind, category, plan, option, planLabel, optionLabel, isinGrowth, isinReinvest, date sql.NullString
	var nav sql.NullFloat64
	err := scan(&view.Code, &view.Name, &view.FundHouse, &kind, &category, &plan, &option, &planLabel, &optionLabel,
		&isinGrowth, &isinReinvest, &nav, &date, &view.Active)
	view.Type, view.Category, view.Plan, view.Option = str(kind), str(category), str(plan), str(option)
	view.PlanLabel, view.OptionLabel, view.ISINGrowth, view.ISINReinvest = str(planLabel), str(optionLabel), str(isinGrowth), str(isinReinvest)
	view.NAV, view.NAVDate = number(nav), str(date)
	return view, err
}

var isinInput = regexp.MustCompile(`^IN[A-Z0-9]{10}$`)

const navNote = "NAV as published by AMFI for the date given. A scheme is active when AMFI published its NAV in the last 30 days."

func (s *Server) handleScheme(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "mutual-funds")
	if set == nil {
		return
	}
	key := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	info(r).subject = key
	var row *sql.Row
	switch {
	case isinInput.MatchString(key):
		row = set.row(`SELECT `+schemeColumns+` WHERE isin_growth = ? OR isin_reinvest = ? LIMIT 1`, key, key)
	default:
		code, err := strconv.Atoi(key)
		if err != nil || code <= 0 {
			s.badRequest(w, r, "Give an AMFI scheme code, such as 122639, or an ISIN.")
			return
		}
		row = set.row(`SELECT `+schemeColumns+` WHERE code = ?`, code)
	}
	view, err := scanScheme(row.Scan)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "No scheme found for "+key+". Search by name at /v1/mutual-funds?q=.")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, view, meta{Source: s.source("mutual-funds"), Note: navNote}, "public, max-age=900")
}

func (s *Server) handleSchemeSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "mutual-funds")
	if set == nil {
		return
	}
	query := r.URL.Query()
	text := strings.TrimSpace(query.Get("q"))
	house := strings.TrimSpace(query.Get("fundHouse"))
	plan := strings.ToLower(strings.TrimSpace(query.Get("plan")))
	option := strings.ToLower(strings.TrimSpace(query.Get("option")))
	category := strings.TrimSpace(query.Get("category"))
	limit, offset, ok := s.listParams(w, r, 20)
	if !ok {
		return
	}
	if plan != "" && plan != "direct" && plan != "regular" {
		s.badRequest(w, r, "plan must be direct or regular.")
		return
	}
	if option != "" && option != "growth" && option != "idcw" && option != "bonus" {
		s.badRequest(w, r, "option must be growth, idcw or bonus.")
		return
	}
	if len(text) < 2 && house == "" && category == "" {
		s.badRequest(w, r, "Give q (2 or more characters of a scheme name), a fundHouse or a category.")
		return
	}
	info(r).subject = text

	where := []string{"1 = 1"}
	var args []any
	// Every word must appear in the name, in any order: "parag flexi direct" finds it.
	for _, word := range strings.Fields(text) {
		where = append(where, `name LIKE ? ESCAPE '\'`)
		args = append(args, likePattern(word, true))
	}
	if house != "" {
		where = append(where, `amc LIKE ? ESCAPE '\'`)
		args = append(args, likePattern(house, true))
	}
	if category != "" {
		where = append(where, `category LIKE ? ESCAPE '\'`)
		args = append(args, likePattern(category, true))
	}
	if plan != "" {
		where = append(where, `plan = ?`)
		args = append(args, plan)
	}
	if option != "" {
		where = append(where, `option = ?`)
		args = append(args, option)
	}
	if query.Get("all") != "true" && query.Get("all") != "1" {
		where = append(where, `active = 1`)
	}
	condition := strings.Join(where, " AND ")

	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM schemes WHERE `+condition, args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.db.Query(`SELECT `+schemeColumns+` WHERE `+condition+` ORDER BY active DESC, name, plan, option LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	schemes := []schemeView{}
	for rows.Next() {
		view, err := scanScheme(rows.Scan)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		schemes = append(schemes, view)
	}
	m := paging(total, limit, offset)
	m.Source, m.Note = s.source("mutual-funds"), navNote
	s.ok(w, r, schemes, m, "public, max-age=900")
}

type fundHouseView struct {
	Name          string `json:"name"`
	Schemes       int    `json:"schemes"`
	ActiveSchemes int    `json:"activeSchemes"`
}

func (s *Server) handleFundHouses(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "mutual-funds")
	if set == nil {
		return
	}
	rows, err := set.rows(`SELECT amc, COUNT(*), SUM(active) FROM schemes GROUP BY amc ORDER BY amc`)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	houses := []fundHouseView{}
	for rows.Next() {
		var house fundHouseView
		if err := rows.Scan(&house.Name, &house.Schemes, &house.ActiveSchemes); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		houses = append(houses, house)
	}
	total := len(houses)
	s.ok(w, r, houses, meta{Total: &total, Source: s.source("mutual-funds")}, cacheAPI)
}

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "get_mutual_fund_nav",
			Description: "Get the latest NAV of an Indian mutual fund scheme by AMFI scheme code or ISIN, with its fund house, category, plan and option.",
			InputSchema: schema([]string{"code"}, map[string]any{"code": text("AMFI scheme code or ISIN.")}),
			path: func(args map[string]any) (string, error) {
				code, err := required(args, "code")
				return "/v1/mutual-funds/" + url.PathEscape(code), err
			},
		},
		mcpTool{
			Name:        "search_mutual_funds",
			Description: "Search Indian mutual fund schemes by name words, fund house or category, filtered by direct/regular plan and growth/IDCW option, with their latest NAVs.",
			InputSchema: schema(nil, map[string]any{
				"query": text("Words of the scheme name."), "fund_house": text("Fund house name."),
				"plan": oneOf("Plan.", "direct", "regular"), "option": oneOf("Option.", "growth", "idcw", "bonus"),
			}),
			path: func(args map[string]any) (string, error) {
				return "/v1/mutual-funds" + query(args, map[string]string{"query": "q", "fund_house": "fundHouse", "plan": "plan", "option": "option"}), nil
			},
		},
	)
}
