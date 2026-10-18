//go:build linux

// Package sandboxd is the sandbox controller: it runs in its own VM, starts a Firecracker
// microVM for every run, session and browser page, and answers the API over a private
// link. See docs/sandbox-design.md and cmd/sandboxd.
package sandboxd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// Config is read from the environment by cmd/sandboxd.
type Config struct {
	Version     string
	Firecracker string // the firecracker binary
	Jailer      string // the jailer binary
	Kernel      string // the guest kernel (vmlinux)
	Images      string // directory of <image>.ext4 files
	Scratch     string // an empty ext4 file, copied as each VM's scratch disk
	JailRoot    string // where jails are made; must be on the same file system as Images
	StateDir    string
	// CgroupParent is the cgroup (relative to /sys/fs/cgroup) the VMs' cgroups go under;
	// New sets it to "vms" below the controller's own cgroup.
	CgroupParent string
	UIDBase      int
	UIDCount     int
	// MemoryCapMB is the guest memory all microVMs may hold together. A request that
	// does not fit waits up to QueueWait, then is refused as busy.
	MemoryCapMB     int
	CodeMemoryMB    int
	BrowserMemoryMB int
	Warm            int // code VMs kept booted for run-once requests
	BootTimeout     time.Duration
	QueueWait       time.Duration
	// PauseFile is the kill switch: while it exists, nothing new starts and everything
	// running is stopped. Its content is the reason shown.
	PauseFile string
	// PerSite is how many pages a minute the browser may open on one site, across all
	// callers.
	PerSite  int
	Postgres PostgresConfig
}

var (
	errBusy       = errors.New("every sandbox slot is in use; try again in a few seconds")
	errPaused     = errors.New("the sandbox service is paused")
	errNotFound   = errors.New("no such session")
	errLimit      = errors.New("this account already has as many sessions as allowed")
	errBadRequest = errors.New("bad request")
)

// badRequest wraps a message as a bad request.
func badRequest(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errBadRequest, fmt.Sprintf(format, args...))
}

// Controller holds every running microVM.
type Controller struct {
	cfg     Config
	metrics *metrics
	proxy   *egress
	uids    *uidPool

	mu        sync.Mutex
	usedMB    int
	freed     chan struct{} // closed and replaced whenever memory is given back
	waiting   int
	running   map[*vm]bool
	warm      []*vm
	sessions  map[string]*session
	paused    string
	languages map[string]sandbox.Language

	db   *postgres
	stop chan struct{}
}

// New prepares the controller and starts its background work.
func New(cfg Config) (*Controller, error) {
	for _, dir := range []string{cfg.JailRoot, filepath.Join(cfg.StateDir, "console")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	parent, err := prepareCgroup()
	if err != nil {
		return nil, fmt.Errorf("cgroup: %w", err)
	}
	cfg.CgroupParent = parent
	c := &Controller{
		cfg: cfg, metrics: &metrics{}, uids: newUIDPool(cfg.UIDBase, cfg.UIDCount),
		freed: make(chan struct{}), running: map[*vm]bool{}, sessions: map[string]*session{},
		languages: map[string]sandbox.Language{}, stop: make(chan struct{}),
	}
	for name, language := range sandbox.Languages {
		c.languages[name] = language
	}
	c.proxy = newEgress(c.metrics, cfg.PerSite)
	if cfg.Postgres.Enabled {
		db, err := newPostgres(cfg.Postgres, c.metrics)
		if err != nil {
			return nil, fmt.Errorf("postgres: %w", err)
		}
		c.db = db
	}
	c.cleanJails()
	go c.housekeeping()
	go c.learnVersions()
	return c, nil
}

// prepareCgroup moves the controller into a leaf of its own cgroup ("supervisor"), so
// the cgroup may hand the CPU, memory and process controllers down, and makes "vms"
// below it for the microVMs. Under systemd this needs Delegate=yes. It returns the VMs'
// parent cgroup relative to /sys/fs/cgroup.
func prepareCgroup() (string, error) {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	own := ""
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if path, ok := strings.CutPrefix(line, "0::"); ok {
			own = path
		}
	}
	if own == "" {
		return "", errors.New("no cgroup v2 hierarchy")
	}
	own = strings.TrimSuffix(own, "/supervisor")
	base := filepath.Join("/sys/fs/cgroup", own)
	supervisor := filepath.Join(base, "supervisor")
	if err := os.MkdirAll(supervisor, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(supervisor, "cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return "", fmt.Errorf("moving into %s: %w", supervisor, err)
	}
	controllers := []byte("+cpu +memory +pids")
	if err := os.WriteFile(filepath.Join(base, "cgroup.subtree_control"), controllers, 0o644); err != nil {
		return "", fmt.Errorf("%s: %w", base, err)
	}
	vms := filepath.Join(base, "vms")
	if err := os.MkdirAll(vms, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(vms, "cgroup.subtree_control"), controllers, 0o644); err != nil {
		return "", fmt.Errorf("%s: %w", vms, err)
	}
	return strings.TrimPrefix(filepath.Join(own, "vms"), "/"), nil
}

// cleanJails removes jails a previous run left behind.
func (c *Controller) cleanJails() {
	dir := filepath.Join(c.cfg.JailRoot, filepath.Base(c.cfg.Firecracker))
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		os.RemoveAll(filepath.Join(dir, entry.Name()))
		os.Remove(filepath.Join("/sys/fs/cgroup", c.cfg.CgroupParent, entry.Name()))
	}
}

// Close stops every microVM.
func (c *Controller) Close() {
	close(c.stop)
	c.stopAll()
	if c.db != nil {
		c.db.close()
	}
}

// reserve waits until mb of guest memory is free and takes it.
func (c *Controller) reserve(ctx context.Context, mb int, what string) (func(), error) {
	started := time.Now()
	deadline := time.NewTimer(c.cfg.QueueWait)
	defer deadline.Stop()
	c.mu.Lock()
	waited := false
	for c.usedMB+mb > c.cfg.MemoryCapMB {
		if !waited {
			waited = true
			c.waiting++
		}
		freed := c.freed
		c.mu.Unlock()
		// A warm VM gives its memory up to a request that would otherwise wait.
		if c.dropWarm() {
			c.mu.Lock()
			continue
		}
		select {
		case <-freed:
		case <-deadline.C:
			c.mu.Lock()
			c.waiting--
			c.mu.Unlock()
			c.metrics.busy.add(what, 1)
			return nil, errBusy
		case <-ctx.Done():
			c.mu.Lock()
			c.waiting--
			c.mu.Unlock()
			return nil, ctx.Err()
		}
		c.mu.Lock()
	}
	if waited {
		c.waiting--
		c.metrics.queueWait.addf("", time.Since(started).Seconds())
		c.metrics.queueCount.add("", 1)
	}
	c.usedMB += mb
	c.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			c.usedMB -= mb
			close(c.freed)
			c.freed = make(chan struct{})
			c.mu.Unlock()
		})
	}, nil
}

// claim is a microVM together with the memory it holds.
type claim struct {
	vm      *vm
	release func()
}

func (c *Controller) launch(ctx context.Context, spec imageSpec, what string) (*claim, error) {
	if reason := c.pausedReason(); reason != "" {
		return nil, errPaused
	}
	release, err := c.reserve(ctx, spec.memoryMB+vmOverheadMB, what)
	if err != nil {
		return nil, err
	}
	machine, err := c.startVM(ctx, spec)
	if err != nil {
		release()
		c.metrics.vmStarts.add(spec.name+"/error", 1)
		return nil, err
	}
	c.metrics.vmStarts.add(spec.name+"/ok", 1)
	c.metrics.bootSeconds.addf(spec.name, machine.booted.Seconds())
	c.metrics.bootCount.add(spec.name, 1)
	c.mu.Lock()
	c.running[machine] = true
	c.mu.Unlock()
	return &claim{vm: machine, release: release}, nil
}

// end destroys a claimed microVM and gives its memory back.
func (c *Controller) end(cl *claim) {
	cl.vm.destroy(c)
	c.mu.Lock()
	delete(c.running, cl.vm)
	c.mu.Unlock()
	cl.release()
}

// takeWarm returns a booted code VM if one is waiting.
func (c *Controller) takeWarm() *claim {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.warm) > 0 {
		machine := c.warm[len(c.warm)-1]
		c.warm = c.warm[:len(c.warm)-1]
		select {
		case <-machine.exited:
			continue // it died while waiting
		default:
		}
		return &claim{vm: machine, release: c.warmRelease(machine)}
	}
	return nil
}

// warmReleases holds the memory release of each warm VM.
var warmReleases sync.Map

func (c *Controller) warmRelease(machine *vm) func() {
	if release, ok := warmReleases.LoadAndDelete(machine); ok {
		return release.(func())
	}
	return func() {}
}

// dropWarm stops one warm VM to free its memory, and reports whether there was one.
func (c *Controller) dropWarm() bool {
	cl := c.takeWarm()
	if cl == nil {
		return false
	}
	c.end(cl)
	return true
}

// fillWarm boots code VMs until Warm are waiting, while memory is free.
func (c *Controller) fillWarm() {
	spec, _ := c.cfg.spec(sandbox.ImageCode)
	for {
		c.mu.Lock()
		short := len(c.warm) < c.cfg.Warm && c.paused == "" && c.waiting == 0 &&
			c.usedMB+spec.memoryMB+vmOverheadMB <= c.cfg.MemoryCapMB
		c.mu.Unlock()
		if !short {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), c.cfg.BootTimeout+time.Second)
		cl, err := c.launch(ctx, spec, "warm")
		cancel()
		if err != nil {
			logf("warm VM: %v", err)
			return
		}
		warmReleases.Store(cl.vm, cl.release)
		c.mu.Lock()
		c.warm = append(c.warm, cl.vm)
		c.mu.Unlock()
	}
}

func (c *Controller) pausedReason() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

// housekeeping watches the kill switch, ends sessions that have run out, keeps the warm
// pool full and drops databases past their hour.
func (c *Controller) housekeeping() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
		}
		reason := ""
		if raw, err := os.ReadFile(c.cfg.PauseFile); err == nil {
			reason = strings.TrimSpace(string(raw))
			if reason == "" {
				reason = "paused by the operator"
			}
		}
		c.mu.Lock()
		was := c.paused
		c.paused = reason
		c.mu.Unlock()
		if reason != "" {
			if was == "" {
				logf("kill switch on: %s; stopping every microVM", reason)
			}
			c.stopAll()
			continue
		}
		c.endExpired()
		c.fillWarm()
		if c.db != nil {
			c.db.sweep()
		}
	}
}

// stopAll ends every session, warm VM and run.
func (c *Controller) stopAll() {
	c.mu.Lock()
	sessions := make([]*session, 0, len(c.sessions))
	for _, s := range c.sessions {
		sessions = append(sessions, s)
	}
	c.mu.Unlock()
	for _, s := range sessions {
		c.endSession(s)
	}
	for c.dropWarm() {
	}
	c.mu.Lock()
	machines := make([]*vm, 0, len(c.running))
	for machine := range c.running {
		machines = append(machines, machine)
	}
	c.mu.Unlock()
	for _, machine := range machines {
		// The run that holds it notices and gives its memory back.
		machine.destroy(c)
	}
}

// environment is what every command in a code VM finds set.
func codeEnvironment(network bool) map[string]string {
	env := map[string]string{
		"GOPATH": "/home/sandbox/go", "GOCACHE": "/home/sandbox/.cache/go-build", "GOTOOLCHAIN": "local",
		"GOFLAGS": "-mod=mod", "GOPROXY": "off", "GONOSUMDB": "*", "GOSUMDB": "off",
		"PIP_DISABLE_PIP_VERSION_CHECK": "1", "PIP_BREAK_SYSTEM_PACKAGES": "1", "PIP_USER": "1",
		"PIP_NO_INPUT": "1", "PYTHONUNBUFFERED": "1", "MPLBACKEND": "Agg",
		"NPM_CONFIG_UPDATE_NOTIFIER": "false", "NPM_CONFIG_FUND": "false", "NPM_CONFIG_AUDIT": "false",
		"COMPOSER_HOME": "/home/sandbox/.composer", "GEM_HOME": "/home/sandbox/.gem",
		"PATH": "/home/sandbox/.local/bin:/home/sandbox/go/bin:/home/sandbox/.gem/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	if network {
		proxy := "http://" + sandbox.GuestProxy
		for _, name := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"} {
			env[name] = proxy
		}
		env["NO_PROXY"], env["no_proxy"] = "localhost,127.0.0.1", "localhost,127.0.0.1"
		env["GOPROXY"] = "https://proxy.golang.org"
		env["GOSUMDB"] = "sum.golang.org"
		delete(env, "GONOSUMDB")
	}
	return env
}

// Run runs code once in a fresh microVM.
func (c *Controller) Run(ctx context.Context, req sandbox.RunRequest) (sandbox.Result, error) {
	language, ok := c.language(req.Language)
	if !ok {
		return sandbox.Result{}, badRequest("unknown language %q", req.Language)
	}
	if err := checkFiles(req); err != nil {
		return sandbox.Result{}, err
	}
	started := time.Now()
	cl := c.takeWarm()
	if cl == nil {
		spec, _ := c.cfg.spec(sandbox.ImageCode)
		var err error
		if cl, err = c.launch(ctx, spec, "run"); err != nil {
			return sandbox.Result{}, err
		}
	}
	defer func() {
		c.end(cl)
		go c.fillWarm()
	}()
	runCtx, cancel := context.WithTimeout(ctx, sandbox.RunTimeout-time.Since(started))
	defer cancel()
	if err := cl.vm.init(runCtx, codeEnvironment(false), false); err != nil {
		return sandbox.Result{}, fmt.Errorf("init: %w", err)
	}
	result, err := c.runIn(runCtx, cl.vm, language, req)
	if err != nil {
		c.metrics.runs.add("error", 1)
		return sandbox.Result{}, err
	}
	c.metrics.runSeconds.addf("", time.Since(started).Seconds())
	c.metrics.cpuSeconds.addf("run", float64(result.CPUMillis)/1000)
	outcome := "ok"
	if len(result.LimitsHit) > 0 {
		outcome = result.LimitsHit[0]
	}
	c.metrics.runs.add(outcome, 1)
	for _, limit := range result.LimitsHit {
		c.metrics.limits.add(limit, 1)
	}
	if containsString(result.LimitsHit, sandbox.LimitTime) && result.CPUMillis >= result.DurationMillis*9/10 {
		c.metrics.cpuBound.add("", 1)
	}
	return result, nil
}

func (c *Controller) language(name string) (sandbox.Language, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if alias, ok := sandbox.LanguageAliases[name]; ok {
		name = alias
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	language, ok := c.languages[name]
	return language, ok
}

func checkFiles(req sandbox.RunRequest) error {
	if len(req.Code) > sandbox.MaxCodeBytes {
		return badRequest("code is over %d KB", sandbox.MaxCodeBytes>>10)
	}
	if len(req.Files) > sandbox.MaxFiles {
		return badRequest("at most %d files", sandbox.MaxFiles)
	}
	for _, file := range req.Files {
		clean := path.Clean("/" + file.Path)
		if file.Path == "" || clean == "/" || strings.HasPrefix(file.Path, "/") || strings.Contains(file.Path, "..") {
			return badRequest("file paths are relative to the working directory: %q", file.Path)
		}
		if len(file.Content) > sandbox.MaxFileBytes {
			return badRequest("%s is over %d KB", file.Path, sandbox.MaxFileBytes>>10)
		}
	}
	return nil
}

// runIn writes the files and the code into a VM and runs the language's command.
func (c *Controller) runIn(ctx context.Context, machine *vm, language sandbox.Language, req sandbox.RunRequest) (sandbox.Result, error) {
	for _, file := range req.Files {
		if err := machine.writeFile(ctx, file.Path, file.Content, 0o644); err != nil {
			return sandbox.Result{}, fmt.Errorf("writing %s: %w", file.Path, err)
		}
	}
	if err := machine.writeFile(ctx, language.File, []byte(req.Code), 0o644); err != nil {
		return sandbox.Result{}, fmt.Errorf("writing the code: %w", err)
	}
	deadline, _ := ctx.Deadline()
	argv := append(append([]string{}, language.Command...), req.Args...)
	out, err := machine.exec(ctx, sandbox.AgentRequest{Argv: argv, TimeoutMillis: int(time.Until(deadline).Milliseconds())}, []byte(req.Stdin))
	if err != nil {
		return sandbox.Result{}, err
	}
	return out.toResult(machine.cpuSince()), nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// status reports what is running.
func (c *Controller) status() sandbox.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	status := sandbox.Status{
		Version: c.cfg.Version, Paused: c.paused, Running: map[string]int{}, Warm: len(c.warm), Queued: c.waiting,
		MemoryUsed: int64(c.usedMB) << 20, MemoryCap: int64(c.cfg.MemoryCapMB) << 20, Sessions: len(c.sessions),
		Languages: map[string]sandbox.Language{}, Images: c.images(),
	}
	for machine := range c.running {
		status.Running[machine.spec.name]++
	}
	for name, language := range c.languages {
		status.Languages[name] = language
	}
	if c.db != nil {
		status.Databases = c.db.count()
	}
	return status
}

// images lists the image files present.
func (c *Controller) images() []string {
	var found []string
	for _, name := range append([]string{sandbox.ImageCode, sandbox.ImageBrowser}, sandbox.Distributions...) {
		if _, err := os.Stat(filepath.Join(c.cfg.Images, name+".ext4")); err == nil {
			found = append(found, name)
		}
	}
	return found
}

// learnVersions asks a code VM which version of each language it holds, for the API's
// documentation of what runs.
func (c *Controller) learnVersions() {
	commands := map[string]string{
		"python": "python3 --version", "javascript": "node --version", "go": "go version | cut -d' ' -f3",
		"c": "gcc -dumpfullversion", "cpp": "g++ -dumpfullversion", "ruby": "ruby -e 'print RUBY_VERSION'",
		"php": "php -r 'echo PHP_VERSION;'", "bash": "echo $BASH_VERSION",
	}
	spec, ok := c.cfg.spec(sandbox.ImageCode)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cl, err := c.launch(ctx, spec, "versions")
	if err != nil {
		logf("learning language versions: %v", err)
		return
	}
	defer c.end(cl)
	if err := cl.vm.init(ctx, codeEnvironment(false), false); err != nil {
		return
	}
	for name, command := range commands {
		out, err := cl.vm.exec(ctx, sandbox.AgentRequest{Argv: []string{"bash", "-c", command}, TimeoutMillis: 10000}, nil)
		if err != nil || out.status.Code != 0 {
			continue
		}
		version := strings.TrimPrefix(strings.TrimSpace(out.stdout.String()), "go")
		version = strings.TrimPrefix(strings.TrimPrefix(version, "Python "), "v")
		c.mu.Lock()
		language := c.languages[name]
		language.Version = version
		c.languages[name] = language
		c.mu.Unlock()
	}
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, `{"msg":%q}`+"\n", fmt.Sprintf(format, args...))
}
