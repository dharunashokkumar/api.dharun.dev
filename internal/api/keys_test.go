package api

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/apikey"
)

// fakeAccountsDB writes a database with the columns the key table reads (the accounts
// package tests the same query against a real PocketBase database) and returns a
// usable key, a revoked one and a function that revokes the first.
func fakeAccountsDB(t *testing.T) (path, good, revoked string, revoke func()) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "data.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	good, _ = apikey.New()
	revoked, _ = apikey.New()
	for _, statement := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, verified BOOLEAN, disabled BOOLEAN, tier TEXT)`,
		`CREATE TABLE api_keys (id TEXT PRIMARY KEY, user TEXT, hash TEXT, revoked BOOLEAN)`,
		`INSERT INTO users VALUES ('user1', 1, 0, '')`,
		`INSERT INTO api_keys VALUES ('key1goodxxxxxxx', 'user1', '` + apikey.Hash(good) + `', 0)`,
		`INSERT INTO api_keys VALUES ('key2revokedxxxx', 'user1', '` + apikey.Hash(revoked) + `', 1)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return path, good, revoked, func() {
		if _, err := db.Exec(`UPDATE api_keys SET revoked = 1 WHERE id = 'key1goodxxxxxxx'`); err != nil {
			t.Fatal(err)
		}
	}
}

func send(server *Server, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.7:1000"
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	return recorder
}

func errorCode(recorder *httptest.ResponseRecorder) string {
	var body errorBody
	json.Unmarshal(recorder.Body.Bytes(), &body)
	return body.Error.Code
}

func TestKeysRaiseTheLimit(t *testing.T) {
	db, good, revoked, revoke := fakeAccountsDB(t)
	server := newTestServer(Config{Version: "t", RateLimitPerMinute: 5, KeyRateLimitPerMinute: 50, AccountsDB: db})
	defer server.Close()

	for i := range 6 {
		got := send(server, "GET", "/v1/states", "")
		if i < 5 && got.Code != http.StatusOK || i == 5 && got.Code != http.StatusTooManyRequests {
			t.Fatalf("keyless call %d = %d", i, got.Code)
		}
		if got.Header().Get("RateLimit-Limit") != "5" || got.Header().Get("RateLimit-Reset") == "" {
			t.Errorf("keyless headers: %v", got.Header())
		}
		if i == 5 && !strings.Contains(got.Body.String(), "/signup") {
			t.Errorf("a keyless 429 does not say where to get a key: %s", got.Body)
		}
	}
	// The same address with a key has its own, larger allowance.
	for _, header := range [][2]string{{"Authorization", "Bearer " + good}, {"X-API-Key", good}} {
		got := send(server, "GET", "/v1/states", "", header[0], header[1])
		if got.Code != http.StatusOK || got.Header().Get("RateLimit-Limit") != "50" {
			t.Errorf("keyed call with %s = %d, limit %q", header[0], got.Code, got.Header().Get("RateLimit-Limit"))
		}
	}

	mistyped := good[:len(good)-1] + "x"
	if mistyped == good {
		mistyped = good[:len(good)-1] + "y"
	}
	unknown, _ := apikey.New()
	for name, key := range map[string]string{"mistyped": mistyped, "unknown": unknown, "revoked": revoked} {
		if got := send(server, "GET", "/v1/states", "", "X-API-Key", key); got.Code != http.StatusUnauthorized || errorCode(got) != "invalid_key" {
			t.Errorf("%s key = %d %s", name, got.Code, got.Body)
		}
	}
	// Some other token a client sends is not ours to judge: it is a keyless call.
	if got := send(server, "GET", "/v1/states", "", "Authorization", "Bearer github_pat_xyz"); got.Code != http.StatusTooManyRequests {
		t.Errorf("an unrelated bearer token = %d, want the keyless limit", got.Code)
	}

	// A revoked key stops working at the next reload.
	revoke()
	server.keys.reload()
	if got := send(server, "GET", "/v1/states", "", "X-API-Key", good); got.Code != http.StatusUnauthorized {
		t.Errorf("revoked key after reload = %d", got.Code)
	}
}

func TestKeyRoutesNoticeThenRequire(t *testing.T) {
	db, good, _, _ := fakeAccountsDB(t)
	defer func(saved func() time.Time) { keyClock = saved }(keyClock)
	keyClock = func() time.Time { return time.Date(2026, 12, 1, 12, 0, 0, 0, indiaTime) }
	notice := newTestServer(Config{Version: "t", AccountsDB: db, KeyRequiredFrom: time.Date(2026, 12, 15, 0, 0, 0, 0, indiaTime)})
	defer notice.Close()

	// Before the date: answered as before (here without a model, so 503), with notice.
	got := send(notice, "GET", "/v1/embed?text=hello", "")
	if got.Code == http.StatusUnauthorized || !strings.Contains(got.Header().Get("X-Key-Notice"), "2026-12-15") {
		t.Errorf("before the date = %d, notice %q", got.Code, got.Header().Get("X-Key-Notice"))
	}
	// A light route gets no notice; a keyed call gets its quota instead.
	if got := send(notice, "GET", "/v1/states", ""); got.Header().Get("X-Key-Notice") != "" {
		t.Error("notice on a light route")
	}
	if got := send(notice, "GET", "/v1/embed?text=hello", "", "X-API-Key", good); got.Header().Get("X-Key-Notice") != "" || got.Header().Get("X-Quota-Limit") == "" {
		t.Errorf("keyed model call headers: %v", got.Header())
	}

	// From the date: refused without a key, in the usual error shape.
	keyClock = func() time.Time { return time.Date(2026, 12, 15, 0, 0, 1, 0, indiaTime) }
	got = send(notice, "GET", "/v1/embed?text=hello", "")
	if got.Code != http.StatusUnauthorized || errorCode(got) != "key_required" || !strings.Contains(got.Body.String(), "/signup") {
		t.Errorf("after the date = %d %s", got.Code, got.Body)
	}
	if got := send(notice, "GET", "/v1/text/redact?text=Call+Ravi&names=true", ""); errorCode(got) != "key_required" {
		t.Errorf("names=true after the date = %d %s", got.Code, got.Body)
	}
	if got := send(notice, "GET", "/v1/text/redact?text=Call+98765+43210", ""); got.Code != http.StatusOK {
		t.Errorf("plain redaction after the date = %d", got.Code)
	}
	if got := send(notice, "GET", "/v1/embed?text=hello", "", "X-API-Key", good); got.Code == http.StatusUnauthorized {
		t.Errorf("keyed call after the date = %d", got.Code)
	}
	// MCP tool calls carry the outer request's key to the inner one.
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"moderate_text","arguments":{"text":"Have a lovely day"}}}`
	keyless := send(notice, "POST", "/mcp", call, "Content-Type", "application/json", "Accept", "application/json, text/event-stream")
	keyed := send(notice, "POST", "/mcp", call, "Content-Type", "application/json", "Accept", "application/json, text/event-stream", "X-API-Key", good)
	if !strings.Contains(keyless.Body.String(), "free API key") || strings.Contains(keyed.Body.String(), "free API key") {
		t.Errorf("MCP: keyless %s, keyed %s", clip(keyless.Body.String(), 300), clip(keyed.Body.String(), 300))
	}
}

func TestModelQuotaPerKey(t *testing.T) {
	db, good, _, _ := fakeAccountsDB(t)
	server := newTestServer(Config{Version: "t", AccountsDB: db, ModelQuotaPerDay: 2, StateDir: t.TempDir()})
	defer server.Close()
	for i := range 3 {
		got := send(server, "GET", "/v1/embed?text=hello", "", "X-API-Key", good)
		if i < 2 && errorCode(got) == "quota_exceeded" || i == 2 && errorCode(got) != "quota_exceeded" {
			t.Fatalf("model call %d = %d %s", i, got.Code, got.Body)
		}
		if want := strconv.Itoa(max(2-i-1, 0)); got.Header().Get("X-Quota-Remaining") != want {
			t.Errorf("call %d: X-Quota-Remaining %q, want %s", i, got.Header().Get("X-Quota-Remaining"), want)
		}
	}
	report := server.usage.report("key1goodxxxxxxx", 2)
	if report.ModelsUsed != 2 || report.Today["ai"] == 0 && len(report.Today) == 0 {
		t.Errorf("usage report %+v", report)
	}
	// Counts survive a restart.
	server.usage.save()
	again := newKeyUsage(filepath.Dir(server.usage.path))
	if again.report("key1goodxxxxxxx", 2).ModelsUsed != 2 {
		t.Error("usage was not saved")
	}
	// A key the accounts database no longer has takes its counts with it.
	again.prune(map[string]bool{}, true)
	if len(again.Groups) != 0 || len(again.Models) != 0 {
		t.Error("deleted key's usage kept")
	}
}

func TestKeyIDLoggedOnKeyRoutesOnly(t *testing.T) {
	db, good, _, _ := fakeAccountsDB(t)
	server := newTestServer(Config{Version: "t", AccountsDB: db, LogRequests: true})
	defer server.Close()
	var logged bytes.Buffer
	server.logOut = &logged
	send(server, "GET", "/v1/states", "", "X-API-Key", good)
	send(server, "GET", "/v1/embed?text=hello", "", "X-API-Key", good)
	lines := strings.Split(strings.TrimSpace(logged.String()), "\n")
	if len(lines) != 2 || strings.Contains(lines[0], "key1good") || !strings.Contains(lines[1], `"key":"key1goodxxxxxxx"`) {
		t.Errorf("log lines: %s", logged.String())
	}
	if strings.Contains(logged.String(), good) {
		t.Error("the key itself was logged")
	}
}

func TestSiteTokens(t *testing.T) {
	defer func(saved func() time.Time) { keyClock = saved }(keyClock)
	keyClock = func() time.Time { return time.Date(2027, 1, 5, 10, 0, 0, 0, indiaTime) }
	server := newTestServer(Config{Version: "t", SiteTokenAllowance: 2, KeyRequiredFrom: time.Date(2027, 1, 1, 0, 0, 0, 0, indiaTime)})
	defer server.Close()
	token, _ := server.newSiteToken("203.0.113.7")
	for i := range 3 {
		got := send(server, "GET", "/v1/embed?text=hello", "", "X-Site-Token", token)
		if i < 2 && got.Code == http.StatusUnauthorized || i < 2 && errorCode(got) == "key_required" || i == 2 && errorCode(got) != "key_required" {
			t.Fatalf("site token call %d = %d %s", i, got.Code, got.Body)
		}
	}
	// A token from another address, or an expired one, is no token.
	other, _ := server.newSiteToken("198.51.100.1")
	if got := send(server, "GET", "/v1/embed?text=hello", "", "X-Site-Token", other); got.Code != http.StatusUnauthorized {
		t.Errorf("token from another address = %d", got.Code)
	}
	if _, ok := server.checkSiteToken(token+"0", "203.0.113.7"); ok {
		t.Error("altered token accepted")
	}
	keyClock = func() time.Time { return time.Date(2027, 1, 5, 12, 0, 0, 0, indiaTime) }
	if _, ok := server.checkSiteToken(token, "203.0.113.7"); ok {
		t.Error("expired token accepted")
	}
}

// solve works out a challenge the way the browser does (assets/pow.js).
func solve(t *testing.T, server *Server) string {
	t.Helper()
	got := send(server, "GET", "/account/api/challenge", "")
	var challenge struct {
		Algorithm, Challenge, Salt, Signature string
		Maxnumber                             int
	}
	if err := json.Unmarshal(got.Body.Bytes(), &challenge); err != nil {
		t.Fatalf("challenge: %s", got.Body)
	}
	for n := 0; n <= challenge.Maxnumber; n++ {
		sum := sha256.Sum256([]byte(challenge.Salt + strconv.Itoa(n)))
		if hex.EncodeToString(sum[:]) == challenge.Challenge {
			payload, _ := json.Marshal(map[string]any{"algorithm": challenge.Algorithm, "challenge": challenge.Challenge, "number": n, "salt": challenge.Salt, "signature": challenge.Signature})
			return base64.StdEncoding.EncodeToString(payload)
		}
	}
	t.Fatal("challenge has no solution")
	return ""
}

// fakeAccounts stands in for the account program and records what reached it.
type fakeAccounts struct {
	mu   sync.Mutex
	seen []string
}

func (f *fakeAccounts) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.seen = append(f.seen, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization")+" "+string(body)+" fwd="+r.Header.Get("X-Forwarded-For"))
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/india/me":
		if r.Header.Get("Authorization") != "tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"id":"user1","keys":[{"id":"key1goodxxxxxxx","name":"app"}]}`)
	default:
		io.WriteString(w, `{"ok":true}`)
	}
}

func (f *fakeAccounts) calls() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.seen, "\n")
}

func TestAccountPassThrough(t *testing.T) {
	accounts := &fakeAccounts{}
	upstream := httptest.NewServer(accounts)
	defer upstream.Close()
	server := newTestServer(Config{Version: "t", AccountsURL: upstream.URL, PublicOrigin: "https://api.example"})
	defer server.Close()

	if got := send(server, "POST", "/account/api/collections/users/auth-with-password", `{"identity":"a@b.c","password":"x"}`); got.Code != http.StatusOK || got.Header().Get("Access-Control-Allow-Origin") != "https://api.example" {
		t.Errorf("login pass-through = %d %v", got.Code, got.Header())
	}
	if got := send(server, "POST", "/account/api/india/keys/abc123def456ghi/revoke", "", "Authorization", "tok"); got.Code != http.StatusOK {
		t.Errorf("revoke = %d", got.Code)
	}
	// The admin screen, superusers, the record API and anything unlisted never pass.
	for _, path := range []string{"_/", "collections/_superusers/auth-with-password", "collections/users/records", "collections/api_keys/records",
		"india/keys/../../collections/users/records", "settings", "backups", "india/keys/ABC/revoke"} {
		for _, method := range []string{"GET", "POST"} {
			if got := send(server, method, "/account/api/"+path, "{}"); got.Code != http.StatusNotFound && got.Code != http.StatusMethodNotAllowed && got.Code != http.StatusTemporaryRedirect {
				t.Errorf("%s %s = %d, want 404", method, path, got.Code)
			}
		}
	}
	if strings.Contains(accounts.calls(), "_superusers") || strings.Contains(accounts.calls(), "/records") || strings.Contains(accounts.calls(), "fwd=203") {
		t.Errorf("reached the account program: %s", accounts.calls())
	}

	// Usage: the login token decides which keys; the counts come from the API itself.
	server.usage.count("key1goodxxxxxxx", "pincode")
	got := send(server, "GET", "/account/api/usage", "", "Authorization", "tok")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"pincode":1`) {
		t.Errorf("usage = %d %s", got.Code, got.Body)
	}
	if got := send(server, "GET", "/account/api/usage", "", "Authorization", "wrong"); got.Code != http.StatusUnauthorized {
		t.Errorf("usage with a bad login = %d", got.Code)
	}

	none := newTestServer(Config{Version: "t"})
	defer none.Close()
	if got := send(none, "POST", "/account/api/collections/users/auth-with-password", "{}"); got.Code != http.StatusServiceUnavailable || errorCode(got) != "accounts_unavailable" {
		t.Errorf("no accounts = %d %s", got.Code, got.Body)
	}
}

func TestSignup(t *testing.T) {
	accounts := &fakeAccounts{}
	upstream := httptest.NewServer(accounts)
	defer upstream.Close()
	server := newTestServer(Config{Version: "t", AccountsURL: upstream.URL, SignupsPerDay: 2})
	defer server.Close()
	signup := func(email, password, proof string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"email": email, "password": password, "altcha": proof})
		return send(server, "POST", "/account/api/signup", string(body), "Content-Type", "application/json")
	}
	if got := signup("a@example.com", "long enough password", "bm9uZQ=="); errorCode(got) != "challenge_failed" {
		t.Errorf("no proof = %d %s", got.Code, got.Body)
	}
	if got := signup("a@mailinator.com", "long enough password", solve(t, server)); got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "Throwaway") {
		t.Errorf("disposable = %d %s", got.Code, got.Body)
	}
	if got := signup("a@example.com", "short", solve(t, server)); got.Code != http.StatusBadRequest {
		t.Errorf("short password = %d", got.Code)
	}
	proof := solve(t, server)
	if got := signup("Student@Example.com", "long enough password", proof); got.Code != http.StatusOK {
		t.Fatalf("signup = %d %s", got.Code, got.Body)
	}
	if got := signup("b@example.com", "long enough password", proof); errorCode(got) != "challenge_failed" {
		t.Errorf("a proof used twice = %d", got.Code)
	}
	calls := accounts.calls()
	if !strings.Contains(calls, "POST /api/collections/users/records") || !strings.Contains(calls, `"email":"student@example.com"`) ||
		!strings.Contains(calls, "POST /api/collections/users/request-verification") {
		t.Errorf("signup did not create and mail: %s", calls)
	}
	signup("c@example.com", "long enough password", solve(t, server))
	if got := signup("d@example.com", "long enough password", solve(t, server)); got.Code != http.StatusTooManyRequests {
		t.Errorf("third signup from one address with a cap of two = %d", got.Code)
	}
}

func TestAccountPagesAndAssets(t *testing.T) {
	server := newTestServer(Config{Version: "t"})
	defer server.Close()
	for _, path := range []string{"/signup", "/login", "/account", "/account/verify", "/account/reset", "/account/oauth"} {
		got := send(server, "GET", path, "")
		if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `id="account-app"`) || !strings.Contains(got.Body.String(), "/assets/account.js") {
			t.Errorf("%s = %d", path, got.Code)
		}
		if noindex := strings.Contains(got.Body.String(), `content="noindex"`); noindex == (path == "/signup") {
			t.Errorf("%s: noindex %v", path, noindex)
		}
	}
	for _, file := range []string{"account.js", "pow.js"} {
		if got := send(server, "GET", "/assets/"+file, ""); got.Code != http.StatusOK || !strings.HasPrefix(got.Header().Get("Content-Type"), "text/javascript") {
			t.Errorf("asset %s = %d", file, got.Code)
		}
	}
	if got := send(server, "GET", "/assets/..%2fserver.go", ""); got.Code != http.StatusNotFound {
		t.Errorf("asset escape = %d", got.Code)
	}
}
