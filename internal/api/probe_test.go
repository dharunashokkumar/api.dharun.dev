package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["probe_url_from_india"] = `{"url":"https://site.in/"}`
}

func TestProbe(t *testing.T) {
	sites := useTestSites(t)
	page := strings.Repeat("<p>the secret page text</p>", 100)
	sites.serve("site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			http.Redirect(w, r, "https://site.in"+r.URL.RequestURI(), http.StatusMovedPermanently)
			return
		}
		switch r.URL.Path {
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/inside":
			http.Redirect(w, r, "http://inside.site.in/", http.StatusFound)
		case "/elsewhere":
			http.Redirect(w, r, "ftp://site.in/file", http.StatusFound)
		default:
			if r.Header.Get("User-Agent") != probeAgent {
				t.Errorf("probe user agent %q", r.Header.Get("User-Agent"))
			}
			w.Header().Set("Cf-Ray", "8f3a2b1c0d9e8f7a-BOM")
			w.Header().Add("Set-Cookie", "session=very-secret-value; Path=/; HttpOnly")
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(page))
		}
	}))
	sites.serve("big.site.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 2<<20)))
	}))

	got := expectStatus(t, "/v1/probe?url="+url.QueryEscape("http://site.in/?q=1#part"), http.StatusOK)
	view := got.data(t)
	redirects := view["redirects"].([]any)
	headers := view["headers"].(map[string]any)
	cdn := view["cdn"].(map[string]any)
	body := view["body"].(map[string]any)
	if view["status"] != float64(200) || view["finalUrl"] != "https://site.in/?q=1" || len(redirects) != 1 || redirects[0].(map[string]any)["status"] != float64(301) ||
		view["address"] != testSiteAddress || view["httpVersion"] != "HTTP/2.0" || view["tls"].(map[string]any)["alpn"] != "h2" {
		t.Errorf("probe: %v", view)
	}
	if cdn["name"] != "Cloudflare" || cdn["edge"] != "BOM" || cdn["city"] != "Mumbai" {
		t.Errorf("CDN: %v", cdn)
	}
	if headers["set-cookie"] != "session=…; Path=/; HttpOnly" || headers["content-type"] != "text/html" {
		t.Errorf("headers: %v", headers)
	}
	if body["bytes"] != float64(len(page)) || body["truncated"] != false || body["sha256"] == nil {
		t.Errorf("body: %v", body)
	}
	if strings.Contains(string(got.body), "secret page text") || strings.Contains(string(got.body), "very-secret-value") {
		t.Error("the probe returned what the site sent")
	}
	if got.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("Cache-Control %q", got.header.Get("Cache-Control"))
	}
	timings := view["timings"].(map[string]any)
	if timings["totalMs"].(float64) <= 0 || timings["tlsMs"] == nil {
		t.Errorf("timings: %v", timings)
	}

	repeated := expectStatus(t, "/v1/probe?url=site.in&rounds=3", http.StatusOK).data(t)
	if len(repeated["rounds"].([]any)) != 3 || repeated["summary"].(map[string]any)["medianMs"] == nil {
		t.Errorf("rounds: %v", repeated)
	}
	loop := expectStatus(t, "/v1/probe?url=https://site.in/loop", http.StatusOK).data(t)
	if loop["status"] != float64(302) || len(loop["redirects"].([]any)) != probeRedirects {
		t.Errorf("redirect loop: %v %d", loop["status"], len(loop["redirects"].([]any)))
	}
	big := expectStatus(t, "/v1/probe?url=https://big.site.in/", http.StatusOK).data(t)["body"].(map[string]any)
	if big["truncated"] != true || big["bytes"] != float64(probeBodyLimit) || big["sha256"] != nil {
		t.Errorf("a large body: %v", big)
	}

	// A redirect into private space is refused when it connects, like the first address.
	expectStatus(t, "/v1/probe?url=https://site.in/inside", http.StatusUnprocessableEntity)
	expectStatus(t, "/v1/probe?url=http://inside.site.in/", http.StatusUnprocessableEntity)
	expectStatus(t, "/v1/probe?url=https://site.in:8443/", http.StatusUnprocessableEntity)
	expectStatus(t, "/v1/probe?url=https://site.in/elsewhere", http.StatusUnprocessableEntity)
	for _, bad := range []string{"", "ftp://site.in/", "https://user:pass@site.in/", "https://single/"} {
		expectStatus(t, "/v1/probe?url="+url.QueryEscape(bad), http.StatusBadRequest)
	}
	expectStatus(t, "/v1/probe?url=site.in&rounds=9", http.StatusBadRequest)
}
