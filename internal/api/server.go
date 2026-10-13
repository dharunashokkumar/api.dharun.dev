// Package api is the HTTP service: one process, one read-only database, every
// endpoint under /v1 with the same response envelope, errors and limits.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// Config is read from the environment by cmd/api.
type Config struct {
	Version string
	// PublicOrigin is the origin used in links, the sitemap and the OpenAPI document.
	// Empty means "derive it from the request", which suits local development.
	PublicOrigin string
	// TrustProxy takes the client address from CF-Connecting-IP / X-Forwarded-For.
	// Only correct when the port is reachable solely through that proxy.
	TrustProxy         bool
	RateLimitPerMinute int
	LogRequests        bool
	PNGCacheBytes      int
	// StateDir holds the lifetime request totals. Empty keeps them in memory only.
	StateDir string
}

// Server holds everything a request needs. It is safe for concurrent use.
type Server struct {
	cfg       Config
	db        *sql.DB
	sources   map[string]dataset.Source
	companies *companyIndex
	counts    datasetCounts
	years     []int
	metrics   *metrics
	limiter   *rateLimiter
	png       *pngCache
	mux       *http.ServeMux
	routes    map[string]bool
	documents sync.Map // rendered docs, keyed by name and origin
	stmts     statements
}

type datasetCounts struct {
	Logos, Pincodes, PostOffices, Banks, Branches, RTOs, Holidays int
}

// New prepares a server over an opened database.
func New(cfg Config, db *sql.DB) (*Server, error) {
	sources, err := dataset.Sources(db)
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:     cfg,
		db:      db,
		sources: sources,
		limiter: newRateLimiter(cfg.RateLimitPerMinute),
		png:     newPNGCache(cfg.PNGCacheBytes),
		mux:     http.NewServeMux(),
		routes:  map[string]bool{},
	}
	if s.companies, err = loadCompanies(db); err != nil {
		return nil, fmt.Errorf("loading companies: %w", err)
	}
	if err := s.loadCounts(); err != nil {
		return nil, err
	}
	if err := s.prepare(); err != nil {
		return nil, err
	}
	s.metrics = newMetrics(cfg.Version, cfg.StateDir, s.counts)
	s.registerRoutes()
	return s, nil
}

func (s *Server) loadCounts() error {
	queries := []struct {
		into  *int
		query string
	}{
		{&s.counts.Logos, `SELECT COUNT(*) FROM logos`},
		{&s.counts.Pincodes, `SELECT COUNT(DISTINCT pincode) FROM pincodes`},
		{&s.counts.PostOffices, `SELECT COUNT(*) FROM pincodes`},
		{&s.counts.Banks, `SELECT COUNT(*) FROM banks`},
		{&s.counts.Branches, `SELECT COUNT(*) FROM ifsc`},
		{&s.counts.RTOs, `SELECT COUNT(*) FROM rto`},
		{&s.counts.Holidays, `SELECT COUNT(*) FROM holidays`},
	}
	for _, q := range queries {
		if err := s.db.QueryRow(q.query).Scan(q.into); err != nil {
			return err
		}
	}
	rows, err := s.db.Query(`SELECT DISTINCT year FROM holidays ORDER BY year`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return err
		}
		s.years = append(s.years, year)
	}
	return rows.Err()
}

// Close flushes the lifetime totals and stops background work.
func (s *Server) Close() {
	s.limiter.close()
	s.metrics.close()
}

// MetricsHandler serves /metrics and /stats. It belongs on a private listener.
func (s *Server) MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
			fmt.Fprint(w, s.metrics.render())
		case "/stats":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(s.metrics.lifetime())
		default:
			http.NotFound(w, r)
		}
	})
}

// requestInfo is filled in by handlers so the access log and metrics can describe
// the request without parsing it again.
type requestInfo struct {
	route   string
	subject string // what was looked up, for the log (never for metric labels)
	private bool   // the path may carry a personal identifier: keep it out of the log
	// internal marks a request the server made to itself (an MCP tool call), which has
	// already been rate limited.
	internal bool
}

type infoKey struct{}

func info(r *http.Request) *requestInfo {
	if value, ok := r.Context().Value(infoKey{}).(*requestInfo); ok {
		return value
	}
	return &requestInfo{}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += n
	return n, err
}

// ServeHTTP is the single entry point: common headers, method rules, then the router,
// with one log line and one metrics observation per request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	details := &requestInfo{route: "other"}
	r = r.WithContext(context.WithValue(r.Context(), infoKey{}, details))
	writer := &statusWriter{ResponseWriter: w}

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf(`{"msg":"panic","path":%q,"error":%q}`, r.URL.Path, fmt.Sprint(recovered))
			if writer.status == 0 {
				s.fail(writer, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			}
		}
		elapsed := time.Since(started)
		s.metrics.observe(details.route, r.Method, writer.status, elapsed)
		if s.cfg.LogRequests && details.route != "health" {
			s.logRequest(r, details, writer.status, elapsed)
		}
	}()

	header := writer.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Expose-Headers", "ETag, X-Logo-Fallback")
	header.Set("Cross-Origin-Resource-Policy", "cross-origin")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	switch {
	case r.Method == http.MethodOptions:
		details.route = "preflight"
		header.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id, Mcp-Protocol-Version")
		header.Set("Access-Control-Max-Age", "86400")
		writer.WriteHeader(http.StatusNoContent)
	case r.Method != http.MethodGet && r.Method != http.MethodHead && r.URL.Path != "/mcp":
		s.fail(writer, r, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET.")
	default:
		s.mux.ServeHTTP(writer, r)
	}
}

func (s *Server) logRequest(r *http.Request, details *requestInfo, status int, elapsed time.Duration) {
	path := r.URL.RequestURI()
	if details.private {
		path = "(redacted)"
	}
	entry := map[string]any{
		"msg":    "request",
		"method": r.Method,
		"route":  details.route,
		"path":   clip(path, 300),
		"status": status,
		"ms":     float64(elapsed.Microseconds()) / 1000,
	}
	if details.subject != "" && !details.private {
		entry["subject"] = clip(details.subject, 80)
	}
	for field, name := range map[string]string{"country": "CF-IPCountry", "ua": "User-Agent", "referer": "Referer"} {
		if value := r.Header.Get(name); value != "" {
			entry[field] = clip(value, 200)
		}
	}
	encoded, _ := json.Marshal(entry)
	fmt.Fprintln(os.Stdout, string(encoded))
}

// handle registers a route under a short name used for metrics and logs. Limited
// routes count against the per-client rate limit.
func (s *Server) handle(route string, limited bool, pattern string, handler http.HandlerFunc) {
	s.routes[pattern] = true
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		details := info(r)
		details.route = route
		if limited && !details.internal && !s.limiter.allow(s.clientKey(r)) {
			details.route = "rate_limited"
			s.metrics.rateLimited()
			w.Header().Set("Retry-After", "60")
			s.fail(w, r, http.StatusTooManyRequests, "rate_limited",
				fmt.Sprintf("Rate limit is %d requests per minute.", s.cfg.RateLimitPerMinute))
			return
		}
		handler(w, r)
	})
}

func (s *Server) clientKey(r *http.Request) string {
	if s.cfg.TrustProxy {
		if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
			return ip
		}
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			return strings.TrimSpace(strings.Split(forwarded, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// origin is the public base URL for links in a response.
func (s *Server) origin(r *http.Request) string {
	if s.cfg.PublicOrigin != "" {
		return s.cfg.PublicOrigin
	}
	return "http://" + r.Host
}

// rateLimiter counts requests per client in fixed one-minute windows. Coarse, but it
// costs one map entry per active client and nothing else.
type rateLimiter struct {
	perMinute int
	mu        sync.Mutex
	counts    map[string]int
	stop      chan struct{}
}

func newRateLimiter(perMinute int) *rateLimiter {
	limiter := &rateLimiter{perMinute: perMinute, counts: map[string]int{}, stop: make(chan struct{})}
	if perMinute > 0 {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					limiter.mu.Lock()
					limiter.counts = map[string]int{}
					limiter.mu.Unlock()
				case <-limiter.stop:
					return
				}
			}
		}()
	}
	return limiter
}

func (l *rateLimiter) allow(key string) bool {
	if l.perMinute <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counts[key]++
	return l.counts[key] <= l.perMinute
}

func (l *rateLimiter) close() { close(l.stop) }

func clip(value string, max int) string {
	if len(value) > max {
		return value[:max]
	}
	return value
}

// intParam reads an integer query parameter within bounds.
func intParam(r *http.Request, name string, fallback, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, fmt.Errorf("%s must be an integer from %d to %d.", name, min, max)
	}
	return value, nil
}
