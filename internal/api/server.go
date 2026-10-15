// Package api is the HTTP service: one process, a read-only file per dataset, every
// endpoint under /v1 with the same response envelope, errors and limits.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// Config is read from the environment by cmd/api.
type Config struct {
	Version string
	// SeedDir holds the dataset files that came with the release. DataDir, when set, holds
	// the copies this server fetched itself; the more recent of the two is served.
	SeedDir string
	DataDir string
	// ReloadEvery is how often the directories are checked for a new copy. Zero means
	// never: the datasets found at start are served until the process ends.
	ReloadEvery time.Duration
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
	// MaxBodyBytes caps the body of a POST request. Zero means 256 KB.
	MaxBodyBytes int
	// HeavyConcurrency is how many CPU-heavy requests run at once; the rest wait in
	// line. Zero means one per CPU.
	HeavyConcurrency int
	// Resvg is the renderer for generated images; empty looks next to the binary and on PATH.
	Resvg string
	// RenderConcurrency is how many generated images are rendered to PNG at once. Each is
	// a resvg process of a few tens of megabytes while it runs. Zero means four.
	RenderConcurrency int
	// ModelConcurrency is how many jobs that run a neural network (speech, denoise,
	// background removal) run at once. Each takes a few hundred megabytes. Zero means one.
	ModelConcurrency int
	// Programs overrides where an outside program is found, by name (typst, piper, ...).
	// A program not named here is looked for next to the server binary, then on PATH.
	Programs map[string]string
}

// Server holds everything a request needs. It is safe for concurrent use.
type Server struct {
	cfg       Config
	store     *store
	companies atomic.Pointer[companyIndex]
	metrics   *metrics
	limiter   *rateLimiter
	png       *pngCache
	mux       *http.ServeMux
	routes    map[string]bool
	private   map[string]bool // route patterns whose input may be personal
	documents sync.Map        // rendered docs, keyed by name and origin
	heavy     chan struct{}
	models    chan struct{} // one per model job running
	extras    extras
	logOut    io.Writer // where request log lines go; standard output when nil
}

// New opens the datasets and prepares a server.
func New(cfg Config) (*Server, error) {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 256 << 10
	}
	if cfg.RenderConcurrency <= 0 {
		cfg.RenderConcurrency = 4
	}
	if cfg.HeavyConcurrency <= 0 {
		cfg.HeavyConcurrency = runtime.NumCPU()
	}
	if cfg.ModelConcurrency <= 0 {
		cfg.ModelConcurrency = 1
	}
	s := &Server{
		cfg:     cfg,
		store:   newStore(cfg.SeedDir, cfg.DataDir),
		limiter: newRateLimiter(cfg.RateLimitPerMinute),
		png:     newPNGCache(cfg.PNGCacheBytes),
		mux:     http.NewServeMux(),
		routes:  map[string]bool{},
		private: map[string]bool{},
		heavy:   make(chan struct{}, cfg.HeavyConcurrency),
		models:  make(chan struct{}, cfg.ModelConcurrency),
	}
	s.extras.start(cfg)
	s.store.changed = s.datasetChanged
	for _, name := range s.store.reload() {
		s.store.notify(name)
	}
	if s.companies.Load() == nil {
		s.companies.Store(newCompanyIndex())
	}
	s.metrics = newMetrics(cfg.Version, cfg.StateDir, s.store.states)
	s.registerRoutes()
	if cfg.ReloadEvery > 0 {
		s.store.watch(cfg.ReloadEvery)
	}
	return s, nil
}

// datasetChanged rebuilds whatever is derived from a dataset that was just swapped.
func (s *Server) datasetChanged(name string) error {
	var err error
	if name == "ip" {
		err = s.openIP()
	}
	if name == "logos" {
		var index *companyIndex
		if index, err = loadCompanies(s.store.get("logos")); err == nil {
			s.companies.Store(index)
		} else {
			logError("loading companies", err)
		}
	}
	// Rendered pages quote record counts and source versions.
	s.documents.Clear()
	s.extras.datasetChanged(name)
	return err
}

// logError writes an error as one JSON line on standard error.
func logError(what string, err error) {
	encoded, _ := json.Marshal(map[string]string{"msg": "error", "error": what + ": " + err.Error()})
	fmt.Fprintln(os.Stderr, string(encoded))
}

// data returns the copy in use of a dataset, or nil.
func (s *Server) data(name string) *handle { return s.store.get(name) }

// need returns a dataset, or answers 503 and returns nil when this server has no copy.
func (s *Server) need(w http.ResponseWriter, r *http.Request, name string) *handle {
	if current := s.store.get(name); current != nil {
		return current
	}
	w.Header().Set("Retry-After", "600")
	s.fail(w, r, http.StatusServiceUnavailable, "dataset_unavailable",
		"This server does not hold the "+name+" dataset yet. Try again later.")
	return nil
}

// Close flushes the lifetime totals and stops background work.
func (s *Server) Close() {
	s.limiter.close()
	s.metrics.close()
	s.extras.close()
	s.store.close()
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
	private bool   // the request may carry a personal identifier: keep it out of the log
	// internal marks a request the server made to itself (an MCP tool call). The /mcp
	// request already paid for one lookup; client is who to charge for the rest.
	internal bool
	client   string
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

// Unwrap lets http.ResponseController reach the connection, to give uploads more time.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += n
	return n, err
}

// ServeHTTP is the single entry point: common headers, then the router, with one log
// line and one metrics observation per request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	details := &requestInfo{route: "other"}
	r = r.WithContext(context.WithValue(r.Context(), infoKey{}, details))
	writer := &statusWriter{ResponseWriter: w}

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf(`{"msg":"panic","route":%q,"error":%q}`, details.route, fmt.Sprint(recovered))
			if writer.status == 0 {
				s.fail(writer, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			}
		}
		elapsed := time.Since(started)
		s.metrics.observe(details.route, r.Method, writer.status, elapsed)
		if details.route == "health" {
			return
		}
		// Where the request came from stands in for its address, which goes no further.
		client := s.clientKey(r)
		place := s.locate(r, client)
		if details.route != "preflight" {
			s.metrics.visit(client, place)
		}
		if s.cfg.LogRequests {
			s.logRequest(r, details, place, writer.status, elapsed)
		}
	}()

	header := writer.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Expose-Headers", "ETag, X-Logo-Fallback")
	header.Set("Cross-Origin-Resource-Policy", "cross-origin")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	switch r.Method {
	case http.MethodOptions:
		details.route = "preflight"
		details.private = s.privatePath(r)
		header.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id, Mcp-Protocol-Version")
		header.Set("Access-Control-Max-Age", "86400")
		writer.WriteHeader(http.StatusNoContent)
	case http.MethodGet, http.MethodHead, http.MethodPost:
		s.mux.ServeHTTP(writer, r)
	default:
		details.private = s.privatePath(r)
		s.fail(writer, r, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET, or POST where the documentation says so.")
	}
}

// privatePath reports whether a request's path belongs to a private route under GET or
// POST, for requests that reach no handler (preflight, other methods, wrong method).
func (s *Server) privatePath(r *http.Request) bool {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		probe := r.Clone(r.Context())
		probe.Method = method
		if _, pattern := s.mux.Handler(probe); s.private[pattern] {
			return true
		}
	}
	return false
}

// handleUnmatched answers a request no route took: 405 when the path exists under the
// other method, otherwise 404.
func (s *Server) handleUnmatched(w http.ResponseWriter, r *http.Request) {
	other := http.MethodPost
	if r.Method == http.MethodPost {
		other = http.MethodGet
	}
	probe := r.Clone(r.Context())
	probe.Method = other
	if _, pattern := s.mux.Handler(probe); pattern != "/" && pattern != "" {
		info(r).private = s.private[pattern]
		w.Header().Set("Allow", other)
		s.fail(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "This endpoint takes "+other+".")
		return
	}
	s.notFound(w, r, "Not found. The API index is at /v1 and the documentation at /.")
}

func (s *Server) logRequest(r *http.Request, details *requestInfo, place clientPlace, status int, elapsed time.Duration) {
	path := r.URL.RequestURI()
	switch {
	case details.private:
		path = "(redacted)"
	case details.route == "other":
		// No route took it, so nothing says what the query holds: keep only the path.
		path = r.URL.EscapedPath()
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
	place.addTo(entry)
	for field, name := range map[string]string{"ua": "User-Agent", "referer": "Referer"} {
		if value := r.Header.Get(name); value != "" {
			entry[field] = clip(value, 200)
		}
	}
	encoded, _ := json.Marshal(entry)
	out := s.logOut
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintln(out, string(encoded))
}

// A route's cost is how many units of the per-minute allowance one request uses.
const (
	costFree  = 0  // documentation and health checks
	costLight = 1  // a lookup
	costHeavy = 10 // real CPU work: rendering, astronomy, live network queries
	// costProbe is a traceroute: probes sent from the server's own connection for several
	// seconds. It waits on the network rather than the processor, so it takes no CPU
	// slot; traceroutes keep their own, smaller limit.
	costProbe = 20
	// costUpload is a file sent in and worked on: OCR, PDF and image tools. The handler
	// reads the whole upload first and only then waits for a CPU slot, so a slow upload
	// does not hold one.
	costUpload = 15
	// costModel runs a neural network for seconds: speech, denoise, background removal.
	// These wait for one of a few model slots instead.
	costModel = 30
)

// heavyWait is how long a CPU-heavy request waits for a free slot before it is refused.
const heavyWait = 10 * time.Second

// handle registers a route under a short name used for metrics and logs. cost is what
// a request charges against the per-client rate limit. A route with costHeavy also
// takes one of a fixed number of slots, so a burst waits in line instead of running all
// at once: more load means more waiting, not more memory.
//
// A route the catalog marks Private is private from the first moment, so a request
// refused by the rate limit or the slots is kept out of the log and shared caches too.
func (s *Server) handle(route string, cost int, pattern string, handler http.HandlerFunc) {
	s.routes[pattern] = true
	_, path, _ := strings.Cut(pattern, " ")
	if path == "" {
		path = pattern
	}
	private := privatePaths[path]
	s.private[pattern] = private
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		details := info(r)
		details.route = route
		details.private = details.private || private
		charge, client := cost, ""
		if details.internal {
			charge, client = cost-costLight, details.client
		} else if cost > 0 {
			client = s.clientKey(r)
		}
		if charge > 0 && !s.limiter.allow(client, charge) {
			details.route = "rate_limited"
			s.metrics.rateLimited()
			w.Header().Set("Retry-After", "60")
			s.fail(w, r, http.StatusTooManyRequests, "rate_limited",
				fmt.Sprintf("Rate limit is %d requests per minute; this endpoint counts as %d.", s.cfg.RateLimitPerMinute, cost))
			return
		}
		if cost == costHeavy {
			release, ok := s.takeSlot(w, r, s.heavy, heavyWait)
			if !ok {
				return
			}
			defer release()
		}
		handler(w, r)
	})
}

// takeSlot waits up to wait for a place in pool. When none comes free it answers 503
// itself and reports false; otherwise it returns the function that gives the place back.
func (s *Server) takeSlot(w http.ResponseWriter, r *http.Request, pool chan struct{}, wait time.Duration) (func(), bool) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case pool <- struct{}{}:
		return func() { <-pool }, true
	case <-r.Context().Done():
		return nil, false
	case <-timer.C:
		s.metrics.busy()
		w.Header().Set("Retry-After", "5")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The server is busy. Try again in a few seconds.")
		return nil, false
	}
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

// rateLimiter counts cost per client in fixed one-minute windows. Coarse, but it
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

func (l *rateLimiter) allow(key string, cost int) bool {
	if l.perMinute <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counts[key] += cost
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

// sourceOf is the credit for a dataset's copy in use.
func (s *Server) sourceOf(name string) (dataset.Source, bool) {
	current := s.store.get(name)
	if current == nil {
		return dataset.Source{}, false
	}
	return current.meta.Source, true
}
