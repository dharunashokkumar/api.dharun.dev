//go:build linux

package sandboxd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// The microVMs themselves are checked with real Firecracker in the round 5 rig; these
// cover the decisions made around them.

func TestProxyPolicy(t *testing.T) {
	for _, c := range []struct {
		policy, host string
		port         int
		want         bool
	}{
		{policyRegistries, "pypi.org", 443, true},
		{policyRegistries, "files.pythonhosted.org", 443, true},
		{policyRegistries, "Registry.NPMjs.org.", 443, true},
		{policyRegistries, "pypi.org", 80, false},
		{policyRegistries, "example.com", 443, false},
		{policyRegistries, "pypi.org.evil.com", 443, false},
		{policyBrowser, "example.com", 443, true},
		{policyBrowser, "example.com", 80, true},
		{policyBrowser, "example.com", 22, false},
		{policyNone, "pypi.org", 443, false},
	} {
		if got := allowed(c.policy, c.host, c.port); got != c.want {
			t.Errorf("%q %s:%d = %v, want %v", c.policy, c.host, c.port, got, c.want)
		}
	}
}

func TestCheckFiles(t *testing.T) {
	ok := sandbox.RunRequest{Code: "x", Files: []sandbox.File{{Path: "data/a.csv", Content: []byte("1")}}}
	if err := checkFiles(ok); err != nil {
		t.Errorf("a relative path was refused: %v", err)
	}
	for _, path := range []string{"/etc/passwd", "../x", "a/../../b", ""} {
		if err := checkFiles(sandbox.RunRequest{Files: []sandbox.File{{Path: path}}}); !errors.Is(err, errBadRequest) {
			t.Errorf("%q was accepted", path)
		}
	}
	if err := checkFiles(sandbox.RunRequest{Code: string(make([]byte, sandbox.MaxCodeBytes+1))}); err == nil {
		t.Error("oversized code was accepted")
	}
}

func TestUIDPool(t *testing.T) {
	pool := newUIDPool(61000, 2)
	a, _ := pool.take()
	b, _ := pool.take()
	if a == b || a < 61000 || b > 61001 {
		t.Errorf("uids %d %d", a, b)
	}
	if _, err := pool.take(); err == nil {
		t.Error("a third uid from a pool of two")
	}
	pool.give(a)
	if again, err := pool.take(); err != nil || again != a {
		t.Errorf("returned uid not reused: %d %v", again, err)
	}
}

func TestMemoryBudget(t *testing.T) {
	c := &Controller{cfg: Config{MemoryCapMB: 1000, QueueWait: 200 * time.Millisecond}, metrics: &metrics{},
		freed: make(chan struct{}), running: map[*vm]bool{}, sessions: map[string]*session{}}
	first, err := c.reserve(context.Background(), 600, "test")
	if err != nil {
		t.Fatal(err)
	}
	// A second that does not fit waits, and gets its turn when the first gives back.
	got := make(chan error, 1)
	go func() {
		release, err := c.reserve(context.Background(), 600, "test")
		if release != nil {
			defer release()
		}
		got <- err
	}()
	time.Sleep(50 * time.Millisecond)
	first()
	first() // giving back twice gives back once
	if err := <-got; err != nil {
		t.Fatalf("the waiting request: %v", err)
	}
	// One that never fits is refused as busy after the wait.
	hold, _ := c.reserve(context.Background(), 900, "test")
	started := time.Now()
	if _, err := c.reserve(context.Background(), 200, "test"); !errors.Is(err, errBusy) || time.Since(started) < 150*time.Millisecond {
		t.Errorf("over budget: %v after %s", err, time.Since(started))
	}
	hold()
	if c.usedMB != 0 {
		t.Errorf("%d MB still counted", c.usedMB)
	}
}
