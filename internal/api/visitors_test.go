package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSketchEstimates(t *testing.T) {
	for _, distinct := range []int{0, 1, 100, 5_000, 200_000} {
		s := newSketch(visitorPrecision)
		for i := 0; i < distinct; i++ {
			sum := sha256.Sum256([]byte(fmt.Sprint(i)))
			hash := binary.BigEndian.Uint64(sum[:8])
			s.add(hash)
			if s.add(hash) {
				t.Fatal("adding the same hash twice changed the sketch")
			}
		}
		got := float64(s.estimate())
		if diff := got - float64(distinct); diff > 0.03*float64(distinct)+0.5 || -diff > 0.03*float64(distinct)+0.5 {
			t.Errorf("%d distinct hashes estimated as %v", distinct, got)
		}
	}
}

// metricsText serves /metrics of a server.
func metricsText(server *Server) string {
	recorder := httptest.NewRecorder()
	server.MetricsHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return recorder.Body.String()
}

// Requests are counted and logged by where they came from, unique visitors survive a
// restart and start again with a new day, and the address itself is never written.
func TestVisitorsAndPlaces(t *testing.T) {
	stateDir := t.TempDir()
	clock := time.Date(2026, 10, 15, 12, 0, 0, 0, indiaTime)
	visitorClock = func() time.Time { return clock }
	defer func() { visitorClock = time.Now }()
	start := func() (*Server, *bytes.Buffer) {
		server := newTestServer(Config{Version: "t", DataDir: testLive, StateDir: stateDir, TrustProxy: true, LogRequests: true})
		var logged bytes.Buffer
		server.logOut = &logged
		return server, &logged
	}
	visit := func(server *Server, client string, headers ...string) {
		req := httptest.NewRequest(http.MethodGet, "/v1/states", nil)
		req.Header.Set("CF-Connecting-IP", client)
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		server.ServeHTTP(httptest.NewRecorder(), req)
	}
	expect := func(text string, want ...string) {
		t.Helper()
		for _, line := range want {
			if !strings.Contains(text, line+"\n") {
				t.Errorf("metrics are missing %q", line)
			}
		}
	}

	server, logged := start()
	addresses := []string{"49.204.0.1", "49.204.0.1", "49.204.7.9", "8.8.8.8", "2401:4900::1", "198.51.100.7"}
	for _, address := range addresses {
		visit(server, address)
	}
	// A reserved address falls back to Cloudflare's country; Tor and junk are unknown.
	visit(server, "198.51.100.8", "CF-IPCountry", "SG")
	visit(server, "198.51.100.9", "CF-IPCountry", "T1")
	visit(server, "198.51.100.10", "CF-IPCountry", `x"}`)
	// Health checks and preflights are not visits.
	server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodOptions, "/v1/states", nil))

	expect(metricsText(server),
		`api_requests_by_country_total{country="IN"} 4`,
		`api_requests_by_country_total{country="US"} 1`,
		`api_requests_by_country_total{country="SG"} 1`,
		`api_requests_by_country_total{country="unknown"} 3`,
		`api_requests_by_state_total{state="Telangana"} 3`,
		`api_requests_by_state_total{state="Bihar"} 1`,
		`api_unique_visitors{period="day"} 8`,
		`api_unique_visitors{period="month"} 8`,
		`api_unique_visitors_by_country{country="IN"} 3`,
		`api_unique_visitors_by_country{country="US"} 1`,
	)
	log := logged.String()
	for _, want := range []string{`"city":"Hyderabad (NBT Nagar)"`, `"region":"Telangana"`, `"country":"IN"`,
		`"network":"Atria Convergence Technologies Pvt. Ltd."`, `"asn":55577`, `"lat":17.41`, `"lon":78.43`, `"country":"SG"`} {
		if !strings.Contains(log, want) {
			t.Errorf("the log is missing %s:\n%s", want, log)
		}
	}
	for _, address := range append(addresses, "198.51.100.8", "T1") {
		if strings.Contains(log, address) {
			t.Errorf("%s reached the log", address)
		}
	}

	// A restart carries on counting the same day; nothing saved holds an address.
	server.Close()
	saved, err := os.ReadFile(filepath.Join(stateDir, "visitors.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range addresses {
		if bytes.Contains(saved, []byte(address)) {
			t.Errorf("%s was saved", address)
		}
	}
	server, _ = start()
	visit(server, "8.8.8.8")
	visit(server, "49.204.9.9")
	expect(metricsText(server), `api_unique_visitors{period="day"} 9`, `api_unique_visitors{period="month"} 9`)

	// The next day starts from nothing and reports yesterday's total; the month goes on.
	server.metrics.mu.Lock()
	clock = clock.Add(24 * time.Hour)
	server.metrics.mu.Unlock()
	visit(server, "8.8.8.8")
	visit(server, "49.204.10.10")
	text := metricsText(server)
	expect(text,
		`api_unique_visitors{period="day"} 2`,
		`api_unique_visitors_previous{period="day"} 9`,
		`api_unique_visitors{period="month"} 10`,
		`api_unique_visitors_by_country{country="US"} 1`,
	)
	if strings.Contains(text, `api_unique_visitors_previous{period="month"}`) {
		t.Error("a previous month is reported that the server never counted")
	}
	server.Close()
}
