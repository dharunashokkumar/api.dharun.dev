package api

// The account addresses (growth plan Part 2). The pages (/signup, /login, /account)
// are ours; the account system behind them is PocketBase (cmd/accounts), which listens
// on localhost only. The API passes through the few addresses the pages use and never
// PocketBase's admin screen or its record API.
//
// Login and signup are one step (Dharun, 2026-11-11): an email address gets a six-digit
// code by mail (Amazon SES), and the code logs in. Asking for a code goes through the
// API, which checks the browser challenge, the address and the daily caps, makes the
// account if there is none yet, and only then asks PocketBase to mail the code.

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"
)

// costAccount is what an account address charges: enough that guessing passwords is
// slow (50 tries a minute per address), with no CPU slot taken.
const costAccount = 12

// accountPassThrough lists what the pages may reach in the account program, by method
// and path below /account/api/. Everything else answers 404.
var accountPassThrough = []struct{ method, path string }{
	{"POST", "collections/users/auth-with-otp"},
	{"POST", "collections/users/auth-refresh"},
	{"GET", "india/me"},
	{"POST", "india/keys"},
	{"POST", "india/keys/*/revoke"},
	{"POST", "india/keys/*/rotate"},
	{"POST", "india/logout-all"},
	{"POST", "india/delete-account"},
}

func passThroughAllowed(method, path string) bool {
	parts := strings.Split(path, "/")
	for _, allowed := range accountPassThrough {
		if allowed.method != method {
			continue
		}
		want := strings.Split(allowed.path, "/")
		if len(want) != len(parts) {
			continue
		}
		match := true
		for i := range want {
			if want[i] == "*" {
				if !idLike(parts[i]) {
					match = false
				}
			} else if want[i] != parts[i] {
				match = false
			}
		}
		if match {
			return true
		}
	}
	return false
}

// idLike accepts a PocketBase record id: 15 lower-case letters and digits.
func idLike(value string) bool {
	if len(value) < 5 || len(value) > 32 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// accountPaths are kept out of the log entirely: they carry tokens and addresses.
var accountPaths = []string{"/account/api/{rest...}", "/account/api/code", "/account/api/challenge", "/account/api/site-token", "/account/api/usage",
	"/signup", "/login", "/account"}

func init() {
	for _, path := range accountPaths {
		privatePaths[path] = true
	}
}

func (s *Server) registerAccounts() {
	s.handle("account", costAccount, "GET /account/api/{rest...}", s.handleAccountPassThrough)
	s.handle("account", costAccount, "POST /account/api/{rest...}", s.handleAccountPassThrough)
	s.handle("account", costAccount, "POST /account/api/code", s.handleCode)
	s.handle("account", costLight, "GET /account/api/challenge", s.handleChallenge)
	s.handle("account", costLight, "POST /account/api/site-token", s.handleSiteToken)
	s.handle("account", costLight, "GET /account/api/usage", s.handleUsage)
	for _, path := range []string{"/signup", "/signup.md", "/login", "/account"} {
		s.handle("account_page", costFree, "GET "+path, s.handleAccountPage)
	}
	s.handle("docs", costFree, "GET /assets/{file}", s.handleAsset)
}

// accountHeaders: the account addresses answer only this site's own pages.
func (s *Server) accountHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", s.origin(r))
	w.Header().Set("Cache-Control", "no-store")
}

func (s *Server) handleAccountPassThrough(w http.ResponseWriter, r *http.Request) {
	s.accountHeaders(w, r)
	rest := r.PathValue("rest")
	if !passThroughAllowed(r.Method, rest) {
		s.notFound(w, r, "Not found.")
		return
	}
	status, body, kind, err := s.accountCall(r, r.Method, "/api/"+rest, r.Body, r.Header.Get("Authorization"))
	if err != nil {
		s.accountsDown(w, r)
		return
	}
	writeBody(w, r, status, body, kind, cacheNone, false)
}

func (s *Server) accountsDown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "60")
	s.fail(w, r, http.StatusServiceUnavailable, "accounts_unavailable", "Accounts are not available on this server right now. Try again later.")
}

// accountClient talks to the account program; short timeouts, never redirects.
var accountClient = &http.Client{
	Timeout:       10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// accountCall sends one request to the account program: only the body (at most 16 KB),
// its type and the login token, never the visitor's address or other headers.
func (s *Server) accountCall(r *http.Request, method, path string, body io.Reader, auth string) (int, []byte, string, error) {
	if s.cfg.AccountsURL == "" {
		return 0, nil, "", errNoAccounts
	}
	var payload []byte
	if body != nil {
		payload, _ = io.ReadAll(io.LimitReader(body, 16<<10))
	}
	req, err := http.NewRequestWithContext(r.Context(), method, strings.TrimRight(s.cfg.AccountsURL, "/")+path, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	response, err := accountClient.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	kind := response.Header.Get("Content-Type")
	if kind == "" {
		kind = typeJSON
	}
	return response.StatusCode, raw, kind, err
}

type accountError string

func (e accountError) Error() string { return string(e) }

const errNoAccounts = accountError("no accounts on this server")

// ---- login codes ----

// codesPerEmail caps the codes one address can be sent a day, so the form cannot be
// used to flood someone's inbox.
const codesPerEmail = 10

// handleCode sends a login code. An address with no account gets one made first, within
// the daily cap of new accounts per caller address. The answer is the same whether the
// account existed or not, so the form cannot tell anyone who has an account.
func (s *Server) handleCode(w http.ResponseWriter, r *http.Request) {
	s.accountHeaders(w, r)
	var body struct {
		Email  string `json:"email"`
		Altcha string `json:"altcha"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&body); err != nil {
		s.badRequest(w, r, "Send email and altcha as JSON.")
		return
	}
	if !s.challenges.verify(s.siteSecret, body.Altcha) {
		s.fail(w, r, http.StatusBadRequest, "challenge_failed", "The check that this is a browser did not pass. Reload the page and try again.")
		return
	}
	address, err := mail.ParseAddress(strings.TrimSpace(body.Email))
	if err != nil || address.Name != "" || !strings.Contains(address.Address, "@") || len(address.Address) > 254 {
		s.badRequest(w, r, "That does not look like an email address.")
		return
	}
	email := strings.ToLower(address.Address)
	if s.disposableDomain(email[strings.LastIndex(email, "@")+1:]) {
		s.badRequest(w, r, "Throwaway email addresses cannot be used. Use an address you keep.")
		return
	}
	if !s.codeMails.take(s.siteHash("mail|"+email), codesPerEmail) {
		s.fail(w, r, http.StatusTooManyRequests, "rate_limited", "Too many codes for this address today. Try again tomorrow.")
		return
	}
	// Make the account if the cap allows; an existing address answers 400 here and is
	// simply sent its code.
	caller := s.siteHash(s.clientKey(r))
	if s.signups.left(caller, s.cfg.SignupsPerDay) {
		random := make([]byte, 24)
		rand.Read(random)
		password := base64.RawURLEncoding.EncodeToString(random)
		create, _ := json.Marshal(map[string]string{"email": email, "password": password, "passwordConfirm": password})
		status, _, _, err := s.accountCall(r, http.MethodPost, "/api/collections/users/records", bytes.NewReader(create), "")
		if err != nil {
			s.accountsDown(w, r)
			return
		}
		if status == http.StatusOK {
			s.signups.take(caller, s.cfg.SignupsPerDay)
		}
	}
	request, _ := json.Marshal(map[string]string{"email": email})
	status, raw, _, err := s.accountCall(r, http.MethodPost, "/api/collections/users/request-otp", bytes.NewReader(request), "")
	if err != nil {
		s.accountsDown(w, r)
		return
	}
	var answer struct {
		OTPID string `json:"otpId"`
	}
	if status != http.StatusOK || json.Unmarshal(raw, &answer) != nil || answer.OTPID == "" {
		s.fail(w, r, http.StatusServiceUnavailable, "accounts_unavailable", "The code could not be sent right now. Try again in a minute.")
		return
	}
	s.ok(w, r, map[string]any{"otpId": answer.OTPID,
		"message": "If the address can be used, a six-digit code is on its way. It works once, for 10 minutes."}, meta{}, cacheNone)
}

// disposableDomain checks the throwaway-domain list the email check uses.
func (s *Server) disposableDomain(domain string) bool {
	set := s.data("disposable-email")
	if set == nil {
		return false
	}
	labels := strings.Split(domain, ".")
	for i := 0; i < len(labels)-1; i++ {
		var one int
		if set.row(`SELECT 1 FROM domains WHERE domain = ?`, strings.Join(labels[i:], ".")).Scan(&one) == nil {
			return true
		}
	}
	return false
}

// ---- the bot check: a small proof of work (the ALTCHA scheme) ----

// A challenge asks the browser to find a number n such that SHA-256(salt + n) is the
// given hash. It takes a browser about a second, a script that wants a thousand
// accounts a thousand seconds; nothing is loaded from another host. The challenge is
// signed, expires in ten minutes and is good once.
const (
	challengeMax  = 60000
	challengeLife = 10 * time.Minute
)

type challenges struct {
	mu   sync.Mutex
	used map[string]time.Time
}

func newChallenges() *challenges { return &challenges{used: map[string]time.Time{}} }

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	s.accountHeaders(w, r)
	random := make([]byte, 12)
	rand.Read(random)
	salt := hex.EncodeToString(random) + "?expires=" + strconv.FormatInt(keyClock().Add(challengeLife).Unix(), 10)
	var n [2]byte
	rand.Read(n[:])
	number := (int(n[0])<<8 | int(n[1])) % challengeMax
	sum := sha256.Sum256([]byte(salt + strconv.Itoa(number)))
	challenge := hex.EncodeToString(sum[:])
	mac := hmac.New(sha256.New, s.siteSecret)
	mac.Write([]byte(challenge))
	writeJSON(w, r, map[string]any{"algorithm": "SHA-256", "challenge": challenge, "salt": salt, "maxnumber": challengeMax,
		"signature": hex.EncodeToString(mac.Sum(nil))})
}

// verify checks a solved challenge: base64 JSON {algorithm, challenge, number, salt, signature}.
func (c *challenges) verify(secret []byte, payload string) bool {
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return false
	}
	var solved struct {
		Algorithm string `json:"algorithm"`
		Challenge string `json:"challenge"`
		Number    int    `json:"number"`
		Salt      string `json:"salt"`
		Signature string `json:"signature"`
	}
	if json.Unmarshal(raw, &solved) != nil || solved.Algorithm != "SHA-256" {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(solved.Challenge))
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(solved.Signature)) {
		return false
	}
	sum := sha256.Sum256([]byte(solved.Salt + strconv.Itoa(solved.Number)))
	if hex.EncodeToString(sum[:]) != solved.Challenge {
		return false
	}
	_, expiry, _ := strings.Cut(solved.Salt, "?expires=")
	expires, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || keyClock().Unix() > expires {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := keyClock()
	for challenge, until := range c.used {
		if now.After(until) {
			delete(c.used, challenge)
		}
	}
	if _, seen := c.used[solved.Challenge]; seen {
		return false
	}
	c.used[solved.Challenge] = time.Unix(expires, 0)
	return true
}

// dailyCounter counts something per hashed address and day, in memory.
type dailyCounter struct {
	mu     sync.Mutex
	day    string
	counts map[string]int
}

// left reports whether who has any of the day's allowance left, without using it.
func (d *dailyCounter) left(who string, limit int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if day := today(); d.day != day {
		d.day, d.counts = day, map[string]int{}
	}
	return d.counts[who] < limit
}

func (d *dailyCounter) take(who string, limit int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if day := today(); d.day != day {
		d.day, d.counts = day, map[string]int{}
	}
	if d.counts[who] >= limit {
		return false
	}
	d.counts[who]++
	return true
}

// ---- site tokens for the tool pages ----

func (s *Server) handleSiteToken(w http.ResponseWriter, r *http.Request) {
	s.accountHeaders(w, r)
	var body struct {
		Altcha string `json:"altcha"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&body); err != nil || !s.challenges.verify(s.siteSecret, body.Altcha) {
		s.fail(w, r, http.StatusBadRequest, "challenge_failed", "The check that this is a browser did not pass. Reload the page.")
		return
	}
	token, expires := s.newSiteToken(s.clientKey(r))
	s.ok(w, r, map[string]any{"token": token, "expires": expires.Format(time.RFC3339), "dailyAllowance": s.cfg.SiteTokenAllowance,
		"note": "Send it as X-Site-Token. Once keys are required it allows a few model requests a day from this address; a free key allows " + strconv.Itoa(s.cfg.ModelQuotaPerDay) + "."}, meta{}, cacheNone)
}

// ---- usage for the account page ----

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	s.accountHeaders(w, r)
	status, raw, _, err := s.accountCall(r, http.MethodGet, "/api/india/me", nil, r.Header.Get("Authorization"))
	if err != nil {
		s.accountsDown(w, r)
		return
	}
	if status != http.StatusOK {
		s.fail(w, r, http.StatusUnauthorized, "not_logged_in", "Log in again.")
		return
	}
	var me struct {
		Keys []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Revoked bool   `json:"revoked"`
		} `json:"keys"`
	}
	json.Unmarshal(raw, &me)
	type keyUse struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		usageReport
	}
	out := []keyUse{}
	for _, key := range me.Keys {
		if !key.Revoked {
			out = append(out, keyUse{ID: key.ID, Name: key.Name, usageReport: s.usage.report(key.ID, s.cfg.ModelQuotaPerDay)})
		}
	}
	s.ok(w, r, map[string]any{"keys": out, "perMinute": s.cfg.KeyRateLimitPerMinute, "day": today()}, meta{}, cacheNone)
}

// keepUsage saves the usage counts every minute and drops those of deleted keys.
func (s *Server) keepUsage() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			known, ok := s.keys.known()
			s.usage.prune(known, ok)
			s.usage.save()
		case <-s.stopKeys:
			return
		}
	}
}

func randomSecret() []byte {
	secret := make([]byte, 32)
	rand.Read(secret)
	return secret
}

// writeJSON sends a plain JSON object (not the envelope): the bot check's challenge is
// read by code that expects this exact shape.
func writeJSON(w http.ResponseWriter, r *http.Request, value any) {
	encoded, _ := json.Marshal(value)
	writeBody(w, r, http.StatusOK, encoded, typeJSON, cacheNone, false)
}
