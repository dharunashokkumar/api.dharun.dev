package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestClients runs the generated JavaScript and Python clients against the test server:
// a lookup, query options, a long text as a body, an image answer, a request bin, and an
// error. Skipped where node or python is missing; CI has both.
func TestClients(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Run("javascript", func(t *testing.T) {
		node, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node is not installed")
		}
		script := filepath.Join(t.TempDir(), "check.mjs")
		module := "file:///" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(root, "clients", "js", "index.js")), "/")
		os.WriteFile(script, []byte(`
import { IndiaAPI, IndiaAPIError, CATALOG } from "`+module+`";
const api = new IndiaAPI({ baseUrl: process.argv[2] });
const assert = (ok, what) => { if (!ok) { console.error("FAIL " + what); process.exit(1); } };
const place = await api.pincodeByPincode("600001");
assert(place.data.state === "Tamil Nadu", "pincode " + JSON.stringify(place));
const words = await api.wordsByNumber("125000.50", { style: "currency" });
assert(/lakh/.test(words.data.words), "words " + JSON.stringify(words));
const redacted = await api.textRedact({}, { body: "Call 98765 43210 now" });
assert(!JSON.stringify(redacted).includes("98765 43210"), "redact " + JSON.stringify(redacted));
const qr = await api.qr({ text: "hello", format: "svg" });
assert(qr.contentType.startsWith("image/svg") && qr.bytes.length > 100, "qr");
const bin = await api.hooks({});
await api.hookById(bin.data.id, {}, { body: "ping" });
const got = await new IndiaAPI({ baseUrl: process.argv[2], token: bin.data.token }).hooksById(bin.data.id);
assert(got.data.requests.length === 1 && got.data.requests[0].body === "ping", "bin " + JSON.stringify(got));
try { await api.pincodeByPincode("999999"); assert(false, "no error"); }
catch (e) { assert(e instanceof IndiaAPIError && e.status === 404 && e.code === "not_found", "error " + e); }
assert(CATALOG.length === 12, "catalog");
console.log("ok");
`), 0o644)
		out, err := exec.Command(node, script, testHTTP.URL).CombinedOutput()
		if err != nil || !strings.Contains(string(out), "ok") {
			t.Fatalf("node: %v\n%s", err, out)
		}
	})
	t.Run("python", func(t *testing.T) {
		// On Windows python3 may be a stub that only offers to install Python.
		python := ""
		for _, name := range []string{"python3", "python"} {
			if found, err := exec.LookPath(name); err == nil && exec.Command(found, "-c", "pass").Run() == nil {
				python = found
				break
			}
		}
		if python == "" {
			t.Skip("python is not installed")
		}
		script := filepath.Join(t.TempDir(), "check.py")
		os.WriteFile(script, []byte(`
import sys
sys.path.insert(0, sys.argv[2])
from india_api import IndiaAPI, IndiaAPIError, CATALOG
api = IndiaAPI(base_url=sys.argv[1])
place = api.pincode_by_pincode("600001")
assert place["data"]["state"] == "Tamil Nadu", place
words = api.words_by_number("125000.50", style="currency")
assert "lakh" in words["data"]["words"], words
redacted = api.text_redact(body="Call 98765 43210 now")
assert "98765 43210" not in str(redacted), redacted
qr = api.qr(text="hello", format="svg")
assert qr.content_type.startswith("image/svg") and len(qr.data) > 100, qr
bin = api.hooks()
api.hook_by_id(bin["data"]["id"], body="ping")
got = IndiaAPI(base_url=sys.argv[1], token=bin["data"]["token"]).hooks_by_id(bin["data"]["id"])
assert len(got["data"]["requests"]) == 1 and got["data"]["requests"][0]["body"] == "ping", got
try:
    api.pincode_by_pincode("999999")
    raise SystemExit("no error")
except IndiaAPIError as e:
    assert e.status == 404 and e.code == "not_found", e
assert len(CATALOG) == 12
print("ok")
`), 0o644)
		out, err := exec.Command(python, "-I", "-B", script, testHTTP.URL, filepath.Join(root, "clients", "python", "src")).CombinedOutput()
		if err != nil || !strings.Contains(string(out), "ok") {
			t.Fatalf("python: %v\n%s", err, out)
		}
	})
}
