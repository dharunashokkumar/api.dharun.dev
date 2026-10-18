//go:build linux

package sandboxd

import (
	"context"
	"crypto/rand"
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

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// imageSpec is how a microVM of one image is made.
type imageSpec struct {
	name     string
	memoryMB int
	vcpus    int
	// writable images get a copy of their root file system of their own (the small
	// distributions); the others share one read-only copy and get a scratch disk.
	writable bool
	nproc    int    // processes and threads the sandbox user may have
	policy   string // what the proxy lets through: policyNone, policyRegistries, policyBrowser
	shell    []string
}

func (c *Config) spec(image string) (imageSpec, bool) {
	switch image {
	case sandbox.ImageCode:
		return imageSpec{name: image, memoryMB: c.CodeMemoryMB, vcpus: 1, nproc: 128, shell: []string{"/bin/bash", "-l"}}, true
	case sandbox.ImageBrowser:
		return imageSpec{name: image, memoryMB: c.BrowserMemoryMB, vcpus: 2, nproc: 1024, policy: policyBrowser}, true
	case sandbox.ImageAlpine:
		return imageSpec{name: image, memoryMB: c.CodeMemoryMB, vcpus: 1, writable: true, nproc: 128, shell: []string{"/bin/sh", "-l"}}, true
	case sandbox.ImageArch, sandbox.ImageDebian, sandbox.ImageFedora:
		return imageSpec{name: image, memoryMB: c.CodeMemoryMB, vcpus: 1, writable: true, nproc: 128, shell: []string{"/bin/bash", "-l"}}, true
	}
	return imageSpec{}, false
}

// vm is one Firecracker microVM, run by the jailer under its own user ID in its own
// chroot and cgroup.
type vm struct {
	id      string
	spec    imageSpec
	uid     int
	chroot  string // the jail's root, as the host sees it
	cgroup  string // the VM's cgroup directory
	cmd     *exec.Cmd
	exited  chan struct{}
	proxy   *proxyListener
	started time.Time
	booted  time.Duration

	mu      sync.Mutex
	stopped bool
	cpuSeen int64 // processor microseconds already reported
}

func randomName(prefix string) string {
	raw := make([]byte, 8)
	rand.Read(raw)
	return prefix + fmt.Sprintf("%x", raw)
}

// firecrackerConfig is the --config-file document.
type firecrackerConfig struct {
	BootSource struct {
		KernelImagePath string `json:"kernel_image_path"`
		BootArgs        string `json:"boot_args"`
	} `json:"boot-source"`
	Drives        []drive       `json:"drives"`
	MachineConfig machineConfig `json:"machine-config"`
	Vsock         vsockCfg      `json:"vsock"`
	Entropy       struct{}      `json:"entropy"`
}

type drive struct {
	DriveID      string       `json:"drive_id"`
	PathOnHost   string       `json:"path_on_host"`
	IsRootDevice bool         `json:"is_root_device"`
	IsReadOnly   bool         `json:"is_read_only"`
	RateLimiter  *rateLimiter `json:"rate_limiter,omitempty"`
}

type rateLimiter struct {
	Bandwidth *tokenBucket `json:"bandwidth,omitempty"`
	Ops       *tokenBucket `json:"ops,omitempty"`
}

type tokenBucket struct {
	Size       int64 `json:"size"`
	RefillTime int64 `json:"refill_time"` // milliseconds
}

type machineConfig struct {
	VCPUCount  int  `json:"vcpu_count"`
	MemSizeMiB int  `json:"mem_size_mib"`
	SMT        bool `json:"smt"`
}

type vsockCfg struct {
	GuestCID int    `json:"guest_cid"`
	UDSPath  string `json:"uds_path"`
}

// diskLimit keeps one VM's disk traffic from starving the others: 50 MB/s and 2,000
// operations a second.
var diskLimit = &rateLimiter{
	Bandwidth: &tokenBucket{Size: 50 << 20, RefillTime: 1000},
	Ops:       &tokenBucket{Size: 2000, RefillTime: 1000},
}

// startVM boots a microVM of an image and waits until its agent answers.
func (c *Controller) startVM(ctx context.Context, spec imageSpec) (*vm, error) {
	uid, err := c.uids.take()
	if err != nil {
		return nil, err
	}
	machine := &vm{id: randomName("sb"), spec: spec, uid: uid, exited: make(chan struct{}), started: time.Now()}
	execName := filepath.Base(c.cfg.Firecracker)
	machine.chroot = filepath.Join(c.cfg.JailRoot, execName, machine.id, "root")
	machine.cgroup = filepath.Join("/sys/fs/cgroup", c.cfg.CgroupParent, machine.id)
	fail := func(err error) (*vm, error) {
		machine.destroy(c)
		return nil, err
	}
	if err := os.MkdirAll(machine.chroot, 0o755); err != nil {
		return fail(err)
	}
	if err := linkOrCopy(c.cfg.Kernel, filepath.Join(machine.chroot, "vmlinux")); err != nil {
		return fail(fmt.Errorf("kernel: %w", err))
	}
	image := filepath.Join(c.cfg.Images, spec.name+".ext4")
	rootfs := filepath.Join(machine.chroot, "rootfs.ext4")
	var config firecrackerConfig
	if spec.writable {
		if err := copySparse(image, rootfs); err != nil {
			return fail(fmt.Errorf("image %s: %w", spec.name, err))
		}
		os.Chown(rootfs, uid, uid)
		config.Drives = []drive{{DriveID: "rootfs", PathOnHost: "/rootfs.ext4", IsRootDevice: true, RateLimiter: diskLimit}}
	} else {
		if err := linkOrCopy(image, rootfs); err != nil {
			return fail(fmt.Errorf("image %s: %w", spec.name, err))
		}
		scratch := filepath.Join(machine.chroot, "scratch.ext4")
		if err := copySparse(c.cfg.Scratch, scratch); err != nil {
			return fail(fmt.Errorf("scratch disk: %w", err))
		}
		os.Chown(scratch, uid, uid)
		config.Drives = []drive{
			{DriveID: "rootfs", PathOnHost: "/rootfs.ext4", IsRootDevice: true, IsReadOnly: true},
			{DriveID: "scratch", PathOnHost: "/scratch.ext4", RateLimiter: diskLimit},
		}
	}
	rootMode := "ro"
	if spec.writable {
		rootMode = "rw"
	}
	config.BootSource.KernelImagePath = "/vmlinux"
	// The agent is the first process. panic=1 and reboot=k make a crash end the VM.
	config.BootSource.BootArgs = "console=ttyS0 reboot=k panic=1 pci=off quiet loglevel=1 " + rootMode +
		" init=/sandbox-agent random.trust_cpu=on"
	config.MachineConfig = machineConfig{VCPUCount: spec.vcpus, MemSizeMiB: spec.memoryMB}
	config.Vsock = vsockCfg{GuestCID: 3, UDSPath: "/v.sock"}
	encoded, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(machine.chroot, "vm.json"), encoded, 0o644); err != nil {
		return fail(err)
	}
	if spec.policy != policyNone {
		listener, err := c.proxy.listen(filepath.Join(machine.chroot, "v.sock_"+strconv.Itoa(sandbox.ProxyPort)), uid, spec.policy)
		if err != nil {
			return fail(fmt.Errorf("proxy: %w", err))
		}
		machine.proxy = listener
	}

	// The VM may use one core per vCPU, its memory plus what Firecracker itself needs,
	// and cannot write files beyond its disks on the host.
	args := []string{
		"--id", machine.id, "--exec-file", c.cfg.Firecracker, "--uid", strconv.Itoa(uid), "--gid", strconv.Itoa(uid),
		"--chroot-base-dir", c.cfg.JailRoot, "--cgroup-version", "2", "--parent-cgroup", c.cfg.CgroupParent,
		"--cgroup", fmt.Sprintf("cpu.max=%d 100000", 100000*spec.vcpus),
		"--cgroup", fmt.Sprintf("memory.max=%d", int64(spec.memoryMB+vmOverheadMB)<<20),
		"--resource-limit", "fsize=" + strconv.FormatInt(8<<30, 10), "--resource-limit", "no-file=1024",
		"--", "--config-file", "/vm.json", "--no-api", "--level", "Error",
	}
	machine.cmd = exec.Command(c.cfg.Jailer, args...)
	machine.cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	console, _ := os.OpenFile(filepath.Join(c.cfg.StateDir, "console", machine.id+".log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if console != nil {
		// The first lines of a guest's console help when a VM does not come up; the file is
		// deleted with the VM.
		machine.cmd.Stdout, machine.cmd.Stderr = &limitedWriter{w: console, left: 64 << 10}, &limitedWriter{w: console, left: 64 << 10}
	}
	machine.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := machine.cmd.Start(); err != nil {
		if console != nil {
			console.Close()
		}
		return fail(fmt.Errorf("jailer: %w", err))
	}
	go func() {
		machine.cmd.Wait()
		if console != nil {
			console.Close()
		}
		close(machine.exited)
	}()

	// Boot: the agent answers once the kernel has started it.
	deadline := time.Now().Add(c.cfg.BootTimeout)
	for {
		conn, err := machine.dial(ctx)
		if err == nil {
			err = request(conn, sandbox.AgentRequest{Op: sandbox.OpPing}, nil)
			conn.Close()
			if err == nil {
				break
			}
		}
		select {
		case <-machine.exited:
			return fail(fmt.Errorf("the microVM stopped while booting (%s)", c.consoleTail(machine.id)))
		case <-ctx.Done():
			return fail(ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return fail(fmt.Errorf("the microVM did not boot in %s (%s)", c.cfg.BootTimeout, c.consoleTail(machine.id)))
		}
	}
	machine.booted = time.Since(machine.started)
	return machine, nil
}

// vmOverheadMB is what Firecracker itself takes beside the guest's memory.
const vmOverheadMB = 48

func (c *Controller) consoleTail(id string) string {
	raw, err := os.ReadFile(filepath.Join(c.cfg.StateDir, "console", id+".log"))
	if err != nil || len(raw) == 0 {
		return "no console output"
	}
	if len(raw) > 400 {
		raw = raw[len(raw)-400:]
	}
	return strings.TrimSpace(string(raw))
}

// dial opens a connection to the agent through Firecracker's vsock socket.
func (m *vm) dial(ctx context.Context) (net.Conn, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", filepath.Join(m.chroot, "v.sock"))
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintf(conn, "CONNECT %d\n", sandbox.AgentPort); err != nil {
		conn.Close()
		return nil, err
	}
	// Firecracker answers "OK <port>\n"; read it byte by byte so nothing after it is lost.
	var line []byte
	one := make([]byte, 1)
	for len(line) < 64 {
		if _, err := conn.Read(one); err != nil {
			conn.Close()
			return nil, err
		}
		if one[0] == '\n' {
			break
		}
		line = append(line, one[0])
	}
	if !strings.HasPrefix(string(line), "OK ") {
		conn.Close()
		return nil, fmt.Errorf("vsock handshake: %q", line)
	}
	conn.SetDeadline(time.Time{})
	return conn, nil
}

// request sends a request line and reads the reply line into reply.
func request(conn net.Conn, req sandbox.AgentRequest, reply *sandbox.AgentReply) error {
	encoded, _ := json.Marshal(req)
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{})
	if _, err := conn.Write(append(encoded, '\n')); err != nil {
		return err
	}
	line, err := readLine(conn)
	if err != nil {
		return err
	}
	var answer sandbox.AgentReply
	if err := json.Unmarshal(line, &answer); err != nil {
		return err
	}
	if answer.Error != "" {
		return errors.New(answer.Error)
	}
	if reply != nil {
		*reply = answer
	}
	return nil
}

// readLine reads up to a newline without reading past it.
func readLine(r io.Reader) ([]byte, error) {
	var line []byte
	one := make([]byte, 1)
	for len(line) < 1<<20 {
		if _, err := r.Read(one); err != nil {
			return nil, err
		}
		if one[0] == '\n' {
			return line, nil
		}
		line = append(line, one[0])
	}
	return nil, errors.New("reply line too long")
}

// init sets the guest's clock, seeds its random pool and gives it its environment.
func (m *vm) init(ctx context.Context, env map[string]string, proxy bool) error {
	conn, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	entropy := make([]byte, 64)
	rand.Read(entropy)
	all := map[string]string{"SANDBOX_NPROC": strconv.Itoa(m.spec.nproc)}
	for name, value := range env {
		all[name] = value
	}
	return request(conn, sandbox.AgentRequest{Op: sandbox.OpInit, TimeUnixNano: time.Now().UnixNano(), Entropy: entropy,
		Hostname: "sandbox", Proxy: proxy, Env: all}, nil)
}

// writeFile puts a file in the guest.
func (m *vm) writeFile(ctx context.Context, path string, data []byte, mode uint32) error {
	conn, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	encoded, _ := json.Marshal(sandbox.AgentRequest{Op: sandbox.OpWrite, Path: path, Mode: mode, Size: int64(len(data))})
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := conn.Write(append(append(encoded, '\n'), data...)); err != nil {
		return err
	}
	line, err := readLine(conn)
	if err != nil {
		return err
	}
	var answer sandbox.AgentReply
	json.Unmarshal(line, &answer)
	if answer.Error != "" {
		return errors.New(answer.Error)
	}
	return nil
}

// readFile reads a file of at most limit bytes from the guest.
func (m *vm) readFile(ctx context.Context, path string, limit int64) ([]byte, error) {
	conn, err := m.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var answer sandbox.AgentReply
	if err := request(conn, sandbox.AgentRequest{Op: sandbox.OpRead, Path: path, Size: limit}, &answer); err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	data := make([]byte, answer.Size)
	_, err = io.ReadFull(conn, data)
	return data, err
}

func (m *vm) list(ctx context.Context, path string) ([]sandbox.Entry, error) {
	conn, err := m.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var answer sandbox.AgentReply
	err = request(conn, sandbox.AgentRequest{Op: sandbox.OpList, Path: path}, &answer)
	return answer.Entries, err
}

// cpuMicros is the processor time the VM has used, from its cgroup.
func (m *vm) cpuMicros() int64 {
	raw, err := os.ReadFile(filepath.Join(m.cgroup, "cpu.stat"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if value, ok := strings.CutPrefix(line, "usage_usec "); ok {
			n, _ := strconv.ParseInt(value, 10, 64)
			return n
		}
	}
	return 0
}

// cpuSince returns processor milliseconds used since the last call.
func (m *vm) cpuSince() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.cpuMicros()
	if now < m.cpuSeen {
		return 0
	}
	used := now - m.cpuSeen
	m.cpuSeen = now
	return int(used / 1000)
}

// destroy stops the VM and removes everything it had on the host.
func (m *vm) destroy(c *Controller) {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		syscall.Kill(-m.cmd.Process.Pid, syscall.SIGKILL)
		m.cmd.Process.Kill()
		select {
		case <-m.exited:
		case <-time.After(5 * time.Second):
		}
	}
	if m.proxy != nil {
		m.proxy.close()
	}
	os.RemoveAll(filepath.Dir(m.chroot))
	// The cgroup goes once its processes have: retry briefly.
	for range 20 {
		if err := os.Remove(m.cgroup); err == nil || errors.Is(err, os.ErrNotExist) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	os.Remove(filepath.Join(c.cfg.StateDir, "console", m.id+".log"))
	c.uids.give(m.uid)
}

// linkOrCopy hard-links a file into a jail, or copies it when the two are on different
// file systems.
func linkOrCopy(from, to string) error {
	if err := os.Link(from, to); err == nil {
		return nil
	}
	return copySparse(from, to)
}

// copySparse copies a disk image, keeping its holes (cp makes a reflink where the file
// system can).
func copySparse(from, to string) error {
	out, err := exec.Command("cp", "--sparse=always", "--reflink=auto", from, to).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type limitedWriter struct {
	mu   sync.Mutex
	w    io.Writer
	left int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.left > 0 {
		chunk := p[:min(len(p), l.left)]
		l.w.Write(chunk)
		l.left -= len(chunk)
	}
	return len(p), nil
}

// uidPool hands out a user ID per running VM, so no two jails share one.
type uidPool struct {
	mu   sync.Mutex
	free []int
}

func newUIDPool(base, count int) *uidPool {
	pool := &uidPool{}
	for uid := base + count - 1; uid >= base; uid-- {
		pool.free = append(pool.free, uid)
	}
	return pool
}

func (p *uidPool) take() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.free) == 0 {
		return 0, errors.New("no free user IDs")
	}
	uid := p.free[len(p.free)-1]
	p.free = p.free[:len(p.free)-1]
	return uid, nil
}

func (p *uidPool) give(uid int) {
	p.mu.Lock()
	p.free = append(p.free, uid)
	p.mu.Unlock()
}
