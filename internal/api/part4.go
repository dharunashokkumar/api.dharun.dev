package api

// Growth plan Part 4: cheap endpoints with proven or obvious demand. Batch lookups,
// today in India and the money calculators live here; avatars, placeholders and charts
// in pictures.go; the calendar feeds in holidays.go.

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/calc"
)

func (s *Server) registerPart4() {
	for _, kind := range calcKinds {
		s.handle("calc", costLight, "GET /v1/calc/"+kind, s.handleCalc)
	}
	s.handle("batch", costLight, "POST /v1/batch", s.handleBatch)
	s.handle("today", costLight, "GET /v1/today", s.handleToday)
	s.handle("avatar", costLight, "GET /v1/avatar/{name}", s.handleAvatar)
	s.handle("placeholder", costLight, "GET /v1/placeholder/{size}", s.handlePlaceholder)
	s.handle("chart", costHeavy, "GET /v1/chart", s.handleChart)
	s.handle("chart", costHeavy, "POST /v1/chart", s.handleChart)
}

// ---- calculators ----

// numbers reads the named query numbers; a missing one is zero.
type queryReader struct {
	values url.Values
	err    string
}

func (q *queryReader) number(name string) float64 {
	raw := strings.ReplaceAll(strings.TrimSpace(q.values.Get(name)), ",", "")
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil && q.err == "" {
		q.err = name + " must be a number."
	}
	return value
}

func (q *queryReader) integer(name string) int { return int(q.number(name)) }

func (q *queryReader) flag(name string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(q.values.Get(name))) {
	case "":
		return fallback
	case "true", "yes", "1":
		return true
	case "false", "no", "0":
		return false
	}
	if q.err == "" {
		q.err = name + " must be true or false."
	}
	return fallback
}

var calcKinds = []string{"sip", "lumpsum", "emi", "gst", "income-tax", "hra", "gratuity", "ppf", "capital-gains", "trade"}

func (s *Server) handleCalc(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimPrefix(r.URL.Path, "/v1/calc/")
	info(r).subject = kind
	q := &queryReader{values: r.URL.Query()}
	text := func(name string) string { return strings.ToLower(strings.TrimSpace(q.values.Get(name))) }
	var result any
	var err error
	switch kind {
	case "sip":
		result, err = calc.SIP(calc.SIPRequest{Monthly: q.number("monthly"), Return: q.number("rate"), Years: q.integer("years"), StepUp: q.number("stepup")})
	case "lumpsum":
		result, err = calc.LumpSum(q.number("amount"), q.number("rate"), q.integer("years"))
	case "emi":
		months := q.integer("months")
		if months == 0 {
			months = q.integer("years") * 12
		}
		result, err = calc.EMI(q.number("amount"), q.number("rate"), months, q.flag("schedule", false))
	case "gst":
		result, err = calc.GST(q.number("amount"), q.number("rate"), q.flag("inclusive", false), q.flag("interstate", false))
	case "income-tax":
		result, err = calc.IncomeTaxFor(calc.IncomeTaxRequest{Income: q.number("income"), Year: text("year"), Age: text("age"),
			Salaried: q.flag("salaried", true), Deductions: q.number("deductions")}, text("regime"))
	case "hra":
		result, err = calc.HRA(q.number("basic"), q.number("hra"), q.number("rent"), q.flag("metro", false))
	case "gratuity":
		result, err = calc.Gratuity(q.number("salary"), q.number("years"), q.flag("covered", true))
	case "ppf":
		years := q.integer("years")
		if years == 0 {
			years = 15
		}
		result, err = calc.PPF(q.number("yearly"), years, q.number("rate"))
	case "capital-gains":
		result, err = calc.CapitalGains(calc.CapitalGainsRequest{Asset: text("asset"), Bought: text("bought"), Sold: text("sold"),
			Cost: q.number("cost"), Price: q.number("price"), Expenses: q.number("expenses")})
	case "trade":
		result, err = calc.Trade(calc.TradeRequest{Buy: q.number("buy"), Sell: q.number("sell"), Quantity: q.integer("quantity"),
			Kind: text("kind"), Brokerage: q.number("brokerage"), DP: q.number("dp")})
	default:
		s.notFound(w, r, "Unknown calculator. Use one of: "+strings.Join(calcKinds, ", ")+".")
		return
	}
	if q.err != "" {
		s.badRequest(w, r, q.err)
		return
	}
	if err != nil {
		s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
		return
	}
	s.ok(w, r, result, meta{Note: "A calculation from the rules named in rule, for planning. It is not tax or financial advice; check figures that matter with the source or an adviser."}, cacheLong)
}

// ---- batch lookups ----

// batchKinds maps a lookup kind to the address it runs, with {} for the value.
var batchKinds = map[string]string{
	"pincode": "/v1/pincode/{}", "ifsc": "/v1/ifsc/{}", "bank": "/v1/banks/{}", "phone": "/v1/phone/{}", "rto": "/v1/rto/{}",
	"hsn": "/v1/hsn/{}", "company": "/v1/companies/{}", "state": "/v1/states/{}", "airport": "/v1/airports/{}", "station": "/v1/stations/{}",
	"gstin": "/v1/validate/gstin/{}", "pan": "/v1/validate/pan/{}", "ifsc-format": "/v1/validate/ifsc/{}", "words": "/v1/words/{}",
}

const batchMax = 1000

type batchItem struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type batchAnswer struct {
	Kind   string          `json:"kind"`
	Value  string          `json:"value"`
	Status int             `json:"status"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

// handleBatch runs up to 1,000 lookups in one request, each exactly as its own
// endpoint answers it. Ten lookups cost one unit.
func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	var body struct {
		Kind    string      `json:"kind"`
		Values  []string    `json:"values"`
		Lookups []batchItem `json:"lookups"`
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 2<<20+1))
	if err != nil || len(raw) > 2<<20 {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "A batch body may be at most 2 MB.")
		return
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "text/csv") || strings.EqualFold(r.URL.Query().Get("format"), "csv") && !strings.HasPrefix(contentType, "application/json") {
		s.batchCSV(w, r, raw)
		return
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		s.badRequest(w, r, `Send JSON: {"kind": "pincode", "values": ["637001", "600001"]} or {"lookups": [{"kind": "ifsc", "value": "SBIN0000001"}]}, or a CSV file as text/csv with kind and column.`)
		return
	}
	items := body.Lookups
	for _, value := range body.Values {
		items = append(items, batchItem{Kind: body.Kind, Value: value})
	}
	if len(items) == 0 || len(items) > batchMax {
		s.badRequest(w, r, "Send 1 to "+strconv.Itoa(batchMax)+" lookups.")
		return
	}
	for _, item := range items {
		if _, ok := batchKinds[strings.ToLower(item.Kind)]; !ok {
			s.badRequest(w, r, "Unknown kind "+strconv.Quote(clip(item.Kind, 30))+". Kinds: "+strings.Join(batchKindNames(), ", ")+".")
			return
		}
	}
	if extra := (len(items)+9)/10 - 1; extra > 0 && !s.chargeMore(w, r, extra) {
		return
	}
	answers := make([]batchAnswer, len(items))
	for i, item := range items {
		answers[i] = s.batchLookup(r, strings.ToLower(item.Kind), item.Value)
	}
	if strings.EqualFold(r.URL.Query().Get("format"), "ndjson") {
		var out strings.Builder
		for _, answer := range answers {
			line, _ := json.Marshal(answer)
			out.Write(line)
			out.WriteByte('\n')
		}
		writeBody(w, r, http.StatusOK, []byte(out.String()), "application/x-ndjson; charset=utf-8", "private, no-store", true)
		return
	}
	total := len(answers)
	s.ok(w, r, answers, meta{Total: &total, Note: "Each answer is what its own endpoint returns for that value, in the order sent. Ten lookups cost one unit of the rate limit."}, "private, no-store")
}

// batchLookup runs one lookup as its own endpoint answers it, charged to the batch.
func (s *Server) batchLookup(r *http.Request, kind, value string) batchAnswer {
	answer := batchAnswer{Kind: kind, Value: value}
	outer := info(r)
	address := strings.Replace(batchKinds[kind], "{}", url.PathEscape(strings.TrimSpace(value)), 1)
	ctx := context.WithValue(r.Context(), infoKey{}, &requestInfo{internal: true, client: "batch|" + outer.bucket, cred: outer.cred})
	inner, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil || strings.TrimSpace(value) == "" {
		answer.Status = http.StatusBadRequest
		answer.Error = json.RawMessage(`{"code":"bad_request","message":"Empty or unusable value."}`)
		return answer
	}
	inner.Host = r.Host
	recorder := &memoryWriter{header: http.Header{}}
	s.mux.ServeHTTP(recorder, inner)
	answer.Status = recorder.status
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	json.Unmarshal(recorder.body.Bytes(), &envelope)
	answer.Data, answer.Error = envelope.Data, envelope.Error
	return answer
}

// batchCSV enriches a CSV file: for each row it looks up the value in one column and
// adds the answer's fields as new columns (nested ones flattened with a dot), so a
// spreadsheet of pincodes or IFSC codes comes back with district, bank and so on.
func (s *Server) batchCSV(w http.ResponseWriter, r *http.Request, raw []byte) {
	query := r.URL.Query()
	kind := strings.ToLower(query.Get("kind"))
	if _, ok := batchKinds[kind]; !ok {
		s.badRequest(w, r, "With a CSV file, kind names the lookup: "+strings.Join(batchKindNames(), ", ")+".")
		return
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		s.badRequest(w, r, "The CSV needs a header row and at least one row.")
		return
	}
	header := records[0]
	column := -1
	want := strings.TrimSpace(query.Get("column"))
	for i, name := range header {
		if strings.EqualFold(strings.TrimSpace(name), want) || (want == "" && strings.EqualFold(strings.TrimSpace(name), kind)) {
			column = i
			break
		}
	}
	if column < 0 {
		s.badRequest(w, r, "column names the CSV column that holds the values (by its header); without it, a column named "+kind+" is used.")
		return
	}
	rows := records[1:]
	if len(rows) > batchMax {
		s.badRequest(w, r, "A CSV may have up to "+strconv.Itoa(batchMax)+" rows.")
		return
	}
	if extra := (len(rows)+9)/10 - 1; extra > 0 && !s.chargeMore(w, r, extra) {
		return
	}
	// The answers' fields, flattened, in the order first seen; then one row each.
	var added []string
	seen := map[string]bool{}
	results := make([]map[string]string, len(rows))
	for i, row := range rows {
		value := ""
		if column < len(row) {
			value = row[column]
		}
		answer := s.batchLookup(r, kind, value)
		flat := map[string]string{"status": strconv.Itoa(answer.Status)}
		var data any
		if json.Unmarshal(answer.Data, &data) == nil {
			flattenInto(flat, "", data)
		}
		keys := make([]string, 0, len(flat))
		for k := range flat {
			if k != "status" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range append([]string{"status"}, keys...) {
			if !seen[k] {
				seen[k] = true
				added = append(added, k)
			}
		}
		results[i] = flat
	}
	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	head := append([]string{}, header...)
	for _, k := range added {
		head = append(head, kind+"."+k)
	}
	writer.Write(head)
	for i, row := range rows {
		line := append([]string{}, row...)
		for len(line) < len(header) {
			line = append(line, "")
		}
		for _, k := range added {
			line = append(line, results[i][k])
		}
		writer.Write(line)
	}
	writer.Flush()
	w.Header().Set("Content-Disposition", `attachment; filename="enriched.csv"`)
	writeBody(w, r, http.StatusOK, out.Bytes(), "text/csv; charset=utf-8", "private, no-store", true)
}

// flattenInto writes a JSON value's leaves as dotted keys; a list of plain values is
// joined with "; ", a list of objects gives its first one (pincodes have several post
// offices) and its length.
func flattenInto(out map[string]string, prefix string, v any) {
	key := func(k string) string {
		if prefix == "" {
			return k
		}
		return prefix + "." + k
	}
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			flattenInto(out, key(k), child)
		}
	case []any:
		if len(x) == 0 {
			return
		}
		if _, nested := x[0].(map[string]any); nested {
			out[key("count")] = strconv.Itoa(len(x))
			flattenInto(out, prefix, x[0])
			return
		}
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = fmt.Sprint(item)
		}
		out[prefix] = strings.Join(parts, "; ")
	case nil:
	case float64:
		out[prefix] = strconv.FormatFloat(x, 'f', -1, 64)
	default:
		out[prefix] = fmt.Sprint(x)
	}
}

func batchKindNames() []string {
	keys := make([]string, 0, len(batchKinds))
	for k := range batchKinds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---- today in India ----

type todayView struct {
	Now            string         `json:"now"`
	Date           string         `json:"date"`
	Weekday        string         `json:"weekday"`
	FinancialYear  string         `json:"financialYear"`
	Quarter        string         `json:"quarter"`
	AssessmentYear string         `json:"assessmentYear"`
	State          string         `json:"state,omitempty"`
	Working        map[string]any `json:"working"`
	Holidays       []holidayView  `json:"holidays"`
	Festivals      any            `json:"festivals"`
	Market         any            `json:"market,omitempty"`
	Sun            any            `json:"sun,omitempty"`
	Panchang       map[string]any `json:"panchang,omitempty"`
}

// inner runs one of this API's own addresses in memory and decodes its data.
func (s *Server) inner(r *http.Request, address string, into any) bool {
	outer := info(r)
	ctx := context.WithValue(r.Context(), infoKey{}, &requestInfo{internal: true, client: outer.bucket, cred: outer.cred})
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return false
	}
	request.Host = r.Host
	recorder := &memoryWriter{header: http.Header{}}
	s.mux.ServeHTTP(recorder, request)
	if recorder.status != http.StatusOK {
		return false
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	return json.Unmarshal(recorder.body.Bytes(), &envelope) == nil && json.Unmarshal(envelope.Data, into) == nil
}

// todayClock tells the time for /v1/today; tests fix it.
var todayClock = time.Now

func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	now := todayClock().In(indiaTime)
	if at := query.Get("date"); at != "" {
		day, err := time.ParseInLocation("2006-01-02", at, indiaTime)
		if err != nil {
			s.badRequest(w, r, "date must be YYYY-MM-DD.")
			return
		}
		now = day.Add(12 * time.Hour)
	}
	state, ok := stateParam(r)
	if !ok {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return
	}
	pincode := strings.TrimSpace(query.Get("pincode"))
	if pincode != "" && (len(pincode) != 6 || strings.Trim(pincode, "0123456789") != "") {
		s.badRequest(w, r, "pincode must be six digits.")
		return
	}
	date := now.Format("2006-01-02")
	fy := now.Year()
	if now.Month() < time.April {
		fy--
	}
	quarter := (int(now.Month())+8)%12/3 + 1
	view := todayView{Now: now.Format(time.RFC3339), Date: date, Weekday: now.Weekday().String(),
		FinancialYear: strconv.Itoa(fy) + "-" + strconv.Itoa(fy + 1)[2:], Quarter: "Q" + strconv.Itoa(quarter),
		AssessmentYear: strconv.Itoa(fy+1) + "-" + strconv.Itoa(fy + 2)[2:], State: state, Working: map[string]any{}, Holidays: []holidayView{}}

	calendars := []struct{ name, address string }{{"market", "/v1/working-days/" + date + "?calendar=market"}, {"central", "/v1/working-days/" + date + "?calendar=central"}}
	if state != "" {
		calendars = append(calendars, struct{ name, address string }{"bank", "/v1/working-days/" + date + "?calendar=bank&state=" + state})
	}
	for _, c := range calendars {
		var day struct {
			Working bool     `json:"working"`
			Reasons []string `json:"reasons"`
		}
		if s.inner(r, c.address, &day) {
			view.Working[c.name] = map[string]any{"working": day.Working, "reasons": day.Reasons}
		}
	}
	holidays := "/v1/holidays/" + now.Format("2006") + "?month=" + strconv.Itoa(int(now.Month()))
	if state != "" {
		holidays += "&state=" + state
	}
	var list []holidayView
	if s.inner(r, holidays, &list) {
		for _, h := range list {
			if h.Date == date {
				view.Holidays = append(view.Holidays, h)
			}
		}
	}
	view.Festivals = todaysFestivals(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), state)
	if query.Get("date") == "" {
		var market any
		if s.inner(r, "/v1/market/status", &market) {
			view.Market = market
		}
	}
	if pincode != "" {
		var sky struct {
			Sun any `json:"sun"`
		}
		if s.inner(r, "/v1/sun-moon/"+date+"?pincode="+pincode, &sky) {
			view.Sun = sky.Sun
		}
		var almanac map[string]any
		if s.inner(r, "/v1/panchang/"+date+"?pincode="+pincode, &almanac) {
			view.Panchang = map[string]any{}
			for _, key := range []string{"tithi", "nakshatra", "yoga", "rahuKalam", "tamilMonth", "lunarMonth"} {
				if value, ok := almanac[key]; ok {
					if list, isList := value.([]any); isList && len(list) > 0 {
						value = list[0]
					}
					view.Panchang[key] = value
				}
			}
		}
	}
	m := meta{Source: s.source("holidays-bank"), Note: "Everything for one day in India time. working.bank needs state; sun and panchang need pincode (the panchang counts as 10 more units)."}
	cache := "public, max-age=60"
	if query.Get("date") != "" {
		cache = cacheAPI
	}
	s.ok(w, r, view, m, cache)
}
