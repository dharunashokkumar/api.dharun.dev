package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/traffic"
)

// Minimal Prometheus text exporter plus all-time request totals that survive restarts.
// Labels are a small fixed set on purpose: what was looked up belongs in the request
// log, not in metric cardinality.

// The buckets run past a second: a heavy request may wait 10 s for a slot, a DNS or
// email check waits on resolvers, and the write timeout is 45 s.
var durationBuckets = []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

type histogram struct {
	buckets []uint64
	sum     float64
	count   uint64
}

type metrics struct {
	version   string
	datasets  func() []datasetState
	startedAt time.Time

	mu          sync.Mutex
	requests    map[string]uint64 // route|method|status
	durations   map[string]*histogram
	logos       map[string]uint64 // format|result
	rateLimits  uint64
	busyCount   uint64
	countries   map[string]uint64 // requests by country code
	states      map[string]uint64 // requests from India by state
	origins     map[string]uint64 // kind|source|medium, from the fixed lists only
	visitors    *visitors
	totals      lifetimeTotals
	totalsDirty bool

	statePath string
	stop      chan struct{}
	done      chan struct{}
}

// lifetimeTotals is what gets written to disk: requests served per route since the
// service first ran.
type lifetimeTotals struct {
	Since    string            `json:"since"`
	Total    uint64            `json:"total"`
	Requests map[string]uint64 `json:"requests"`
}

func newMetrics(version, stateDir string, datasets func() []datasetState) *metrics {
	m := &metrics{
		version:   version,
		datasets:  datasets,
		startedAt: time.Now(),
		requests:  map[string]uint64{},
		durations: map[string]*histogram{},
		logos:     map[string]uint64{},
		countries: map[string]uint64{},
		states:    map[string]uint64{},
		origins:   map[string]uint64{},
		visitors:  newVisitors(stateDir),
		totals:    lifetimeTotals{Since: time.Now().UTC().Format("2006-01-02"), Requests: map[string]uint64{}},
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	if stateDir != "" {
		m.statePath = filepath.Join(stateDir, "totals.json")
		if raw, err := os.ReadFile(m.statePath); err == nil {
			var saved lifetimeTotals
			if json.Unmarshal(raw, &saved) == nil && saved.Requests != nil {
				m.totals = saved
			}
		}
	}
	go m.saveLoop()
	return m
}

func (m *metrics) saveLoop() {
	defer close(m.done)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.save()
		case <-m.stop:
			m.save()
			return
		}
	}
}

// save writes the totals and the visitor sketches.
func (m *metrics) save() {
	if m.statePath == "" {
		return
	}
	m.mu.Lock()
	var totals []byte
	if m.totalsDirty {
		totals, _ = json.MarshalIndent(m.totals, "", "  ")
		m.totalsDirty = false
	}
	visitors := m.visitors.encode()
	m.mu.Unlock()

	if totals != nil {
		if err := writeState(m.statePath, append(totals, '\n'), 0o644); err != nil {
			logError("saving totals", err)
			// Not saved: the next save must write them even if nothing else is counted.
			m.mu.Lock()
			m.totalsDirty = true
			m.mu.Unlock()
		}
	}
	if visitors != nil {
		if err := writeState(m.visitors.path, visitors, 0o600); err != nil {
			logError("saving visitor counts", err)
			m.mu.Lock()
			m.visitors.dirty = true
			m.mu.Unlock()
		}
	}
}

// writeState writes a file through a temporary one, so a power cut mid-write leaves the
// previous version intact.
func writeState(path string, content []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	err := os.WriteFile(tmp, content, mode)
	if err == nil {
		err = os.Chmod(tmp, mode)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	return err
}

func (m *metrics) close() {
	close(m.stop)
	<-m.done
}

func (m *metrics) observe(route, method string, status int, elapsed time.Duration) {
	if status == 0 {
		status = 200
	}
	switch method {
	case "GET", "HEAD", "POST", "OPTIONS":
	default:
		method = "OTHER"
	}
	seconds := elapsed.Seconds()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[fmt.Sprintf("%s|%s|%d", route, method, status)]++
	h := m.durations[route]
	if h == nil {
		h = &histogram{buckets: make([]uint64, len(durationBuckets))}
		m.durations[route] = h
	}
	for i, bound := range durationBuckets {
		if seconds <= bound {
			h.buckets[i]++
		}
	}
	h.sum += seconds
	h.count++

	if route != "health" && route != "preflight" {
		m.totals.Requests[route]++
		m.totals.Total++
		m.totalsDirty = true
	}
}

// visit counts a request by where it came from, and its client among the unique
// visitors. Health checks and preflights are not visits.
func (m *metrics) visit(client string, place clientPlace) {
	country, state := place.country, place.state
	if country == "" {
		country = "unknown"
	}
	if country == "IN" && state == "" {
		state = "unknown"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.countries[country]++
	if state != "" {
		m.states[state]++
	}
	m.visitors.add(client, place.country)
}

// origin counts a request by who sent it and which of our links it followed. Only the
// names from the fixed lists are labels, so callers cannot add series.
func (m *metrics) origin(from traffic.Origin) {
	m.mu.Lock()
	m.origins[from.Kind+"|"+from.Known+"|"+from.KnownMedium]++
	m.mu.Unlock()
}

// observeLogo records a logo lookup. result is found, fallback or not_found.
func (m *metrics) observeLogo(format, result string) {
	m.mu.Lock()
	m.logos[format+"|"+result]++
	m.mu.Unlock()
}

func (m *metrics) rateLimited() {
	m.mu.Lock()
	m.rateLimits++
	m.mu.Unlock()
}

func (m *metrics) busy() {
	m.mu.Lock()
	m.busyCount++
	m.mu.Unlock()
}

func (m *metrics) lifetime() lifetimeTotals {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := lifetimeTotals{Since: m.totals.Since, Total: m.totals.Total, Requests: map[string]uint64{}}
	for route, count := range m.totals.Requests {
		copied.Requests[route] = count
	}
	return copied
}

func (m *metrics) render() string {
	// Read before taking the lock: the store has its own.
	states := m.datasets()

	m.mu.Lock()
	defer m.mu.Unlock()

	var out strings.Builder
	family := func(name, kind, help string) {
		fmt.Fprintf(&out, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
	}

	family("api_build_info", "gauge", "Running version of the API.")
	fmt.Fprintf(&out, "api_build_info{version=%q} 1\n", m.version)

	family("api_dataset_records", "gauge", "Records in the copy in use, by dataset and kind.")
	for _, state := range states {
		for _, kind := range sortedCountKeys(state.Records) {
			if !strings.Contains(kind, ":") {
				fmt.Fprintf(&out, "api_dataset_records{dataset=%q,kind=%q} %d\n", state.Name, kind, state.Records[kind])
			}
		}
	}
	family("api_dataset_available", "gauge", "1 when the server holds a copy of the dataset. copy is seed (came with the release) or server (fetched here).")
	for _, state := range states {
		available := 1
		if state.Copy == "none" {
			available = 0
		}
		fmt.Fprintf(&out, "api_dataset_available{dataset=%q,copy=%q} %d\n", state.Name, state.Copy, available)
	}
	family("api_dataset_fetched_timestamp_seconds", "gauge", "When the source of the copy in use was fetched.")
	for _, state := range states {
		if at := unixOf(state.Fetched); at > 0 {
			fmt.Fprintf(&out, "api_dataset_fetched_timestamp_seconds{dataset=%q} %d\n", state.Name, at)
		}
	}
	family("api_dataset_refresh_ok", "gauge", "1 when the last refresh attempt of a dataset this server refreshes worked (updated, or found the source unchanged), 0 when it failed or was held back.")
	for _, state := range states {
		if state.scheduled && state.Refresh != "" {
			ok := 0
			if state.Refresh == "updated" || state.Refresh == "unchanged" {
				ok = 1
			}
			fmt.Fprintf(&out, "api_dataset_refresh_ok{dataset=%q} %d\n", state.Name, ok)
		}
	}
	family("api_dataset_refresh_interval_seconds", "gauge", "How often this server refreshes the dataset. Absent for datasets that only change with a release.")
	for _, state := range states {
		if state.scheduled {
			fmt.Fprintf(&out, "api_dataset_refresh_interval_seconds{dataset=%q} %d\n", state.Name, int(state.every.Seconds()))
		}
	}
	family("api_dataset_refresh_success_timestamp_seconds", "gauge", "When a refresh of the dataset last worked on this server.")
	for _, state := range states {
		if at := unixOf(state.LastSuccess); at > 0 {
			fmt.Fprintf(&out, "api_dataset_refresh_success_timestamp_seconds{dataset=%q} %d\n", state.Name, at)
		}
	}

	family("api_http_requests_total", "counter", "HTTP requests since start, by route, method and status.")
	for _, key := range sortedKeys(m.requests) {
		parts := strings.Split(key, "|")
		fmt.Fprintf(&out, "api_http_requests_total{route=%q,method=%q,status=%q} %d\n", parts[0], parts[1], parts[2], m.requests[key])
	}

	family("api_http_request_duration_seconds", "histogram", "Time to produce a response, by route.")
	routes := make([]string, 0, len(m.durations))
	for route := range m.durations {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	for _, route := range routes {
		h := m.durations[route]
		for i, bound := range durationBuckets {
			fmt.Fprintf(&out, "api_http_request_duration_seconds_bucket{route=%q,le=\"%g\"} %d\n", route, bound, h.buckets[i])
		}
		fmt.Fprintf(&out, "api_http_request_duration_seconds_bucket{route=%q,le=\"+Inf\"} %d\n", route, h.count)
		fmt.Fprintf(&out, "api_http_request_duration_seconds_sum{route=%q} %g\n", route, h.sum)
		fmt.Fprintf(&out, "api_http_request_duration_seconds_count{route=%q} %d\n", route, h.count)
	}

	family("api_logo_requests_total", "counter", "Logo lookups by output format and outcome.")
	for _, key := range sortedKeys(m.logos) {
		parts := strings.Split(key, "|")
		fmt.Fprintf(&out, "api_logo_requests_total{format=%q,result=%q} %d\n", parts[0], parts[1], m.logos[key])
	}

	family("api_rate_limited_total", "counter", "Requests rejected by the rate limiter.")
	fmt.Fprintf(&out, "api_rate_limited_total %d\n", m.rateLimits)

	family("api_busy_total", "counter", "CPU-heavy requests refused because every slot stayed taken.")
	fmt.Fprintf(&out, "api_busy_total %d\n", m.busyCount)

	family("api_requests_by_country_total", "counter", "Requests since start by the country they came from (two-letter code, or unknown). Health checks and preflights are left out.")
	for _, country := range sortedKeys(m.countries) {
		fmt.Fprintf(&out, "api_requests_by_country_total{country=%q} %d\n", country, m.countries[country])
	}
	family("api_requests_by_state_total", "counter", "Requests since start from India, by state or union territory (or unknown).")
	for _, state := range sortedKeys(m.states) {
		fmt.Fprintf(&out, "api_requests_by_state_total{state=%q} %d\n", state, m.states[state])
	}

	family("api_requests_by_origin_total", "counter", "Requests since start by kind of client (person, ai-assistant, program, crawlers, bot, own, unknown), source (who sent it or led to it, from utm_source, the MCP client name, the referring site or the user agent; other when not a known name) and medium (which of our links it followed, from utm_medium). Health checks and preflights are left out.")
	for _, key := range sortedKeys(m.origins) {
		parts := strings.Split(key, "|")
		fmt.Fprintf(&out, "api_requests_by_origin_total{kind=%q,source=%q,medium=%q} %d\n", parts[0], parts[1], parts[2], m.origins[key])
	}

	m.visitors.roll()
	family("api_unique_visitors", "gauge", "Distinct client addresses so far this day or month, India time (estimated, within about 1%).")
	fmt.Fprintf(&out, "api_unique_visitors{period=\"day\"} %d\n", m.visitors.Day.All.estimate())
	fmt.Fprintf(&out, "api_unique_visitors{period=\"month\"} %d\n", m.visitors.Month.All.estimate())
	family("api_unique_visitors_previous", "gauge", "Distinct client addresses over the whole previous day or month, India time, when the server counted it to the end.")
	for _, period := range []struct {
		name  string
		value *visitorPeriod
	}{{"day", m.visitors.Day}, {"month", m.visitors.Month}} {
		if period.value.PreviousKey != "" {
			fmt.Fprintf(&out, "api_unique_visitors_previous{period=%q} %d\n", period.name, period.value.Previous)
		}
	}
	family("api_unique_visitors_by_country", "gauge", "Distinct client addresses so far today (India time), by country (estimated, within about 3%).")
	for _, country := range sortedSketchKeys(m.visitors.Day.Countries) {
		fmt.Fprintf(&out, "api_unique_visitors_by_country{country=%q} %d\n", country, m.visitors.Day.Countries[country].estimate())
	}

	family("api_lifetime_requests_total", "counter", "Requests served since the service first ran, kept across restarts, by route.")
	for _, route := range sortedKeys(m.totals.Requests) {
		fmt.Fprintf(&out, "api_lifetime_requests_total{route=%q} %d\n", route, m.totals.Requests[route])
	}

	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	family("process_resident_memory_bytes", "gauge", "Resident memory size in bytes.")
	fmt.Fprintf(&out, "process_resident_memory_bytes %d\n", residentMemory(memory.Sys))
	family("go_memstats_heap_alloc_bytes", "gauge", "Heap bytes allocated and in use.")
	fmt.Fprintf(&out, "go_memstats_heap_alloc_bytes %d\n", memory.HeapAlloc)
	family("go_goroutines", "gauge", "Number of goroutines.")
	fmt.Fprintf(&out, "go_goroutines %d\n", runtime.NumGoroutine())
	family("process_start_time_seconds", "gauge", "Start time of the process since unix epoch.")
	fmt.Fprintf(&out, "process_start_time_seconds %d\n", m.startedAt.Unix())

	return out.String()
}

// residentMemory reads the real resident set size on Linux, and falls back to the
// memory the Go runtime obtained from the system elsewhere.
func residentMemory(fallback uint64) uint64 {
	raw, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return fallback
	}
	var size, resident uint64
	if _, err := fmt.Sscan(string(raw), &size, &resident); err != nil {
		return fallback
	}
	return resident * uint64(os.Getpagesize())
}

func sortedKeys(counts map[string]uint64) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSketchKeys(sketches map[string]sketch) []string {
	keys := make([]string, 0, len(sketches))
	for key := range sketches {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedCountKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// unixOf turns a date or timestamp as stored with a dataset into Unix seconds, 0 if empty.
func unixOf(value string) int64 {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Unix()
		}
	}
	return 0
}
