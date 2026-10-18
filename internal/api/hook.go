package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// Request bin: a URL that records every request sent to it, read back as JSON by whoever
// holds the bin's token. For testing webhooks. What it records is strangers' data, so it
// is never shown as a page under the domain, never logged, kept for a day at most, and
// the headers that carry the sender's address are dropped before anything is stored.

const (
	hookLifetime    = 24 * time.Hour
	hookKeep        = 100      // requests a bin keeps, newest first
	hookBodyLimit   = 64 << 10 // of each request's body
	hookHeaderLimit = 16 << 10 // of each request's headers, names and values together
	hookPerMinute   = 60       // requests one bin records a minute; more are refused
	hookMaxBins     = 5000     // bins alive at once across all callers
	hookMaxBytes    = 256 << 20
	// hookExample is the bin the documentation reads: fixed requests, never stored.
	hookExample = "example"
)

// hookDropped are headers that name the sender's address or were added on the way here.
var hookDropped = map[string]bool{
	"Cf-Connecting-Ip": true, "Cf-Connecting-Ipv6": true, "Cf-Ipcountry": true, "Cf-Ray": true, "Cf-Visitor": true,
	"Cf-Warp-Tag-Id": true, "Cf-Pseudo-Ipv4": true, "Cdn-Loop": true, "True-Client-Ip": true, "X-Forwarded-For": true,
	"X-Forwarded-Proto": true, "X-Forwarded-Host": true, "X-Real-Ip": true, "Forwarded": true, "Connection": true,
	"Accept-Encoding": true,
}

func init() {
	// The bin's own URL with any path after it is as private as the bin's URL.
	privatePaths["/v1/hook/{id}/{rest...}"] = true
}

type hookStore struct {
	db    *sql.DB
	bytes atomic.Int64 // body bytes stored across all bins
	mu    sync.Mutex
	seen  map[string]int // requests recorded this minute, per bin
}

const hookSchema = `
CREATE TABLE IF NOT EXISTS bins (
	id TEXT PRIMARY KEY,
	token BLOB NOT NULL,
	created INTEGER NOT NULL,
	expires INTEGER NOT NULL,
	next INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS requests (
	bin TEXT NOT NULL,
	seq INTEGER NOT NULL,
	received INTEGER NOT NULL,
	method TEXT NOT NULL,
	path TEXT NOT NULL,
	query TEXT NOT NULL,
	headers TEXT NOT NULL,
	body BLOB NOT NULL,
	size INTEGER NOT NULL,
	PRIMARY KEY (bin, seq)
);
CREATE INDEX IF NOT EXISTS bins_expires ON bins (expires);`

func openHookStore(stateDir string) (*hookStore, error) {
	db, err := openStateDB(stateDir, "hooks", hookSchema)
	if err != nil {
		return nil, err
	}
	store := &hookStore{db: db, seen: map[string]int{}}
	var stored int64
	if err := db.QueryRow(`SELECT COALESCE(SUM(length(body)), 0) FROM requests`).Scan(&stored); err != nil {
		db.Close()
		return nil, err
	}
	store.bytes.Store(stored)
	return store, nil
}

func (h *hookStore) close() { h.db.Close() }

// sweep deletes bins that have run out, and starts the per-minute counts again.
func (h *hookStore) sweep(now time.Time) {
	h.mu.Lock()
	h.seen = map[string]int{}
	h.mu.Unlock()
	rows, err := h.db.Query(`DELETE FROM requests WHERE bin IN (SELECT id FROM bins WHERE expires <= ?) RETURNING length(body)`, now.Unix())
	if err != nil {
		logError("sweeping request bins", err)
		return
	}
	h.release(rows)
	if _, err := h.db.Exec(`DELETE FROM bins WHERE expires <= ?`, now.Unix()); err != nil {
		logError("sweeping request bins", err)
	}
}

// release subtracts the sizes a DELETE ... RETURNING reports from the stored total.
func (h *hookStore) release(rows *sql.Rows) {
	defer rows.Close()
	for rows.Next() {
		var size int64
		if rows.Scan(&size) == nil {
			h.bytes.Add(-size)
		}
	}
}

// allow counts one request for a bin and reports whether it is within the bin's minute.
func (h *hookStore) allow(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seen[id]++
	return h.seen[id] <= hookPerMinute
}

type hookBin struct {
	ID      string
	token   []byte
	Created time.Time
	Expires time.Time
}

func (h *hookStore) bin(id string, now time.Time) (*hookBin, error) {
	var bin hookBin
	var created, expires int64
	err := h.db.QueryRow(`SELECT id, token, created, expires FROM bins WHERE id = ? AND expires > ?`, id, now.Unix()).
		Scan(&bin.ID, &bin.token, &created, &expires)
	if err != nil {
		return nil, err
	}
	bin.Created, bin.Expires = time.Unix(created, 0).UTC(), time.Unix(expires, 0).UTC()
	return &bin, nil
}

// randomID is n random bytes in lower-case base32, which reads well in a URL.
func randomID(n int) string {
	raw := make([]byte, n)
	rand.Read(raw)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
}

// randomToken is a secret for a caller to hold; only its SHA-256 is stored.
func randomToken() (token string, hash []byte) {
	raw := make([]byte, 24)
	rand.Read(raw)
	token = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:]
}

// bearer reads a token from the Authorization header, or else from ?token=.
func bearer(r *http.Request) string {
	if value, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return strings.TrimSpace(value)
	}
	return r.URL.Query().Get("token")
}

func tokenMatches(token string, hash []byte) bool {
	sum := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(sum[:], hash) == 1
}

func (s *Server) hooks(w http.ResponseWriter, r *http.Request) *hookStore {
	if store := s.extras.hooks; store != nil {
		return store
	}
	s.fail(w, r, http.StatusServiceUnavailable, "not_available", "Request bins are not available on this server.")
	return nil
}

type hookCreated struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ReadURL   string `json:"readUrl"`
	Token     string `json:"token"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
	Keeps     int    `json:"keeps"`
}

func (s *Server) handleHookCreate(w http.ResponseWriter, r *http.Request) {
	store := s.hooks(w, r)
	if store == nil {
		return
	}
	info(r).private = true
	now := time.Now().UTC()
	var live int
	store.db.QueryRow(`SELECT COUNT(*) FROM bins WHERE expires > ?`, now.Unix()).Scan(&live)
	if live >= hookMaxBins || store.bytes.Load() >= hookMaxBytes {
		w.Header().Set("Retry-After", "600")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "Every request bin is in use. Try again later.")
		return
	}
	id := randomID(10)
	token, hash := randomToken()
	expires := now.Add(hookLifetime)
	if _, err := store.db.Exec(`INSERT INTO bins (id, token, created, expires) VALUES (?, ?, ?, ?)`, id, hash, now.Unix(), expires.Unix()); err != nil {
		logError("creating a request bin", err)
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	origin := s.origin(r)
	s.ok(w, r, hookCreated{
		ID: id, URL: origin + "/v1/hook/" + id, ReadURL: origin + "/v1/hooks/" + id, Token: token,
		CreatedAt: now.Format(time.RFC3339), ExpiresAt: expires.Format(time.RFC3339), Keeps: hookKeep,
	}, meta{Note: "Send anything to url, with any method and any path after it. Read it back from readUrl with the header Authorization: Bearer <token>. The token is shown only now."}, cacheNone)
}

// handleHookReceive records one request sent to a bin.
func (s *Server) handleHookReceive(w http.ResponseWriter, r *http.Request) {
	store := s.hooks(w, r)
	if store == nil {
		return
	}
	details := info(r)
	details.private = true
	id := r.PathValue("id")
	received := func() {
		w.Header().Set("Cache-Control", cacheNone)
		s.ok(w, r, map[string]any{"received": true}, meta{}, cacheNone)
	}
	if id == hookExample {
		received() // the documentation's bin keeps nothing
		return
	}
	now := time.Now().UTC()
	if _, err := store.bin(id, now); err != nil {
		s.notFound(w, r, "No request bin with that ID, or it has expired. Bins last 24 hours.")
		return
	}
	if !store.allow(id) {
		w.Header().Set("Retry-After", "60")
		s.fail(w, r, http.StatusTooManyRequests, "rate_limited", "This bin records up to "+strconv.Itoa(hookPerMinute)+" requests a minute.")
		return
	}
	if store.bytes.Load() >= hookMaxBytes {
		w.Header().Set("Retry-After", "600")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "Request bins are full. Try again later.")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, hookBodyLimit+1))
	size := len(body)
	if size > hookBodyLimit {
		// The rest is counted, not kept.
		rest, _ := io.Copy(io.Discard, io.LimitReader(r.Body, int64(s.cfg.MaxBodyBytes)))
		size += int(rest)
		body = body[:hookBodyLimit]
	}
	path := "/" + r.PathValue("rest")
	headers := encodeJSON(hookHeaders(r))

	tx, err := store.db.Begin()
	if err != nil {
		logError("recording a request", err)
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer tx.Rollback()
	var seq int64
	if err := tx.QueryRow(`UPDATE bins SET next = next + 1 WHERE id = ? RETURNING next - 1`, id).Scan(&seq); err != nil {
		s.notFound(w, r, "No request bin with that ID, or it has expired. Bins last 24 hours.")
		return
	}
	if _, err := tx.Exec(`INSERT INTO requests (bin, seq, received, method, path, query, headers, body, size) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, seq, now.UnixMilli(), r.Method, clip(path, 2000), clip(r.URL.RawQuery, 4000), headers, body, size); err != nil {
		logError("recording a request", err)
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := tx.Query(`DELETE FROM requests WHERE bin = ? AND seq <= ? RETURNING length(body)`, id, seq-hookKeep)
	if err == nil {
		var freed int64
		for rows.Next() {
			var n int64
			if rows.Scan(&n) == nil {
				freed += n
			}
		}
		rows.Close()
		store.bytes.Add(-freed)
	}
	if err := tx.Commit(); err != nil {
		logError("recording a request", err)
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	store.bytes.Add(int64(len(body)))
	received()
}

// hookHeaders keeps the request's headers, less those naming the sender's address or
// added on the way, within hookHeaderLimit.
func hookHeaders(r *http.Request) map[string][]string {
	names := make([]string, 0, len(r.Header))
	for name := range r.Header {
		if !hookDropped[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	kept := map[string][]string{}
	if r.Host != "" {
		kept["Host"] = []string{r.Host}
	}
	used := 0
	for _, name := range names {
		for _, value := range r.Header[name] {
			if used += len(name) + len(value); used > hookHeaderLimit {
				return kept
			}
			kept[name] = append(kept[name], value)
		}
	}
	return kept
}

type hookRequest struct {
	Seq          int64               `json:"seq"`
	ReceivedAt   string              `json:"receivedAt"`
	Method       string              `json:"method"`
	Path         string              `json:"path"`
	Query        string              `json:"query"`
	Headers      map[string][]string `json:"headers"`
	Body         string              `json:"body"`
	BodyEncoding string              `json:"bodyEncoding"` // text or base64
	Size         int                 `json:"size"`
	Truncated    bool                `json:"truncated"`
}

type hookContents struct {
	ID        string        `json:"id"`
	URL       string        `json:"url"`
	CreatedAt string        `json:"createdAt"`
	ExpiresAt string        `json:"expiresAt"`
	Requests  []hookRequest `json:"requests"`
}

func (s *Server) handleHookRead(w http.ResponseWriter, r *http.Request) {
	store := s.hooks(w, r)
	if store == nil {
		return
	}
	info(r).private = true
	id := r.PathValue("id")
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	limit := hookKeep
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > hookKeep {
			s.badRequest(w, r, "limit must be 1 to "+strconv.Itoa(hookKeep)+".")
			return
		}
		limit = parsed
	}
	origin := s.origin(r)
	if id == hookExample {
		if bearer(r) != hookExample {
			s.fail(w, r, http.StatusUnauthorized, "unauthorized", "The example bin's token is example.")
			return
		}
		s.ok(w, r, hookExampleContents(origin), meta{Note: "The documentation's example bin: fixed requests, nothing sent to it is kept."}, cacheNone)
		return
	}
	now := time.Now().UTC()
	bin, err := store.bin(id, now)
	if err != nil {
		s.notFound(w, r, "No request bin with that ID, or it has expired. Bins last 24 hours.")
		return
	}
	if token := bearer(r); token == "" || !tokenMatches(token, bin.token) {
		s.fail(w, r, http.StatusUnauthorized, "unauthorized", "Send the bin's token as Authorization: Bearer <token>.")
		return
	}
	rows, err := store.db.Query(`SELECT seq, received, method, path, query, headers, body, size FROM requests
		WHERE bin = ? AND seq > ? ORDER BY seq DESC LIMIT ?`, id, since, limit)
	if err != nil {
		logError("reading a request bin", err)
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	contents := hookContents{ID: id, URL: origin + "/v1/hook/" + id, CreatedAt: bin.Created.Format(time.RFC3339),
		ExpiresAt: bin.Expires.Format(time.RFC3339), Requests: []hookRequest{}}
	for rows.Next() {
		var request hookRequest
		var received int64
		var headers string
		var body []byte
		if err := rows.Scan(&request.Seq, &received, &request.Method, &request.Path, &request.Query, &headers, &body, &request.Size); err != nil {
			continue
		}
		request.ReceivedAt = time.UnixMilli(received).UTC().Format(time.RFC3339Nano)
		decodeJSON(headers, &request.Headers)
		request.Body, request.BodyEncoding = hookBody(body)
		request.Truncated = request.Size > len(body)
		contents.Requests = append(contents.Requests, request)
	}
	s.ok(w, r, contents, meta{}, cacheNone)
}

func hookBody(body []byte) (string, string) {
	if utf8.Valid(body) {
		return string(body), "text"
	}
	return base64.StdEncoding.EncodeToString(body), "base64"
}

func (s *Server) handleHookDelete(w http.ResponseWriter, r *http.Request) {
	store := s.hooks(w, r)
	if store == nil {
		return
	}
	info(r).private = true
	id := r.PathValue("id")
	if id == hookExample {
		s.ok(w, r, map[string]any{"deleted": false}, meta{Note: "The documentation's example bin cannot be deleted."}, cacheNone)
		return
	}
	bin, err := store.bin(id, time.Now().UTC())
	if err != nil {
		s.notFound(w, r, "No request bin with that ID, or it has expired.")
		return
	}
	if token := bearer(r); token == "" || !tokenMatches(token, bin.token) {
		s.fail(w, r, http.StatusUnauthorized, "unauthorized", "Send the bin's token as Authorization: Bearer <token>.")
		return
	}
	if rows, err := store.db.Query(`DELETE FROM requests WHERE bin = ? RETURNING length(body)`, id); err == nil {
		store.release(rows)
	}
	store.db.Exec(`DELETE FROM bins WHERE id = ?`, id)
	s.ok(w, r, map[string]any{"deleted": true}, meta{}, cacheNone)
}

func hookExampleContents(origin string) hookContents {
	created := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	return hookContents{
		ID: hookExample, URL: origin + "/v1/hook/" + hookExample,
		CreatedAt: created.Format(time.RFC3339), ExpiresAt: created.Add(hookLifetime).Format(time.RFC3339),
		Requests: []hookRequest{
			{
				Seq: 2, ReceivedAt: created.Add(4 * time.Minute).Format(time.RFC3339Nano), Method: "POST", Path: "/payments",
				Query: "attempt=1",
				Headers: map[string][]string{
					"Host": {"api.dharun.dev"}, "Content-Type": {"application/json"}, "User-Agent": {"Razorpay-Webhook/v1"},
					"X-Razorpay-Signature": {"9d4f…"},
				},
				Body: `{"event":"payment.captured","payload":{"payment":{"entity":{"amount":49900,"currency":"INR"}}}}`, BodyEncoding: "text", Size: 92,
			},
			{
				Seq: 1, ReceivedAt: created.Add(time.Minute).Format(time.RFC3339Nano), Method: "GET", Path: "/",
				Headers: map[string][]string{"Host": {"api.dharun.dev"}, "User-Agent": {"curl/8.14.1"}}, BodyEncoding: "text",
			},
		},
	}
}
