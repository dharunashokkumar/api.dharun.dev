package api

// Free API keys (growth plan Part 2, Dharun 2026-11-10 and 2026-11-11).
//
// Light endpoints keep working without a key, charged per address as before. A key,
// sent as "Authorization: Bearer india_live_..." or "X-API-Key: ...", is charged per
// key with a larger allowance, so apps behind one shared address stop sharing one
// allowance. The model endpoints (marked Key in the catalog) will need a key from the
// date set in KEY_REQUIRED_FROM; until then they answer as before and say so in the
// X-Key-Notice header. With a key they count against a daily quota and run one at a
// time per key.
//
// Checking a key costs no network call: the table of key hashes is read from the
// accounts database (read-only) every 15 seconds, with one direct lookup when an
// unknown key arrives, so a revoked key stops working within that time.

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/apikey"
)

const (
	defaultKeyRateLimit = 3000
	defaultModelQuota   = 500
	defaultSiteAllow    = 10
	keyReloadEvery      = 15 * time.Second
	// keyMissMemory is how long an unknown key is remembered as unknown, so a script
	// sending made-up keys cannot make the API query the accounts database every time.
	keyMissMemory = time.Minute
	usageKeepDays = 62
)

// keyEntry is one row of the key table.
type keyEntry struct {
	ID, User, Tier string
	Usable         bool
}

// keyring holds the table of keys, read from the accounts database.
type keyring struct {
	path   string
	mu     sync.Mutex
	db     *sql.DB
	table  atomic.Pointer[map[string]keyEntry] // by hash
	loaded atomic.Bool
	misses map[string]time.Time
	stop   chan struct{}
}

func newKeyring(path string) *keyring {
	k := &keyring{path: path, misses: map[string]time.Time{}, stop: make(chan struct{})}
	empty := map[string]keyEntry{}
	k.table.Store(&empty)
	if path == "" {
		return k
	}
	k.reload()
	go func() {
		ticker := time.NewTicker(keyReloadEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				k.reload()
			case <-k.stop:
				return
			}
		}
	}()
	return k
}

func (k *keyring) open() *sql.DB {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.db != nil {
		return k.db
	}
	if _, err := os.Stat(k.path); err != nil {
		return nil
	}
	db, err := sql.Open("sqlite", "file:"+k.path+"?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(2000)")
	if err != nil {
		logError("opening the accounts database", err)
		return nil
	}
	db.SetMaxOpenConns(2)
	k.db = db
	return db
}

// reload reads the whole table again.
func (k *keyring) reload() {
	db := k.open()
	if db == nil {
		return
	}
	rows, err := db.Query(apikey.TableQuery)
	if err != nil {
		logError("reading the key table", err)
		return
	}
	defer rows.Close()
	table := map[string]keyEntry{}
	for rows.Next() {
		var entry keyEntry
		var hash string
		var usable int
		if rows.Scan(&entry.ID, &hash, &entry.User, &entry.Tier, &usable) == nil {
			entry.Usable = usable == 1
			table[hash] = entry
		}
	}
	if rows.Err() != nil {
		return
	}
	k.table.Store(&table)
	k.loaded.Store(true)
	k.mu.Lock()
	k.misses = map[string]time.Time{}
	k.mu.Unlock()
}

// find looks a key's hash up: in the table, then once in the database for a key made
// since the last reload.
func (k *keyring) find(hash string) (keyEntry, bool) {
	if entry, ok := (*k.table.Load())[hash]; ok {
		return entry, true
	}
	k.mu.Lock()
	missed, seen := k.misses[hash]
	k.mu.Unlock()
	if seen && time.Since(missed) < keyMissMemory {
		return keyEntry{}, false
	}
	db := k.open()
	if db == nil {
		return keyEntry{}, false
	}
	var entry keyEntry
	var stored string
	var usable int
	if err := db.QueryRow(apikey.LookupQuery, hash).Scan(&entry.ID, &stored, &entry.User, &entry.Tier, &usable); err != nil {
		k.mu.Lock()
		if len(k.misses) > 10000 {
			k.misses = map[string]time.Time{}
		}
		k.misses[hash] = time.Now()
		k.mu.Unlock()
		return keyEntry{}, false
	}
	entry.Usable = usable == 1
	return entry, true
}

// known reports whether a key id is still in the table (for dropping usage counts of
// deleted keys). Before the first successful read nothing is dropped.
func (k *keyring) known() (map[string]bool, bool) {
	if !k.loaded.Load() {
		return nil, false
	}
	ids := map[string]bool{}
	for _, entry := range *k.table.Load() {
		ids[entry.ID] = true
	}
	return ids, true
}

func (k *keyring) close() {
	close(k.stop)
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.db != nil {
		k.db.Close()
	}
}

// credential is what a request brought to identify itself.
type credential struct {
	kind  string // "" (none), "key" or "site" (a site token from a tool page)
	keyID string
	user  string
	tier  string
	site  string // the address hash a site token is tied to
}

// identify reads a key or a site token from the request. A value that is plainly meant
// as one of our keys but is wrong answers 401; other bearer tokens a client happens to
// send are ignored.
func (s *Server) identify(r *http.Request) (credential, string) {
	value := ""
	if auth := r.Header.Get("Authorization"); auth != "" {
		if scheme, token, ok := strings.Cut(auth, " "); ok && strings.EqualFold(scheme, "Bearer") && apikey.Looks(strings.TrimSpace(token)) {
			value = strings.TrimSpace(token)
		}
	}
	if header := strings.TrimSpace(r.Header.Get("X-API-Key")); header != "" {
		value = header
	}
	if value != "" {
		if apikey.Check(value) != nil {
			return credential{}, "This is not a well-formed India API key. Copy it again from " + s.origin(r) + "/account."
		}
		entry, ok := s.keys.find(apikey.Hash(value))
		if !ok || !entry.Usable {
			return credential{}, "This key is not known, was revoked, or its account is not confirmed yet. See " + s.origin(r) + "/account."
		}
		return credential{kind: "key", keyID: entry.ID, user: entry.User, tier: entry.Tier}, ""
	}
	if token := strings.TrimSpace(r.Header.Get("X-Site-Token")); token != "" {
		if site, ok := s.checkSiteToken(token, s.clientKey(r)); ok {
			return credential{kind: "site", site: site}, ""
		}
	}
	return credential{}, ""
}

// keyRequired reports whether keyless calls to the key routes are refused now.
func (s *Server) keyRequired() bool {
	return !s.cfg.KeyRequiredFrom.IsZero() && !keyClock().Before(s.cfg.KeyRequiredFrom)
}

// keyClock tells the time for the notice date and the quotas; tests move it.
var keyClock = time.Now

// keyNotice is the text of the X-Key-Notice header on keyless calls to key routes.
func (s *Server) keyNotice(r *http.Request) string {
	if s.cfg.KeyRequiredFrom.IsZero() {
		return "This endpoint will need a free API key; the date will be announced here. Get a key at " + s.origin(r) + "/signup"
	}
	return "From " + s.cfg.KeyRequiredFrom.Format("2006-01-02") + " this endpoint needs a free API key. Get one at " + s.origin(r) + "/signup"
}

// keyGate decides a request to a key route: a key within its quota, a site token
// within its small allowance, or (during the notice period) no key at all. It answers
// the refusal itself; otherwise it returns the function that ends the request's turn.
func (s *Server) keyGate(w http.ResponseWriter, r *http.Request) (func(), bool) {
	details := info(r)
	details.keyRoute = true
	cred := details.cred
	// The documentation's own samples are drawn in memory at build time.
	if details.internal && details.client == docsSampleKey {
		return func() {}, true
	}
	switch cred.kind {
	case "key":
		used, ok := s.usage.takeModel(cred.keyID, s.cfg.ModelQuotaPerDay)
		w.Header().Set("X-Quota-Limit", strconv.Itoa(s.cfg.ModelQuotaPerDay))
		w.Header().Set("X-Quota-Remaining", strconv.Itoa(max(s.cfg.ModelQuotaPerDay-used, 0)))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(secondsToMidnight()))
			s.fail(w, r, http.StatusTooManyRequests, "quota_exceeded",
				"This key has used its "+strconv.Itoa(s.cfg.ModelQuotaPerDay)+" model requests for today (India time). The quota resets at midnight IST.")
			return nil, false
		}
		// One model request at a time per key: the next waits for the one before.
		turn := s.keyTurns.lane(cred.keyID)
		release, ok := s.takeSlot(w, r, turn, heavyWait)
		if !ok {
			return nil, false
		}
		return release, true
	case "site":
		if s.keyRequired() {
			if !s.usage.takeSite(cred.site, s.cfg.SiteTokenAllowance) {
				s.fail(w, r, http.StatusTooManyRequests, "key_required",
					"The free tool pages allow "+strconv.Itoa(s.cfg.SiteTokenAllowance)+" of these a day. Sign up for a free key for more: "+s.origin(r)+"/signup")
				return nil, false
			}
		}
		return func() {}, true
	}
	if s.keyRequired() {
		w.Header().Set("WWW-Authenticate", `Bearer realm="India API"`)
		s.fail(w, r, http.StatusUnauthorized, "key_required",
			"This endpoint needs a free API key, sent as Authorization: Bearer <key> or X-API-Key. Get one at "+s.origin(r)+"/signup")
		return nil, false
	}
	w.Header().Set("X-Key-Notice", s.keyNotice(r))
	return func() {}, true
}

func secondsToMidnight() int {
	now := keyClock().In(indiaTime)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, indiaTime)
	return int(midnight.Sub(now).Seconds()) + 1
}

// keyTurns gives each key one lane for model requests.
type keyTurns struct {
	mu    sync.Mutex
	lanes map[string]chan struct{}
}

func (t *keyTurns) lane(id string) chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lanes == nil || len(t.lanes) > 50000 {
		t.lanes = map[string]chan struct{}{}
	}
	lane := t.lanes[id]
	if lane == nil {
		lane = make(chan struct{}, 1)
		t.lanes[id] = lane
	}
	return lane
}

// keyUsage counts requests per key, day and API group, and the model requests that
// the daily quota covers. It is kept in STATE_DIR like the visitor counts, never in the
// request log. Site token use is counted per address hash and day, in memory only.
type keyUsage struct {
	mu     sync.Mutex
	path   string
	dirty  bool
	Groups map[string]map[string]map[string]int `json:"groups"` // key, day, group
	Models map[string]map[string]int            `json:"models"` // key, day
	site   map[string]int                       // day|address hash
	day    string
}

func newKeyUsage(stateDir string) *keyUsage {
	u := &keyUsage{Groups: map[string]map[string]map[string]int{}, Models: map[string]map[string]int{}, site: map[string]int{}}
	if stateDir != "" {
		u.path = filepath.Join(stateDir, "key-usage.json")
		if raw, err := os.ReadFile(u.path); err == nil {
			json.Unmarshal(raw, u)
		}
		if u.Groups == nil {
			u.Groups = map[string]map[string]map[string]int{}
		}
		if u.Models == nil {
			u.Models = map[string]map[string]int{}
		}
	}
	return u
}

func today() string { return keyClock().In(indiaTime).Format("2006-01-02") }

// count adds one request by a key to its group.
func (u *keyUsage) count(id, group string) {
	if id == "" || group == "" {
		return
	}
	day := today()
	u.mu.Lock()
	defer u.mu.Unlock()
	days := u.Groups[id]
	if days == nil {
		days = map[string]map[string]int{}
		u.Groups[id] = days
	}
	if days[day] == nil {
		days[day] = map[string]int{}
	}
	days[day][group]++
	u.dirty = true
}

// takeModel counts a model request against the key's daily quota, unless it is used up.
func (u *keyUsage) takeModel(id string, quota int) (int, bool) {
	day := today()
	u.mu.Lock()
	defer u.mu.Unlock()
	days := u.Models[id]
	if days == nil {
		days = map[string]int{}
		u.Models[id] = days
	}
	if days[day] >= quota {
		return days[day], false
	}
	days[day]++
	u.dirty = true
	return days[day], true
}

func (u *keyUsage) takeSite(site string, allowance int) bool {
	day := today()
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.day != day {
		u.site, u.day = map[string]int{}, day
	}
	if u.site[site] >= allowance {
		return false
	}
	u.site[site]++
	return true
}

// report is what the account page shows for one key.
type usageReport struct {
	Today      map[string]int `json:"today"`
	Month      map[string]int `json:"month"`
	ModelsUsed int            `json:"modelRequestsToday"`
	ModelQuota int            `json:"modelQuota"`
}

func (u *keyUsage) report(id string, quota int) usageReport {
	day := today()
	month := day[:7]
	u.mu.Lock()
	defer u.mu.Unlock()
	out := usageReport{Today: map[string]int{}, Month: map[string]int{}, ModelQuota: quota, ModelsUsed: u.Models[id][day]}
	for d, groups := range u.Groups[id] {
		for group, n := range groups {
			if d == day {
				out.Today[group] += n
			}
			if strings.HasPrefix(d, month) {
				out.Month[group] += n
			}
		}
	}
	return out
}

// prune drops old days, and keys the accounts database no longer holds (a deleted
// account takes its usage counts with it).
func (u *keyUsage) prune(known map[string]bool, haveTable bool) {
	oldest := keyClock().In(indiaTime).AddDate(0, 0, -usageKeepDays).Format("2006-01-02")
	u.mu.Lock()
	defer u.mu.Unlock()
	for id, days := range u.Groups {
		if haveTable && !known[id] {
			delete(u.Groups, id)
			delete(u.Models, id)
			u.dirty = true
			continue
		}
		for d := range days {
			if d < oldest {
				delete(days, d)
				u.dirty = true
			}
		}
	}
	for id, days := range u.Models {
		if haveTable && !known[id] {
			delete(u.Models, id)
			u.dirty = true
			continue
		}
		for d := range days {
			if d < oldest {
				delete(days, d)
			}
		}
	}
}

func (u *keyUsage) save() {
	u.mu.Lock()
	if u.path == "" || !u.dirty {
		u.mu.Unlock()
		return
	}
	encoded, _ := json.Marshal(u)
	u.dirty = false
	u.mu.Unlock()
	if err := writeState(u.path, encoded, 0o600); err != nil {
		logError("saving key usage", err)
	}
}

// ---- site tokens: the free tool pages ----

// A site token lets a visitor of a tool page use a key route a few times a day without
// signing up (Dharun, 2026-11-11). It is signed, lasts an hour and is tied to the
// address it was given to (by a keyed hash: the address itself is not in it).
const siteTokenLife = time.Hour

func (s *Server) siteHash(client string) string {
	mac := hmac.New(sha256.New, s.siteSecret)
	mac.Write([]byte("site|" + client))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func (s *Server) newSiteToken(client string) (string, time.Time) {
	expires := keyClock().Add(siteTokenLife).UTC().Truncate(time.Second)
	body := strconv.FormatInt(expires.Unix(), 10) + "." + s.siteHash(client)
	return body + "." + s.sign(body), expires
}

func (s *Server) checkSiteToken(token, client string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	body := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(s.sign(body)), []byte(parts[2])) {
		return "", false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || keyClock().Unix() > expires || parts[1] != s.siteHash(client) {
		return "", false
	}
	return parts[1], true
}

func (s *Server) sign(body string) string {
	mac := hmac.New(sha256.New, s.siteSecret)
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// keyPaths holds the catalog paths of the endpoints that will need a key.
var keyPaths = func() map[string]bool {
	paths := map[string]bool{}
	for _, group := range catalog {
		for _, e := range group.Endpoints {
			if e.Key {
				paths[e.Path] = true
			}
		}
	}
	return paths
}()

// groupOfPath names the API group each catalog path belongs to, for usage counts.
var groupOfPath = func() map[string]string {
	groups := map[string]string{}
	for _, group := range catalog {
		for _, e := range group.Endpoints {
			groups[e.Path] = group.ID
		}
	}
	return groups
}()
