package api

import (
	"crypto/tls"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
)

func init() {
	mcpTestCalls["check_tls"] = `{"host":"site.in"}`
}

func TestTLSCheck(t *testing.T) {
	sites := useTestSites(t)
	inner := sites.intermediate("India API Test Intermediate")
	chained := sites.issue("old.site.in", time.Now().Add(-time.Hour), time.Now().Add(60*24*time.Hour), &inner)
	sites.present("old.site.in", &tls.Config{Certificates: []tls.Certificate{chained}})
	noInner := sites.issue("noinner.site.in", time.Now().Add(-time.Hour), time.Now().Add(60*24*time.Hour), &inner)
	noInner.Certificate = noInner.Certificate[:1]
	sites.present("noinner.site.in", &tls.Config{Certificates: []tls.Certificate{noInner}})
	sites.present("expired.site.in", &tls.Config{Certificates: []tls.Certificate{sites.issue("expired.site.in", time.Now().Add(-90*24*time.Hour), time.Now().Add(-24*time.Hour), nil)}})
	sites.present("wrong.site.in", &tls.Config{Certificates: []tls.Certificate{sites.issue("other.example", time.Now().Add(-time.Hour), time.Now().Add(30*24*time.Hour), nil)}})
	sites.present("self.site.in", &tls.Config{Certificates: []tls.Certificate{selfSigned("self.site.in")}})
	sites.present("legacy.site.in", &tls.Config{
		Certificates: []tls.Certificate{sites.issue("legacy.site.in", time.Now().Add(-time.Hour), time.Now().Add(10*24*time.Hour), nil)},
		MinVersion:   tls.VersionTLS10, MaxVersion: tls.VersionTLS12,
		CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA},
	})

	check := func(host string) map[string]any {
		t.Helper()
		return expectStatus(t, "/v1/tls/"+host, http.StatusOK).data(t)
	}
	codes := func(view map[string]any) string {
		var list []string
		for _, p := range view["problems"].([]any) {
			list = append(list, p.(map[string]any)["code"].(string))
		}
		return strings.Join(list, " ")
	}

	good := check("site.in")
	session := good["tls"].(map[string]any)
	versions := good["versions"].(map[string]any)
	if good["valid"] != true || good["verified"] != true || codes(good) != "" || session["version"] != "TLS 1.3" || session["alpn"] != "h2" ||
		versions["TLS 1.2"] != true || versions["TLS 1.3"] != true || versions["TLS 1.0"] != false || good["weakCipher"] != nil || good["address"] != testSiteAddress {
		t.Errorf("a good site: %v", good)
	}
	certificate := good["certificate"].(map[string]any)
	if certificate["issuer"] != "India API Test Root" || certificate["key"] != "ECDSA P-256" || certificate["names"].([]any)[0] != "site.in" || certificate["daysLeft"].(float64) < 80 {
		t.Errorf("certificate: %v", certificate)
	}
	if chain := check("old.site.in"); chain["valid"] != true || len(chain["chain"].([]any)) != 2 {
		t.Errorf("a full chain: %v", chain)
	}
	for host, want := range map[string]string{
		"expired.site.in": "expired", "wrong.site.in": "name_mismatch", "self.site.in": "self_signed", "noinner.site.in": "incomplete_chain",
	} {
		view := check(host)
		if view["valid"] != false || !strings.Contains(codes(view), want) {
			t.Errorf("%s: problems %q, want %s (%v)", host, codes(view), want, view["verifyError"])
		}
	}
	legacy := check("legacy.site.in")
	versions = legacy["versions"].(map[string]any)
	if versions["TLS 1.0"] != true || versions["TLS 1.1"] != true || versions["TLS 1.3"] != false || legacy["weakCipher"] != "TLS_ECDHE_ECDSA_WITH_RC4_128_SHA" ||
		codes(legacy) != "expires_soon old_versions no_tls13 weak_cipher" || legacy["valid"] != true {
		t.Errorf("an old setup: %s %v", codes(legacy), legacy)
	}

	// A server that only takes old ciphers is still described.
	sites.present("rc4.site.in", &tls.Config{
		Certificates: []tls.Certificate{sites.issue("rc4.site.in", time.Now().Add(-time.Hour), time.Now().Add(60*24*time.Hour), nil)},
		MaxVersion:   tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA},
	})
	if rc4 := check("rc4.site.in"); rc4["tls"].(map[string]any)["cipher"] != "TLS_ECDHE_ECDSA_WITH_RC4_128_SHA" || !strings.Contains(codes(rc4), "weak_cipher") {
		t.Errorf("an RC4-only server: %v", rc4)
	}

	// Private addresses, the server's own address and IPv6 are refused before any
	// connection is made.
	for _, host := range []string{"inside.site.in", "home.site.in", "10.1.2.3", testOwnAddress} {
		expectStatus(t, "/v1/tls/"+host, http.StatusUnprocessableEntity)
	}
	expectStatus(t, "/v1/tls/2606:4700::1111", http.StatusUnprocessableEntity)
	expectStatus(t, "/v1/tls/nothing.site.in", http.StatusNotFound)
	expectStatus(t, "/v1/tls/not%20a%20host", http.StatusBadRequest)
}

// One site gets a few checks a minute from all callers together.
func TestCheckLimitPerSite(t *testing.T) {
	useTestSites(t)
	tools := testServer.extras.network
	saved := tools.targets
	tools.targets = &netguard.TargetLimit{PerMinute: 1}
	defer func() { tools.targets = saved }()
	expectStatus(t, "/v1/tls/site.in", http.StatusOK)
	limited := expectStatus(t, "/v1/tls/www.site.in", http.StatusTooManyRequests)
	if !strings.Contains(string(limited.body), "target_limited") || limited.header.Get("Retry-After") == "" {
		t.Errorf("per-site limit: %s", limited.body)
	}
}
