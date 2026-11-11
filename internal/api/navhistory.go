package api

import (
	"database/sql"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Mutual fund history and returns, from the server-only nav-history dataset: month-end
// NAVs for ten years and every NAV of the last 45 days.

func (s *Server) registerNAVHistory() {
	s.handle("mutual_fund_history", costLight, "GET /v1/mutual-funds/{code}/history", s.handleNAVHistory)
	s.handle("mutual_fund_returns", costLight, "GET /v1/mutual-funds/{code}/returns", s.handleNAVReturns)
}

type navPoint struct {
	Date     string  `json:"date"`
	NAV      float64 `json:"nav"`
	MonthEnd bool    `json:"monthEnd"`
}

// schemeCode resolves a scheme code or ISIN through the mutual-funds dataset.
func (s *Server) schemeCode(w http.ResponseWriter, r *http.Request) (int, string, bool) {
	set := s.need(w, r, "mutual-funds")
	if set == nil {
		return 0, "", false
	}
	raw := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	info(r).subject = raw
	var code int
	var name string
	err := set.row(`SELECT code, name FROM schemes WHERE code = ? OR isin_growth = ? OR isin_reinvest = ?`, raw, raw, raw).Scan(&code, &name)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "No scheme with code or ISIN "+raw+".")
		return 0, "", false
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return 0, "", false
	}
	return code, name, true
}

func (s *Server) navPoints(w http.ResponseWriter, r *http.Request, code int) ([]navPoint, bool) {
	set := s.need(w, r, "nav-history")
	if set == nil {
		return nil, false
	}
	rows, err := set.rows(`SELECT date, nav, month_end FROM navs WHERE code = ? ORDER BY date`, code)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return nil, false
	}
	defer rows.Close()
	var out []navPoint
	for rows.Next() {
		var p navPoint
		var monthEnd int
		if rows.Scan(&p.Date, &p.NAV, &monthEnd) == nil {
			p.MonthEnd = monthEnd == 1
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		s.notFound(w, r, "No NAV history for scheme "+strconv.Itoa(code)+": it may be new or wound up.")
		return nil, false
	}
	return out, true
}

const navHistoryNote = "Month-end NAVs for up to ten years and every NAV of the last 45 days, from AMFI. Past returns do not promise future ones."

func (s *Server) handleNAVHistory(w http.ResponseWriter, r *http.Request) {
	code, name, ok := s.schemeCode(w, r)
	if !ok {
		return
	}
	points, ok := s.navPoints(w, r, code)
	if !ok {
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	kept := []navPoint{}
	for _, p := range points {
		if (from == "" || p.Date >= from) && (to == "" || p.Date <= to) {
			kept = append(kept, p)
		}
	}
	total := len(kept)
	s.ok(w, r, map[string]any{"code": code, "name": name, "navs": kept}, meta{Total: &total, Source: s.source("nav-history"), Note: navHistoryNote}, cacheAPI)
}

type periodReturn struct {
	Period  string   `json:"period"`
	From    string   `json:"from"`
	To      string   `json:"to"`
	NAVFrom float64  `json:"navFrom"`
	NAVTo   float64  `json:"navTo"`
	Change  float64  `json:"changePercent"`
	CAGR    *float64 `json:"cagrPercent,omitempty"` // for a year or more
}

type sipReturn struct {
	Years       int     `json:"years"`
	Instalments int     `json:"instalments"`
	Invested    float64 `json:"invested"`
	Value       float64 `json:"value"`
	XIRR        float64 `json:"xirrPercent"`
}

// pointBefore is the last point on or before date, if it is within slack days of it.
func pointBefore(points []navPoint, date time.Time, slack int) (navPoint, bool) {
	want := date.Format(time.DateOnly)
	earliest := date.AddDate(0, 0, -slack).Format(time.DateOnly)
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Date <= want {
			return points[i], points[i].Date >= earliest
		}
	}
	return navPoint{}, false
}

// xirr is the yearly rate that makes the flows' present value zero (bisection).
func xirr(amounts []float64, dates []time.Time) float64 {
	value := func(rate float64) float64 {
		total := 0.0
		for i, a := range amounts {
			years := dates[i].Sub(dates[0]).Hours() / 24 / 365
			total += a / math.Pow(1+rate, years)
		}
		return total
	}
	lo, hi := -0.99, 10.0
	for i := 0; i < 200; i++ {
		mid := (lo + hi) / 2
		if value(mid) > 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

func (s *Server) handleNAVReturns(w http.ResponseWriter, r *http.Request) {
	code, name, ok := s.schemeCode(w, r)
	if !ok {
		return
	}
	points, ok := s.navPoints(w, r, code)
	if !ok {
		return
	}
	latest := points[len(points)-1]
	end, _ := time.Parse(time.DateOnly, latest.Date)
	periods := []struct {
		name   string
		months int
		slack  int
	}{{"1m", 1, 7}, {"3m", 3, 20}, {"6m", 6, 20}, {"1y", 12, 20}, {"3y", 36, 20}, {"5y", 60, 20}, {"10y", 120, 20}}
	returns := []periodReturn{}
	for _, p := range periods {
		start, found := pointBefore(points, end.AddDate(0, -p.months, 0), p.slack)
		if !found {
			continue
		}
		from, _ := time.Parse(time.DateOnly, start.Date)
		pr := periodReturn{Period: p.name, From: start.Date, To: latest.Date, NAVFrom: start.NAV, NAVTo: latest.NAV, Change: roundCents((latest.NAV/start.NAV - 1) * 100)}
		if p.months >= 12 {
			years := end.Sub(from).Hours() / 24 / 365.25
			cagr := roundCents((math.Pow(latest.NAV/start.NAV, 1/years) - 1) * 100)
			pr.CAGR = &cagr
		}
		returns = append(returns, pr)
	}
	// A monthly SIP of 1,000 on each month-end, valued at the latest NAV.
	sips := []sipReturn{}
	for _, years := range []int{1, 3, 5, 10} {
		cutoff := end.AddDate(-years, 0, 0).Format(time.DateOnly)
		var amounts []float64
		var dates []time.Time
		units := 0.0
		for _, p := range points {
			if p.MonthEnd && p.Date > cutoff && p.Date < latest.Date {
				d, _ := time.Parse(time.DateOnly, p.Date)
				amounts, dates = append(amounts, -1000), append(dates, d)
				units += 1000 / p.NAV
			}
		}
		if len(amounts) < years*12-1 {
			continue
		}
		value := units * latest.NAV
		amounts, dates = append(amounts, value), append(dates, end)
		sips = append(sips, sipReturn{Years: years, Instalments: len(amounts) - 1, Invested: float64(1000 * (len(amounts) - 1)), Value: roundCents(value), XIRR: roundCents(xirr(amounts, dates) * 100)})
	}
	s.ok(w, r, map[string]any{"code": code, "name": name, "latest": latest, "returns": returns, "sip": sips},
		meta{Source: s.source("nav-history"), Note: "Point to point from the NAV nearest each start date (dates given), CAGR for a year or more. SIP: 1,000 on each month-end, valued at the latest NAV, as XIRR. " + navHistoryNote}, cacheAPI)
}

// roundCents rounds to two decimals, half away from zero, negatives included.
func roundCents(v float64) float64 { return math.Round(v*100) / 100 }
