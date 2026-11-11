package api

// The rest of the idea catalogue's set A (growth plan, 05): number formatting, Indian
// date tools, UPI handles, payment QR text, README badges and the address reader. The
// company cross-search was already /v1/companies.

import (
	"fmt"
	"image/color"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
	"github.com/dharunashokkumar/api.dharun.dev/internal/indiafmt"
	"github.com/dharunashokkumar/api.dharun.dev/internal/panchang"
	"github.com/dharunashokkumar/api.dharun.dev/internal/payqr"
	"github.com/dharunashokkumar/api.dharun.dev/internal/typeset"
	"github.com/dharunashokkumar/api.dharun.dev/internal/words"
)

func (s *Server) registerSetA() {
	s.handle("format_number", costLight, "GET /v1/format/number/{number}", s.handleFormatNumber)
	s.handle("dates", costLight, "GET /v1/dates/{date}", s.handleDate)
	s.handle("dates_diff", costLight, "GET /v1/dates/diff", s.handleDateDiff)
	s.handle("upi_handle", costLight, "GET /v1/upi-handles", s.handleUPIHandles)
	s.handle("upi_handle", costLight, "GET /v1/upi-handles/{handle}", s.handleUPIHandle)
	s.handle("qr_parse", costLight, "GET /v1/qr/parse", s.handleQRParse)
	s.handle("qr_parse", costLight, "POST /v1/qr/parse", s.handleQRParse)
	s.handle("badge", costLight, "GET /v1/badge", s.handleBadge)
	s.handle("badge", costLight, "GET /v1/badge/{kind}", s.handleBadge)
	s.handle("badge", costLight, "GET /v1/badge/dataset/{name}", s.handleBadge)
	s.handle("address_parse", costLight, "GET /v1/address/parse", s.handleAddressParse)
	s.handle("address_parse", costLight, "POST /v1/address/parse", s.handleAddressParse)
}

// ---- number formatting ----

type numberView struct {
	Input         string            `json:"input"`
	Value         float64           `json:"value"`
	Indian        string            `json:"indian"`
	International string            `json:"international"`
	Rupees        string            `json:"rupees"`
	Short         indiafmt.Short    `json:"short"`
	Words         string            `json:"words,omitempty"`
	Scripts       map[string]string `json:"scripts"`
}

func (s *Server) handleFormatNumber(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("number")
	info(r).subject = raw
	n, err := indiafmt.Parse(raw)
	if err != nil {
		s.badRequest(w, r, "The number cannot be read ("+err.Error()+"). Write it with digits, a point and optional commas, such as 1234567.89.")
		return
	}
	decimals := -1
	if d := r.URL.Query().Get("decimals"); d != "" {
		decimals, err = strconv.Atoi(d)
		if err != nil || decimals < 0 || decimals > 10 {
			s.badRequest(w, r, "decimals is 0 to 10.")
			return
		}
		n = n.Round(decimals)
	}
	shortDecimals := 2
	if decimals >= 0 {
		shortDecimals = decimals
	}
	view := numberView{Input: raw, Value: n.Float(), Indian: n.Indian(), International: n.International(), Rupees: n.Rupees(),
		Short: indiafmt.ShortForm(n.Float(), shortDecimals), Scripts: map[string]string{}}
	if whole, err := strconv.ParseInt(n.Whole, 10, 64); err == nil && whole < 1e15 {
		if text, err := words.Number("en", n.Negative, whole, n.Fraction); err == nil {
			view.Words = text
		}
	}
	for _, script := range indiafmt.ScriptNames() {
		view.Scripts[script], _ = indiafmt.Digits(view.Indian, script)
	}
	s.ok(w, r, view, meta{Note: "Indian grouping puts commas after the thousands, then every two digits (lakh, crore). For words in Hindi or Tamil, see /v1/words."}, cacheLong)
}

// ---- dates ----

type dateView struct {
	Date          string                 `json:"date"`
	Weekday       string                 `json:"weekday"`
	ISOWeek       string                 `json:"isoWeek"`
	DayOfYear     int                    `json:"dayOfYear"`
	LeapYear      bool                   `json:"leapYear"`
	FinancialYear indiafmt.FinancialYear `json:"financialYear"`
	Saka          indiafmt.CalendarDate  `json:"saka"`
	Hijri         indiafmt.CalendarDate  `json:"hijri"`
	Tamil         *tamilDate             `json:"tamil,omitempty"`
	Vikram        int                    `json:"vikramSamvat,omitempty"`
	Unix          int64                  `json:"unix"`
	FromToday     indiafmt.Span          `json:"fromToday"`
}

type tamilDate struct {
	Month string `json:"month"`
	Tamil string `json:"tamil,omitempty"`
	Day   int    `json:"day"`
	Year  string `json:"year"`
}

// datesClock tells the time for the date tools; tests fix it.
var datesClock = time.Now

func (s *Server) handleDate(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("date")
	info(r).subject = raw
	var t time.Time
	var err error
	if raw == "today" {
		now := datesClock().In(indiaTime)
		t = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	} else if t, err = time.Parse(time.DateOnly, raw); err != nil || t.Year() < 1 || t.Year() > 9999 {
		s.badRequest(w, r, "The date is YYYY-MM-DD, or today.")
		return
	}
	year, week := t.ISOWeek()
	now := datesClock().In(indiaTime)
	view := dateView{Date: t.Format(time.DateOnly), Weekday: t.Weekday().String(), ISOWeek: fmt.Sprintf("%d-W%02d", year, week), DayOfYear: t.YearDay(),
		LeapYear: t.Year()%4 == 0 && (t.Year()%100 != 0 || t.Year()%400 == 0), FinancialYear: indiafmt.FY(t), Saka: indiafmt.Saka(t), Hijri: indiafmt.Hijri(t),
		Unix: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, indiaTime).Unix(), FromToday: indiafmt.Between(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), t)}
	// The Tamil solar calendar and the Vikram era need the sun's position, which the
	// panchang computes; it is reckoned at Chennai.
	if t.Year() >= 1900 && t.Year() <= 2100 {
		place := panchang.Place{Latitude: 13.0827, Longitude: 80.2707, Zone: indiaTime}
		if day, err := panchang.Compute(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, indiaTime), place); err == nil {
			view.Tamil = &tamilDate{Month: day.TamilMonth.Name, Tamil: day.TamilMonth.Tamil, Day: day.TamilMonth.Day, Year: day.TamilYear.English}
			view.Vikram = day.Vikram
		}
	}
	cache := cacheLong
	if raw == "today" {
		cache = "public, max-age=300"
	}
	s.ok(w, r, view, meta{Note: "Saka is the Indian national calendar. The Hijri date is the tabular one and may differ by a day or two from the date set by the sighting of the moon. The Tamil date is reckoned at Chennai."}, cache)
}

func (s *Server) handleDateDiff(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	from, to := query.Get("from"), query.Get("to")
	info(r).subject = from + ".." + to
	now := datesClock().In(indiaTime)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	parse := func(value string) (time.Time, error) {
		if value == "" || value == "today" {
			return today, nil
		}
		return time.Parse(time.DateOnly, value)
	}
	a, err1 := parse(from)
	b, err2 := parse(to)
	if from == "" || err1 != nil || err2 != nil {
		s.badRequest(w, r, "from is a date YYYY-MM-DD; to is one too and defaults to today. For an age, give from=the date of birth.")
		return
	}
	span := indiafmt.Between(a, b)
	result := map[string]any{"span": span}
	// For an age: the next birthday.
	if !b.Before(a) {
		next := a.AddDate(span.Years+1, 0, 0)
		if a.Month() == time.February && a.Day() == 29 && next.Month() != time.February {
			next = time.Date(next.Year(), time.March, 1, 0, 0, 0, 0, time.UTC)
		}
		result["nextAnniversary"] = map[string]any{"date": next.Format(time.DateOnly), "inDays": int(next.Sub(b).Hours() / 24), "turns": span.Years + 1}
	}
	s.ok(w, r, result, meta{Note: "Whole years, months and days, counted the way an age is. A 29 February anniversary falls on 1 March in other years."}, "public, max-age=300")
}

// ---- UPI handles ----

type upiHandle struct {
	Handle string `json:"handle"`
	App    string `json:"app"`
	Bank   string `json:"bank"`
	Kind   string `json:"kind"` // app: a third-party app's handle at a bank; bank: a bank's own app
}

// upiHandles is typed from NPCI's published list of UPI apps and the handles each one
// issues with its partner banks.
var upiHandles = []upiHandle{
	{"okaxis", "Google Pay", "Axis Bank", "app"}, {"okhdfcbank", "Google Pay", "HDFC Bank", "app"}, {"okicici", "Google Pay", "ICICI Bank", "app"}, {"oksbi", "Google Pay", "State Bank of India", "app"},
	{"ybl", "PhonePe", "Yes Bank", "app"}, {"ibl", "PhonePe", "ICICI Bank", "app"}, {"axl", "PhonePe", "Axis Bank", "app"},
	{"paytm", "Paytm", "Paytm Payments Bank", "app"}, {"ptyes", "Paytm", "Yes Bank", "app"}, {"ptaxis", "Paytm", "Axis Bank", "app"}, {"pthdfc", "Paytm", "HDFC Bank", "app"}, {"ptsbi", "Paytm", "State Bank of India", "app"},
	{"apl", "Amazon Pay", "Axis Bank", "app"}, {"yapl", "Amazon Pay", "Yes Bank", "app"}, {"rapl", "Amazon Pay", "RBL Bank", "app"},
	{"waaxis", "WhatsApp", "Axis Bank", "app"}, {"wahdfcbank", "WhatsApp", "HDFC Bank", "app"}, {"wasbi", "WhatsApp", "State Bank of India", "app"}, {"waicici", "WhatsApp", "ICICI Bank", "app"},
	{"upi", "BHIM", "NPCI (the bank the user links)", "app"}, {"axisb", "CRED", "Axis Bank", "app"}, {"freecharge", "Freecharge", "Axis Bank", "app"}, {"ikwik", "MobiKwik", "HDFC Bank", "app"},
	{"jupiteraxis", "Jupiter", "Axis Bank", "app"}, {"naviaxis", "Navi", "Axis Bank", "app"}, {"superyes", "super.money", "Yes Bank", "app"},
	{"airtel", "Airtel Thanks", "Airtel Payments Bank", "bank"}, {"jio", "JioFinance", "Jio Payments Bank", "bank"},
	{"sbi", "SBI (YONO, BHIM SBI Pay)", "State Bank of India", "bank"}, {"icici", "iMobile Pay", "ICICI Bank", "bank"}, {"hdfcbank", "HDFC Bank", "HDFC Bank", "bank"},
	{"axisbank", "Axis Mobile", "Axis Bank", "bank"}, {"kotak", "Kotak", "Kotak Mahindra Bank", "bank"}, {"pnb", "PNB ONE", "Punjab National Bank", "bank"},
	{"barodampay", "bob World", "Bank of Baroda", "bank"}, {"boi", "BOI Mobile", "Bank of India", "bank"}, {"cnrb", "Canara ai1", "Canara Bank", "bank"},
	{"unionbank", "Union Bank", "Union Bank of India", "bank"}, {"idfcbank", "IDFC FIRST Bank", "IDFC FIRST Bank", "bank"}, {"indus", "IndusInd Bank", "IndusInd Bank", "bank"},
	{"federal", "FedMobile", "Federal Bank", "bank"}, {"aubank", "AU 0101", "AU Small Finance Bank", "bank"}, {"rbl", "RBL MoBank", "RBL Bank", "bank"},
	{"idbi", "IDBI Bank", "IDBI Bank", "bank"}, {"kvb", "KVB", "Karur Vysya Bank", "bank"}, {"kbl", "KBL", "Karnataka Bank", "bank"}, {"sib", "SIB Mirror+", "South Indian Bank", "bank"},
	{"dbs", "digibank", "DBS Bank India", "bank"}, {"hsbc", "HSBC India", "HSBC", "bank"}, {"sc", "SC Mobile", "Standard Chartered", "bank"}, {"citi", "Citi", "Citibank", "bank"},
	{"yesbank", "Yes Bank", "Yes Bank", "bank"}, {"equitas", "Equitas", "Equitas Small Finance Bank", "bank"}, {"ujjivan", "Ujjivan", "Ujjivan Small Finance Bank", "bank"},
}

var upiHandleIndex = func() map[string]upiHandle {
	index := map[string]upiHandle{}
	for _, h := range upiHandles {
		index[h.Handle] = h
	}
	return index
}()

const upiHandleNote = "A handle says which app and partner bank issued a UPI ID; the money can sit in any bank the person linked. It says nothing about who the person is. Typed from NPCI's list of UPI apps; banks add handles from time to time."

func (s *Server) handleUPIHandles(w http.ResponseWriter, r *http.Request) {
	list := append([]upiHandle(nil), upiHandles...)
	sort.Slice(list, func(i, j int) bool { return list[i].Handle < list[j].Handle })
	total := len(list)
	s.ok(w, r, list, meta{Total: &total, Note: upiHandleNote}, cacheLong)
}

func (s *Server) handleUPIHandle(w http.ResponseWriter, r *http.Request) {
	raw := strings.ToLower(strings.TrimSpace(r.PathValue("handle")))
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		raw = raw[at+1:] // a full UPI ID: only the handle is looked at, and the name is never logged
	}
	info(r).subject = raw
	handle, ok := upiHandleIndex[raw]
	if !ok {
		s.notFound(w, r, "Unknown UPI handle @"+raw+". See /v1/upi-handles for the known ones.")
		return
	}
	s.ok(w, r, handle, meta{Note: upiHandleNote}, cacheLong)
}

// ---- payment QR text ----

func (s *Server) handleQRParse(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	result, err := payqr.Parse(in.Text)
	if err != nil {
		s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
		return
	}
	if result.Payee != "" {
		if _, handle, ok := strings.Cut(result.Payee, "@"); ok {
			if h, known := upiHandleIndex[strings.ToLower(handle)]; known {
				result.Fields["_handle"] = h.App + " (" + h.Bank + ")"
			}
		}
	}
	s.ok(w, r, result, meta{Note: "Reads what the code says; it cannot tell whether the payee is genuine. To read a QR code from a picture, use POST /v1/barcode/read."}, "private, no-store")
}

// ---- badges ----

var badgeColors = map[string]string{
	"green": "#2e8540", "brightgreen": "#3fa34d", "red": "#c0392b", "orange": "#d9730d", "yellow": "#b8860b", "blue": "#2563eb", "grey": "#6b7280", "gray": "#6b7280",
	"purple": "#7c3aed", "teal": "#0f9d8a",
}

func badgeColor(raw, fallback string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return fallback, nil
	}
	if named, ok := badgeColors[raw]; ok {
		return named, nil
	}
	c, err := parseColor(raw, color.RGBA{0x25, 0x63, 0xeb, 0xff})
	if err != nil {
		return "", err
	}
	return cssColor(c), nil
}

// drawBadge draws a two-part badge: a grey label and a coloured message.
func (s *Server) drawBadge(label, message, colour, labelColour string) []byte {
	var svg []byte
	s.extras.render.work(func() {
		d := newCanvas()
		const size, pad, height = 11.0, 6.0, 20
		measure := func(text string) float64 {
			if text == "" {
				return 0
			}
			return float64(d.Shape(text, typeset.Style{}).Width) * size / typeset.Em
		}
		lw := 0.0
		if label != "" {
			lw = math.Ceil(measure(label) + 2*pad)
		}
		mw := math.Ceil(measure(message) + 2*pad)
		width := int(lw + mw)
		d.printf(`<clipPath id="r"><rect width="%d" height="%d" rx="3"/></clipPath><g clip-path="url(#r)">`, width, height)
		if lw > 0 {
			d.printf(`<rect width="%s" height="%d" fill="%s"/>`, coordinate(lw), height, labelColour)
		}
		d.printf(`<rect x="%s" width="%s" height="%d" fill="%s"/></g>`, coordinate(lw), coordinate(mw), height, colour)
		if label != "" {
			textAt(d, label, typeset.Style{}, size, lw/2, height/2, "#fff", "")
		}
		textAt(d, message, typeset.Style{}, size, lw+mw/2, height/2, "#fff", "")
		text := message
		if label != "" {
			text = label + ": " + message
		}
		svg = d.svg(width, height, text)
	})
	return svg
}

func (s *Server) handleBadge(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	kind := r.PathValue("kind")
	if r.PathValue("name") != "" {
		kind = "dataset"
	}
	info(r).subject = kind
	format, err := pictureFormat(query.Get("format"))
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	label, message, colour := query.Get("label"), "", badgeColors["blue"]
	cache := "public, max-age=300"
	switch kind {
	case "":
		message = query.Get("message")
		if strings.TrimSpace(message) == "" {
			s.badRequest(w, r, "message is required for a badge of your own (label is optional).")
			return
		}
		if colour, err = badgeColor(query.Get("color"), colour); err != nil {
			s.badRequest(w, r, "color: "+err.Error())
			return
		}
		cache = cacheLong
	case "market":
		var status struct {
			Open    bool   `json:"open"`
			Session string `json:"session"`
		}
		if !s.inner(r, "/v1/market/status", &status) {
			s.fail(w, r, http.StatusServiceUnavailable, "not_available", "The market calendar is not available.")
			return
		}
		if label == "" {
			label = "NSE"
		}
		message, colour = strings.ReplaceAll(status.Session, "-", " "), badgeColors["red"]
		if status.Open {
			colour = badgeColors["green"]
		} else if status.Session != "closed" {
			colour = badgeColors["orange"]
		}
		cache = "public, max-age=60"
	case "holiday":
		now := datesClock().In(indiaTime)
		today := now.Format(time.DateOnly)
		kindOf := query.Get("type")
		if kindOf == "" {
			kindOf = "bank"
		}
		var next *holidayView
		for year := now.Year(); year <= now.Year()+1 && next == nil; year++ {
			address := "/v1/holidays/" + strconv.Itoa(year) + "?type=" + kindOf
			if state := query.Get("state"); state != "" {
				address += "&state=" + state
			}
			var list []holidayView
			if s.inner(r, address, &list) {
				for i := range list {
					if list[i].Date >= today {
						next = &list[i]
						break
					}
				}
			}
		}
		if label == "" {
			label = "next " + kindOf + " holiday"
		}
		message = "not known yet"
		if next != nil {
			at, _ := time.Parse(time.DateOnly, next.Date)
			message = at.Format("2 Jan") + " · " + next.Name
		}
		colour = badgeColors["teal"]
	case "dataset":
		name := r.PathValue("name")
		source, ok := s.sourceOf(name)
		if !ok {
			s.notFound(w, r, "No dataset named "+name+". /healthz lists them.")
			return
		}
		if label == "" {
			label = name
		}
		retrieved := source.Retrieved
		if len(retrieved) >= 10 {
			retrieved = retrieved[:10]
		}
		message, colour = "updated "+retrieved, badgeColors["blue"]
		if at, err := time.Parse(time.DateOnly, retrieved); err == nil && datesClock().Sub(at) > 45*24*time.Hour {
			colour = badgeColors["orange"]
		}
	default:
		s.notFound(w, r, "Unknown badge. Use /v1/badge?label=&message=, /v1/badge/market, /v1/badge/holiday or /v1/badge/dataset/{name}.")
		return
	}
	if utf8.RuneCountInString(label) > 60 || utf8.RuneCountInString(message) > 80 {
		s.badRequest(w, r, "label is at most 60 characters and message at most 80.")
		return
	}
	labelColour, err := badgeColor(query.Get("labelColor"), "#555")
	if err != nil {
		s.badRequest(w, r, "labelColor: "+err.Error())
		return
	}
	svg := s.drawBadge(drawable(label), drawable(message), colour, labelColour)
	s.sendImage(w, r, svg, format, 2, false, cache)
}

// ---- address reader ----

type addressView struct {
	Input    string   `json:"input"`
	House    string   `json:"house,omitempty"`
	Street   string   `json:"street,omitempty"`
	Landmark string   `json:"landmark,omitempty"`
	Locality string   `json:"locality,omitempty"`
	City     string   `json:"city,omitempty"`
	District string   `json:"district,omitempty"`
	State    string   `json:"state,omitempty"`
	Pincode  string   `json:"pincode,omitempty"`
	Checked  bool     `json:"pincodeChecked"`
	Offices  []string `json:"postOffices,omitempty"`
	Warnings []string `json:"warnings"`
}

var (
	pincodeInText = regexp.MustCompile(`\b([1-9][0-9]{2})\s?([0-9]{3})\b`)
	houseWords    = regexp.MustCompile(`(?i)^(no\.?|#|flat|door|house|h\.?\s?no\.?|plot|d\.?\s?no\.?|apt\.?|block|unit|floor|shop)\b|^\d+[a-z]?([/-]\d+[a-z]?)*\b`)
	streetWords   = regexp.MustCompile(`(?i)\b(road|rd\.?|street|st\.?|lane|ln\.?|marg|salai|avenue|ave\.?|cross|main|nagar|colony|layout|sector|phase|extension|path|gali|chowk|highway|veedhi|theru|bazaar|bazar)\b`)
	landmarkWords = regexp.MustCompile(`(?i)^(near|opp\.?|opposite|behind|beside|next to|off)\b`)
)

func (s *Server) handleAddressParse(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	text := strings.Join(strings.Fields(in.Text), " ")
	if utf8.RuneCountInString(text) > 500 {
		s.badRequest(w, r, "An address is at most 500 characters.")
		return
	}
	view := addressView{Input: text, Warnings: []string{}}
	rest := text
	if m := pincodeInText.FindAllStringSubmatchIndex(text, -1); len(m) > 0 {
		last := m[len(m)-1]
		view.Pincode = text[last[2]:last[3]] + text[last[4]:last[5]]
		rest = text[:last[0]] + text[last[1]:]
	}
	var parts []string
	for _, part := range strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
		part = strings.Trim(strings.TrimSpace(part), "-–.")
		part = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(part), "India"), "INDIA"))
		if part != "" && !strings.EqualFold(part, "india") {
			parts = append(parts, part)
		}
	}
	// A state named at the end.
	if len(parts) > 0 {
		if st, ok := india.Lookup(parts[len(parts)-1]); ok {
			view.State = st.Name
			parts = parts[:len(parts)-1]
		}
	}
	var pin struct {
		District string `json:"district"`
		State    string `json:"state"`
		Offices  []struct {
			Name string `json:"name"`
		} `json:"offices"`
	}
	if view.Pincode != "" && s.inner(r, "/v1/pincode/"+view.Pincode, &pin) {
		view.Checked = true
		view.District = pin.District
		for _, o := range pin.Offices {
			view.Offices = append(view.Offices, o.Name)
		}
		if view.State == "" {
			view.State = pin.State
		} else if !strings.EqualFold(view.State, pin.State) {
			view.Warnings = append(view.Warnings, "The pincode "+view.Pincode+" is in "+pin.State+", not "+view.State+".")
		}
	} else if view.Pincode != "" {
		view.Warnings = append(view.Warnings, "The pincode "+view.Pincode+" is not a known pincode.")
	} else {
		view.Warnings = append(view.Warnings, "No six-digit pincode found.")
	}
	// The city is the last part left; the district counts as the city when they match.
	if len(parts) > 0 {
		view.City = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
		if view.District != "" && !strings.EqualFold(view.City, view.District) && len(parts) > 0 && strings.EqualFold(parts[len(parts)-1], view.District) {
			parts = parts[:len(parts)-1]
		}
	}
	var leftover []string
	for i, part := range parts {
		switch {
		case view.House == "" && i == 0 && houseWords.MatchString(part):
			view.House = part
		case view.Landmark == "" && landmarkWords.MatchString(part):
			view.Landmark = part
		case view.Street == "" && streetWords.MatchString(part):
			view.Street = part
		default:
			leftover = append(leftover, part)
		}
	}
	if len(leftover) > 0 {
		view.Locality = strings.Join(leftover, ", ")
	}
	if view.District != "" && view.City != "" && !strings.EqualFold(view.City, view.District) {
		matched := false
		for _, o := range view.Offices {
			if strings.Contains(strings.ToLower(o), strings.ToLower(view.City)) || strings.Contains(strings.ToLower(view.City), strings.ToLower(strings.Fields(o)[0])) {
				matched = true
			}
		}
		if !matched && !strings.Contains(strings.ToLower(view.District), strings.ToLower(view.City)) {
			view.Warnings = append(view.Warnings, "The city "+view.City+" is not one of the post offices of "+view.Pincode+" (district "+view.District+"); check both.")
		}
	}
	s.ok(w, r, view, meta{Source: s.source("pincode"), Note: "Read by rules, not a model: the parts are guesses from commas and common words, checked against the pincode. What you send is never logged."}, "private, no-store")
}
