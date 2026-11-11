package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

func init() {
	mcpTestCalls["run_code"] = `{"language":"python","code":"print(1)"}`
}

// fakeController answers /run as the sandbox controller would, echoing the code.
func fakeController(t *testing.T) (*httptest.Server, *[]sandbox.RunRequest) {
	var seen []sandbox.RunRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" || r.URL.Path != "/run" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req sandbox.RunRequest
		json.NewDecoder(r.Body).Decode(&req)
		seen = append(seen, req)
		if strings.Contains(req.Code, "busy") {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(sandbox.Error{Code: sandbox.CodeBusy, Message: "busy"})
			return
		}
		json.NewEncoder(w).Encode(sandbox.Result{Stdout: "ran " + req.Language + "\n", ExitCode: 0, DurationMillis: 40, CPUMillis: 30000})
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

func TestRunCode(t *testing.T) {
	db, good, _, _ := fakeAccountsDB(t)
	controller, seen := fakeController(t)
	state := t.TempDir()
	server := newTestServer(Config{Version: "t", RateLimitPerMinute: 100, KeyRateLimitPerMinute: 1000, AccountsDB: db,
		SandboxURL: controller.URL, SandboxSecret: "test-secret", StateDir: state})
	defer server.Close()
	body := `{"language":"py","code":"print(1)"}`

	if got := send(server, "POST", "/v1/run", body, "Content-Type", "application/json"); got.Code != http.StatusUnauthorized || errorCode(got) != "key_required" {
		t.Errorf("keyless run = %d %s", got.Code, got.Body)
	}
	auth := []string{"Content-Type", "application/json", "Authorization", "Bearer " + good}
	got := send(server, "POST", "/v1/run", body, auth...)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"stdout":"ran python\n"`) {
		t.Fatalf("run = %d %s", got.Code, got.Body)
	}
	if len(*seen) != 1 || (*seen)[0].Language != "python" || (*seen)[0].Account != "key1goodxxxxxxx" {
		t.Errorf("controller saw %+v", *seen)
	}
	if got := send(server, "POST", "/v1/run", `{"language":"ruby","code":"puts 1"}`, auth...); got.Code != http.StatusBadRequest {
		t.Errorf("ruby = %d", got.Code)
	}
	if got := send(server, "POST", "/v1/run", `{"language":"python","code":"busy"}`, auth...); got.Code != http.StatusServiceUnavailable || errorCode(got) != "busy" {
		t.Errorf("busy = %d %s", got.Code, got.Body)
	}
	// The audit log holds a hash of the code, never the code.
	files, _ := filepath.Glob(filepath.Join(state, "sandbox-audit", "*.jsonl"))
	if len(files) != 1 {
		t.Fatalf("audit files %v", files)
	}
	audit, _ := os.ReadFile(files[0])
	if strings.Contains(string(audit), "print(1)") || !strings.Contains(string(audit), `"code_sha256":"`) || strings.Count(string(audit), "\n") != 2 {
		t.Errorf("audit: %s", audit)
	}
	// Each fake run takes 30 CPU seconds: the 10 minutes are used up after twenty.
	for i := 0; i < 25; i++ {
		got = send(server, "POST", "/v1/run", body, auth...)
		if got.Code == http.StatusTooManyRequests {
			break
		}
	}
	if got.Code != http.StatusTooManyRequests || errorCode(got) != "quota_exceeded" {
		t.Errorf("quota not reached: %d %s", got.Code, got.Body)
	}
}

func TestRunCodeOff(t *testing.T) {
	expectStatus(t, "/v1/run/languages", http.StatusOK)
	got := postBody(t, "/v1/run", "application/json", []byte(`{"language":"python","code":"print(1)"}`))
	if got.status != http.StatusServiceUnavailable {
		t.Errorf("run without a controller = %d", got.status)
	}
}
