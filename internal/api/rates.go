package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RBI's current rates and FBIL reference exchange rates (the server-only rbi-rates
// dataset), and derivative expiry dates worked out from the market holidays.

func (s *Server) registerRates() {
	s.handle("rates", costLight, "GET /v1/rates", s.handleRates)
	s.handle("fx", costLight, "GET /v1/fx", s.handleFX)
	s.handle("expiries", costLight, "GET /v1/expiries", s.handleExpiries)
}

type rateView struct {
	Name   string   `json:"name"`
	Value  string   `json:"value"`
	Number *float64 `json:"number,omitempty"`
	Date   string   `json:"date"`
}

var rateKinds = map[string]string{"policy": "policy", "ratio": "reserveRatios", "fx": "exchangeRates", "lending": "lendingAndDeposit", "money": "moneyMarket", "tbill": "treasuryBills", "bond": "governmentBonds"}

func (s *Server) handleRates(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "rbi-rates")
	if set == nil {
		return
	}
	// The latest value of each figure.
	rows, err := set.rows(`SELECT r.kind, r.name, r.value, r.number, r.date FROM rates r
WHERE r.date = (SELECT max(date) FROM rates x WHERE x.kind = r.kind AND x.name = r.name) ORDER BY r.kind, r.name`)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	out := map[string][]rateView{}
	for rows.Next() {
		var kind string
		var v rateView
		if rows.Scan(&kind, &v.Name, &v.Value, &v.Number, &v.Date) == nil {
			if key, ok := rateKinds[kind]; ok {
				out[key] = append(out[key], v)
			}
		}
	}
	s.ok(w, r, out, meta{Source: s.source("rbi-rates"), Note: "As the Reserve Bank of India's home page shows them. Exchange rates are FBIL reference rates, rupees per unit (per 100 yen, per 10,000 rupiah). Treasury bills: cut-off yield at the last auction."}, "public, max-age=900")
}

// fxCurrencies maps a currency code to the name RBI gives its rate.
var fxCurrencies = map[string]string{"USD": "INR / 1 USD", "GBP": "INR / 1 GBP", "EUR": "INR / 1 EUR", "JPY": "INR / 100 JPY", "AED": "INR / 1 AED", "IDR": "INR / 10000 IDR", "RUB": "INR / 1 RUB"}

func (s *Server) handleFX(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "rbi-rates")
	if set == nil {
		return
	}
	query := r.URL.Query()
	currency := strings.ToUpper(query.Get("currency"))
	days := 0
	if raw := query.Get("days"); raw != "" {
		var err error
		if days, err = strconv.Atoi(raw); err != nil || days < 1 || days > 3650 {
			s.badRequest(w, r, "days is 1 to 3650.")
			return
		}
	}
	names := []string{}
	if currency != "" {
		name, ok := fxCurrencies[currency]
		if !ok {
			s.badRequest(w, r, "currency is one of USD, GBP, EUR, JPY, AED, IDR, RUB (the FBIL reference rates).")
			return
		}
		names = append(names, name)
	} else {
		for _, c := range []string{"USD", "GBP", "EUR", "JPY", "AED", "IDR", "RUB"} {
			names = append(names, fxCurrencies[c])
		}
	}
	type fxPoint struct {
		Currency string  `json:"currency"`
		Per      int     `json:"per"` // units of the currency the rupee figure is for
		Rupees   float64 `json:"rupees"`
		Date     string  `json:"date"`
	}
	out := []fxPoint{}
	for _, name := range names {
		code := ""
		for c, n := range fxCurrencies {
			if n == name {
				code = c
			}
		}
		per := map[string]int{"JPY": 100, "IDR": 10000}[code]
		if per == 0 {
			per = 1
		}
		query, args := `SELECT number, date FROM rates WHERE kind = 'fx' AND name = ? AND number IS NOT NULL ORDER BY date DESC LIMIT 1`, []any{name}
		if days > 0 {
			query, args = `SELECT number, date FROM rates WHERE kind = 'fx' AND name = ? AND number IS NOT NULL AND date >= date((SELECT max(date) FROM rates WHERE kind = 'fx'), ?) ORDER BY date`, []any{name, "-" + strconv.Itoa(days) + " day"}
		}
		rows, err := set.rows(query, args...)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		for rows.Next() {
			p := fxPoint{Currency: code, Per: per}
			if rows.Scan(&p.Rupees, &p.Date) == nil {
				out = append(out, p)
			}
		}
		rows.Close()
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("rbi-rates"), Note: "FBIL reference rates, published around 1.30 pm on working days. History starts on the day this server began recording."}, "public, max-age=900")
}

// ---- derivative expiries ----

type expiryView struct {
	Date      string `json:"date"`
	Weekday   string `json:"weekday"`
	Kind      string `json:"kind"` // weekly or monthly
	Contracts string `json:"contracts"`
	Moved     string `json:"moved,omitempty"` // why it is not on the usual day
}

// expiryRules are the exchanges' expiry days from 1 September 2025 (SEBI's circular of
// May 2025 and the exchanges' notices): NSE on Tuesdays, BSE on Thursdays; a weekly
// expiry only for one benchmark index on each exchange; monthly contracts on the last
// such day of the month. A holiday moves an expiry to the trading day before.
var expiryRules = map[string]struct {
	weekday         time.Weekday
	weekly, monthly string
}{
	"nse": {time.Tuesday, "Nifty 50 weekly options", "Monthly futures and options: Nifty 50, Nifty Bank, Nifty Financial Services, Nifty Midcap Select, Nifty Next 50 and stocks"},
	"bse": {time.Thursday, "Sensex weekly options", "Monthly futures and options: Sensex, Bankex, Sensex 50 and stocks"},
}

func (s *Server) handleExpiries(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	exchange := strings.ToLower(query.Get("exchange"))
	if exchange == "" {
		exchange = "nse"
	}
	rule, ok := expiryRules[exchange]
	if !ok {
		s.badRequest(w, r, "exchange is nse or bse.")
		return
	}
	now := datesClock().In(indiaTime)
	year, month := now.Year(), 0
	if raw := query.Get("year"); raw != "" {
		var err error
		if year, err = strconv.Atoi(raw); err != nil {
			s.badRequest(w, r, "year is a four-digit year.")
			return
		}
	}
	if raw := query.Get("month"); raw != "" {
		var err error
		if month, err = strconv.Atoi(raw); err != nil || month < 1 || month > 12 {
			s.badRequest(w, r, "month is 1 to 12.")
			return
		}
	}
	if year < 2025 || (year == 2025 && month != 0 && month < 9) {
		s.badRequest(w, r, "Expiries follow the rules in force from September 2025; earlier dates are not worked out here.")
		return
	}
	book := s.openWorkBook(w, r, "market", year, year)
	if book == nil {
		return
	}
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	if year == 2025 {
		start = time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	}
	if !book.covers(start) {
		s.notFound(w, r, book.uncovered(start))
		return
	}
	weekly := query.Get("weekly") != "false"
	out := []expiryView{}
	for day := start; day.Year() == year; day = day.AddDate(0, 0, 1) {
		if day.Weekday() != rule.weekday || (month != 0 && int(day.Month()) != month) {
			continue
		}
		last := day.AddDate(0, 0, 7).Month() != day.Month()
		if !last && !weekly {
			continue
		}
		v := expiryView{Kind: "weekly", Contracts: rule.weekly}
		if last {
			v.Kind, v.Contracts = "monthly", rule.monthly+"; also "+rule.weekly
		}
		actual := day
		if why := book.reasons(day); len(why) > 0 {
			previous, ok := book.step(day, -1)
			if !ok {
				continue
			}
			actual = previous
			v.Moved = day.Format("Mon 2 Jan") + " is a holiday (" + strings.Join(why, ", ") + ")"
		}
		v.Date, v.Weekday = actual.Format(time.DateOnly), actual.Weekday().String()
		out = append(out, v)
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("holidays-market"),
		Note: "Worked out from the exchange rules in force from 1 September 2025 (NSE Tuesdays, BSE Thursdays; monthly contracts on the month's last one) and the exchange's holiday list. The exchange's own circulars decide; check them for special sessions."}, cacheAPI)
}
