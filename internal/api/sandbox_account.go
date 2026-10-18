package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Accounts for the sandbox services (round 5). Everything else in the API is keyless;
// running strangers' code is not, because free compute is found by miners and spammers
// within days. A caller signs in with GitHub through the device flow, which works from a
// terminal and from an agent (no redirect to catch), and gets a token of ours for the
// sandbox services only. GitHub's own token is used once to learn who the caller is and
// then forgotten. Kept: the GitHub account's numeric ID, login and creation date, our
// tokens' hashes, daily usage, and an audit log of what ran (never the code itself).

const (
	tokenLifetime = 30 * 24 * time.Hour
	// A new GitHub account gets a smaller allowance: throwaway accounts are the cheap way
	// around a per-account quota.
	newAccountAge = 30 * 24 * time.Hour
	auditKeep     = 365 * 24 * time.Hour
)

// quota is what an account may use in one day, India time.
type quota struct {
	Runs      int `json:"runs"`
	CPUMillis int `json:"cpuMs"`
}

var (
	fullQuota = quota{Runs: 200, CPUMillis: 30 * 60 * 1000}
	newQuota  = quota{Runs: 50, CPUMillis: 10 * 60 * 1000}
)

const accountSchema = `
CREATE TABLE IF NOT EXISTS accounts (
	id INTEGER PRIMARY KEY,           -- GitHub's numeric user ID
	login TEXT NOT NULL,
	github_created INTEGER NOT NULL,  -- when the GitHub account was made
	first_seen INTEGER NOT NULL,
	blocked TEXT                      -- why, when an account is blocked
);
CREATE TABLE IF NOT EXISTS tokens (
	hash BLOB PRIMARY KEY,
	account INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	created INTEGER NOT NULL,
	expires INTEGER NOT NULL,
	last_used INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS usage (
	account INTEGER NOT NULL,
	day TEXT NOT NULL,
	runs INTEGER NOT NULL DEFAULT 0,
	cpu_ms INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (account, day)
);
CREATE INDEX IF NOT EXISTS tokens_expires ON tokens (expires);`

type accountStore struct {
	db       *sql.DB
	auditDir string // empty: the audit log goes nowhere (tests without a state directory)
	auditMu  sync.Mutex
}

func openAccountStore(stateDir string) (*accountStore, error) {
	db, err := openStateDB(stateDir, "sandbox", accountSchema)
	if err != nil {
		return nil, err
	}
	store := &accountStore{db: db}
	if stateDir != "" {
		store.auditDir = filepath.Join(stateDir, "audit")
		if err := os.MkdirAll(store.auditDir, 0o700); err != nil {
			db.Close()
			return nil, err
		}
	}
	return store, nil
}

func (a *accountStore) close() { a.db.Close() }

// account is a signed-in caller.
type account struct {
	ID            int64     `json:"id"`
	Login         string    `json:"login"`
	GitHubCreated time.Time `json:"githubCreated"`
	blocked       string
}

func (a account) quota(now time.Time) quota {
	if now.Sub(a.GitHubCreated) < newAccountAge {
		return newQuota
	}
	return fullQuota
}

func indiaDay(now time.Time) string { return now.In(indiaTime).Format("2006-01-02") }

// nextIndiaMidnight is when the day's allowance starts again.
func nextIndiaMidnight(now time.Time) time.Time {
	local := now.In(indiaTime)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, indiaTime)
}

// signIn records a GitHub user and issues a token for them.
func (a *accountStore) signIn(user githubUser, now time.Time) (string, account, error) {
	created, _ := time.Parse(time.RFC3339, user.CreatedAt)
	if created.IsZero() {
		created = now
	}
	_, err := a.db.Exec(`INSERT INTO accounts (id, login, github_created, first_seen) VALUES (?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET login = excluded.login, github_created = excluded.github_created`,
		user.ID, user.Login, created.Unix(), now.Unix())
	if err != nil {
		return "", account{}, err
	}
	token, _ := randomToken()
	token = "isb_" + token // recognisable if it leaks into a repository
	hash := sha256.Sum256([]byte(token))
	if _, err := a.db.Exec(`INSERT INTO tokens (hash, account, created, expires, last_used) VALUES (?, ?, ?, ?, ?)`,
		hash[:], user.ID, now.Unix(), now.Add(tokenLifetime).Unix(), now.Unix()); err != nil {
		return "", account{}, err
	}
	a.db.Exec(`DELETE FROM tokens WHERE expires <= ?`, now.Unix())
	who, err := a.byID(user.ID)
	return token, who, err
}

func (a *accountStore) byID(id int64) (account, error) {
	var who account
	var created int64
	var blocked sql.NullString
	err := a.db.QueryRow(`SELECT id, login, github_created, blocked FROM accounts WHERE id = ?`, id).Scan(&who.ID, &who.Login, &created, &blocked)
	who.GitHubCreated, who.blocked = time.Unix(created, 0).UTC(), blocked.String
	return who, err
}

// byToken finds the account a token belongs to.
func (a *accountStore) byToken(token string, now time.Time) (account, error) {
	sum := sha256.Sum256([]byte(token))
	var id int64
	if err := a.db.QueryRow(`SELECT account FROM tokens WHERE hash = ? AND expires > ?`, sum[:], now.Unix()).Scan(&id); err != nil {
		return account{}, err
	}
	a.db.Exec(`UPDATE tokens SET last_used = ? WHERE hash = ?`, now.Unix(), sum[:])
	return a.byID(id)
}

func (a *accountStore) signOut(token string) {
	sum := sha256.Sum256([]byte(token))
	a.db.Exec(`DELETE FROM tokens WHERE hash = ?`, sum[:])
}

// used is what an account has used today.
func (a *accountStore) used(id int64, now time.Time) quota {
	var used quota
	a.db.QueryRow(`SELECT runs, cpu_ms FROM usage WHERE account = ? AND day = ?`, id, indiaDay(now)).Scan(&used.Runs, &used.CPUMillis)
	return used
}

// charge adds runs and processor time to today's usage.
func (a *accountStore) charge(id int64, runs, cpuMillis int, now time.Time) {
	if _, err := a.db.Exec(`INSERT INTO usage (account, day, runs, cpu_ms) VALUES (?, ?, ?, ?)
		ON CONFLICT (account, day) DO UPDATE SET runs = runs + excluded.runs, cpu_ms = cpu_ms + excluded.cpu_ms`,
		id, indiaDay(now), runs, cpuMillis); err != nil {
		logError("charging a sandbox account", err)
	}
}

func (a *accountStore) block(login, reason string) error {
	result, err := a.db.Exec(`UPDATE accounts SET blocked = ? WHERE login = ?`, nullable(reason), login)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func nullable(text string) any {
	if text == "" {
		return nil
	}
	return text
}

// auditEntry is one line of the audit log: who ran what, never the code itself.
type auditEntry struct {
	Time      string   `json:"time"`
	Account   int64    `json:"account"`
	Login     string   `json:"login"`
	Action    string   `json:"action"` // run, session, exec, tty, db, ...
	ID        string   `json:"id,omitempty"`
	Image     string   `json:"image,omitempty"`
	CodeHash  string   `json:"codeSha256,omitempty"`
	ExitCode  *int     `json:"exitCode,omitempty"`
	CPUMillis int      `json:"cpuMs,omitempty"`
	Limits    []string `json:"limitsHit,omitempty"`
	Outcome   string   `json:"outcome,omitempty"`
}

// audit appends an entry to this month's file and removes files older than a year.
func (a *accountStore) audit(entry auditEntry, now time.Time) {
	if a.auditDir == "" {
		return
	}
	entry.Time = now.UTC().Format(time.RFC3339)
	line, _ := json.Marshal(entry)
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	name := filepath.Join(a.auditDir, now.UTC().Format("2006-01")+".jsonl")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		logError("writing the sandbox audit log", err)
		return
	}
	file.Write(append(line, '\n'))
	file.Close()
}

// pruneAudit deletes monthly files whose every entry is older than a year.
func (a *accountStore) pruneAudit(now time.Time) {
	if a.auditDir == "" {
		return
	}
	entries, _ := os.ReadDir(a.auditDir)
	for _, entry := range entries {
		month, err := time.Parse("2006-01", strings.TrimSuffix(entry.Name(), ".jsonl"))
		if err != nil {
			continue
		}
		if month.AddDate(0, 1, 0).Before(now.Add(-auditKeep)) {
			os.Remove(filepath.Join(a.auditDir, entry.Name()))
		}
	}
}

func codeHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// ---- GitHub device flow ----

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	CreatedAt string `json:"created_at"`
}

// github talks to GitHub for the sign-in. The URLs are fields so tests can point them at
// a stand-in.
type github struct {
	clientID string
	web      string // https://github.com
	api      string // https://api.github.com
	client   *http.Client
}

func (g *github) post(ctx context.Context, path string, form url.Values, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.web+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	response, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("github answered %d", response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(into)
}

type deviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (g *github) start(ctx context.Context) (deviceCode, error) {
	var code deviceCode
	// No scope: the token only needs to read the public profile.
	err := g.post(ctx, "/login/device/code", url.Values{"client_id": {g.clientID}}, &code)
	if err == nil && code.DeviceCode == "" {
		err = errors.New("github returned no device code")
	}
	return code, err
}

// errPending means the person has not finished signing in yet; errSlowDown that the
// caller polls too often.
var (
	errPending  = errors.New("authorization_pending")
	errSlowDown = errors.New("slow_down")
	errDenied   = errors.New("denied or expired")
)

// finish trades a device code for the GitHub user who approved it.
func (g *github) finish(ctx context.Context, device string) (githubUser, error) {
	var answer struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := g.post(ctx, "/login/oauth/access_token", url.Values{
		"client_id": {g.clientID}, "device_code": {device}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
	}, &answer); err != nil {
		return githubUser{}, err
	}
	switch answer.Error {
	case "":
	case "authorization_pending":
		return githubUser{}, errPending
	case "slow_down":
		return githubUser{}, errSlowDown
	default:
		return githubUser{}, errDenied
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.api+"/user", nil)
	if err != nil {
		return githubUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+answer.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	response, err := g.client.Do(req)
	if err != nil {
		return githubUser{}, err
	}
	defer response.Body.Close()
	var user githubUser
	if response.StatusCode != http.StatusOK {
		return user, fmt.Errorf("github /user answered %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&user); err != nil || user.ID == 0 || user.Login == "" {
		return user, errors.New("github returned no user")
	}
	return user, nil
}

// ---- handlers ----

func (s *Server) accounts(w http.ResponseWriter, r *http.Request) *accountStore {
	if s.extras.accounts == nil || s.extras.github == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "Sandbox accounts are not available on this server yet.")
		return nil
	}
	return s.extras.accounts
}

func (s *Server) handleLoginStart(w http.ResponseWriter, r *http.Request) {
	if s.accounts(w, r) == nil {
		return
	}
	info(r).private = true
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	code, err := s.extras.github.start(ctx)
	if err != nil {
		logError("starting a GitHub sign-in", err)
		s.fail(w, r, http.StatusBadGateway, "upstream_error", "GitHub did not answer. Try again in a minute.")
		return
	}
	s.ok(w, r, map[string]any{
		"userCode": code.UserCode, "verificationUri": code.VerificationURI, "deviceCode": code.DeviceCode,
		"interval": max(code.Interval, 5), "expiresIn": code.ExpiresIn,
	}, meta{Note: "Open verificationUri, enter userCode and approve. Then POST {\"deviceCode\": ...} to /v1/sandbox/login/poll every interval seconds until it returns a token."}, cacheNone)
}

func (s *Server) handleLoginPoll(w http.ResponseWriter, r *http.Request) {
	store := s.accounts(w, r)
	if store == nil {
		return
	}
	info(r).private = true
	var body struct {
		DeviceCode string `json:"deviceCode"`
	}
	if !s.readJSON(w, r, &body) {
		return
	}
	if body.DeviceCode == "" || len(body.DeviceCode) > 200 {
		s.badRequest(w, r, "Send the deviceCode from /v1/sandbox/login.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	user, err := s.extras.github.finish(ctx, body.DeviceCode)
	switch {
	case errors.Is(err, errPending):
		s.ok(w, r, map[string]any{"status": "pending"}, meta{Note: "Not approved yet. Poll again after the interval."}, cacheNone)
		return
	case errors.Is(err, errSlowDown):
		s.ok(w, r, map[string]any{"status": "slow_down"}, meta{Note: "Polling too often: wait 5 seconds longer between polls."}, cacheNone)
		return
	case errors.Is(err, errDenied):
		s.fail(w, r, http.StatusForbidden, "denied", "The sign-in was refused or the code expired. Start again.")
		return
	case err != nil:
		logError("finishing a GitHub sign-in", err)
		s.fail(w, r, http.StatusBadGateway, "upstream_error", "GitHub did not answer. Try again.")
		return
	}
	now := time.Now()
	token, who, err := store.signIn(user, now)
	if err != nil {
		logError("signing in", err)
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	store.audit(auditEntry{Account: who.ID, Login: who.Login, Action: "login"}, now)
	s.ok(w, r, map[string]any{
		"status": "ok", "token": token, "expiresAt": now.Add(tokenLifetime).UTC().Format(time.RFC3339),
		"account": s.accountView(store, who, now),
	}, meta{Note: "Send the token as Authorization: Bearer <token> to the sandbox services. It is shown only now and lasts 30 days."}, cacheNone)
}

func (s *Server) accountView(store *accountStore, who account, now time.Time) map[string]any {
	limit, used := who.quota(now), store.used(who.ID, now)
	return map[string]any{
		"login": who.Login, "quota": limit, "used": used,
		"left":   quota{Runs: max(limit.Runs-used.Runs, 0), CPUMillis: max(limit.CPUMillis-used.CPUMillis, 0)},
		"resets": nextIndiaMidnight(now).UTC().Format(time.RFC3339),
	}
}

// signedIn returns the caller's account, or answers 401 or 403 and returns false.
func (s *Server) signedIn(w http.ResponseWriter, r *http.Request) (*accountStore, account, bool) {
	store := s.accounts(w, r)
	if store == nil {
		return nil, account{}, false
	}
	info(r).private = true
	token := bearer(r)
	if token == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="sandbox"`)
		s.fail(w, r, http.StatusUnauthorized, "unauthorized", "The sandbox services need a token: sign in with GitHub at POST /v1/sandbox/login.")
		return nil, account{}, false
	}
	who, err := store.byToken(token, time.Now())
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="sandbox", error="invalid_token"`)
		s.fail(w, r, http.StatusUnauthorized, "unauthorized", "The token is not valid or has expired. Sign in again at POST /v1/sandbox/login.")
		return nil, account{}, false
	}
	if who.blocked != "" {
		s.fail(w, r, http.StatusForbidden, "blocked", "This account may not use the sandbox services: "+who.blocked)
		return nil, account{}, false
	}
	return store, who, true
}

// withinQuota answers 429 and reports false when an account has used its day.
func (s *Server) withinQuota(w http.ResponseWriter, r *http.Request, store *accountStore, who account) bool {
	now := time.Now()
	limit, used := who.quota(now), store.used(who.ID, now)
	if used.Runs < limit.Runs && used.CPUMillis < limit.CPUMillis {
		return true
	}
	s.metrics.sandboxRefused("quota")
	w.Header().Set("Retry-After", strconv.Itoa(int(nextIndiaMidnight(now).Sub(now).Seconds())))
	s.fail(w, r, http.StatusTooManyRequests, "quota_exceeded",
		fmt.Sprintf("Today's allowance is used: %d runs and %d CPU-minutes a day, reset at midnight India time.", limit.Runs, limit.CPUMillis/60000))
	return false
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	store, who, ok := s.signedIn(w, r)
	if !ok {
		return
	}
	s.ok(w, r, s.accountView(store, who, time.Now()), meta{}, cacheNone)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	store, _, ok := s.signedIn(w, r)
	if !ok {
		return
	}
	store.signOut(bearer(r))
	s.ok(w, r, map[string]any{"signedOut": true}, meta{}, cacheNone)
}
