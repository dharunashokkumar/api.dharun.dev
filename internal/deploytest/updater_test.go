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
	restart   string // the restart command the updater runs
	env       []string
}

func newRig(t *testing.T) *rig {
	r := &rig{t: t, published: t.TempDir(), home: t.TempDir(), restart: "true"}
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
	var markers []string
	if broken {
		markers = append(markers, "BROKEN")
	}
	r.publishWith(version, corruptChecksum, markers...)
}

// publishWith publishes a release holding empty marker files beside VERSION.
func (r *rig) publishWith(version string, corruptChecksum bool, markers ...string) {
	r.t.Helper()
	var archive bytes.Buffer
	zipped := gzip.NewWriter(&archive)
	files := tar.NewWriter(zipped)
	add := func(name, content string) {
		files.WriteHeader(&tar.Header{Name: "dharun-api/" + name, Mode: 0o644, Size: int64(len(content))})
		files.Write([]byte(content))
	}
	add("VERSION", version+"\n")
	for _, marker := range markers {
		add(marker, "")
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
		"API_RESTART_CMD=" + r.restart,
		"API_HEALTH_TRIES=2",
	}
	command.Env = append(command.Env, r.env...)
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

	// A restart that fails is a failed deploy too: rolled back and remembered, not left
	// running the new release for the next run to mistake for a finished deploy.
	r.restart = "test ! -e " + filepath.Join(r.home, "current", "NORESTART")
	r.publishWith("1.2.5", false, "NORESTART")
	expect("restart fails", 1, "1.1.0", "rolled back to 1.1.0")
	if _, err := os.Stat(filepath.Join(r.home, "failed", "1.2.5")); err != nil {
		t.Error("a release whose restart failed should be remembered")
	}
	expect("restart failure is not retried", 0, "1.1.0", "skipping 1.2.5")
	r.restart = "true"

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

func TestUpdaterPurgesCloudflareCache(t *testing.T) {
	r := newRig(t)
	var requests []string
	answer := `{"success": true, "errors": [], "messages": [], "result": {"id": "x"}}`
	cloudflare := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(req.Body)
		requests = append(requests, req.Method+" "+req.URL.Path+" "+req.Header.Get("Authorization")+" "+body.String())
		fmt.Fprint(w, answer)
	}))
	t.Cleanup(cloudflare.Close)

	// Without credentials nothing is purged.
	r.publish("1.0.0", false, false)
	if code, output := r.update(); code != 0 || strings.Contains(output, "purge") || len(requests) != 0 {
		t.Fatalf("without a token: exit %d, %d requests\n%s", code, len(requests), output)
	}

	r.env = []string{"CLOUDFLARE_API_TOKEN=secret-token", "CLOUDFLARE_ZONE_ID=zone123",
		"CLOUDFLARE_API_URL=" + cloudflare.URL, "PUBLIC_ORIGIN=https://api.example.test"}
	r.publish("1.1.0", false, false)
	code, output := r.update()
	if code != 0 || !strings.Contains(output, "purged Cloudflare's cache for api.example.test") {
		t.Fatalf("exit %d\n%s", code, output)
	}
	want := `POST /zones/zone123/purge_cache Bearer secret-token {"hosts":["api.example.test"]}`
	if len(requests) != 1 || requests[0] != want {
		t.Fatalf("requests: %q, want %q", requests, want)
	}

	// A refused purge is reported but the deploy stands.
	answer = `{"success": false, "errors": [{"code": 10000, "message": "Authentication error"}]}`
	r.publish("1.2.0", false, false)
	code, output = r.update()
	if code != 0 || r.current() != "1.2.0" || !strings.Contains(output, "WARNING: Cloudflare cache purge for api.example.test failed") {
		t.Fatalf("refused purge: exit %d, running %s\n%s", code, r.current(), output)
	}

	// No purge after a failed deploy: the old release was never replaced.
	requests = nil
	r.publish("1.3.0", true, false)
	if code, _ := r.update(); code != 1 || len(requests) != 0 {
		t.Errorf("failed deploy: exit %d, %d purge requests", code, len(requests))
	}
}
