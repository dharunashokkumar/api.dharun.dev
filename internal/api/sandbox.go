package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// The sandbox services (round 5): code runs, sessions with a terminal, distributions,
// throwaway Postgres and the browser tools. The work happens in microVMs in a separate
// VM; this side checks who is asking, their quota and the target, then calls the
// controller over the private link (sandbox.Client). Everything here is private: code,
// commands, files and URLs are never logged, and the audit log keeps only a hash of
// what ran.

// costRemote is a request whose work runs on the sandbox VM, not here. The account's
// daily quota is the real limit; this keeps one address from hammering the endpoints.
const costRemote = 5

// sandboxService is the API's side of the sandbox VM.
type sandboxService struct {
	client    *sandbox.Client
	pgAddr    string // the VM's Postgres, host:port
	turnstile *turnstile
	dbConns   chan struct{} // proxied Postgres connections open at once

	mu        sync.Mutex
	terminals map[string]bool // sessions made from the terminal page, which may open a TTY
	status    sandbox.Status
	statusAt  time.Time
}

func newSandboxService(cfg Config) *sandboxService {
	if cfg.SandboxURL == "" || cfg.SandboxSecret == "" {
		return nil
	}
	service := &sandboxService{
		client: sandbox.NewClient(cfg.SandboxURL, cfg.SandboxSecret), pgAddr: cfg.SandboxPGAddr,
		dbConns: make(chan struct{}, 200), terminals: map[string]bool{},
	}
	if cfg.TurnstileSecret != "" && cfg.TurnstileSiteKey != "" {
		service.turnstile = &turnstile{secret: cfg.TurnstileSecret, siteKey: cfg.TurnstileSiteKey, verifyURL: cfg.TurnstileVerifyURL,
			client: &http.Client{Timeout: 10 * time.Second}}
	}
	return service
}

func (s *Server) sandboxService(w http.ResponseWriter, r *http.Request) *sandboxService {
	if service := s.extras.sandbox; service != nil {
		return service
	}
	s.fail(w, r, http.StatusServiceUnavailable, "not_available", "The sandbox services are not available on this server yet.")
	return nil
}

// accountKey is how the controller knows an account: by GitHub's ID, which a rename
// does not change.
func accountKey(who account) string { return "gh:" + strconv.FormatInt(who.ID, 10) }

// sandboxFailed answers an error from the controller.
func (s *Server) sandboxFailed(w http.ResponseWriter, r *http.Request, what string, err error) {
	var remote *sandbox.RemoteError
	if errors.As(err, &remote) {
		s.metrics.sandboxEvent(what, remote.Detail.Code)
		switch remote.Detail.Code {
		case sandbox.CodeBusy:
			w.Header().Set("Retry-After", "5")
			s.fail(w, r, http.StatusServiceUnavailable, "busy", "Every sandbox is in use. Try again in a few seconds.")
		case sandbox.CodePaused:
			w.Header().Set("Retry-After", "600")
			s.fail(w, r, http.StatusServiceUnavailable, "paused", "The sandbox services are paused for now. Try again later.")
		case sandbox.CodeNotFound:
			s.notFound(w, r, "No such session or database, or it has ended.")
		case sandbox.CodeLimit:
			s.fail(w, r, http.StatusTooManyRequests, "limit", remote.Detail.Message+".")
		case sandbox.CodeBadRequest:
			s.badRequest(w, r, strings.TrimPrefix(remote.Detail.Message, "bad request: ")+".")
		case sandbox.CodeUnavailable:
			s.fail(w, r, http.StatusServiceUnavailable, "not_available", remote.Detail.Message+".")
		case sandbox.CodeFailed:
			if remote.Status == http.StatusUnprocessableEntity {
				s.fail(w, r, http.StatusUnprocessableEntity, "page_failed", remote.Detail.Message)
				return
			}
			logError("sandbox "+what, err)
			s.fail(w, r, http.StatusBadGateway, "sandbox_error", "The sandbox could not do that. Try again.")
		default:
			logError("sandbox "+what, err)
			s.fail(w, r, http.StatusBadGateway, "sandbox_error", "The sandbox could not do that. Try again.")
		}
		return
	}
	s.metrics.sandboxEvent(what, "unreachable")
	logError("sandbox "+what, err)
	w.Header().Set("Retry-After", "60")
	s.fail(w, r, http.StatusServiceUnavailable, "not_available", "The sandbox VM is not answering. Try again in a minute.")
}

// sandboxFile is a file sent in with code, as text or base64.
type sandboxFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding,omitempty"` // text (default) or base64
}

type runBody struct {
	Language string        `json:"language"`
	Code     string        `json:"code"`
	Stdin    string        `json:"stdin,omitempty"`
	Args     []string      `json:"args,omitempty"`
	Files    []sandboxFile `json:"files,omitempty"`
}

func (b *runBody) setText(text string) { b.Code = text }

func (b runBody) request(r *http.Request, who account) (sandbox.RunRequest, error) {
	language := b.Language
	if language == "" {
		language = r.URL.Query().Get("language")
	}
	req := sandbox.RunRequest{Language: language, Code: b.Code, Stdin: b.Stdin, Args: b.Args, Account: accountKey(who)}
	if strings.TrimSpace(req.Language) == "" || req.Code == "" {
		return req, errors.New("give language and code")
	}
	if len(req.Code) > sandbox.MaxCodeBytes {
		return req, errors.New("the code is over " + itoa(sandbox.MaxCodeBytes>>10) + " KB")
	}
	for _, file := range b.Files {
		content := []byte(file.Content)
		if file.Encoding == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(file.Content)
			if err != nil {
				return req, errors.New(file.Path + " is not valid base64")
			}
			content = decoded
		}
		req.Files = append(req.Files, sandbox.File{Path: file.Path, Content: content})
	}
	return req, nil
}

// charge counts a request against the account and writes the audit line.
func (s *Server) charge(store *accountStore, who account, entry auditEntry, runs, cpuMillis int) {
	now := time.Now()
	store.charge(who.ID, runs, cpuMillis, now)
	entry.Account, entry.Login, entry.CPUMillis = who.ID, who.Login, cpuMillis
	store.audit(entry, now)
}

func (s *Server) handleSandboxRun(w http.ResponseWriter, r *http.Request) {
	store, who, ok := s.signedIn(w, r)
	if !ok || !s.withinQuota(w, r, store, who) {
		return
	}
	service := s.sandboxService(w, r)
	if service == nil {
		return
	}
	var body runBody
	if !s.readJSON(w, r, &body) {
		return
	}
	req, err := body.request(r, who)
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	result, err := service.client.Run(r.Context(), req)
	if err != nil {
		s.charge(store, who, auditEntry{Action: "run", Image: req.Language, CodeHash: codeHash(req.Code), Outcome: "error"}, 1, 0)
		s.sandboxFailed(w, r, "run", err)
		return
	}
	exit := result.ExitCode
	s.charge(store, who, auditEntry{Action: "run", Image: req.Language, CodeHash: codeHash(req.Code), ExitCode: &exit, Limits: result.LimitsHit}, 1, result.CPUMillis)
	s.metrics.sandboxEvent("run", "ok")
	s.ok(w, r, result, meta{}, cacheNone)
}

// handleSandboxLanguages lists the languages, with the versions the image holds when the
// controller has reported them.
func (s *Server) handleSandboxLanguages(w http.ResponseWriter, r *http.Request) {
	languages := map[string]sandbox.Language{}
	for name, language := range sandbox.Languages {
		languages[name] = language
	}
	if service := s.extras.sandbox; service != nil {
		if status, err := service.cachedStatus(r.Context()); err == nil {
			for name, language := range status.Languages {
				languages[name] = language
			}
		}
	}
	type view struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Version string   `json:"version,omitempty"`
		File    string   `json:"file"`
		Command string   `json:"command"`
		Aliases []string `json:"aliases,omitempty"`
	}
	out := []view{}
	for _, id := range sortedKeys(languages) {
		language := languages[id]
		item := view{ID: id, Name: language.Name, Version: language.Version, File: language.File, Command: strings.Join(language.Command, " ")}
		for _, alias := range sortedKeys(sandbox.LanguageAliases) {
			if sandbox.LanguageAliases[alias] == id {
				item.Aliases = append(item.Aliases, alias)
			}
		}
		out = append(out, item)
	}
	s.ok(w, r, map[string]any{"languages": out, "images": sandbox.SessionImages, "limits": sandboxLimits()}, meta{}, cacheShort)
}

func sandboxLimits() map[string]any {
	return map[string]any{
		"runSeconds": int(sandbox.RunTimeout.Seconds()), "memoryMB": 256, "codeKB": sandbox.MaxCodeBytes >> 10,
		"outputKB": sandbox.MaxOutputBytes >> 10, "files": sandbox.MaxFiles, "fileKB": sandbox.MaxFileBytes >> 10,
		"sessionMinutes": int(sandbox.SessionLifetime.Minutes()), "distroMinutes": int(sandbox.DistroLifetime.Minutes()),
		"sessionsAtOnce": sandbox.SessionsPerUser, "execSeconds": int(sandbox.ExecTimeoutLimit.Seconds()),
		"downloadMB": sandbox.DownloadCap >> 20, "runsPerDay": fullQuota.Runs, "cpuMinutesPerDay": fullQuota.CPUMillis / 60000,
		"newAccountRunsPerDay": newQuota.Runs, "newAccountCPUMinutesPerDay": newQuota.CPUMillis / 60000,
	}
}

func (service *sandboxService) cachedStatus(ctx context.Context) (sandbox.Status, error) {
	service.mu.Lock()
	if time.Since(service.statusAt) < 5*time.Minute {
		status := service.status
		service.mu.Unlock()
		return status, nil
	}
	service.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	status, err := service.client.Status(ctx)
	if err != nil {
		return status, err
	}
	service.mu.Lock()
	service.status, service.statusAt = status, time.Now()
	service.mu.Unlock()
	return status, nil
}

// ---- sessions ----

type sessionBody struct {
	Image     string `json:"image"`
	Network   bool   `json:"network"`
	Terminal  bool   `json:"terminal"`
	Turnstile string `json:"turnstile"`
}

func (s *Server) sessionView(r *http.Request, session sandbox.Session) map[string]any {
	base := s.origin(r) + "/v1/sandbox/sessions/" + session.ID
	return map[string]any{
		"id": session.ID, "image": session.Image, "network": session.Network,
		"createdAt": session.CreatedAt, "expiresAt": session.ExpiresAt,
		"links": map[string]string{
			"exec": base + "/exec", "run": base + "/run", "upload": base + "/upload", "download": base + "/download", "stop": base + "/stop",
			"tty": strings.Replace(base, "http", "ws", 1) + "/tty",
		},
	}
}

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	store, who, ok := s.signedIn(w, r)
	if !ok || !s.withinQuota(w, r, store, who) {
		return
	}
	service := s.sandboxService(w, r)
	if service == nil {
		return
	}
	var body sessionBody
	if !s.readJSON(w, r, &body) {
		return
	}
	if body.Terminal {
		if service.turnstile == nil {
			s.fail(w, r, http.StatusServiceUnavailable, "not_available", "The terminal page is not set up on this server yet.")
			return
		}
		if !service.turnstile.verify(r.Context(), body.Turnstile) {
			s.fail(w, r, http.StatusForbidden, "turnstile_failed", "The browser check did not pass. Reload the page and try again.")
			return
		}
	}
	session, err := service.client.CreateSession(r.Context(), sandbox.SessionRequest{Image: body.Image, Account: accountKey(who), Network: body.Network})
	if err != nil {
		s.sandboxFailed(w, r, "session", err)
		return
	}
	if body.Terminal {
		service.mu.Lock()
		service.terminals[session.ID] = true
		service.mu.Unlock()
	}
	s.charge(store, who, auditEntry{Action: "session", ID: session.ID, Image: session.Image}, 1, 0)
	s.metrics.sandboxEvent("session", "ok")
	s.ok(w, r, s.sessionView(r, session), meta{Note: "The session ends at expiresAt, or when stopped. Files and installed packages last as long as it does."}, cacheNone)
}

// ownedSession finds a session of the caller's, charging the processor time it used
// since last asked. It answers 404 itself for someone else's session.
func (s *Server) ownedSession(w http.ResponseWriter, r *http.Request) (*sandboxService, *accountStore, account, sandbox.Session, bool) {
	store, who, ok := s.signedIn(w, r)
	if !ok {
		return nil, nil, account{}, sandbox.Session{}, false
	}
	service := s.sandboxService(w, r)
	if service == nil {
		return nil, nil, account{}, sandbox.Session{}, false
	}
	session, err := service.client.Session(r.Context(), r.PathValue("id"))
	if err != nil {
		s.sandboxFailed(w, r, "session", err)
		return nil, nil, account{}, sandbox.Session{}, false
	}
	if session.Account != accountKey(who) {
		s.notFound(w, r, "No such session, or it has ended.")
		return nil, nil, account{}, sandbox.Session{}, false
	}
	if session.CPUMillis > 0 {
		store.charge(who.ID, 0, session.CPUMillis, time.Now())
	}
	return service, store, who, session, true
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	_, _, _, session, ok := s.ownedSession(w, r)
	if !ok {
		return
	}
	s.ok(w, r, s.sessionView(r, session), meta{}, cacheNone)
}

func (s *Server) handleSessionExec(w http.ResponseWriter, r *http.Request) {
	service, store, who, session, ok := s.ownedSession(w, r)
	if !ok || !s.withinQuota(w, r, store, who) {
		return
	}
	var body sandbox.ExecRequest
	if !s.readJSON(w, r, &body) {
		return
	}
	result, err := service.client.Exec(r.Context(), session.ID, body)
	if err != nil {
		s.sandboxFailed(w, r, "exec", err)
		return
	}
	exit := result.ExitCode
	what := body.Command
	if what == "" {
		what = strings.Join(body.Argv, "\x00")
	}
	s.charge(store, who, auditEntry{Action: "exec", ID: session.ID, Image: session.Image, CodeHash: codeHash(what), ExitCode: &exit, Limits: result.LimitsHit}, 1, result.CPUMillis)
	s.metrics.sandboxEvent("exec", "ok")
	s.ok(w, r, result, meta{}, cacheNone)
}

func (s *Server) handleSessionRun(w http.ResponseWriter, r *http.Request) {
	service, store, who, session, ok := s.ownedSession(w, r)
	if !ok || !s.withinQuota(w, r, store, who) {
		return
	}
	var body runBody
	if !s.readJSON(w, r, &body) {
		return
	}
	req, err := body.request(r, who)
	if err != nil {
		s.badRequest(w, r, err.Error()+".")
		return
	}
	result, err := service.client.RunIn(r.Context(), session.ID, req)
	if err != nil {
		s.sandboxFailed(w, r, "run", err)
		return
	}
	exit := result.ExitCode
	s.charge(store, who, auditEntry{Action: "run", ID: session.ID, Image: req.Language, CodeHash: codeHash(req.Code), ExitCode: &exit, Limits: result.LimitsHit}, 1, result.CPUMillis)
	s.metrics.sandboxEvent("run", "ok")
	s.ok(w, r, result, meta{}, cacheNone)
}

func (s *Server) handleSessionPutFile(w http.ResponseWriter, r *http.Request) {
	service, _, _, session, ok := s.ownedSession(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		s.badRequest(w, r, "Give the file's path as ?path=, relative to /work or absolute.")
		return
	}
	data, err := readSessionFile(w, r)
	if err != nil {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "A file is at most "+itoa(sandbox.MaxFileBytes>>10)+" KB, sent as the body or as the first file of a form.")
		return
	}
	if err := service.client.PutFile(r.Context(), session.ID, path, data); err != nil {
		s.sandboxFailed(w, r, "files", err)
		return
	}
	s.ok(w, r, map[string]any{"path": path, "bytes": len(data)}, meta{}, cacheNone)
}

// readSessionFile reads a file sent as the body, or as the first file of a multipart form.
func readSessionFile(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body := http.MaxBytesReader(w, r.Body, sandbox.MaxFileBytes+64<<10)
	kind, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if kind != "multipart/form-data" {
		return readCapped(body, sandbox.MaxFileBytes)
	}
	reader := multipart.NewReader(body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err != nil {
			return nil, err
		}
		if part.FileName() != "" {
			return readCapped(part, sandbox.MaxFileBytes)
		}
	}
}

func readCapped(r io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err == nil && len(data) > limit {
		err = errors.New("too large")
	}
	return data, err
}

func (s *Server) handleSessionGetFile(w http.ResponseWriter, r *http.Request) {
	service, _, _, session, ok := s.ownedSession(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if r.URL.Query().Get("list") == "true" || path == "" || strings.HasSuffix(path, "/") {
		entries, err := service.client.ListFiles(r.Context(), session.ID, path)
		if err != nil {
			s.sandboxFailed(w, r, "files", err)
			return
		}
		s.ok(w, r, map[string]any{"path": path, "entries": entries}, meta{}, cacheNone)
		return
	}
	data, err := service.client.GetFile(r.Context(), session.ID, path)
	if err != nil {
		s.sandboxFailed(w, r, "files", err)
		return
	}
	if len(data) > sandbox.MaxFileBytes {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "Files over "+itoa(sandbox.MaxFileBytes>>10)+" KB cannot be read back.")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", "attachment")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}

func (s *Server) handleSessionStop(w http.ResponseWriter, r *http.Request) {
	store, who, ok := s.signedIn(w, r)
	if !ok {
		return
	}
	service := s.sandboxService(w, r)
	if service == nil {
		return
	}
	id := r.PathValue("id")
	session, err := service.client.Session(r.Context(), id)
	if err != nil || session.Account != accountKey(who) {
		s.notFound(w, r, "No such session, or it has ended.")
		return
	}
	stopped, err := service.client.Stop(r.Context(), id)
	if err != nil {
		s.sandboxFailed(w, r, "stop", err)
		return
	}
	service.mu.Lock()
	delete(service.terminals, id)
	service.mu.Unlock()
	store.charge(who.ID, 0, session.CPUMillis+stopped.CPUMillis, time.Now())
	s.ok(w, r, map[string]any{"stopped": true, "id": id}, meta{}, cacheNone)
}

// handleSessionTTY carries a terminal between the caller's WebSocket and the session.
// Only sessions made from the terminal page (which passed the browser check) may open one;
// programs use exec.
func (s *Server) handleSessionTTY(w http.ResponseWriter, r *http.Request) {
	service, store, who, session, ok := s.ownedSession(w, r)
	if !ok {
		return
	}
	service.mu.Lock()
	allowed := service.terminals[session.ID]
	service.mu.Unlock()
	if !allowed {
		s.fail(w, r, http.StatusForbidden, "terminal_only", "A terminal opens only for sessions started from "+s.origin(r)+"/terminal. Programs use exec.")
		return
	}
	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	remote, err := service.client.TTY(ctx, session.ID, max(cols, 20), max(rows, 5))
	if err != nil {
		s.sandboxFailed(w, r, "tty", err)
		return
	}
	defer remote.CloseNow()
	// A terminal stays open for minutes: the server's usual deadlines do not apply.
	controller := http.NewResponseController(w)
	controller.SetReadDeadline(time.Time{})
	controller.SetWriteDeadline(time.Time{})
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	started := time.Now()
	pipeWebSockets(ctx, cancel, conn, remote)
	if after, err := service.client.Session(context.Background(), session.ID); err == nil && after.CPUMillis > 0 {
		store.charge(who.ID, 0, after.CPUMillis, time.Now())
	}
	store.audit(auditEntry{Account: who.ID, Login: who.Login, Action: "tty", ID: session.ID, Image: session.Image,
		Outcome: time.Since(started).Round(time.Second).String()}, time.Now())
}

// pipeWebSockets copies messages both ways until either side closes.
func pipeWebSockets(ctx context.Context, cancel func(), a, b *websocket.Conn) {
	copyOne := func(from, to *websocket.Conn) {
		defer cancel()
		for {
			kind, data, err := from.Read(ctx)
			if err != nil {
				return
			}
			if err := to.Write(ctx, kind, data); err != nil {
				return
			}
		}
	}
	go copyOne(a, b)
	copyOne(b, a)
	a.Close(websocket.StatusNormalClosure, "")
	b.Close(websocket.StatusNormalClosure, "")
}

// ---- throwaway Postgres ----

func (s *Server) databaseView(r *http.Request, database sandbox.Database) map[string]any {
	host := strings.TrimPrefix(strings.TrimPrefix(s.origin(r), "https://"), "http://")
	view := map[string]any{
		"name": database.Name, "user": database.User, "createdAt": database.CreatedAt, "expiresAt": database.ExpiresAt,
		"sizeBytes": database.SizeBytes, "limitBytes": database.LimitBytes, "connections": sandbox.DatabaseConnections,
		"websocket": strings.Replace(s.origin(r), "http", "ws", 1) + "/v1/db/ws",
	}
	if database.Password != "" {
		view["password"] = database.Password
		view["connectionString"] = "postgresql://" + database.User + ":" + database.Password + "@" + host + "/" + database.Name + "?sslmode=disable"
	}
	return view
}

func (s *Server) handleDatabaseCreate(w http.ResponseWriter, r *http.Request) {
	store, who, ok := s.signedIn(w, r)
	if !ok || !s.withinQuota(w, r, store, who) {
		return
	}
	service := s.sandboxService(w, r)
	if service == nil {
		return
	}
	database, err := service.client.CreateDatabase(r.Context(), accountKey(who))
	if err != nil {
		s.sandboxFailed(w, r, "db", err)
		return
	}
	s.charge(store, who, auditEntry{Action: "db", ID: database.Name}, 1, 0)
	s.metrics.sandboxEvent("db", "ok")
	s.ok(w, r, s.databaseView(r, database), meta{Note: "Connect over WebSocket (websocket), for example with the Neon serverless driver, or run the client's db bridge for psql. The password is shown only now. The database is deleted at expiresAt."}, cacheNone)
}

func (s *Server) ownedDatabase(w http.ResponseWriter, r *http.Request) (*sandboxService, sandbox.Database, bool) {
	_, who, ok := s.signedIn(w, r)
	if !ok {
		return nil, sandbox.Database{}, false
	}
	service := s.sandboxService(w, r)
	if service == nil {
		return nil, sandbox.Database{}, false
	}
	database, err := service.client.Database(r.Context(), r.PathValue("name"))
	if err != nil {
		s.sandboxFailed(w, r, "db", err)
		return nil, sandbox.Database{}, false
	}
	if database.Account != accountKey(who) {
		s.notFound(w, r, "No such database, or it has been deleted.")
		return nil, sandbox.Database{}, false
	}
	return service, database, true
}

func (s *Server) handleDatabaseGet(w http.ResponseWriter, r *http.Request) {
	_, database, ok := s.ownedDatabase(w, r)
	if !ok {
		return
	}
	s.ok(w, r, s.databaseView(r, database), meta{}, cacheNone)
}

func (s *Server) handleDatabaseDrop(w http.ResponseWriter, r *http.Request) {
	service, database, ok := s.ownedDatabase(w, r)
	if !ok {
		return
	}
	if err := service.client.DropDatabase(r.Context(), database.Name); err != nil {
		s.sandboxFailed(w, r, "db", err)
		return
	}
	s.ok(w, r, map[string]any{"dropped": true, "name": database.Name}, meta{}, cacheNone)
}

// handleDatabaseSocket carries a Postgres connection over a WebSocket to the sandbox
// VM's server. Postgres itself checks the user and password; a user can reach only its
// own database.
func (s *Server) handleDatabaseSocket(w http.ResponseWriter, r *http.Request) {
	service := s.sandboxService(w, r)
	if service == nil {
		return
	}
	info(r).private = true
	if service.pgAddr == "" {
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "Throwaway databases are not available on this server yet.")
		return
	}
	select {
	case service.dbConns <- struct{}{}:
		defer func() { <-service.dbConns }()
	default:
		w.Header().Set("Retry-After", "10")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "Too many database connections are open. Try again shortly.")
		return
	}
	var dialer net.Dialer
	dialCtx, cancelDial := context.WithTimeout(r.Context(), 5*time.Second)
	upstream, err := dialer.DialContext(dialCtx, "tcp", service.pgAddr)
	cancelDial()
	if err != nil {
		s.sandboxFailed(w, r, "db", err)
		return
	}
	defer upstream.Close()
	controller := http.NewResponseController(w)
	controller.SetReadDeadline(time.Time{})
	controller.SetWriteDeadline(time.Time{})
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	socket := websocket.NetConn(ctx, conn, websocket.MessageBinary)
	// An idle connection is closed after ten minutes, as the database's own timeouts would.
	done := make(chan struct{}, 2)
	go func() { io.Copy(upstream, socket); done <- struct{}{} }()
	go func() { io.Copy(socket, upstream); done <- struct{}{} }()
	select {
	case <-done:
	case <-time.After(sandbox.DatabaseLifetime):
	}
}

// ---- Turnstile ----

type turnstile struct {
	secret, siteKey string
	verifyURL       string
	client          *http.Client
}

// verify asks Cloudflare whether a widget's answer is good. The caller's address is not
// sent along.
func (t *turnstile) verify(ctx context.Context, response string) bool {
	if response == "" || len(response) > 2048 {
		return false
	}
	address := t.verifyURL
	if address == "" {
		address = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address,
		strings.NewReader(url.Values{"secret": {t.secret}, "response": {response}}.Encode()))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	answer, err := t.client.Do(req)
	if err != nil {
		logError("turnstile", err)
		return false
	}
	defer answer.Body.Close()
	var result struct {
		Success bool `json:"success"`
	}
	json.NewDecoder(io.LimitReader(answer.Body, 64<<10)).Decode(&result)
	return result.Success
}
