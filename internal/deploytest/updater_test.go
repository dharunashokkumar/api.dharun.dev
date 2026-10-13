//go:build linux

// Package deploytest exercises deploy/dharun-api-update.sh end to end against a fake
// release server and a fake service. The script needs bash, GNU tar and symlinks, so
// these tests only build on Linux, which is where CI and the server run.
package deploytest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const bundle = "dharun-api-linux-amd64.tar.gz"

type rig struct {
	t         *testing.T
	published string // what the fake release server serves
	home      string // the install directory the updater manages
	releases  *httptest.Server
	service   *httptest.Server
}

func newRig(t *testing.T) *rig {
	r := &rig{t: t, published: t.TempDir(), home: t.TempDir()}
	r.releases = httptest.NewServer(http.FileServer(http.Dir(r.published)))
	t.Cleanup(r.releases.Close)

	// Stands in for the running service: it reports whatever VERSION `current` points
	// at, and is "down" when that release contains a BROKEN marker.
	r.service = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := os.Stat(filepath.Join(r.home, "current", "BROKEN")); err == nil {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		version, err := os.ReadFile(filepath.Join(r.home, "current", "VERSION"))
		if err != nil {
			http.Error(w, "not installed", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, `{"status":"ok","version":"%s","logos":1}`, strings.TrimSpace(string(version)))
	}))
	t.Cleanup(r.service.Close)
	return r
}

// publish lays out a release the way the release workflow does.
func (r *rig) publish(version string, broken, corruptChecksum bool) {
	r.t.Helper()
	var archive bytes.Buffer
	zipped := gzip.NewWriter(&archive)
	files := tar.NewWriter(zipped)
	add := func(name, content string) {
		files.WriteHeader(&tar.Header{Name: "dharun-api/" + name, Mode: 0o644, Size: int64(len(content))})
		files.Write([]byte(content))
	}
	add("VERSION", version+"\n")
	if broken {
		add("BROKEN", "")
	}
	files.Close()
	zipped.Close()

	sum := sha256.Sum256(archive.Bytes())
	digest := hex.EncodeToString(sum[:])
	if corruptChecksum {
		digest = strings.Repeat("0", 64)
	}
	for name, content := range map[string][]byte{
		bundle:             archive.Bytes(),
		bundle + ".sha256": []byte(digest + "  " + bundle + "\n"),
		"VERSION":          []byte(version + "\n"),
	} {
		if err := os.WriteFile(filepath.Join(r.published, name), content, 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *rig) update() (int, string) {
	command := exec.Command("bash", filepath.Join("..", "..", "deploy", "dharun-api-update.sh"))
	command.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"API_RELEASE_URL=" + r.releases.URL,
		"API_HOME=" + r.home,
		"API_HEALTH_URL=" + r.service.URL + "/healthz",
		"API_RESTART_CMD=true",
		"API_HEALTH_TRIES=2",
	}
	output, err := command.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), string(output)
	}
	if err != nil {
		r.t.Fatalf("could not run the updater: %v", err)
	}
	return 0, string(output)
}

func (r *rig) current() string {
	version, _ := os.ReadFile(filepath.Join(r.home, "current", "VERSION"))
	return strings.TrimSpace(string(version))
}

func TestUpdater(t *testing.T) {
	r := newRig(t)
	expect := func(step string, wantCode int, wantVersion, wantOutput string) {
		t.Helper()
		code, output := r.update()
		if code != wantCode {
			t.Fatalf("%s: exit %d, want %d\n%s", step, code, wantCode, output)
		}
		if got := r.current(); got != wantVersion {
			t.Fatalf("%s: running %q, want %q\n%s", step, got, wantVersion, output)
		}
		if !strings.Contains(output, wantOutput) {
			t.Fatalf("%s: output does not mention %q\n%s", step, wantOutput, output)
		}
	}

	r.publish("1.0.0", false, false)
	expect("first install", 0, "1.0.0", "deployed 1.0.0")
	if link, _ := os.Readlink(filepath.Join(r.home, "current")); link != "releases/1.0.0" {
		t.Errorf("current -> %q", link)
	}

	if code, output := r.update(); code != 0 || output != "" {
		t.Errorf("nothing new should be silent and succeed: exit %d, %q", code, output)
	}

	r.publish("1.1.0", false, false)
	expect("upgrade", 0, "1.1.0", "deployed 1.1.0")
	if _, err := os.Stat(filepath.Join(r.home, "releases", "1.0.0")); err != nil {
		t.Error("the previous release should be kept for a manual rollback")
	}

	r.publish("1.2.0", true, false)
	expect("unhealthy release", 1, "1.1.0", "rolled back to 1.1.0")
	if _, err := os.Stat(filepath.Join(r.home, "failed", "1.2.0")); err != nil {
		t.Error("the failed release should be remembered")
	}
	expect("failed release is not retried", 0, "1.1.0", "skipping 1.2.0")

	r.publish("1.3.0", false, true)
	if code, _ := r.update(); code == 0 {
		t.Error("a bundle with a wrong checksum must fail")
	}
	if r.current() != "1.1.0" {
		t.Errorf("a bad bundle was activated: running %q", r.current())
	}
	if _, err := os.Stat(filepath.Join(r.home, "releases", "1.3.0")); err == nil {
		t.Error("a bad bundle was unpacked into releases")
	}

	os.WriteFile(filepath.Join(r.published, "VERSION"), []byte("../../etc\n"), 0o644)
	expect("hostile version string", 1, "1.1.0", "not a version number")

	for _, version := range []string{"1.4.0", "1.5.0", "1.6.0"} {
		r.publish(version, false, false)
		expect("upgrade to "+version, 0, version, "deployed "+version)
	}
	kept, _ := os.ReadDir(filepath.Join(r.home, "releases"))
	if len(kept) != 3 {
		t.Errorf("%d releases kept, want 3", len(kept))
	}
	for _, entry := range kept {
		if entry.Name() == "1.0.0" {
			t.Error("the oldest release was not pruned")
		}
	}
}
