package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// copySeed copies some of the test datasets into a directory of their own, so a test
// can add and remove files without disturbing the shared server.
func copySeed(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		raw, err := os.ReadFile(dataset.SeedPath(testSeed, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dataset.SeedPath(dir, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func serve(server *Server, method, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}

// writeRTO builds an RTO dataset holding a single made-up office, as a server's own copy.
func writeRTO(t *testing.T, liveDir string, at time.Time, office string, schema int) {
	t.Helper()
	source := t.TempDir()
	encoded, _ := json.Marshal([]dataset.RTO{{Code: "TN-28", Prefix: "TN", Office: office}})
	os.WriteFile(filepath.Join(source, "rto.json"), encoded, 0o644)
	spec, _ := dataset.Lookup("rto")
	altered := *spec
	altered.Schema = schema
	if _, _, err := dataset.Build(&altered, dataset.BuildOptions{
		Dir: source, Out: filepath.Join(liveDir, dataset.LiveName("rto", at)),
		Source: dataset.Source{Name: "Server copy", Licence: "CC BY-SA 4.0", Retrieved: at.UTC().Format(time.RFC3339)},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestServerPicksUpARefreshedDataset(t *testing.T) {
	seed, live := copySeed(t, "rto", "pincode"), t.TempDir()
	server := newTestServer(Config{Version: "t", SeedDir: seed, DataDir: live})
	defer server.Close()
	spec, _ := dataset.Lookup("rto")
	office := func() string {
		recorder := serve(server, http.MethodGet, "/v1/rto/TN-28")
		if recorder.Code != http.StatusOK {
			t.Fatalf("rto lookup: %d %s", recorder.Code, recorder.Body)
		}
		var body struct {
			Data struct{ Office string }
			Meta struct {
				Source struct{ Name, Retrieved string }
			}
		}
		json.Unmarshal(recorder.Body.Bytes(), &body)
		return body.Data.Office + " | " + body.Meta.Source.Name
	}
	seeded := office()
	if !strings.HasPrefix(seeded, "Namakkal") {
		t.Fatalf("seed answer: %q", seeded)
	}

	// A copy built for other tables must not be served.
	writeRTO(t, live, time.Now().Add(-time.Hour), "Wrong tables", spec.Schema+1)
	server.store.reload()
	if got := office(); got != seeded {
		t.Errorf("a copy with another schema was served: %q", got)
	}

	// The refresh job drops a new copy in; the next check swaps to it without a restart.
	writeRTO(t, live, time.Now(), "Fresh Office", spec.Schema)
	if swapped := server.store.reload(); len(swapped) != 1 || swapped[0] != "rto" {
		t.Fatalf("swapped %v, want only rto", swapped)
	}
	if got := office(); got != "Fresh Office | Server copy" {
		t.Errorf("after the swap: %q", got)
	}
	if swapped := server.store.reload(); len(swapped) != 0 {
		t.Errorf("an unchanged directory swapped %v", swapped)
	}

	health := serve(server, http.MethodGet, "/healthz").Body.String()
	if !strings.Contains(health, `"rto":{"copy":"server"`) || !strings.Contains(health, `"pincode":{"copy":"seed"`) {
		t.Errorf("health does not tell the copies apart: %s", health)
	}
	metrics := httptest.NewRecorder()
	server.MetricsHandler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, want := range []string{
		`api_dataset_available{dataset="rto",copy="server"} 1`,
		`api_dataset_records{dataset="rto",kind="rto_codes"} 1`,
		`api_dataset_available{dataset="ifsc",copy="none"} 0`,
	} {
		if !strings.Contains(metrics.Body.String(), want) {
			t.Errorf("metrics are missing %q", want)
		}
	}
}

func TestRefreshStatusReachesTheMetrics(t *testing.T) {
	seed, live := copySeed(t, "rto"), t.TempDir()
	os.WriteFile(filepath.Join(live, dataset.StatusFile), []byte(`{"updated":"2026-10-13T06:00:00Z","datasets":{
		"rto":{"lastAttempt":"2026-10-13T06:00:00Z","lastSuccess":"2026-09-13T06:00:00Z","result":"failed","error":"503"}}}`), 0o644)
	server := newTestServer(Config{Version: "t", SeedDir: seed, DataDir: live})
	defer server.Close()

	metrics := httptest.NewRecorder()
	server.MetricsHandler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, want := range []string{
		`api_dataset_refresh_ok{dataset="rto"} 0`,
		`api_dataset_refresh_success_timestamp_seconds{dataset="rto"} 1789279200`,
	} {
		if !strings.Contains(metrics.Body.String(), want) {
			t.Errorf("metrics are missing %q\n%s", want, metrics.Body.String())
		}
	}
}

func TestMissingDatasetAnswers503(t *testing.T) {
	server := newTestServer(Config{Version: "t", SeedDir: copySeed(t, "rto")})
	defer server.Close()
	for _, path := range []string{"/v1/pincode/637001", "/v1/ifsc/SBIN0000001", "/v1/banks?q=state", "/v1/holidays/2026"} {
		recorder := serve(server, http.MethodGet, path)
		if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "dataset_unavailable") || recorder.Header().Get("Retry-After") == "" {
			t.Errorf("%s without its dataset: %d %s", path, recorder.Code, recorder.Body)
		}
	}
	// What the server does hold, and what needs no data, still works.
	for _, path := range []string{"/v1/rto/TN-28", "/v1/words/100", "/", "/v1", "/healthz", "/openapi.json"} {
		if recorder := serve(server, http.MethodGet, path); recorder.Code != http.StatusOK {
			t.Errorf("%s: %d %s", path, recorder.Code, clip(recorder.Body.String(), 200))
		}
	}
	// A logo lookup with no logo dataset is "not found", and the placeholder still draws.
	if recorder := serve(server, http.MethodGet, "/v1/logos/TCS"); recorder.Code != http.StatusNotFound {
		t.Errorf("logo without the dataset: %d", recorder.Code)
	}
	if recorder := serve(server, http.MethodGet, "/v1/logos/TCS?fallback=monogram"); recorder.Code != http.StatusOK {
		t.Errorf("placeholder without the dataset: %d", recorder.Code)
	}
}
