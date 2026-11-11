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
	t.Cleanup(func() {
		// PocketBase finishes some work after it answers (sending a login code runs in
		// the background); shutting it down under that work is a data race.
		time.Sleep(500 * time.Millisecond)
		app.ResetBootstrapState()
	})
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
	// Passwords do not log in: the account's password is a random one nobody knows.
	if status, _ := call(t, handler, "POST", "/api/collections/users/auth-with-password", "", map[string]any{"identity": email, "password": password}); status == http.StatusOK {
		t.Error("password login works; only codes should")
	}
	// A wrong code does not log in; the mailed one does, and confirms the address.
	otpID := requestCode(t, handler, email)
	code := codeFrom(t, mails.wait(1))
	if status, _ := call(t, handler, "POST", "/api/collections/users/auth-with-otp", "", map[string]any{"otpId": otpID, "password": "000000"}); status == http.StatusOK && code != "000000" {
		t.Error("a wrong code logged in")
	}
	token := loginWith(t, handler, otpID, code)
	if record, _ := app.FindAuthRecordByEmail("users", email); !record.Verified() {
		t.Error("a code login did not mark the address verified")
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
	otpID = requestCode(t, handler, email)
	if status, _ := call(t, handler, "POST", "/api/collections/users/auth-with-otp", "", map[string]any{"otpId": otpID, "password": codeFrom(t, mails.wait(2))}); status == http.StatusOK {
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

// requestCode asks for a login code, as the API's /account/api/code does.
func requestCode(t *testing.T, handler http.Handler, email string) string {
	t.Helper()
	status, body := call(t, handler, "POST", "/api/collections/users/request-otp", "", map[string]any{"email": email})
	otpID, _ := body["otpId"].(string)
	if status != http.StatusOK || otpID == "" {
		t.Fatalf("request code = %d %v", status, body)
	}
	return otpID
}

func codeFrom(t *testing.T, mail string) string {
	t.Helper()
	code := regexp.MustCompile(`>(\d{6})<`).FindStringSubmatch(mail)
	if code == nil {
		t.Fatalf("no six-digit code in the mail: %q", mail)
	}
	return code[1]
}

func loginWith(t *testing.T, handler http.Handler, otpID, code string) string {
	t.Helper()
	_, auth := call(t, handler, "POST", "/api/collections/users/auth-with-otp", "", map[string]any{"otpId": otpID, "password": code})
	token, _ := auth["token"].(string)
	if token == "" {
		t.Fatalf("code login gave no token: %v", auth)
	}
	return token
}

func TestCodeMailAndUnknownAddress(t *testing.T) {
	handler, _, mails := testAccounts(t)
	// An address with no account gets an id back (so nobody can tell who has one) and no mail.
	requestCode(t, handler, "nobody@example.com")
	if mail := mails.wait(1); mail != "" {
		t.Errorf("mail sent to an address with no account: %q", mail)
	}
	call(t, handler, "POST", "/api/collections/users/records", "", map[string]any{"email": "a@example.com", "password": "random long enough", "passwordConfirm": "random long enough"})
	requestCode(t, handler, "a@example.com")
	if mail := mails.wait(1); !strings.Contains(mail, "India API login code") || !strings.Contains(mail, "10 minutes") {
		t.Fatalf("code mail: %q", mail)
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
