package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// A stand-in for GitHub's device flow: the first poll is pending, the next approves.
func fakeGitHub(t *testing.T, userID int64, login, created string) *httptest.Server {
	var mu sync.Mutex
	polls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("client_id") != "test-client" {
			http.Error(w, "bad client", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"device_code": "dev-123", "user_code": "ABCD-1234",
			"verification_uri": "https://github.com/login/device", "expires_in": 900, "interval": 5})
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		mu.Lock()
		if r.Form.Get("device_code") == "dev-123" {
			polls++
		}
		n := polls
		mu.Unlock()
		switch {
		case r.Form.Get("device_code") == "denied":
			json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
		case n == 1:
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
		default:
			json.NewEncoder(w).Encode(map[string]string{"access_token": "gho_test", "token_type": "bearer"})
		}
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gho_test" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": userID, "login": login, "created_at": created})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// fakeController stands in for sandboxd: it keeps sessions and databases in memory.
type fakeController struct {
	mu        sync.Mutex
	sessions  map[string]sandbox.Session
	databases map[string]sandbox.Database
	files     map[string][]byte
	runs      []sandbox.RunRequest
	browser   []sandbox.BrowserRequest
}

func newFakeController(t *testing.T) (*fakeController, *httptest.Server) {
	c := &fakeController{sessions: map[string]sandbox.Session{}, databases: map[string]sandbox.Database{}, files: map[string][]byte{}}
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, v any) { json.NewEncoder(w).Encode(v) }
	missing := func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusNotFound)
		reply(w, sandbox.Error{Code: sandbox.CodeNotFound, Message: "no such session"})
	}
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		reply(w, sandbox.Status{Languages: map[string]sandbox.Language{"python": {Name: "Python", File: "main.py", Command: []string{"python3", "main.py"}, Version: "3.13.5"}}})
	})
	mux.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.RunRequest
		json.NewDecoder(r.Body).Decode(&req)
		c.mu.Lock()
		c.runs = append(c.runs, req)
		c.mu.Unlock()
		if req.Language == "cobol" {
			w.WriteHeader(http.StatusBadRequest)
			reply(w, sandbox.Error{Code: sandbox.CodeBadRequest, Message: `bad request: unknown language "cobol"`})
			return
		}
		reply(w, sandbox.Result{Stdout: "45\n", ExitCode: 0, DurationMillis: 120, CPUMillis: 90, LimitsHit: []string{}})
	})
	mux.HandleFunc("POST /sessions", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.SessionRequest
		json.NewDecoder(r.Body).Decode(&req)
		c.mu.Lock()
		defer c.mu.Unlock()
		session := sandbox.Session{ID: "ses" + itoa(len(c.sessions)+1), Image: cmpOr(req.Image, "code"), Network: req.Network,
			Account: req.Account, CreatedAt: "2026-10-17T10:00:00Z", ExpiresAt: "2026-10-17T10:05:00Z"}
		c.sessions[session.ID] = session
		reply(w, session)
	})
	mux.HandleFunc("GET /sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		session, ok := c.sessions[r.PathValue("id")]
		c.mu.Unlock()
		if !ok {
			missing(w)
			return
		}
		session.CPUMillis = 10
		reply(w, session)
	})
	mux.HandleFunc("DELETE /sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		session, ok := c.sessions[r.PathValue("id")]
		delete(c.sessions, r.PathValue("id"))
		c.mu.Unlock()
		if !ok {
			missing(w)
			return
		}
		reply(w, session)
	})
	mux.HandleFunc("POST /sessions/{id}/exec", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.ExecRequest
		json.NewDecoder(r.Body).Decode(&req)
		reply(w, sandbox.Result{Stdout: "ran: " + req.Command, CPUMillis: 40, LimitsHit: []string{}})
	})
	mux.HandleFunc("PUT /sessions/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.files[r.PathValue("id")+"/"+r.URL.Query().Get("path")] = data
		c.mu.Unlock()
		reply(w, map[string]int{"written": len(data)})
	})
	mux.HandleFunc("GET /sessions/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("list") == "true" {
			reply(w, []sandbox.Entry{{Name: "a.txt", Size: 4}})
			return
		}
		c.mu.Lock()
		data := c.files[r.PathValue("id")+"/"+r.URL.Query().Get("path")]
		c.mu.Unlock()
		w.Write(data)
	})
	mux.HandleFunc("GET /sessions/{id}/tty", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			kind, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			conn.Write(r.Context(), kind, append([]byte("echo:"), data...))
		}
	})
	mux.HandleFunc("POST /databases", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Account string }
		json.NewDecoder(r.Body).Decode(&req)
		c.mu.Lock()
		defer c.mu.Unlock()
		database := sandbox.Database{Name: "sb_" + itoa(len(c.databases)+1), Account: req.Account, LimitBytes: sandbox.DatabaseLimit}
		database.User = database.Name
		c.databases[database.Name] = database
		database.Password = "secret-password"
		reply(w, database)
	})
	mux.HandleFunc("GET /databases/{name}", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		database, ok := c.databases[r.PathValue("name")]
		c.mu.Unlock()
		if !ok {
			missing(w)
			return
		}
		reply(w, database)
	})
	mux.HandleFunc("DELETE /databases/{name}", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		delete(c.databases, r.PathValue("name"))
		c.mu.Unlock()
		reply(w, map[string]bool{"dropped": true})
	})
	mux.HandleFunc("POST /browser", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.BrowserRequest
		json.NewDecoder(r.Body).Decode(&req)
		c.mu.Lock()
		c.browser = append(c.browser, req)
		c.mu.Unlock()
		switch req.Action {
		case sandbox.BrowserScreenshot:
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("\x89PNG fake"))
		case sandbox.BrowserPDF:
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF-1.7 fake"))
		default:
			w.Header().Set("Content-Type", "application/json")
			reply(w, sandbox.PageText{URL: req.URL, Title: "Example", Text: "Hello", Format: "text", Status: 200})
		}
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer controller-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return c, server
}

type sandboxRig struct {
	t          *testing.T
	server     *Server
	http       *httptest.Server
	controller *fakeController
	stateDir   string
}

func newSandboxRig(t *testing.T, created string) *sandboxRig {
	gh := fakeGitHub(t, 4242, "dharunashokkumar", created)
	controller, controllerServer := newFakeController(t)
	turnstileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		json.NewEncoder(w).Encode(map[string]bool{"success": r.Form.Get("response") == "good" && r.Form.Get("secret") == "ts-secret"})
	}))
	t.Cleanup(turnstileServer.Close)
	stateDir := t.TempDir()
	server := newTestServer(Config{Version: "t", StateDir: stateDir, PublicOrigin: testOrigin, TrustProxy: true,
		GitHubClientID: "test-client", GitHubWebURL: gh.URL, GitHubAPIURL: gh.URL,
		SandboxURL: controllerServer.URL, SandboxSecret: "controller-secret", SandboxPGAddr: "127.0.0.1:1",
		TurnstileSiteKey: "ts-site", TurnstileSecret: "ts-secret", TurnstileVerifyURL: turnstileServer.URL})
	t.Cleanup(server.Close)
	front := httptest.NewServer(server)
	t.Cleanup(front.Close)
	return &sandboxRig{t: t, server: server, http: front, controller: controller, stateDir: stateDir}
}

func (r *sandboxRig) do(method, path, token, body string) result {
	r.t.Helper()
	req, _ := http.NewRequest(method, r.http.URL+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	return result{response.StatusCode, response.Header, raw}
}

func (r *sandboxRig) login() string {
	r.t.Helper()
	start := r.do(http.MethodPost, "/v1/sandbox/login", "", "")
	if start.status != http.StatusOK || start.data(r.t)["userCode"] != "ABCD-1234" {
		r.t.Fatalf("login start = %d %s", start.status, start.body)
	}
	if pending := r.do(http.MethodPost, "/v1/sandbox/login/poll", "", `{"deviceCode":"dev-123"}`).data(r.t); pending["status"] != "pending" {
		r.t.Fatalf("first poll = %v", pending)
	}
	done := r.do(http.MethodPost, "/v1/sandbox/login/poll", "", `{"deviceCode":"dev-123"}`).data(r.t)
	token, _ := done["token"].(string)
	if done["status"] != "ok" || !strings.HasPrefix(token, "isb_") {
		r.t.Fatalf("second poll = %v", done)
	}
	return token
}

func TestSandboxLoginAndQuota(t *testing.T) {
	rig := newSandboxRig(t, "2015-03-01T00:00:00Z")
	if got := rig.do(http.MethodPost, "/v1/sandbox/run", "", `{"language":"python","code":"print(1)"}`); got.status != http.StatusUnauthorized {
		t.Errorf("run without a token = %d", got.status)
	}
	if got := rig.do(http.MethodGet, "/v1/sandbox/account", "isb_madeup", ""); got.status != http.StatusUnauthorized {
		t.Errorf("made-up token = %d", got.status)
	}
	if got := rig.do(http.MethodPost, "/v1/sandbox/login/poll", "", `{"deviceCode":"denied"}`); got.status != http.StatusForbidden {
		t.Errorf("denied sign-in = %d", got.status)
	}
	token := rig.login()
	account := rig.do(http.MethodGet, "/v1/sandbox/account", token, "").data(t)
	if account["login"] != "dharunashokkumar" || account["quota"].(map[string]any)["runs"] != float64(fullQuota.Runs) {
		t.Errorf("account = %v", account)
	}

	run := rig.do(http.MethodPost, "/v1/sandbox/run", token, `{"language":"python","code":"print(sum(range(10)))","files":[{"path":"a.bin","content":"AAE=","encoding":"base64"}]}`)
	if run.status != http.StatusOK || run.data(t)["stdout"] != "45\n" {
		t.Fatalf("run = %d %s", run.status, run.body)
	}
	if req := rig.controller.runs[0]; req.Account != "gh:4242" || len(req.Files) != 1 || !bytes.Equal(req.Files[0].Content, []byte{0, 1}) {
		t.Errorf("controller got %+v", req)
	}
	if bad := rig.do(http.MethodPost, "/v1/sandbox/run", token, `{"language":"cobol","code":"x"}`); bad.status != http.StatusBadRequest {
		t.Errorf("unknown language = %d %s", bad.status, bad.body)
	}
	used := rig.do(http.MethodGet, "/v1/sandbox/account", token, "").data(t)["used"].(map[string]any)
	if used["runs"] != float64(2) || used["cpuMs"] != float64(90) {
		t.Errorf("used = %v", used)
	}

	// The day's allowance runs out.
	rig.server.extras.accounts.charge(4242, fullQuota.Runs, 0, time.Now())
	if got := rig.do(http.MethodPost, "/v1/sandbox/run", token, `{"language":"python","code":"print(1)"}`); got.status != http.StatusTooManyRequests {
		t.Errorf("over quota = %d %s", got.status, got.body)
	}

	// The audit log holds a hash of the code, never the code.
	raw, _ := os.ReadFile(filepath.Join(rig.stateDir, "audit", time.Now().UTC().Format("2006-01")+".jsonl"))
	if !bytes.Contains(raw, []byte(codeHash("print(sum(range(10)))"))) || bytes.Contains(raw, []byte("range(10)")) {
		t.Errorf("audit log:\n%s", raw)
	}

	// A blocked account is refused; a revoked token stops working.
	rig.server.extras.accounts.block("dharunashokkumar", "mining")
	if got := rig.do(http.MethodGet, "/v1/sandbox/account", token, ""); got.status != http.StatusForbidden {
		t.Errorf("blocked = %d", got.status)
	}
	rig.server.extras.accounts.block("dharunashokkumar", "")
	rig.do(http.MethodPost, "/v1/sandbox/logout", token, "")
	if got := rig.do(http.MethodGet, "/v1/sandbox/account", token, ""); got.status != http.StatusUnauthorized {
		t.Errorf("after logout = %d", got.status)
	}
}

func TestSandboxNewAccountQuota(t *testing.T) {
	rig := newSandboxRig(t, time.Now().Add(-48*time.Hour).UTC().Format(time.RFC3339))
	token := rig.login()
	quota := rig.do(http.MethodGet, "/v1/sandbox/account", token, "").data(t)["quota"].(map[string]any)
	if quota["runs"] != float64(newQuota.Runs) {
		t.Errorf("a two-day-old GitHub account gets %v", quota)
	}
}

func TestSandboxSessions(t *testing.T) {
	rig := newSandboxRig(t, "2015-03-01T00:00:00Z")
	token := rig.login()
	session := rig.do(http.MethodPost, "/v1/sandbox/sessions", token, `{"image":"code","network":true}`).data(t)
	id, _ := session["id"].(string)
	if id == "" || session["links"].(map[string]any)["tty"] != "wss://api.example.test/v1/sandbox/sessions/"+id+"/tty" {
		t.Fatalf("session = %v", session)
	}
	if got := rig.do(http.MethodPost, "/v1/sandbox/sessions/"+id+"/exec", token, `{"command":"ls"}`).data(t); got["stdout"] != "ran: ls" {
		t.Errorf("exec = %v", got)
	}
	if got := rig.do(http.MethodPost, "/v1/sandbox/sessions/"+id+"/upload?path=a.txt", token, "data"); got.status != http.StatusOK {
		t.Errorf("upload = %d %s", got.status, got.body)
	}
	if got := rig.do(http.MethodGet, "/v1/sandbox/sessions/"+id+"/download?path=a.txt", token, ""); string(got.body) != "data" || got.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("download = %q %v", got.body, got.header)
	}
	if got := rig.do(http.MethodGet, "/v1/sandbox/sessions/"+id+"/download", token, "").data(t); len(got["entries"].([]any)) != 1 {
		t.Errorf("listing = %v", got)
	}

	// Someone else's session does not exist for them.
	other := rig.server.extras.accounts
	otherToken, _, _ := other.signIn(githubUser{ID: 99, Login: "someone", CreatedAt: "2015-01-01T00:00:00Z"}, time.Now())
	for _, path := range []string{"/v1/sandbox/sessions/" + id, "/v1/sandbox/sessions/" + id + "/download?path=a.txt"} {
		if got := rig.do(http.MethodGet, path, otherToken, ""); got.status != http.StatusNotFound {
			t.Errorf("%s for another account = %d", path, got.status)
		}
	}
	if got := rig.do(http.MethodPost, "/v1/sandbox/sessions/"+id+"/stop", otherToken, ""); got.status != http.StatusNotFound {
		t.Errorf("another account stopped the session: %d", got.status)
	}

	// The API's session has no terminal; one from the terminal page needs the browser check.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsBase := strings.Replace(rig.http.URL, "http", "ws", 1)
	if _, response, err := websocket.Dial(ctx, wsBase+"/v1/sandbox/sessions/"+id+"/tty?token="+token, nil); err == nil || response.StatusCode != http.StatusForbidden {
		t.Errorf("TTY for an API session: %v", err)
	}
	if got := rig.do(http.MethodPost, "/v1/sandbox/sessions", token, `{"terminal":true,"turnstile":"bad"}`); got.status != http.StatusForbidden {
		t.Errorf("failed browser check = %d", got.status)
	}
	terminal := rig.do(http.MethodPost, "/v1/sandbox/sessions", token, `{"image":"debian","terminal":true,"turnstile":"good"}`).data(t)
	conn, _, err := websocket.Dial(ctx, wsBase+"/v1/sandbox/sessions/"+terminal["id"].(string)+"/tty?token="+token, nil)
	if err != nil {
		t.Fatalf("terminal: %v", err)
	}
	conn.Write(ctx, websocket.MessageBinary, []byte("ls\r"))
	if _, data, err := conn.Read(ctx); err != nil || string(data) != "echo:ls\r" {
		t.Errorf("terminal echo = %q %v", data, err)
	}
	conn.Close(websocket.StatusNormalClosure, "")

	if got := rig.do(http.MethodPost, "/v1/sandbox/sessions/"+id+"/stop", token, ""); got.status != http.StatusOK {
		t.Errorf("stop = %d %s", got.status, got.body)
	}
	if got := rig.do(http.MethodGet, "/v1/sandbox/sessions/"+id, token, ""); got.status != http.StatusNotFound {
		t.Errorf("stopped session = %d", got.status)
	}
}

func TestSandboxDatabases(t *testing.T) {
	rig := newSandboxRig(t, "2015-03-01T00:00:00Z")
	token := rig.login()
	database := rig.do(http.MethodPost, "/v1/db", token, "").data(t)
	name, _ := database["name"].(string)
	if database["password"] != "secret-password" || database["websocket"] != "wss://api.example.test/v1/db/ws" ||
		database["connectionString"] != "postgresql://"+name+":secret-password@api.example.test/"+name+"?sslmode=disable" {
		t.Fatalf("database = %v", database)
	}
	if got := rig.do(http.MethodGet, "/v1/db/"+name, token, "").data(t); got["password"] != nil {
		t.Errorf("the password came back: %v", got)
	}
	otherToken, _, _ := rig.server.extras.accounts.signIn(githubUser{ID: 99, Login: "someone", CreatedAt: "2015-01-01T00:00:00Z"}, time.Now())
	if got := rig.do(http.MethodPost, "/v1/db/"+name+"/drop", otherToken, ""); got.status != http.StatusNotFound {
		t.Errorf("another account dropped it: %d", got.status)
	}
	if got := rig.do(http.MethodPost, "/v1/db/"+name+"/drop", token, ""); got.status != http.StatusOK {
		t.Errorf("drop = %d", got.status)
	}
}

func TestBrowserTools(t *testing.T) {
	rig := newSandboxRig(t, "2015-03-01T00:00:00Z")
	sites := useTestSites(t)
	fakeZone["pages.test. A"] = []string{"pages.test. 300 IN A 104.21.0.9"}
	fakeZone["closed.test. A"] = []string{"closed.test. 300 IN A 104.21.0.10"}
	sites.serve("pages.test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "User-agent: *\nDisallow: /private\nAllow: /private/ok\n")
	}))
	sites.serve("closed.test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "User-agent: india-api-browser\nDisallow: /\n\nUser-agent: *\nAllow: /\n")
	}))
	shot := rig.do(http.MethodGet, "/v1/browser/screenshot?url=http://pages.test/&width=800", "", "")
	if shot.status != http.StatusOK || shot.header.Get("Content-Type") != "image/png" || shot.header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("screenshot = %d %v %s", shot.status, shot.header, shot.body)
	}
	pdf := rig.do(http.MethodPost, "/v1/browser/pdf", "", `{"html":"<h1>hi</h1>","paper":"a4"}`)
	if pdf.status != http.StatusOK || !bytes.HasPrefix(pdf.body, []byte("%PDF")) {
		t.Errorf("pdf = %d %s", pdf.status, pdf.body)
	}
	if text := rig.do(http.MethodGet, "/v1/browser/text?url=http://pages.test/page", "", "").data(t); text["title"] != "Example" {
		t.Errorf("text = %v", text)
	}
	for path, want := range map[string]int{
		"/v1/browser/text?url=http://pages.test/private/x":    http.StatusForbidden,
		"/v1/browser/text?url=http://pages.test/private/ok/x": http.StatusOK,
		"/v1/browser/text?url=http://closed.test/":            http.StatusForbidden,
		"/v1/browser/screenshot?url=http://closed.test/":      http.StatusOK, // robots.txt is for the text tool
		"/v1/browser/screenshot?url=http://10.0.0.1/":         http.StatusUnprocessableEntity,
		"/v1/browser/screenshot?url=ftp://pages.test/":        http.StatusBadRequest,
		"/v1/browser/screenshot?url=http://pages.test:8080/":  http.StatusUnprocessableEntity,
		"/v1/browser/screenshot":                              http.StatusBadRequest,
	} {
		if got := rig.do(http.MethodGet, path, "", ""); got.status != want {
			t.Errorf("%s = %d, want %d: %s", path, got.status, want, clip(string(got.body), 200))
		}
	}
	if got := rig.do(http.MethodPost, "/v1/browser/text", "", `{"html":"<p>x</p>"}`); got.status != http.StatusBadRequest {
		t.Errorf("text from html = %d", got.status)
	}
	if got := rig.controller.browser[0]; got.Width != 800 || got.Action != sandbox.BrowserScreenshot || got.URL != "http://pages.test/" {
		t.Errorf("controller got %+v", got)
	}
}

func TestRobotsRules(t *testing.T) {
	rules := parseRobots(strings.NewReader(`# comment
User-agent: Googlebot
Disallow: /

User-agent: *
Disallow: /admin
Disallow: /*.pdf$
Allow: /admin/public
`), browserAgent)
	for path, want := range map[string]bool{
		"/": true, "/admin": false, "/admin/x": false, "/admin/public/page": true, "/doc.pdf": false, "/doc.pdf?x=1": true,
	} {
		if got := rules.allows(path); got != want {
			t.Errorf("%s: %v, want %v", path, got, want)
		}
	}
	if !parseRobots(strings.NewReader(""), browserAgent).allows("/x") {
		t.Error("an empty robots.txt allows everything")
	}
}

func TestSandboxThroughMCP(t *testing.T) {
	rig := newSandboxRig(t, "2015-03-01T00:00:00Z")
	token := rig.login()
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_code","arguments":{"language":"python","code":"print(45)"}}}`
	for _, c := range []struct {
		name, header, call string
		ok                 bool
	}{
		{"no token", "", call, false},
		{"token as the connection's header", "Bearer " + token, call, true},
		{"token as an argument", "", strings.Replace(call, `"code":`, `"sandbox_token":"`+token+`","code":`, 1), true},
	} {
		req, _ := http.NewRequest(http.MethodPost, rig.http.URL+"/mcp", strings.NewReader(c.call))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(response.Body)
		response.Body.Close()
		failed := bytes.Contains(raw, []byte(`"isError":true`))
		if failed == c.ok || (c.ok && !bytes.Contains(raw, []byte(`45`))) {
			t.Errorf("%s: %s", c.name, raw)
		}
	}
}

func TestTerminalPage(t *testing.T) {
	rig := newSandboxRig(t, "2015-03-01T00:00:00Z")
	page := rig.do(http.MethodGet, "/terminal", "", "")
	if page.status != http.StatusOK || !bytes.Contains(page.body, []byte(`data-sitekey="ts-site"`)) ||
		!strings.Contains(page.header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Errorf("terminal page = %d %v", page.status, page.header)
	}
	if off := get(t, "/terminal"); !bytes.Contains(off.body, []byte(`data-sitekey=""`)) {
		t.Error("a server without Turnstile offers the terminal")
	}
	for _, name := range []string{"xterm.js", "addon-fit.js", "xterm.css", "terminal.js", "terminal.css", "LICENSE-xterm.txt"} {
		if got := get(t, "/terminal/"+name); got.status != http.StatusOK || len(got.body) == 0 {
			t.Errorf("/terminal/%s = %d", name, got.status)
		}
	}
	expectStatus(t, "/terminal/index.html", http.StatusNotFound)
	// The vendored xterm.js is the published one.
	sums, _ := terminalFiles.ReadFile("terminal/SHA256SUMS")
	for _, line := range strings.Split(strings.TrimSpace(string(sums)), "\n") {
		want, name, _ := strings.Cut(line, "  ")
		data, _ := terminalFiles.ReadFile("terminal/" + name)
		if got := sha256Hex(data); got != want {
			t.Errorf("%s changed: %s", name, got)
		}
	}
}

func TestSandboxOffWithoutSettings(t *testing.T) {
	for _, path := range []string{"/v1/sandbox/account", "/v1/browser/screenshot?url=https://example.com"} {
		if got := get(t, path); got.status != http.StatusServiceUnavailable {
			t.Errorf("%s without the sandbox set up = %d", path, got.status)
		}
	}
	if languages := expectStatus(t, "/v1/sandbox/languages", http.StatusOK).data(t); len(languages["languages"].([]any)) != len(sandbox.Languages) {
		t.Errorf("languages = %v", languages)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
