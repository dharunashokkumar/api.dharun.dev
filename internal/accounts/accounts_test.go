package accounts

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	_ "github.com/pocketbase/pocketbase/migrations"
	_ "modernc.org/sqlite"

	"github.com/dharunashokkumar/api.dharun.dev/internal/apikey"
)

// testAccounts starts the account system in a temporary directory, with mail caught
// instead of sent. No network.
func testAccounts(t *testing.T) (http.Handler, core.App, *mailbox) {
	t.Helper()
	dir := t.TempDir()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: dir})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	mails := &mailbox{}
	app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error {
		mails.add(e.Message.HTML)
		return nil // caught: never sent
	})
	Setup(app, Config{PublicOrigin: "https://api.example", SMTP: SMTP{From: "accounts@example.com"}, MaxKeys: 2})
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	var handler http.Handler
	event := &core.ServeEvent{App: app, Router: router}
	if err := app.OnServe().Trigger(event, func(e *core.ServeEvent) error {
		mux, err := e.Router.BuildMux()
		handler = mux
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return handler, app, mails
}

// mailbox holds the mails PocketBase sent; it sends them in the background.
type mailbox struct {
	mu   sync.Mutex
	sent []string
}

func (m *mailbox) add(body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, body)
}

// wait returns the newest mail once there are at least n.
func (m *mailbox) wait(n int) string {
	for range 200 {
		m.mu.Lock()
		if len(m.sent) >= n {
			defer m.mu.Unlock()
			return m.sent[len(m.sent)-1]
		}
		m.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return ""
}

func call(t *testing.T, handler http.Handler, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	encoded, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, strings.NewReader(string(encoded)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var decoded map[string]any
	json.Unmarshal(rec.Body.Bytes(), &decoded)
	return rec.Code, decoded
}

func TestAccountLifecycle(t *testing.T) {
	handler, app, mails := testAccounts(t)
	email, password := "student@example.com", "correct horse battery"

	if status, body := call(t, handler, "POST", "/api/collections/users/records", "", map[string]any{
		"email": email, "password": password, "passwordConfirm": password, "tier": "pro",
	}); status != http.StatusBadRequest && status != http.StatusForbidden {
		t.Fatalf("signup that sets its own tier = %d %v, want refused", status, body)
	}
	if status, body := call(t, handler, "POST", "/api/collections/users/records", "", map[string]any{
		"email": email, "password": password, "passwordConfirm": password,
	}); status != http.StatusOK {
		t.Fatalf("signup = %d %v", status, body)
	}
	_, auth := call(t, handler, "POST", "/api/collections/users/auth-with-password", "", map[string]any{"identity": email, "password": password})
	token, _ := auth["token"].(string)
	if token == "" {
		t.Fatalf("login gave no token: %v", auth)
	}

	// No key before the address is confirmed.
	if status, _ := call(t, handler, "POST", "/api/india/keys", token, map[string]any{"name": "app"}); status != http.StatusForbidden {
		t.Errorf("key before verification = %d, want 403", status)
	}
	if status, body := call(t, handler, "POST", "/api/collections/users/request-verification", "", map[string]any{"email": email}); status != http.StatusNoContent {
		t.Fatalf("request verification = %d %v", status, body)
	}
	mail := mails.wait(1)
	link := regexp.MustCompile(`https://api\.example/account/verify#([A-Za-z0-9_.\-]+)`).FindStringSubmatch(mail)
	if link == nil {
		t.Fatalf("the mail does not link to our page: %q", mail)
	}
	if status, body := call(t, handler, "POST", "/api/collections/users/confirm-verification", "", map[string]any{"token": link[1]}); status != http.StatusNoContent {
		t.Fatalf("confirm = %d %v", status, body)
	}

	status, created := call(t, handler, "POST", "/api/india/keys", token, map[string]any{"name": "my app"})
	key, _ := created["key"].(string)
	if status != http.StatusOK || apikey.Check(key) != nil {
		t.Fatalf("create key = %d %v", status, created)
	}
	call(t, handler, "POST", "/api/india/keys", token, map[string]any{"name": "second"})
	if status, _ := call(t, handler, "POST", "/api/india/keys", token, map[string]any{"name": "third"}); status != http.StatusBadRequest {
		t.Errorf("third key with a cap of two = %d, want 400", status)
	}

	// The key itself is never shown again: only its hint.
	_, me := call(t, handler, "GET", "/api/india/me", token, nil)
	if raw, _ := json.Marshal(me); strings.Contains(string(raw), key) || !strings.Contains(string(raw), key[len(key)-4:]) {
		t.Errorf("me shows the key or not its hint: %s", raw)
	}

	// The API reads the table straight from the database, read-only.
	db, err := sql.Open("sqlite", "file:"+filepath.Join(app.DataDir(), "data.db")+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var id, hash, user, tier string
	var usable int
	if err := db.QueryRow(apikey.LookupQuery, apikey.Hash(key)).Scan(&id, &hash, &user, &tier, &usable); err != nil {
		t.Fatalf("lookup query: %v", err)
	}
	if usable != 1 || tier != "free" || id != created["id"] {
		t.Errorf("lookup = %s %s %d, want the key usable on the free tier", id, tier, usable)
	}

	// Revoked keys stop being usable.
	call(t, handler, "POST", "/api/india/keys/"+id+"/revoke", token, nil)
	db.QueryRow(apikey.LookupQuery, apikey.Hash(key)).Scan(&id, &hash, &user, &tier, &usable)
	if usable != 0 {
		t.Error("a revoked key is still usable")
	}

	// A disabled account cannot log in, and its keys stop working.
	record, _ := app.FindAuthRecordByEmail("users", email)
	record.Set("disabled", true)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	if status, _ := call(t, handler, "POST", "/api/collections/users/auth-with-password", "", map[string]any{"identity": email, "password": password}); status == http.StatusOK {
		t.Error("a disabled account logged in")
	}
	record.Set("disabled", false)
	app.Save(record)

	// Deleting the account removes its keys.
	if status, body := call(t, handler, "POST", "/api/india/delete-account", token, map[string]any{"confirm": "someone@else"}); status != http.StatusBadRequest {
		t.Errorf("delete without the right confirmation = %d %v", status, body)
	}
	if status, body := call(t, handler, "POST", "/api/india/delete-account", token, map[string]any{"confirm": email}); status != http.StatusOK {
		t.Fatalf("delete = %d %v", status, body)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&count)
	if count != 0 {
		t.Errorf("%d keys left after the account was deleted", count)
	}
}

func TestPasswordResetMailLinksToOurPage(t *testing.T) {
	handler, _, mails := testAccounts(t)
	call(t, handler, "POST", "/api/collections/users/records", "", map[string]any{"email": "a@example.com", "password": "long enough pw", "passwordConfirm": "long enough pw"})
	call(t, handler, "POST", "/api/collections/users/request-password-reset", "", map[string]any{"email": "a@example.com"})
	if mail := mails.wait(1); !strings.Contains(mail, "https://api.example/account/reset#") {
		t.Fatalf("reset mail: %q", mail)
	}
}

func TestPrepareIsRepeatable(t *testing.T) {
	_, app, _ := testAccounts(t)
	for range 2 {
		if err := Prepare(app, Config{}); err != nil {
			t.Fatal(err)
		}
	}
	if app.Settings().Logs.MaxDays != 0 || app.Settings().Logs.LogIP {
		t.Error("PocketBase's own request log is on; it would hold addresses")
	}
}
