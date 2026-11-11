package api

// Set D of the idea catalogue (growth plan, 05): developer utilities that are pure
// logic. The logic is in internal/devtools; files are in setd_files.go and the things a
// caller owns (request bins, databases) and fetches (link previews, pages) in
// setd_owned.go.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // zone names work even where the system has no zone files

	"github.com/dharunashokkumar/api.dharun.dev/internal/devtools"
	"github.com/dharunashokkumar/api.dharun.dev/internal/traffic"
)

func (s *Server) registerSetD() {
	both := func(route string, cost int, path string, handler http.HandlerFunc) {
		s.handle(route, cost, "GET "+path, handler)
		s.handle(route, cost, "POST "+path, handler)
	}
	both("convert", costLight, "/v1/convert", s.handleConvert)
	s.handle("diff", costLight, "POST /v1/diff", s.handleDiff)
	s.handle("cron", costLight, "GET /v1/cron", s.handleCron)
	s.handle("ids", costLight, "GET /v1/uuid", s.handleNewIDs)
	s.handle("ids", costLight, "GET /v1/uuid/{id}", s.handleReadID)
	both("hash", costLight, "/v1/hash", s.handleHash)
	both("jwt", costLight, "/v1/jwt/decode", s.handleJWT)
	s.handle("user_agent", costLight, "GET /v1/user-agent", s.handleUserAgent)
	s.handle("time", costLight, "GET /v1/time", s.handleTime)
	both("markdown", costLight, "/v1/markdown", s.handleMarkdown)
	s.handle("color", costLight, "GET /v1/color/contrast", s.handleContrast)
	s.handle("color", costLight, "GET /v1/color/{color}", s.handleColor)
	s.registerSetDFiles()
	s.registerSetDOwned()
}

// ---- convert ----

func formatOf(raw, fallback string) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		raw = fallback
	}
	raw = strings.TrimPrefix(raw, ".")
	if raw == "yml" {
		raw = "yaml"
	}
	for _, f := range devtools.Formats {
		if f == raw {
			return raw, true
		}
	}
	return "", false
}

// guessFormat looks at the start of a document.
func guessFormat(text string) string {
	trimmed := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["):
		return "json"
	case strings.HasPrefix(trimmed, "<"):
		return "xml"
	}
	first, _, _ := strings.Cut(trimmed, "\n")
	if strings.HasPrefix(first, "[") || strings.Contains(first, " = ") {
		return "toml"
	}
	if strings.Count(first, ",") > 0 && !strings.Contains(first, ": ") {
		return "csv"
	}
	return "yaml"
}

func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	from, okFrom := formatOf(in.option(r, "from"), guessFormat(in.Text))
	to, okTo := formatOf(in.option(r, "to"), "json")
	if !okFrom || !okTo {
		s.badRequest(w, r, "from and to are each one of "+strings.Join(devtools.Formats, ", ")+".")
		return
	}
	info(r).subject = from + ">" + to
	value, err := devtools.Decode(from, []byte(in.Text))
	if err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The document cannot be read as "+from+": "+err.Error()+".")
		return
	}
	out, err := devtools.Encode(to, value, true)
	if err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "It cannot be written as "+to+": "+err.Error()+".")
		return
	}
	if in.option(r, "raw") == "true" {
		types := map[string]string{"json": "application/json", "yaml": "application/yaml", "toml": "application/toml", "csv": "text/csv", "xml": "application/xml"}
		writeBody(w, r, http.StatusOK, out, types[to]+"; charset=utf-8", "private, no-store", true)
		return
	}
	s.ok(w, r, map[string]any{"from": from, "to": to, "output": string(out)}, meta{Note: "XML elements become keys, repeated ones lists, attributes keys starting with @. CSV rows become objects named by the first line. raw=true answers with the converted document itself."}, "private, no-store")
}

// ---- diff ----

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	var body struct {
		A       any    `json:"a"`
		B       any    `json:"b"`
		Mode    string `json:"mode"`
		Context *int   `json:"context"`
		Changes bool   `json:"changes"`
	}
	raw, ok := s.readJSONBody(w, r)
	if !ok {
		return
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		s.badRequest(w, r, `Send JSON: {"a": "first text", "b": "second text"}, or two JSON documents with "mode": "json".`)
		return
	}
	context := 3
	if body.Context != nil {
		context = min(max(*body.Context, 0), 20)
	}
	mode := strings.ToLower(body.Mode)
	textA, aIsText := body.A.(string)
	textB, bIsText := body.B.(string)
	if mode == "" {
		mode = "text"
		if !aIsText || !bIsText {
			mode = "json"
		}
	}
	switch mode {
	case "text":
		if !aIsText || !bIsText {
			s.badRequest(w, r, "For a text difference, a and b are both strings.")
			return
		}
		d, err := devtools.DiffText(textA, textB, context, body.Changes)
		if err != nil {
			s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
			return
		}
		s.ok(w, r, d, meta{}, "private, no-store")
	case "json":
		// Strings that hold JSON are read as JSON.
		for _, side := range []*any{&body.A, &body.B} {
			if text, ok := (*side).(string); ok {
				var v any
				if json.Unmarshal([]byte(text), &v) == nil {
					*side = v
				}
			}
		}
		changes := devtools.DiffJSON(body.A, body.B)
		if changes == nil {
			changes = []devtools.JSONChange{}
		}
		total := len(changes)
		s.ok(w, r, map[string]any{"same": total == 0, "changes": changes}, meta{Total: &total, Note: "Each change is a JSON Patch (RFC 6902) operation, with old kept for reading."}, "private, no-store")
	default:
		s.badRequest(w, r, "mode is text or json.")
	}
}

// readJSONBody reads a POST body of at most MAX_BODY_KB.
func (s *Server) readJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	raw, err := readAllLimited(w, r, int64(s.cfg.MaxBodyBytes))
	if err != nil {
		if err == errBodyTooLarge {
			s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The body is larger than "+byteSize(s.cfg.MaxBodyBytes)+".")
		} else {
			s.badRequest(w, r, "The request body could not be read.")
		}
		return nil, false
	}
	return raw, true
}

// readAllLimited reads a body of at most limit bytes.
func readAllLimited(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return nil, errBodyTooLarge
	}
	return raw, err
}

// ---- cron ----

func (s *Server) handleCron(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	expression := query.Get("expr")
	info(r).subject = expression
	c, err := devtools.ParseCron(expression)
	if err != nil {
		s.badRequest(w, r, "The expression cannot be read: "+err.Error()+".")
		return
	}
	zoneName := query.Get("tz")
	if zoneName == "" {
		zoneName = "Asia/Kolkata"
	}
	zone, err := devtools.Zone(zoneName)
	if err != nil {
		s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
		return
	}
	count := 5
	if raw := query.Get("count"); raw != "" {
		if count, err = strconv.Atoi(raw); err != nil || count < 1 || count > 50 {
			s.badRequest(w, r, "count is 1 to 50.")
			return
		}
	}
	from := datesClock().In(zone)
	if raw := query.Get("from"); raw != "" {
		if from, err = devtools.ParseTime(raw, zone, datesClock()); err != nil {
			s.badRequest(w, r, "from: "+err.Error()+".")
			return
		}
	}
	runs := []string{}
	for _, t := range c.Next(from, count) {
		runs = append(runs, t.Format(time.RFC3339))
	}
	cache := "public, max-age=60"
	if query.Get("from") != "" {
		cache = cacheLong
	}
	s.ok(w, r, map[string]any{"expression": c.Expression, "description": c.Describe(), "timeZone": zoneName, "next": runs},
		meta{Note: "Five fields: minute hour day-of-month month day-of-week, read as crontab does (when both day fields are set, either one matching is enough). The runs are in the time zone given (default India)."}, cache)
}

// ---- ids ----

func (s *Server) handleNewIDs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	kind := strings.ToLower(query.Get("kind"))
	if kind == "" {
		kind = "uuid4"
	}
	count := 1
	if raw := query.Get("count"); raw != "" {
		var err error
		if count, err = strconv.Atoi(raw); err != nil || count < 1 || count > 100 {
			s.badRequest(w, r, "count is 1 to 100.")
			return
		}
	}
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		id, err := devtools.NewID(kind, time.Now())
		if err != nil {
			s.badRequest(w, r, "kind is one of "+strings.Join(devtools.IDKinds, ", ")+".")
			return
		}
		ids = append(ids, id)
	}
	s.ok(w, r, map[string]any{"kind": kind, "ids": ids}, meta{Note: "uuid7 and ulid start with the time, so they sort in the order they were made."}, cacheNone)
}

func (s *Server) handleReadID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info(r).subject = id
	got, err := devtools.ReadID(id)
	if err != nil {
		s.badRequest(w, r, "That is not a UUID or ULID.")
		return
	}
	s.ok(w, r, got, meta{}, cacheLong)
}

// ---- hashes ----

func (s *Server) handleHash(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	kinds := []string{"md5", "sha1", "sha256", "sha512"}
	if raw := in.option(r, "algo"); raw != "" {
		kinds = strings.Split(strings.ToLower(strings.ReplaceAll(raw, " ", "")), ",")
	}
	var key []byte
	if k := in.option(r, "key"); k != "" {
		key = []byte(k)
	}
	sums, err := devtools.Hash([]byte(in.Text), kinds, key)
	if err != nil {
		s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
		return
	}
	note := "Digests of the text's UTF-8 bytes."
	if key != nil {
		note = "HMACs of the text with the key. Send keys in a POST body, not in an address."
	}
	s.ok(w, r, sums, meta{Note: note}, "private, no-store")
}

// ---- JWT ----

func (s *Server) handleJWT(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	token, err := devtools.DecodeJWT(in.Text, time.Now())
	if err != nil {
		s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
		return
	}
	s.ok(w, r, token, meta{Note: "Decoded only: the signature is NOT checked, so nothing here proves who made the token. Never paste a live token you do not own."}, "private, no-store")
}

// ---- user agent ----

func (s *Server) handleUserAgent(w http.ResponseWriter, r *http.Request) {
	ua := r.URL.Query().Get("ua")
	own := ua == ""
	if own {
		ua = r.Header.Get("User-Agent")
	}
	origin := traffic.Classify(traffic.Request{UserAgent: ua})
	view := map[string]any{"userAgent": ua, "agent": devtools.ParseAgent(ua), "kind": origin.Kind}
	if origin.Source != "" {
		view["name"] = origin.Source
	}
	cache := cacheLong
	if own {
		cache = "private, no-store"
		w.Header().Set("Vary", "User-Agent")
	}
	s.ok(w, r, view, meta{Note: "kind is how this API labels the program: person, ai-assistant, ai-crawler, search-crawler, bot, program or unknown. Without ua, it reads your own request's User-Agent."}, cache)
}

// ---- time ----

func (s *Server) handleTime(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	fromName := query.Get("from")
	if fromName == "" {
		fromName = "Asia/Kolkata"
	}
	from, err := devtools.Zone(fromName)
	if err != nil {
		s.badRequest(w, r, "from: "+err.Error()+".")
		return
	}
	at, err := devtools.ParseTime(query.Get("at"), from, time.Now())
	if err != nil {
		s.badRequest(w, r, "at: "+err.Error()+".")
		return
	}
	targets := query.Get("to")
	if targets == "" {
		targets = "UTC"
	}
	zones := []devtools.ZoneTime{devtools.In(at, from, fromName, at.In(from))}
	for _, name := range strings.Split(targets, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		loc, err := devtools.Zone(name)
		if err != nil {
			s.badRequest(w, r, "to: "+err.Error()+".")
			return
		}
		zones = append(zones, devtools.In(at, loc, name, at.In(from)))
		if len(zones) > 25 {
			s.badRequest(w, r, "At most 24 zones in to.")
			return
		}
	}
	cache := cacheLong
	if query.Get("at") == "" || strings.EqualFold(query.Get("at"), "now") {
		cache = cacheNone
	}
	s.ok(w, r, map[string]any{"unix": at.Unix(), "utc": at.UTC().Format(time.RFC3339), "zones": zones},
		meta{Note: "The first zone is from (default India). Short names mean the common reading: IST is India, PST/PDT the US Pacific zone, which moves with daylight saving."}, cache)
}

// ---- markdown ----

func (s *Server) handleMarkdown(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	to := strings.ToLower(in.option(r, "to"))
	if to == "" {
		to = "markdown"
		if !strings.Contains(in.Text, "<") {
			to = "html"
		}
	}
	switch to {
	case "html":
		out, err := devtools.MarkdownToHTML(in.Text)
		if err != nil {
			s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The Markdown cannot be read.")
			return
		}
		s.ok(w, r, map[string]any{"html": out}, meta{Note: "GitHub-flavoured Markdown. Raw HTML in the input is left out, so the output is safe to show."}, "private, no-store")
	case "markdown", "md":
		page, err := devtools.HTMLToMarkdown(in.Text, nil, in.option(r, "main") == "true")
		if err != nil {
			s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The HTML cannot be read.")
			return
		}
		s.ok(w, r, page, meta{Note: "main=true keeps only the page's main content, without navigation, headers, footers and forms."}, "private, no-store")
	default:
		s.badRequest(w, r, "to is html or markdown.")
	}
}

// ---- colours ----

type colourView struct {
	Hex       string  `json:"hex"`
	RGB       [3]int  `json:"rgb"`
	HSL       any     `json:"hsl"`
	OKLCH     any     `json:"oklch"`
	CSS       any     `json:"css"`
	Luminance float64 `json:"luminance"`
	TextColor string  `json:"textColor"` // black or white, whichever reads better on it
}

func describeColour(c devtools.RGB) colourView {
	h, sat, l := c.HSL()
	L, C, H := c.OKLCH()
	text := "#000000"
	if devtools.ContrastOf(devtools.RGB{R: 255, G: 255, B: 255}, c).Ratio > devtools.ContrastOf(devtools.RGB{}, c).Ratio {
		text = "#ffffff"
	}
	return colourView{Hex: c.Hex(), RGB: [3]int{int(c.R), int(c.G), int(c.B)},
		HSL: map[string]float64{"h": h, "s": sat, "l": l}, OKLCH: map[string]float64{"l": L, "c": C, "h": H},
		CSS: map[string]string{"rgb": "rgb(" + strconv.Itoa(int(c.R)) + " " + strconv.Itoa(int(c.G)) + " " + strconv.Itoa(int(c.B)) + ")",
			"hsl":   "hsl(" + strconv.FormatFloat(h, 'f', -1, 64) + " " + strconv.FormatFloat(sat, 'f', -1, 64) + "% " + strconv.FormatFloat(l, 'f', -1, 64) + "%)",
			"oklch": "oklch(" + strconv.FormatFloat(L, 'f', -1, 64) + " " + strconv.FormatFloat(C, 'f', -1, 64) + " " + strconv.FormatFloat(H, 'f', -1, 64) + ")"},
		Luminance: float64(int(c.Luminance()*10000)) / 10000, TextColor: text}
}

func (s *Server) handleColor(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("color")
	info(r).subject = raw
	c, err := devtools.ParseColor(raw)
	if err != nil {
		s.badRequest(w, r, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+". In an address, write # as %23 or leave it out.")
		return
	}
	s.ok(w, r, describeColour(c), meta{}, cacheLong)
}

func (s *Server) handleContrast(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	fg, err1 := devtools.ParseColor(query.Get("fg"))
	bg, err2 := devtools.ParseColor(query.Get("bg"))
	if err1 != nil || err2 != nil {
		s.badRequest(w, r, "fg (text) and bg (background) are colours: hex such as 2563eb, rgb(...), hsl(...) or a name.")
		return
	}
	info(r).subject = fg.Hex() + "/" + bg.Hex()
	s.ok(w, r, map[string]any{"fg": fg.Hex(), "bg": bg.Hex(), "contrast": devtools.ContrastOf(fg, bg)},
		meta{Note: "WCAG 2 contrast: AA needs 4.5 for body text and 3 for large text; AAA needs 7 and 4.5. suggestion is the nearest text colour that reaches AA."}, cacheLong)
}
