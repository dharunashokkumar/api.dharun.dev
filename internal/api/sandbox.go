package api

// Running code (growth plan Part 9, phase 1): code in, output out, in a fresh Firecracker
// microVM on the separate sandbox VM, with no network and a 30-second limit. Only with a
// key, always (not only after KEY_REQUIRED_FROM): a daily quota of runs and CPU time per
// key, and an audit log of who ran what kept for 30 days, holding a hash of the code,
// never the code. Without SANDBOX_URL and SANDBOX_SECRET the endpoint answers 503.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// Phase 1 languages (Dharun, 2026-11-11): Python and Node.js.
var runLanguages = []string{"python", "javascript"}

const (
	runsPerDay       = 50
	runCPUPerDay     = 10 * time.Minute
	sandboxAuditDays = 30
)

type sandboxRuns struct {
	client *sandbox.Client
	audit  string // directory of the daily audit files

	mu   sync.Mutex
	day  string
	runs map[string]int           // by key ID
	cpu  map[string]time.Duration // by key ID
}

func newSandboxRuns(cfg Config) *sandboxRuns {
	if cfg.SandboxURL == "" || cfg.SandboxSecret == "" {
		return nil
	}
	audit := ""
	if cfg.StateDir != "" {
		audit = filepath.Join(cfg.StateDir, "sandbox-audit")
	}
	return &sandboxRuns{client: sandbox.NewClient(cfg.SandboxURL, cfg.SandboxSecret), audit: audit}
}

// take counts a run against a key's day; ok is false when the runs or the CPU time are
// used up.
func (q *sandboxRuns) take(key string) (runs int, cpu time.Duration, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	today := time.Now().In(indiaTime).Format(time.DateOnly)
	if q.day != today {
		q.day, q.runs, q.cpu = today, map[string]int{}, map[string]time.Duration{}
	}
	if q.runs[key] >= runsPerDay || q.cpu[key] >= runCPUPerDay {
		return q.runs[key], q.cpu[key], false
	}
	q.runs[key]++
	return q.runs[key], q.cpu[key], true
}

func (q *sandboxRuns) spent(key string, cpu time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cpu != nil {
		q.cpu[key] += cpu
	}
}

type auditLine struct {
	Time     string   `json:"time"`
	Key      string   `json:"key"`
	Language string   `json:"language"`
	CodeHash string   `json:"code_sha256"`
	Exit     int      `json:"exit"`
	CPUMs    int      `json:"cpu_ms"`
	Limits   []string `json:"limits_hit,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// record appends to the day's audit file and removes files older than 30 days.
func (q *sandboxRuns) record(line auditLine) {
	if q.audit == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := os.MkdirAll(q.audit, 0o700); err != nil {
		return
	}
	now := time.Now().UTC()
	name := filepath.Join(q.audit, now.Format(time.DateOnly)+".jsonl")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	encoded, _ := json.Marshal(line)
	file.Write(append(encoded, '\n'))
	file.Close()
	cutoff := now.AddDate(0, 0, -sandboxAuditDays).Format(time.DateOnly)
	old, _ := filepath.Glob(filepath.Join(q.audit, "*.jsonl"))
	for _, f := range old {
		if strings.TrimSuffix(filepath.Base(f), ".jsonl") < cutoff {
			os.Remove(f)
		}
	}
}

func (s *Server) registerSandbox() {
	s.sandbox = newSandboxRuns(s.cfg)
	s.handle("run", costHeavy, "POST /v1/run", s.handleRun)
	s.handle("run", costFree, "GET /v1/run/languages", s.handleRunLanguages)
}

func (s *Server) handleRunLanguages(w http.ResponseWriter, r *http.Request) {
	type language struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		File string `json:"file"`
	}
	out := []language{}
	for _, id := range runLanguages {
		l := sandbox.Languages[id]
		out = append(out, language{ID: id, Name: l.Name, File: l.File})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Note: "Each run gets a fresh microVM with no network, 30 seconds, 256 MB and 64 KB of output. " + strconv.Itoa(runsPerDay) + " runs and " + strconv.Itoa(int(runCPUPerDay.Minutes())) + " CPU minutes a day per key."}, cacheAPI)
}

type runRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Stdin    string `json:"stdin"`
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	details := info(r)
	details.private = true
	details.keyRoute = true
	if s.sandbox == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "Running code is not switched on on this server yet: it needs the sandbox VM.")
		return
	}
	if details.cred.kind != "key" {
		s.fail(w, r, http.StatusUnauthorized, "key_required", "Running code needs a free API key, sent as Authorization: Bearer. Get one at "+s.origin(r)+"/signup")
		return
	}
	raw, ok := s.readJSONBody(w, r)
	if !ok {
		return
	}
	var in runRequest
	if json.Unmarshal(raw, &in) != nil {
		s.badRequest(w, r, `Send JSON {"language": "python", "code": "print(1)"}.`)
		return
	}
	language := strings.ToLower(strings.TrimSpace(in.Language))
	if alias, ok := sandbox.LanguageAliases[language]; ok {
		language = alias
	}
	if !contains(runLanguages, language) {
		s.badRequest(w, r, "language is python or javascript.")
		return
	}
	if strings.TrimSpace(in.Code) == "" || len(in.Code) > sandbox.MaxCodeBytes {
		s.badRequest(w, r, "code is the program, up to 64 KB.")
		return
	}
	if len(in.Stdin) > sandbox.MaxCodeBytes {
		s.badRequest(w, r, "stdin is up to 64 KB.")
		return
	}
	key := details.cred.keyID
	runs, cpu, ok := s.sandbox.take(key)
	w.Header().Set("X-Runs-Remaining", strconv.Itoa(max(runsPerDay-runs, 0)))
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(secondsToMidnight()))
		s.fail(w, r, http.StatusTooManyRequests, "quota_exceeded", "This key has used its "+strconv.Itoa(runsPerDay)+" runs or "+
			strconv.Itoa(int(runCPUPerDay.Minutes()))+" CPU minutes for today (used "+cpu.Round(time.Second).String()+"). It resets at midnight IST.")
		return
	}
	sum := sha256.Sum256([]byte(in.Code))
	line := auditLine{Time: time.Now().UTC().Format(time.RFC3339), Key: key, Language: language, CodeHash: hex.EncodeToString(sum[:])}
	ctx, cancel := context.WithTimeout(r.Context(), sandbox.RunTimeout+30*time.Second)
	defer cancel()
	result, err := s.sandbox.client.Run(ctx, sandbox.RunRequest{Language: language, Code: in.Code, Stdin: in.Stdin, Account: key})
	if err != nil {
		var remote *sandbox.RemoteError
		code := "unreachable"
		if errors.As(err, &remote) {
			code = remote.Detail.Code
		}
		line.Error = code
		s.sandbox.record(line)
		switch code {
		case sandbox.CodeBusy:
			s.fail(w, r, http.StatusServiceUnavailable, "busy", "Every sandbox is in use. Try again in a few seconds.")
		case sandbox.CodePaused:
			s.fail(w, r, http.StatusServiceUnavailable, "paused", "Running code is paused for now.")
		case sandbox.CodeBadRequest:
			s.badRequest(w, r, remote.Detail.Message)
		default:
			logError("sandbox run", errors.New("the controller answered "+code))
			s.fail(w, r, http.StatusBadGateway, "sandbox_failed", "The sandbox could not run the code. Try again later.")
		}
		return
	}
	s.sandbox.spent(key, time.Duration(result.CPUMillis)*time.Millisecond)
	line.Exit, line.CPUMs, line.Limits = result.ExitCode, result.CPUMillis, result.LimitsHit
	s.sandbox.record(line)
	if result.LimitsHit == nil {
		result.LimitsHit = []string{}
	}
	result.Files = nil
	s.ok(w, r, result, meta{Note: "A fresh microVM with no network ran the code once and was thrown away. limitsHit names a limit reached (time, memory, output, processes)."}, cacheNone)
}
