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
)

// Minimal Prometheus text exporter plus all-time request totals that survive restarts.
// Labels are a small fixed set on purpose: what was looked up belongs in the request
// log, not in metric cardinality.

var durationBuckets = []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}

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

// save writes the totals through a temporary file, so a power cut mid-write leaves
// the previous totals intact.
func (m *metrics) save() {
	if m.statePath == "" {
		return
	}
	m.mu.Lock()
	if !m.totalsDirty {
		m.mu.Unlock()
		return
	}
	encoded, _ := json.MarshalIndent(m.totals, "", "  ")
	m.totalsDirty = false
	m.mu.Unlock()

	tmp := m.statePath + ".tmp"
	if err := os.WriteFile(tmp, append(encoded, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, `{"msg":"error","error":"saving totals: %s"}`+"\n", err)
		return
	}
	if err := os.Rename(tmp, m.statePath); err != nil {
		fmt.Fprintf(os.Stderr, `{"msg":"error","error":"saving totals: %s"}`+"\n", err)
	}
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
