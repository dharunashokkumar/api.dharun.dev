//go:build linux

// Command sandbox-agent runs inside each sandbox microVM. It is the VM's first process:
// it mounts what a Linux system needs, starts itself again to serve the controller over
// vsock, and reaps orphaned processes for as long as the VM lives. The controller sends
// it commands to run, files to write or read, and the time; nothing else reaches the VM,
// which has no network device. With a proxy asked for, it listens on 127.0.0.1:3128 and
// carries each connection over vsock to the controller, which decides where it may go.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// The unprivileged user commands run as. Every image has it.
const (
	sandboxUID = 1000
	sandboxGID = 1000
	workDir    = "/work"
	homeDir    = "/home/sandbox"
	// homeTemplate is the read-only home the code image ships.
	homeTemplate = "/opt/sandbox-home"
)

var version = "dev"

func main() {
	switch {
	case os.Getpid() == 1:
		boot()
	case len(os.Args) > 1 && os.Args[1] == "serve":
		if err := serve(); err != nil {
			fmt.Fprintln(os.Stderr, "sandbox-agent:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "sandbox-agent is the init process of a sandbox microVM")
		os.Exit(2)
	}
}

// boot prepares the system, starts the server and reaps every process that ends up
// with it, until the server stops; then the VM powers off.
func boot() {
	mounts := []struct {
		source, target, kind, data string
		flags                      uintptr
	}{
		{"proc", "/proc", "proc", "", unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC},
		{"sysfs", "/sys", "sysfs", "", unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RDONLY},
		{"devtmpfs", "/dev", "devtmpfs", "mode=0755", unix.MS_NOSUID},
		{"devpts", "/dev/pts", "devpts", "gid=5,mode=0620,ptmxmode=0666", unix.MS_NOSUID | unix.MS_NOEXEC},
		{"tmpfs", "/dev/shm", "tmpfs", "mode=1777,size=64m", unix.MS_NOSUID | unix.MS_NODEV},
		{"tmpfs", "/run", "tmpfs", "mode=0755,size=16m", unix.MS_NOSUID | unix.MS_NODEV},
	}
	for _, m := range mounts {
		os.MkdirAll(m.target, 0o755)
		if err := unix.Mount(m.source, m.target, m.kind, m.flags, m.data); err != nil && !errors.Is(err, unix.EBUSY) {
			fmt.Fprintf(os.Stderr, "mount %s: %v\n", m.target, err)
		}
	}
	prepareScratch()
	loopbackUp()

	server := exec.Command("/proc/self/exe", "serve")
	server.Stdout, server.Stderr = os.Stdout, os.Stderr
	if err := server.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "sandbox-agent: start:", err)
		unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	}
	for {
		var status unix.WaitStatus
		pid, err := unix.Wait4(-1, &status, 0, nil)
		if err == unix.ECHILD {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if pid == server.Process.Pid {
			fmt.Fprintln(os.Stderr, "sandbox-agent: server stopped:", status)
			unix.Sync()
			unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
		}
	}
}

// prepareScratch puts the writable places on the scratch disk when the VM has one; the
// root file system of the code image is read-only and shared by every VM.
func prepareScratch() {
	if _, err := os.Stat("/dev/vdb"); err != nil {
		// A distribution's VM has a writable root of its own and no scratch disk.
		for _, dir := range []string{workDir, homeDir, "/tmp"} {
			os.MkdirAll(dir, 0o755)
		}
		os.Chmod("/tmp", 0o1777)
		os.Chown(workDir, sandboxUID, sandboxGID)
		os.Chown(homeDir, sandboxUID, sandboxGID)
		return
	}
	if err := unix.Mount("/dev/vdb", "/scratch", "ext4", unix.MS_NOSUID|unix.MS_NODEV, ""); err != nil {
		fmt.Fprintln(os.Stderr, "mount scratch:", err)
		return
	}
	places := []struct {
		dir, target string
		mode        os.FileMode
		owned       bool
	}{
		{"/scratch/work", workDir, 0o755, true},
		{"/scratch/tmp", "/tmp", 0o1777, false},
	}
	// The code image ships a prepared home (a warm Go build cache, settings) under
	// /opt/sandbox-home; the sandbox's own changes go over it on the scratch disk.
	if _, err := os.Stat(homeTemplate); err == nil {
		for _, dir := range []string{"/scratch/home-upper", "/scratch/home-work"} {
			os.MkdirAll(dir, 0o755)
		}
		os.Chown("/scratch/home-upper", sandboxUID, sandboxGID)
		if err := unix.Mount("overlay", homeDir, "overlay", unix.MS_NOSUID|unix.MS_NODEV,
			"lowerdir="+homeTemplate+",upperdir=/scratch/home-upper,workdir=/scratch/home-work"); err != nil {
			fmt.Fprintln(os.Stderr, "overlay home:", err)
			places = append(places, struct {
				dir, target string
				mode        os.FileMode
				owned       bool
			}{"/scratch/home", homeDir, 0o755, true})
		}
	} else {
		places = append(places, struct {
			dir, target string
			mode        os.FileMode
			owned       bool
		}{"/scratch/home", homeDir, 0o755, true})
	}
	for _, place := range places {
		os.MkdirAll(place.dir, place.mode)
		os.Chmod(place.dir, place.mode)
		if place.owned {
			os.Chown(place.dir, sandboxUID, sandboxGID)
		}
		if err := unix.Mount(place.dir, place.target, "", unix.MS_BIND, ""); err != nil {
			fmt.Fprintf(os.Stderr, "bind %s: %v\n", place.target, err)
		}
	}
}

func loopbackUp() {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return
	}
	defer unix.Close(fd)
	request, err := unix.NewIfreq("lo")
	if err != nil {
		return
	}
	if unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, request) == nil {
		request.SetUint16(request.Uint16() | unix.IFF_UP)
		unix.IoctlIfreq(fd, unix.SIOCSIFFLAGS, request)
	}
}

// state is what init set for every later command.
var state struct {
	sync.Mutex
	env map[string]string
}

func serve() error {
	// Limits every command inherits. Root is not held to the process count; the sandbox
	// user is, which stops a fork bomb at a number the VM can carry.
	setLimit(unix.RLIMIT_CORE, 0)
	setLimit(unix.RLIMIT_NOFILE, 4096)
	os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	state.env = map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME": homeDir, "USER": "sandbox", "LOGNAME": "sandbox", "LANG": "C.UTF-8", "SHELL": "/bin/bash",
	}
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("vsock: %w", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_ANY, Port: sandbox.AgentPort}); err != nil {
		return fmt.Errorf("vsock bind: %w", err)
	}
	if err := unix.Listen(fd, 64); err != nil {
		return fmt.Errorf("vsock listen: %w", err)
	}
	for {
		conn, _, err := unix.Accept4(fd, unix.SOCK_CLOEXEC)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return fmt.Errorf("vsock accept: %w", err)
		}
		go handle(os.NewFile(uintptr(conn), "vsock"))
	}
}

func setLimit(resource int, value uint64) {
	unix.Setrlimit(resource, &unix.Rlimit{Cur: value, Max: value})
}

func reply(w io.Writer, answer sandbox.AgentReply) {
	encoded, _ := json.Marshal(answer)
	w.Write(append(encoded, '\n'))
}

func handle(conn *os.File) {
	defer conn.Close()
	reader := bufio.NewReaderSize(conn, 64<<10)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return
	}
	var request sandbox.AgentRequest
	if err := json.Unmarshal(line, &request); err != nil {
		reply(conn, sandbox.AgentReply{Error: "bad request: " + err.Error()})
		return
	}
	switch request.Op {
	case sandbox.OpPing:
		reply(conn, sandbox.AgentReply{Facts: facts()})
	case sandbox.OpInit:
		reply(conn, sandbox.AgentReply{Error: errorText(initialise(request))})
	case sandbox.OpExec:
		run(conn, reader, request)
	case sandbox.OpWrite:
		reply(conn, sandbox.AgentReply{Error: errorText(writeFile(reader, request))})
	case sandbox.OpRead:
		readFile(conn, request)
	case sandbox.OpList:
		entries, err := list(request.Path)
		reply(conn, sandbox.AgentReply{Entries: entries, Error: errorText(err)})
	default:
		reply(conn, sandbox.AgentReply{Error: "unknown op " + request.Op})
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func facts() map[string]string {
	var uname unix.Utsname
	unix.Uname(&uname)
	return map[string]string{"agent": version, "kernel": unix.ByteSliceToString(uname.Release[:])}
}

// initialise sets the clock (a VM starts with none of its own worth trusting), stirs the
// random pool with the controller's bytes, and keeps the environment for every command.
func initialise(request sandbox.AgentRequest) error {
	if request.TimeUnixNano > 0 {
		tv := unix.NsecToTimeval(request.TimeUnixNano)
		if err := unix.Settimeofday(&tv); err != nil {
			return fmt.Errorf("clock: %w", err)
		}
	}
	if len(request.Entropy) > 0 {
		if random, err := os.OpenFile("/dev/urandom", os.O_WRONLY, 0); err == nil {
			random.Write(request.Entropy)
			random.Close()
		}
	}
	if request.Hostname != "" {
		unix.Sethostname([]byte(request.Hostname))
	}
	state.Lock()
	for name, value := range request.Env {
		state.env[name] = value
	}
	state.Unlock()
	if nproc, err := strconv.Atoi(request.Env["SANDBOX_NPROC"]); err == nil && nproc > 0 {
		setLimit(unix.RLIMIT_NPROC, uint64(nproc))
	} else {
		setLimit(unix.RLIMIT_NPROC, 256)
	}
	if request.Proxy {
		go guestProxy()
	}
	return nil
}

// guestProxy accepts connections on 127.0.0.1:3128 and carries each one to the host.
func guestProxy() {
	listener, err := net.Listen("tcp4", sandbox.GuestProxy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proxy:", err)
		return
	}
	for {
		local, err := listener.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer local.Close()
			fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
			if err != nil {
				return
			}
			if err := unix.Connect(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_HOST, Port: sandbox.ProxyPort}); err != nil {
				unix.Close(fd)
				return
			}
			remote := os.NewFile(uintptr(fd), "vsock-proxy")
			defer remote.Close()
			done := make(chan struct{}, 2)
			go func() { io.Copy(remote, local); done <- struct{}{} }()
			go func() { io.Copy(local, remote); done <- struct{}{} }()
			<-done
		}()
	}
}

func environment(extra map[string]string, tty bool) []string {
	state.Lock()
	merged := make(map[string]string, len(state.env)+len(extra)+1)
	for name, value := range state.env {
		merged[name] = value
	}
	state.Unlock()
	if tty {
		merged["TERM"] = "xterm-256color"
	}
	for name, value := range extra {
		merged[name] = value
	}
	env := make([]string, 0, len(merged))
	for name, value := range merged {
		if !strings.HasPrefix(name, "SANDBOX_") {
			env = append(env, name+"="+value)
		}
	}
	return env
}

// frameWriter sends frames of one kind, from several goroutines in turn.
type frameWriter struct {
	mu   *sync.Mutex
	conn io.Writer
}

func (f frameWriter) send(kind byte, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return sandbox.WriteFrame(f.conn, kind, payload)
}

// capped forwards output as frames up to a limit and counts the rest.
type capped struct {
	out       frameWriter
	kind      byte
	left      int
	truncated bool
}

func (c *capped) Write(p []byte) (int, error) {
	if c.left <= 0 {
		c.truncated = c.truncated || len(p) > 0
		return len(p), nil
	}
	chunk := p
	if len(chunk) > c.left {
		chunk = chunk[:c.left]
		c.truncated = true
	}
	for len(chunk) > 0 {
		piece := chunk[:min(len(chunk), 32<<10)]
		if err := c.out.send(c.kind, piece); err != nil {
			return 0, err
		}
		chunk = chunk[len(piece):]
		c.left -= len(piece)
	}
	return len(p), nil
}

func oomKills() int {
	raw, err := os.ReadFile("/proc/vmstat")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if value, ok := strings.CutPrefix(line, "oom_kill "); ok {
			n, _ := strconv.Atoi(value)
			return n
		}
	}
	return 0
}

// run starts a command and streams its output as frames until it ends.
func run(conn *os.File, reader *bufio.Reader, request sandbox.AgentRequest) {
	out := frameWriter{mu: &sync.Mutex{}, conn: conn}
	fail := func(err error) { out.send(sandbox.FrameError, []byte(err.Error())) }
	if len(request.Argv) == 0 {
		fail(errors.New("no command"))
		return
	}
	limit := request.MaxOutput
	if limit <= 0 {
		limit = sandbox.MaxOutputBytes
	}
	cwd := request.Cwd
	if cwd == "" {
		cwd = workDir
	}
	cmd := exec.Command(request.Argv[0], request.Argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = environment(request.Env, request.TTY != nil)
	// The command is found on its own PATH; the agent itself started with none, so
	// exec.Command's own lookup may have failed and must be cleared.
	if path, err := lookPath(request.Argv[0], cmd.Env); err == nil {
		cmd.Path, cmd.Err = path, nil
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if !request.Root {
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: sandboxUID, Gid: sandboxGID, Groups: []uint32{sandboxGID}}
	}
	stdout := &capped{out: out, kind: sandbox.FrameStdout, left: limit}
	stderr := &capped{out: out, kind: sandbox.FrameStderr, left: limit}

	var master, slave *os.File
	var stdin io.WriteCloser
	if request.TTY != nil {
		var err error
		master, slave, err = openPTY(request.TTY)
		if err != nil {
			fail(err)
			return
		}
		defer master.Close()
		cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
		cmd.SysProcAttr.Setctty = true
		cmd.SysProcAttr.Ctty = 0
		stdin = master
	} else {
		cmd.Stdout, cmd.Stderr = stdout, stderr
		if request.Stdin {
			pipe, err := cmd.StdinPipe()
			if err != nil {
				fail(err)
				return
			}
			stdin = pipe
		}
	}
	// Output still held by a process that outlived the command is not waited for long.
	cmd.WaitDelay = 500 * time.Millisecond
	oomBefore := oomKills()
	started := time.Now()
	if err := cmd.Start(); err != nil {
		if slave != nil {
			slave.Close()
		}
		fail(err)
		return
	}
	copied := make(chan struct{})
	if master != nil {
		// The command holds the terminal now; the copy ends when its last holder does.
		slave.Close()
		go func() { io.Copy(stdout, master); close(copied) }()
	} else {
		close(copied)
	}
	var timedOut, finished bool
	var mu sync.Mutex
	kill := func(timeout bool) {
		mu.Lock()
		defer mu.Unlock()
		if finished {
			return // the process is gone and its number may already be someone else's
		}
		timedOut = timedOut || timeout
		// The command leads its own session: the whole group goes.
		unix.Kill(-cmd.Process.Pid, unix.SIGKILL)
	}
	if request.TimeoutMillis > 0 {
		timer := time.AfterFunc(time.Duration(request.TimeoutMillis)*time.Millisecond, func() { kill(true) })
		defer timer.Stop()
	}
	// Frames from the controller: stdin, its end, a new terminal size, or kill.
	go func() {
		for {
			kind, payload, err := sandbox.ReadFrame(reader)
			if err != nil {
				// The controller went away: nobody will read the output.
				kill(false)
				return
			}
			switch kind {
			case sandbox.FrameStdin:
				if stdin != nil {
					stdin.Write(payload)
				}
			case sandbox.FrameCloseStdin:
				if stdin != nil && master == nil {
					stdin.Close()
				}
			case sandbox.FrameResize:
				var size sandbox.TermSize
				if master != nil && json.Unmarshal(payload, &size) == nil {
					resize(master, &size)
				}
			case sandbox.FrameKill:
				kill(false)
			}
		}
	}()
	err := cmd.Wait()
	mu.Lock()
	finished = true
	mu.Unlock()
	// The terminal's last output, unless a process left behind still holds it.
	select {
	case <-copied:
	case <-time.After(200 * time.Millisecond):
	}
	status := sandbox.ExitStatus{DurationMillis: int(time.Since(started).Milliseconds()),
		StdoutTruncated: stdout.truncated, StderrTruncated: stderr.truncated, TimedOut: timedOut}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		if waitStatus, ok := exitErr.Sys().(syscall.WaitStatus); ok && waitStatus.Signaled() {
			status.Signal = unix.SignalName(waitStatus.Signal())
			status.Code = 128 + int(waitStatus.Signal())
		} else {
			status.Code = exitErr.ExitCode()
		}
	default:
		if !errors.Is(err, exec.ErrWaitDelay) {
			fail(err)
			return
		}
	}
	status.OutOfMemory = oomKills() > oomBefore
	encoded, _ := json.Marshal(status)
	out.send(sandbox.FrameExit, encoded)
}

// lookPath finds a command on the PATH of the command's own environment.
func lookPath(name string, env []string) (string, error) {
	if strings.Contains(name, "/") {
		return name, nil
	}
	path := ""
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			path = value
		}
	}
	for _, dir := range filepath.SplitList(path) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func openPTY(size *sandbox.TermSize) (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		master.Close()
		return nil, nil, err
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		master.Close()
		return nil, nil, err
	}
	name := "/dev/pts/" + strconv.Itoa(number)
	slave, err := os.OpenFile(name, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, err
	}
	os.Chown(name, sandboxUID, 5)
	resize(master, size)
	return master, slave, nil
}

func resize(master *os.File, size *sandbox.TermSize) {
	if size.Cols == 0 || size.Rows == 0 {
		return
	}
	unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: size.Rows, Col: size.Cols})
}

// place resolves a path: a relative one is under the working directory.
func place(path string) string {
	if path == "" {
		return workDir
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, path)
	}
	return filepath.Clean(path)
}

// writeFile writes request.Size bytes that follow the request line. Files under the
// working and home directories belong to the sandbox user.
func writeFile(reader io.Reader, request sandbox.AgentRequest) error {
	if request.Size < 0 || request.Size > 64<<20 {
		return errors.New("file too large")
	}
	target := place(request.Path)
	mode := os.FileMode(request.Mode & 0o777)
	if mode == 0 {
		mode = 0o644
	}
	owned := strings.HasPrefix(target, workDir+"/") || strings.HasPrefix(target, homeDir+"/") || strings.HasPrefix(target, "/tmp/")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if owned {
		// Directories made on the way belong to the user too.
		for dir := filepath.Dir(target); dir != workDir && dir != homeDir && dir != "/tmp" && len(dir) > 1; dir = filepath.Dir(dir) {
			os.Chown(dir, sandboxUID, sandboxGID)
		}
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(file, reader, request.Size); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if owned {
		os.Chown(target, sandboxUID, sandboxGID)
	}
	return nil
}

func readFile(conn io.Writer, request sandbox.AgentRequest) {
	file, err := os.Open(place(request.Path))
	if err != nil {
		reply(conn, sandbox.AgentReply{Error: err.Error()})
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		reply(conn, sandbox.AgentReply{Error: "not a file"})
		return
	}
	size := info.Size()
	if request.Size > 0 && size > request.Size {
		reply(conn, sandbox.AgentReply{Error: "file too large", Size: size})
		return
	}
	reply(conn, sandbox.AgentReply{Size: size})
	io.CopyN(conn, file, size)
}

func list(path string) ([]sandbox.Entry, error) {
	entries, err := os.ReadDir(place(path))
	if err != nil {
		return nil, err
	}
	out := make([]sandbox.Entry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, sandbox.Entry{Name: entry.Name(), Size: info.Size(), Dir: entry.IsDir(), Mode: uint32(info.Mode().Perm())})
		if len(out) == 1000 {
			break
		}
	}
	return out, nil
}
