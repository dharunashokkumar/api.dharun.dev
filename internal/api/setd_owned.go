package api

// Set D tools that hold something for their caller or fetch a page for them: request
// bins, throwaway databases, link previews and web pages as Markdown. Making a bin or a database goes through the key gate (catalog Key), so after
// KEY_REQUIRED_FROM it needs a key; what is made is read back with the secret it was
// given, never by anyone else. Everything is kept for a day at most.

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/dharunashokkumar/api.dharun.dev/internal/devtools"
	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
)

const (
	ownedLifetime    = 24 * time.Hour
	hookMaxRequests  = 100
	hookMaxBody      = 64 << 10
	hookMaxBins      = 5000
	hookMaxBytes     = 64 << 20 // every bin together
	ownedPerCaller   = 5
	scratchMaxBytes  = 10 << 20
	scratchMaxDBs    = 500
	scratchMaxRows   = 1000
	scratchQueryTime = 3 * time.Second
	readerAgent      = "IndiaAPI-Reader/1.0 (+https://api.dharun.dev/docs/web)"
)

func (s *Server) registerSetDOwned() {
	s.handle("hooks", costLight, "POST /v1/hooks", s.handleHookCreate)
	s.handle("hooks", costLight, "GET /v1/hooks/{id}", s.handleHookRead)
	s.handle("hooks", costLight, "POST /v1/hooks/{id}/delete", s.handleHookDelete)
	s.handle("hook", costLight, "GET /v1/hook/{id}", s.handleHookCatch)
	s.handle("hook", costLight, "POST /v1/hook/{id}", s.handleHookCatch)
	s.handle("db", costLight, "POST /v1/db", s.handleDBCreate)
	s.handle("db", costLight, "POST /v1/db/{id}/query", s.handleDBQuery)
	s.handle("db", costLight, "POST /v1/db/{id}/delete", s.handleDBDelete)
	s.handle("unfurl", costProbe, "GET /v1/unfurl", s.handleUnfurl)
	s.handle("read", costProbe, "GET /v1/read", s.handleRead)
}

// ---- what callers own ----

type ownedStore struct {
	mu    sync.Mutex
	hooks map[string]*hookBin
	dbs   map[string]*scratchDB
	bytes int // held by every bin's requests
	dir   string
}

// demoID names the public demo bin and database: their secret is demo too, so the
// documentation's examples and anyone trying the API can read them.
const demoID = "demo"

var demoForever = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

func (o *ownedStore) init() {
	if o.hooks != nil {
		return
	}
	o.hooks, o.dbs = map[string]*hookBin{}, map[string]*scratchDB{}
	o.hooks[demoID] = &hookBin{ID: demoID, URL: "/v1/hook/" + demoID, Read: "/v1/hooks/" + demoID, Secret: demoID, Created: time.Now().UTC(), Expires: demoForever, status: http.StatusOK, Requests: []hookRequest{}}
	if db := openDemoDB(); db != nil {
		o.dbs[demoID] = &scratchDB{ID: demoID, Secret: demoID, Query: "/v1/db/" + demoID + "/query", Created: time.Now().UTC(), Expires: demoForever, db: db}
	}
}

// openDemoDB makes the read-only demo database: the states and union territories.
func openDemoDB() *sql.DB {
	db, err := sql.Open("sqlite", "file:demo-states?mode=memory&cache=private")
	if err != nil {
		return nil
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	if _, err := db.Exec("CREATE TABLE states (code TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL, gst_code TEXT, vehicle_code TEXT)"); err != nil {
		db.Close()
		return nil
	}
	for _, st := range india.States {
		db.Exec("INSERT INTO states VALUES (?, ?, ?, ?, ?)", st.Code, st.Name, st.Type, st.GSTCode, st.VehicleCodes[0])
	}
	db.Exec("PRAGMA query_only = 1")
	return db
}

func randomToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ownerOf is who a new bin or database counts against: the key, or else the address.
func ownerOf(r *http.Request) string {
	details := info(r)
	if details.cred.kind == "key" {
		return "key:" + details.cred.keyID
	}
	return "ip:" + details.bucket
}

func secretOf(r *http.Request) string {
	if secret := r.Header.Get("X-Secret"); secret != "" {
		return secret
	}
	return r.URL.Query().Get("secret")
}

func sameSecret(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// sweep removes what has expired. It runs on every change, so nothing needs a timer.
func (o *ownedStore) sweep(now time.Time) {
	for id, bin := range o.hooks {
		if id != demoID && now.After(bin.Expires) {
			o.bytes -= bin.size
			delete(o.hooks, id)
		}
	}
	for id, db := range o.dbs {
		if id != demoID && now.After(db.Expires) {
			db.close()
			delete(o.dbs, id)
		}
	}
}

func (o *ownedStore) count(owner string) int {
	n := 0
	if owner == "" {
		return 0
	}
	for _, bin := range o.hooks {
		if bin.owner == owner {
			n++
		}
	}
	for _, db := range o.dbs {
		if db.owner == owner {
			n++
		}
	}
	return n
}

// ---- request bins ----

type hookBin struct {
	ID       string        `json:"id"`
	Secret   string        `json:"secret,omitempty"`
	URL      string        `json:"url"`
	Read     string        `json:"read"`
	Created  time.Time     `json:"created"`
	Expires  time.Time     `json:"expires"`
	Requests []hookRequest `json:"requests"`
	owner    string
	size     int
	status   int
}

type hookRequest struct {
	At        string              `json:"at"`
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	Query     map[string][]string `json:"query,omitempty"`
	Headers   map[string]string   `json:"headers"`
	Body      string              `json:"body,omitempty"`
	BodyBytes int                 `json:"bodyBytes"`
	Truncated bool                `json:"truncated,omitempty"`
}

// Headers that say where the caller came from are never kept.
var hookDropped = map[string]bool{"Cf-Connecting-Ip": true, "X-Forwarded-For": true, "X-Real-Ip": true, "True-Client-Ip": true, "Cf-Ipcountry": true,
	"Cf-Ray": true, "Cf-Visitor": true, "Cdn-Loop": true, "Forwarded": true, "Cf-Worker": true, "X-Forwarded-Proto": true, "Cf-Ew-Via": true}

func (s *Server) handleHookCreate(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	o := &s.owned
	o.mu.Lock()
	defer o.mu.Unlock()
	o.init()
	now := time.Now()
	o.sweep(now)
	owner := ownerOf(r)
	if o.count(owner) >= ownedPerCaller {
		s.fail(w, r, http.StatusTooManyRequests, "too_many", "You have "+itoa(ownedPerCaller)+" bins and databases already. Delete one, or wait until one expires (a day after it was made).")
		return
	}
	if len(o.hooks) >= hookMaxBins {
		w.Header().Set("Retry-After", "3600")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The server holds as many request bins as it can. Try again later.")
		return
	}
	status := http.StatusOK
	if raw := r.URL.Query().Get("status"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 200 || v > 599 {
			s.badRequest(w, r, "status is the HTTP status the bin answers with, 200 to 599.")
			return
		}
		status = v
	}
	id := randomToken(10)
	origin := s.origin(r)
	bin := &hookBin{ID: id, Secret: randomToken(16), URL: origin + "/v1/hook/" + id, Created: now.UTC(), Expires: now.Add(ownedLifetime).UTC(), owner: owner, status: status, Requests: []hookRequest{}}
	bin.Read = origin + "/v1/hooks/" + id
	o.hooks[id] = bin
	s.ok(w, r, bin, meta{Note: "Send anything (GET or POST) to url; read it back from read with the secret in an X-Secret header. The bin keeps the last 100 requests, 64 KB of body each, for a day, in memory only. The secret is shown once."}, "private, no-store")
}

func (s *Server) handleHookCatch(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	o := &s.owned
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(io.LimitReader(r.Body, hookMaxBody+1))
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.init()
	bin, ok := o.hooks[r.PathValue("id")]
	if !ok || time.Now().After(bin.Expires) {
		s.notFound(w, r, "No such request bin: it may have expired. Make one with POST /v1/hooks.")
		return
	}
	caught := hookRequest{At: time.Now().UTC().Format(time.RFC3339Nano), Method: r.Method, Path: r.URL.Path, Headers: map[string]string{}, BodyBytes: len(body)}
	if len(r.URL.Query()) > 0 {
		caught.Query = r.URL.Query()
	}
	for name, values := range r.Header {
		if !hookDropped[name] {
			caught.Headers[name] = strings.Join(values, ", ")
		}
	}
	if len(body) > hookMaxBody {
		body, caught.Truncated = body[:hookMaxBody], true
	}
	caught.Body = string(body)
	size := len(caught.Body) + 512
	for len(bin.Requests) >= hookMaxRequests || (o.bytes+size > hookMaxBytes && len(bin.Requests) > 0) {
		o.bytes -= len(bin.Requests[0].Body) + 512
		bin.size -= len(bin.Requests[0].Body) + 512
		bin.Requests = bin.Requests[1:]
	}
	if o.bytes+size > hookMaxBytes {
		w.Header().Set("Retry-After", "600")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The server holds as much as it can for request bins right now.")
		return
	}
	bin.Requests = append(bin.Requests, caught)
	bin.size += size
	o.bytes += size
	answer := map[string]any{"ok": true, "caught": len(bin.Requests)}
	if bin.status == http.StatusOK {
		s.ok(w, r, answer, meta{}, "no-store")
		return
	}
	encoded, _ := json.Marshal(envelope{Data: answer})
	writeBody(w, r, bin.status, append(encoded, '\n'), typeJSON, "no-store", false)
}

func (s *Server) ownedBin(w http.ResponseWriter, r *http.Request) *hookBin {
	o := &s.owned
	o.init()
	bin, ok := o.hooks[r.PathValue("id")]
	if !ok || time.Now().After(bin.Expires) || !sameSecret(bin.Secret, secretOf(r)) {
		s.notFound(w, r, "No request bin with that id and secret (send the secret as X-Secret).")
		return nil
	}
	return bin
}

func (s *Server) handleHookRead(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	s.owned.mu.Lock()
	defer s.owned.mu.Unlock()
	bin := s.ownedBin(w, r)
	if bin == nil {
		return
	}
	view := *bin
	view.Secret = ""
	view.Requests = append([]hookRequest(nil), bin.Requests...)
	total := len(view.Requests)
	s.ok(w, r, view, meta{Total: &total, Note: "Oldest first. The caller's address is never kept."}, "private, no-store")
}

func (s *Server) handleHookDelete(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	s.owned.mu.Lock()
	defer s.owned.mu.Unlock()
	bin := s.ownedBin(w, r)
	if bin == nil {
		return
	}
	if bin.ID == demoID {
		s.ok(w, r, map[string]any{"deleted": false, "id": demoID}, meta{Note: "The demo bin is never deleted."}, "private, no-store")
		return
	}
	s.owned.bytes -= bin.size
	delete(s.owned.hooks, bin.ID)
	s.ok(w, r, map[string]any{"deleted": bin.ID}, meta{}, "private, no-store")
}

// ---- throwaway databases ----

type scratchDB struct {
	ID      string    `json:"id"`
	Secret  string    `json:"secret,omitempty"`
	Query   string    `json:"query"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
	owner   string
	path    string
	db      *sql.DB
	mu      sync.Mutex // one statement at a time
}

func (d *scratchDB) close() {
	if d.db != nil {
		d.db.Close()
	}
	os.Remove(d.path)
	os.Remove(d.path + "-journal")
	os.Remove(d.path + "-wal")
}

func (s *Server) scratchDir() (string, error) {
	if s.owned.dir == "" {
		dir, err := os.MkdirTemp("", "dharun-api-scratch-")
		if err != nil {
			return "", err
		}
		s.owned.dir = dir
	}
	return s.owned.dir, nil
}

func (s *Server) handleDBCreate(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	o := &s.owned
	o.mu.Lock()
	defer o.mu.Unlock()
	o.init()
	now := time.Now()
	o.sweep(now)
	owner := ownerOf(r)
	if o.count(owner) >= ownedPerCaller {
		s.fail(w, r, http.StatusTooManyRequests, "too_many", "You have "+itoa(ownedPerCaller)+" bins and databases already. Delete one, or wait until one expires (a day after it was made).")
		return
	}
	if len(o.dbs) >= scratchMaxDBs {
		w.Header().Set("Retry-After", "3600")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The server holds as many databases as it can. Try again later.")
		return
	}
	dir, err := s.scratchDir()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	id := randomToken(10)
	path := filepath.Join(dir, id+".db")
	// 2,560 pages of 4 KB: a database cannot grow past 10 MB.
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=page_size(4096)&_pragma=max_page_count(2560)&_pragma=trusted_schema(0)&_pragma=journal_mode(DELETE)&_pragma=foreign_keys(1)")
	if err == nil {
		db.SetMaxOpenConns(1)
		err = db.Ping()
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	origin := s.origin(r)
	d := &scratchDB{ID: id, Secret: randomToken(16), Query: origin + "/v1/db/" + id + "/query", Created: now.UTC(), Expires: now.Add(ownedLifetime).UTC(), owner: owner, path: path, db: db}
	o.dbs[id] = d
	s.ok(w, r, d, meta{Note: "A private SQLite database for a day, up to 10 MB. POST SQL to query with the secret in an X-Secret header. The secret is shown once."}, "private, no-store")
}

var (
	sqlStrings   = regexp.MustCompile(`'(?:[^']|'')*'|"(?:[^"]|"")*"`)
	sqlComments  = regexp.MustCompile(`(?s)--[^\n]*|/\*.*?\*/`)
	sqlForbidden = regexp.MustCompile(`(?i)\b(attach|detach|vacuum|load_extension|readfile|writefile|fts3_tokenizer)\b`)
	sqlPragma    = regexp.MustCompile(`(?i)\bpragma\s+([a-z_]+)`)
	safePragmas  = map[string]bool{"table_info": true, "table_list": true, "index_list": true, "index_info": true, "foreign_key_list": true, "database_list": false}
)

// checkSQL refuses what could reach outside the database file or change its limits.
func checkSQL(statement string) error {
	bare := sqlComments.ReplaceAllString(sqlStrings.ReplaceAllString(statement, "''"), " ")
	if m := sqlForbidden.FindString(bare); m != "" {
		return errors.New(strings.ToUpper(m) + " is not allowed here")
	}
	for _, m := range sqlPragma.FindAllStringSubmatch(bare, -1) {
		if !safePragmas[strings.ToLower(m[1])] {
			return errors.New("PRAGMA " + m[1] + " is not allowed here; table_info, table_list, index_list, index_info and foreign_key_list are")
		}
	}
	return nil
}

func (s *Server) handleDBQuery(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	s.owned.mu.Lock()
	s.owned.init()
	d, ok := s.owned.dbs[r.PathValue("id")]
	s.owned.mu.Unlock()
	if !ok || time.Now().After(d.Expires) || !sameSecret(d.Secret, secretOf(r)) {
		s.notFound(w, r, "No database with that id and secret (send the secret as X-Secret).")
		return
	}
	raw, ok := s.readJSONBody(w, r)
	if !ok {
		return
	}
	var body struct {
		SQL    string `json:"sql"`
		Params []any  `json:"params"`
	}
	if json.Unmarshal(raw, &body) != nil {
		body.SQL = string(raw) // plain text is the SQL itself
	}
	body.SQL = strings.TrimSpace(body.SQL)
	if body.SQL == "" {
		s.badRequest(w, r, `Send the SQL as text, or JSON {"sql": "...", "params": [...]}.`)
		return
	}
	if err := checkSQL(body.SQL); err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), scratchQueryTime)
	defer cancel()
	started := time.Now()
	rows, err := d.db.QueryContext(ctx, body.SQL, body.Params...)
	if err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "sql_error", sqlMessage(err))
		return
	}
	defer rows.Close()
	columns, _ := rows.Columns()
	out := [][]any{}
	truncated := false
	for rows.Next() {
		if len(out) == scratchMaxRows {
			truncated = true
			break
		}
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			s.fail(w, r, http.StatusUnprocessableEntity, "sql_error", sqlMessage(err))
			return
		}
		for i, v := range values {
			if b, isBytes := v.([]byte); isBytes {
				values[i] = string(b)
			}
		}
		out = append(out, values)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "sql_error", sqlMessage(err))
		return
	}
	var changes int64
	d.db.QueryRowContext(ctx, "SELECT changes()").Scan(&changes)
	if columns == nil {
		columns = []string{}
	}
	result := map[string]any{"columns": columns, "rows": out, "changes": changes, "ms": time.Since(started).Milliseconds()}
	if truncated {
		result["truncated"] = true
	}
	s.ok(w, r, result, meta{Note: "One statement at a time, three seconds each, at most 1,000 rows back. ATTACH, VACUUM and most PRAGMAs are refused."}, "private, no-store")
}

func sqlMessage(err error) string {
	message := err.Error()
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(message, "interrupted"):
		return "The statement ran longer than three seconds and was stopped."
	case strings.Contains(message, "database or disk is full"):
		return "The database is full (10 MB)."
	}
	if i := strings.Index(message, ": "); i >= 0 && strings.HasPrefix(message, "SQL logic error") {
		message = message[i+2:]
	}
	return "SQL error: " + clip(message, 300)
}

func (s *Server) handleDBDelete(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	s.owned.mu.Lock()
	defer s.owned.mu.Unlock()
	s.owned.init()
	d, ok := s.owned.dbs[r.PathValue("id")]
	if !ok || !sameSecret(d.Secret, secretOf(r)) {
		s.notFound(w, r, "No database with that id and secret (send the secret as X-Secret).")
		return
	}
	if d.ID == demoID {
		s.ok(w, r, map[string]any{"deleted": false, "id": demoID}, meta{Note: "The demo database is never deleted."}, "private, no-store")
		return
	}
	d.mu.Lock()
	d.close()
	d.mu.Unlock()
	delete(s.owned.dbs, d.ID)
	s.ok(w, r, map[string]any{"deleted": d.ID}, meta{}, "private, no-store")
}

// ---- fetching pages ----

// fetchPage gets one page through the guarded client: public addresses only, the usual
// ports, at most limit bytes.
func (s *Server) fetchPage(ctx context.Context, target *url.URL, agent string, limit int64) (*http.Response, []byte, error) {
	client := s.guard(80, 443).Client(netguard.ClientOptions{Timeout: 12 * time.Second, Roots: s.extras.network.roots, UserAgent: agent, Redirects: 5})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.5")
	req.Header.Set("Accept-Language", "en-IN,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	return resp, body, err
}

type unfurlView struct {
	URL         string   `json:"url"`
	Status      int      `json:"status"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Image       string   `json:"image,omitempty"`
	ImageAlt    string   `json:"imageAlt,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	SiteName    string   `json:"siteName,omitempty"`
	Type        string   `json:"type,omitempty"`
	Canonical   string   `json:"canonical,omitempty"`
	ThemeColor  string   `json:"themeColor,omitempty"`
	Video       string   `json:"video,omitempty"`
	Twitter     string   `json:"twitterCard,omitempty"`
	Language    string   `json:"language,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

func parseUnfurl(body []byte, base *url.URL) unfurlView {
	view := unfurlView{}
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return view
	}
	metaTags := map[string]string{}
	var title string
	var icons []struct{ rel, href, sizes string }
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Html:
				view.Language = attrOf(n, "lang")
			case atom.Title:
				if title == "" && n.FirstChild != nil {
					title = strings.TrimSpace(n.FirstChild.Data)
				}
			case atom.Meta:
				key := strings.ToLower(attrOf(n, "property"))
				if key == "" {
					key = strings.ToLower(attrOf(n, "name"))
				}
				if key != "" {
					if _, seen := metaTags[key]; !seen {
						metaTags[key] = strings.TrimSpace(attrOf(n, "content"))
					}
				}
			case atom.Link:
				rel := strings.ToLower(attrOf(n, "rel"))
				if strings.Contains(rel, "icon") {
					icons = append(icons, struct{ rel, href, sizes string }{rel, attrOf(n, "href"), attrOf(n, "sizes")})
				}
				if rel == "canonical" {
					view.Canonical = resolveAgainst(base, attrOf(n, "href"))
				}
			case atom.Body:
				return // everything wanted is in the head
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v := metaTags[k]; v != "" {
				return v
			}
		}
		return ""
	}
	view.Title = clip(pick("og:title", "twitter:title"), 300)
	if view.Title == "" {
		view.Title = clip(title, 300)
	}
	view.Description = clip(pick("og:description", "twitter:description", "description"), 600)
	view.Image = resolveAgainst(base, pick("og:image:secure_url", "og:image", "og:image:url", "twitter:image", "twitter:image:src"))
	view.ImageAlt = pick("og:image:alt", "twitter:image:alt")
	view.SiteName = pick("og:site_name", "application-name", "apple-mobile-web-app-title")
	view.Type = pick("og:type")
	view.ThemeColor = pick("theme-color")
	view.Video = resolveAgainst(base, pick("og:video:secure_url", "og:video", "og:video:url"))
	view.Twitter = pick("twitter:card")
	if k := pick("keywords"); k != "" {
		for _, word := range strings.Split(k, ",") {
			if word = strings.TrimSpace(word); word != "" && len(view.Keywords) < 20 {
				view.Keywords = append(view.Keywords, word)
			}
		}
	}
	best := -1
	for _, icon := range icons {
		size, _ := strconv.Atoi(strings.SplitN(strings.ToLower(icon.sizes), "x", 2)[0])
		if strings.Contains(icon.rel, "apple-touch") && size == 0 {
			size = 180
		}
		if size > best || view.Icon == "" {
			best, view.Icon = size, resolveAgainst(base, icon.href)
		}
	}
	if view.Icon == "" && base != nil {
		view.Icon = base.Scheme + "://" + base.Host + "/favicon.ico"
	}
	return view
}

func attrOf(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func resolveAgainst(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || base == nil {
		return ref
	}
	u, err := base.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.String()
}

func (s *Server) handleUnfurl(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	target, host, err := probeURL(r.URL.Query().Get("url"))
	if err != nil {
		if errors.Is(err, netguard.ErrPort) {
			s.failNetwork(w, r, "", err)
			return
		}
		s.badRequest(w, r, "Give url: an http or https address.")
		return
	}
	release, ok := s.startCheck(w, r, host)
	if !ok {
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, body, err := s.fetchPage(ctx, target, probeAgent, 1<<20)
	if err != nil {
		s.failNetwork(w, r, host, err)
		return
	}
	final := resp.Request.URL
	view := parseUnfurl(body, final)
	view.URL, view.Status = final.String(), resp.StatusCode
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") {
		view.Type = resp.Header.Get("Content-Type")
	}
	s.ok(w, r, view, meta{Note: "From the page's Open Graph, Twitter card and HTML head tags, read from India. Only public addresses are fetched; the address is never logged."}, "private, no-store")
}

// robots says whether a site's robots.txt lets our reader fetch path.
func (s *Server) robotsAllow(ctx context.Context, target *url.URL) bool {
	robots := &url.URL{Scheme: target.Scheme, Host: target.Host, Path: "/robots.txt"}
	resp, body, err := s.fetchPage(ctx, robots, readerAgent, 256<<10)
	if err != nil || resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return true // no robots.txt: allowed
	}
	if resp.StatusCode >= 500 {
		return false // the site is in trouble: stay away, as RFC 9309 asks
	}
	return robotsAllows(string(body), "indiaapi-reader", target.EscapedPath())
}

// robotsAllows applies RFC 9309: the group for our agent if there is one, otherwise *;
// the longest matching rule wins, and Allow wins a tie.
func robotsAllows(text, agent, path string) bool {
	type rule struct {
		allow   bool
		pattern string
	}
	groups := map[string][]rule{}
	var current []string
	inRules := false
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
		switch key {
		case "user-agent":
			if inRules {
				current, inRules = nil, false
			}
			current = append(current, strings.ToLower(value))
		case "allow", "disallow":
			inRules = true
			for _, a := range current {
				groups[a] = append(groups[a], rule{key == "allow", value})
			}
		}
	}
	rules, ok := groups[agent]
	if !ok {
		rules = groups["*"]
	}
	if path == "" {
		path = "/"
	}
	best, allowed := -1, true
	for _, rl := range rules {
		if rl.pattern == "" {
			continue
		}
		if robotsMatch(rl.pattern, path) && (len(rl.pattern) > best || len(rl.pattern) == best && rl.allow) {
			best, allowed = len(rl.pattern), rl.allow
		}
	}
	return allowed
}

func robotsMatch(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	pattern = strings.TrimSuffix(pattern, "$")
	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(path, parts[0]) {
		return false
	}
	rest := path[len(parts[0]):]
	for _, part := range parts[1:] {
		i := strings.Index(rest, part)
		if i < 0 {
			return false
		}
		rest = rest[i+len(part):]
	}
	return !anchored || rest == "" || strings.HasSuffix(path, parts[len(parts)-1]) && len(parts) > 1
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	target, host, err := probeURL(r.URL.Query().Get("url"))
	if err != nil {
		if errors.Is(err, netguard.ErrPort) {
			s.failNetwork(w, r, "", err)
			return
		}
		s.badRequest(w, r, "Give url: an http or https address of one page.")
		return
	}
	release, ok := s.startCheck(w, r, host)
	if !ok {
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	if !s.robotsAllow(ctx, target) {
		s.fail(w, r, http.StatusForbidden, "robots_disallowed", "The site's robots.txt asks programs not to read this page, so this API does not.")
		return
	}
	resp, body, err := s.fetchPage(ctx, target, readerAgent, 3<<20)
	if err != nil {
		s.failNetwork(w, r, host, err)
		return
	}
	if resp.StatusCode >= 500 {
		s.fail(w, r, http.StatusBadGateway, "upstream_error", "The site answered "+strconv.Itoa(resp.StatusCode)+".")
		return
	}
	kind := strings.ToLower(resp.Header.Get("Content-Type"))
	final := resp.Request.URL
	var page devtools.Page
	switch {
	case strings.Contains(kind, "html") || kind == "":
		if page, err = devtools.HTMLToMarkdown(string(body), final, r.URL.Query().Get("full") != "true"); err != nil {
			s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The page cannot be read.")
			return
		}
	case strings.HasPrefix(kind, "text/"):
		page = devtools.Page{Markdown: string(body), Words: len(strings.Fields(string(body)))}
	default:
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The address is not a web page or text ("+clip(kind, 60)+"). For a PDF, download it and use POST /v1/pdf/text.")
		return
	}
	if strings.EqualFold(r.URL.Query().Get("format"), "markdown") {
		writeBody(w, r, http.StatusOK, []byte(page.Markdown+"\n"), "text/markdown; charset=utf-8", "private, no-store", true)
		return
	}
	s.ok(w, r, map[string]any{"url": final.String(), "status": resp.StatusCode, "title": page.Title, "description": page.Description, "language": page.Language, "words": page.Words, "markdown": page.Markdown},
		meta{Note: "One page, read from India and turned into Markdown (the main content unless full=true). The site's robots.txt is respected; a site gets at most 10 reads a minute from every caller together."}, "private, no-store")
}
