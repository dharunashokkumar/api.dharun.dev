//go:build linux

package sandboxd

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// A session is a microVM kept for a few minutes: files stay between commands, packages
// can be installed (through the proxy, when it has network), and a terminal can be opened.
type session struct {
	id       string
	image    string
	account  string
	network  bool
	claim    *claim
	created  time.Time
	expires  time.Time
	shell    []string
	reported bool
}

func (s *session) view() sandbox.Session {
	return sandbox.Session{ID: s.id, Image: s.image, Network: s.network, Account: s.account,
		CreatedAt: s.created.UTC().Format(time.RFC3339), ExpiresAt: s.expires.UTC().Format(time.RFC3339)}
}

// CreateSession starts a session.
func (c *Controller) CreateSession(ctx context.Context, req sandbox.SessionRequest) (sandbox.Session, error) {
	if req.Image == "" {
		req.Image = sandbox.ImageCode
	}
	if !slices.Contains(sandbox.SessionImages, req.Image) {
		return sandbox.Session{}, badRequest("image must be one of %s", strings.Join(sandbox.SessionImages, ", "))
	}
	spec, _ := c.cfg.spec(req.Image)
	network := req.Network && req.Image == sandbox.ImageCode
	if network {
		spec.policy = policyRegistries
	}
	c.mu.Lock()
	open := 0
	for _, s := range c.sessions {
		if s.account == req.Account {
			open++
		}
	}
	c.mu.Unlock()
	if req.Account != "" && open >= sandbox.SessionsPerUser {
		return sandbox.Session{}, errLimit
	}
	cl, err := c.launch(ctx, spec, "session")
	if err != nil {
		return sandbox.Session{}, err
	}
	env := map[string]string{}
	if req.Image == sandbox.ImageCode {
		env = codeEnvironment(network)
	}
	if err := cl.vm.init(ctx, env, network); err != nil {
		c.end(cl)
		return sandbox.Session{}, fmt.Errorf("init: %w", err)
	}
	lifetime := sandbox.SessionLifetime
	if req.Image != sandbox.ImageCode {
		lifetime = sandbox.DistroLifetime
	}
	now := time.Now()
	s := &session{id: randomName("ses"), image: req.Image, account: req.Account, network: network, claim: cl,
		created: now, expires: now.Add(lifetime), shell: spec.shell}
	c.mu.Lock()
	c.sessions[s.id] = s
	c.mu.Unlock()
	c.metrics.sessions.add(req.Image, 1)
	return s.view(), nil
}

func (c *Controller) session(id string) (*session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.sessions[id]
	if !ok {
		return nil, errNotFound
	}
	return s, nil
}

// Session describes a session and reports the processor time used since last asked.
func (c *Controller) Session(id string) (sandbox.Session, error) {
	s, err := c.session(id)
	if err != nil {
		return sandbox.Session{}, err
	}
	view := s.view()
	view.CPUMillis = c.chargeCPU(s)
	return view, nil
}

func (c *Controller) chargeCPU(s *session) int {
	cpu := s.claim.vm.cpuSince()
	c.metrics.cpuSeconds.addf("session", float64(cpu)/1000)
	return cpu
}

// Exec runs a command in a session.
func (c *Controller) Exec(ctx context.Context, id string, req sandbox.ExecRequest) (sandbox.Result, error) {
	s, err := c.session(id)
	if err != nil {
		return sandbox.Result{}, err
	}
	argv := req.Argv
	if len(argv) == 0 {
		if strings.TrimSpace(req.Command) == "" {
			return sandbox.Result{}, badRequest("give command or argv")
		}
		shell := "bash"
		if s.image == sandbox.ImageAlpine {
			shell = "sh"
		}
		argv = []string{shell, "-c", req.Command}
	}
	timeout := time.Duration(req.TimeoutMillis) * time.Millisecond
	if timeout <= 0 || timeout > sandbox.ExecTimeoutLimit {
		timeout = sandbox.ExecTimeoutLimit
	}
	if left := time.Until(s.expires); left < timeout {
		timeout = left
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := s.claim.vm.exec(runCtx, sandbox.AgentRequest{Argv: argv, Cwd: req.Cwd, Env: req.Env,
		TimeoutMillis: int(timeout.Milliseconds())}, []byte(req.Stdin))
	if err != nil {
		return sandbox.Result{}, err
	}
	result := out.toResult(c.chargeCPU(s))
	for _, limit := range result.LimitsHit {
		c.metrics.limits.add(limit, 1)
	}
	return result, nil
}

// RunInSession runs code in a session, keeping its files and packages.
func (c *Controller) RunInSession(ctx context.Context, id string, req sandbox.RunRequest) (sandbox.Result, error) {
	s, err := c.session(id)
	if err != nil {
		return sandbox.Result{}, err
	}
	if s.image != sandbox.ImageCode {
		return sandbox.Result{}, badRequest("run code in a session of the code image; in a distribution use exec")
	}
	language, ok := c.language(req.Language)
	if !ok {
		return sandbox.Result{}, badRequest("unknown language %q", req.Language)
	}
	if err := checkFiles(req); err != nil {
		return sandbox.Result{}, err
	}
	timeout := min(sandbox.ExecTimeoutLimit, time.Until(s.expires))
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := c.runIn(runCtx, s.claim.vm, language, req)
	if err != nil {
		return sandbox.Result{}, err
	}
	c.metrics.cpuSeconds.addf("session", float64(result.CPUMillis)/1000)
	return result, nil
}

// PutFile writes a file into a session.
func (c *Controller) PutFile(ctx context.Context, id, path string, data []byte) error {
	s, err := c.session(id)
	if err != nil {
		return err
	}
	if path == "" {
		return badRequest("give the file's path")
	}
	return s.claim.vm.writeFile(ctx, path, data, 0o644)
}

// GetFile reads a file from a session.
func (c *Controller) GetFile(ctx context.Context, id, path string) ([]byte, error) {
	s, err := c.session(id)
	if err != nil {
		return nil, err
	}
	data, err := s.claim.vm.readFile(ctx, path, sandbox.MaxFileBytes)
	if err != nil {
		return nil, badRequest("%v", err)
	}
	return data, nil
}

// ListFiles lists a directory in a session.
func (c *Controller) ListFiles(ctx context.Context, id, path string) ([]sandbox.Entry, error) {
	s, err := c.session(id)
	if err != nil {
		return nil, err
	}
	entries, err := s.claim.vm.list(ctx, path)
	if err != nil {
		return nil, badRequest("%v", err)
	}
	return entries, nil
}

// OpenTTY starts the session's shell on a terminal and returns the agent connection its
// frames come on.
func (c *Controller) OpenTTY(ctx context.Context, id string, size sandbox.TermSize) (net.Conn, error) {
	s, err := c.session(id)
	if err != nil {
		return nil, err
	}
	return s.claim.vm.startExec(ctx, sandbox.AgentRequest{Argv: s.shell, Cwd: "/home/sandbox", TTY: &size,
		Stdin: true, MaxOutput: 1 << 40})
}

// StopSession ends a session at once.
func (c *Controller) StopSession(id string) (sandbox.Session, error) {
	s, err := c.session(id)
	if err != nil {
		return sandbox.Session{}, err
	}
	view := s.view()
	view.CPUMillis = c.chargeCPU(s)
	c.endSession(s)
	return view, nil
}

func (c *Controller) endSession(s *session) {
	c.mu.Lock()
	_, present := c.sessions[s.id]
	delete(c.sessions, s.id)
	c.mu.Unlock()
	if present {
		c.end(s.claim)
	}
}

// endExpired ends sessions past their time.
func (c *Controller) endExpired() {
	now := time.Now()
	c.mu.Lock()
	var expired []*session
	for _, s := range c.sessions {
		if now.After(s.expires) {
			expired = append(expired, s)
		}
	}
	c.mu.Unlock()
	for _, s := range expired {
		c.chargeCPU(s)
		c.endSession(s)
	}
}
