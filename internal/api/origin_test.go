package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Each request is labelled with who sent it and which of our links it followed, in the
// log and in the metrics; a caller's own labels never reach the log where the query
// does not, and never become a metric label unless they are a known name.
func TestRequestOrigin(t *testing.T) {
	server := newTestServer(Config{Version: "t", LogRequests: true})
	defer server.Close()
	var logged bytes.Buffer
	server.logOut = &logged
	send := func(method, path, userAgent, body string) map[string]any {
		t.Helper()
		logged.Reset()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "203.0.113.5:1000"
		req.Header.Set("User-Agent", userAgent)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		server.ServeHTTP(httptest.NewRecorder(), req)
		var entry map[string]any
		if err := json.Unmarshal(logged.Bytes(), &entry); err != nil {
			t.Fatalf("%s: log line %q: %v", path, logged.String(), err)
		}
		return entry
	}
	expect := func(entry map[string]any, kind, source, medium string) {
		t.Helper()
		if entry["kind"] != kind || (entry["src"] != source && !(source == "" && entry["src"] == nil)) ||
			(entry["medium"] != medium && !(medium == "" && entry["medium"] == nil)) {
			t.Errorf("logged kind %v src %v medium %v, want %q %q %q", entry["kind"], entry["src"], entry["medium"], kind, source, medium)
		}
	}

	expect(send("GET", "/v1/pincode/637001?utm_source=claude&utm_medium=llms", "curl/8.7.1", ""), "ai-assistant", "claude", "llms")
	expect(send("GET", "/v1/pincode/637001?utm_medium=docs", "Mozilla/5.0 (Windows NT 10.0; rv:158.0) Gecko/20100101 Firefox/158.0", ""), "person", "browser", "docs")
	expect(send("GET", "/v1/pincode/637001?utm_source=My%20Newsletter&utm_medium=email", "python-requests/2.32", ""), "program", "my-newsletter", "email")
	expect(send("GET", "/docs/pincode", "Mozilla/5.0 (compatible; GPTBot/1.2; +https://openai.com/gptbot)", ""), "ai-crawler", "gptbot", "")
	expect(send("GET", "/v1/pincode/637001", "Uptime-Kuma/2.5.5", ""), "own", "own", "")

	// Private routes and unmatched paths keep only the known names.
	private := send("GET", "/v1/phone/9876501234?utm_source=SECRETNAME&utm_medium=SECRETMEDIUM", "curl/8.7.1", "")
	expect(private, "program", "other", "other")
	unmatched := send("GET", "/v1/no-such-thing?utm_source=SECRETNAME", "curl/8.7.1", "")
	expect(unmatched, "program", "other", "")
	expect(send("GET", "/v1/phone/9876501234?utm_source=chatgpt.com", "curl/8.7.1", ""), "ai-assistant", "chatgpt", "")

	// An MCP client is named by what it says on initialize.
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"claude-ai","version":"0.1.0"}}}`
	expect(send("POST", "/mcp", "python-httpx/0.28.1", initialize), "ai-assistant", "claude-ai", "mcp")
	expect(send("POST", "/mcp", "node", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`), "program", "node", "mcp")

	metrics := server.metrics.render()
	for _, want := range []string{
		`api_requests_by_origin_total{kind="ai-assistant",source="claude",medium="llms"} 1`,
		`api_requests_by_origin_total{kind="program",source="other",medium="other"} 2`,
		`api_requests_by_origin_total{kind="ai-assistant",source="claude",medium="mcp"} 1`,
		`api_requests_by_origin_total{kind="own",source="own",medium="none"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Errorf("metrics lack %s", want)
		}
	}
	for _, secret := range []string{"secret", "newsletter", "email"} {
		if strings.Contains(strings.ToLower(metrics), `"`+secret) {
			t.Errorf("a caller's label %q became a metric label", secret)
		}
	}
	if strings.Contains(strings.ToLower(private["path"].(string)+unmatched["path"].(string)), "secret") {
		t.Errorf("a private or unmatched query reached the log: %v %v", private, unmatched)
	}
}

// Our own links say where they were read, and the files agents read ask them to say
// who they are.
func TestLinksCarryTheirMedium(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{"/", "?utm_medium=home"},
		{"/docs/pincode", "/v1/pincode/637001?utm_medium=docs"},
		{"/docs/pincode.md", "/v1/pincode/637001?utm_medium=docs-md"},
		{"/docs/logos", "utm_medium=docs-preview"},
		{"/llms.txt", "utm_medium=llms"},
		{"/llms.txt", "utm_source=<your name>"},
		{"/llms-full.txt", "utm_medium=llms"},
		{"/guides/pincode-autofill", "utm_medium=guide"},
		{"/v1", "utm_medium=index"},
		{"/openapi.json", "utm_source"},
		{"/.well-known/agent-skills/india-api/SKILL.md", "utm_medium=skill"},
	} {
		got := request(t, http.MethodGet, c.path, "")
		if got.status != http.StatusOK || !strings.Contains(string(got.body), c.want) {
			t.Errorf("%s (%d) does not contain %s", c.path, got.status, c.want)
		}
	}
}
