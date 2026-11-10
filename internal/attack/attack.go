// Package attack labels requests that look like attacks: probes for files and admin
// pages the API does not have, injection payloads, and known attack tools. It only
// reads a request and names what it found; nothing about the answer changes.
//
// The types are a fixed list, so they can be metric labels. Rules aim at precision:
// a scanner that tries /.env is caught, a person sending SQL to the code-image
// endpoint is not, because the server only looks for payloads in requests that
// failed (see Request.Payload).
package attack

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The attack types, in the order they are tried.
const (
	Log4Shell        = "log4shell"
	PathTraversal    = "path-traversal"
	CommandInjection = "command-injection"
	SQLInjection     = "sql-injection"
	TemplateInject   = "template-injection"
	XSS              = "xss"
	WebShell         = "web-shell"
	SourceControl    = "source-control"
	ExploitProbe     = "exploit-probe"
	SecretsFile      = "secrets-file"
	BackupFile       = "backup-file"
	WordPress        = "wordpress"
	AdminPanel       = "admin-panel"
	ScriptProbe      = "script-probe"
	ScannerTool      = "scanner-tool"
)

// Types lists every type Check can return.
var Types = []string{Log4Shell, PathTraversal, CommandInjection, SQLInjection, TemplateInject, XSS,
	WebShell, SourceControl, ExploitProbe, SecretsFile, BackupFile, WordPress, AdminPanel, ScriptProbe, ScannerTool}

// Where a finding was made.
const (
	InPath      = "path"
	InQuery     = "query"
	InHeader    = "header"
	InBody      = "body"
	InUserAgent = "user-agent"
)

// SnippetLength is the most characters a snippet holds.
const SnippetLength = 200

// Request is what Check reads.
type Request struct {
	Path     string // the escaped path, as sent
	RawQuery string
	Header   http.Header
	Body     []byte // only for paths the API does not have; at most a few KB
	// Payload turns on the injection rules for the path and query. The server sets it
	// for a request that failed or reached no route: a successful request to an endpoint
	// that takes free text (regex, code images) may carry SQL or HTML on purpose.
	Payload bool
	// Probe turns on the rules for files and pages the API does not have. The server
	// sets it only for a path no route took: a failed logo lookup for SHELL or ADMIN is
	// a company name, not a probe.
	Probe bool
}

// Finding is what Check found. Type is empty when nothing was.
type Finding struct {
	Type    string
	In      string
	Snippet string // the text around the match, cleaned and cut to SnippetLength
}

type rule struct {
	name    string
	pattern *regexp.Regexp
}

// Payload rules look at decoded text anywhere the attacker controls.
var payloadRules = []rule{
	{PathTraversal, regexp.MustCompile(`(?i)(\.\.[/\\]|[/\\]\.\.(?:[/\\]|$)|/etc/(?:passwd|shadow|hosts)\b|\bwin\.ini\b|\bboot\.ini\b|/proc/self/|\x00)`)},
	{CommandInjection, regexp.MustCompile(`(?i)((?:;|\||&&|\$\(|` + "`" + `)\s*(?:wget|curl|bash|sh|nc|ncat|python3?|perl|chmod|busybox|uname|whoami|id|cat\s+/)\b|/bin/(?:ba)?sh\b|\bcmd(?:\.exe)?\s*/c\b|\bpowershell(?:\.exe)?\s+-)`)},
	{SQLInjection, regexp.MustCompile(`(?i)(\bunion(?:\s|\+|/\*.*?\*/)+(?:all(?:\s|\+)+)?select\b|['"]\s*(?:or|and)\s+['"]?\w+['"]?\s*=\s*['"]?\w+|\b(?:sleep|benchmark|pg_sleep)\s*\(\s*\d|\bwaitfor\s+delay\b|\binformation_schema\b|\bxp_cmdshell\b|;\s*drop\s+table\b|['"]\s*(?:--|#|/\*)|\bselect\b.{1,40}\bfrom\b.{1,40}\bwhere\b)`)},
	{TemplateInject, regexp.MustCompile(`(\{\{\s*\d+\s*\*\s*\d+\s*\}\}|\$\{\s*\d+\s*\*\s*\d+\s*\}|#\{\s*\d+\s*\*\s*\d+\s*\}|\{\{\s*(?:config|self|request)\b)`)},
	{XSS, regexp.MustCompile(`(?i)(<\s*script\b|<\s*(?:img|svg|iframe|body|details)\b[^>]*\bon\w+\s*=|\bjavascript\s*:|\bon(?:error|load|mouseover|focus)\s*=\s*\S|\balert\s*\(\s*(?:1|document|['"]))`)},
}

// log4shell is tried first, everywhere, headers included.
var log4shell = regexp.MustCompile(`(?i)\$\{\s*(?:jndi\s*:|\$\{\s*(?:lower|upper|::-|env|sys|date|java|main)\b[^}]*\})`)

// webShellBody is code meant to run on the server, sent in a body.
var webShellBody = regexp.MustCompile(`(?i)(<\?php|\beval\s*\(\s*\$_(?:post|get|request)|\bbase64_decode\s*\(|\bsystem\s*\(\s*\$_|\bassert\s*\(\s*\$_)`)

// Path rules look at the decoded path of a request no route took, or that failed.
var pathRules = []rule{
	{WebShell, regexp.MustCompile(`(?i)/(?:shell|cmd|c99|r57|wso\d*|alfa\w*|b374k|webshell|up|uploader|bypass|x|xx|1|a)\.(?:php\d?|phtml|aspx?|jsp)$`)},
	{SourceControl, regexp.MustCompile(`(?i)/\.(?:git|svn|hg|bzr)(?:/|$)|/\.gitignore$|/\.gitlab-ci\.yml$`)},
	{ExploitProbe, regexp.MustCompile(`(?i)/(?:actuator|cgi-bin|boaform|hnap1|vendor/phpunit|_ignition|autodiscover|owa|ecp|remote/fgt_lang|\+cscoe\+|geoserver|hudson|struts|invoker|jmx-console|web-console|setup\.cgi|goform|tmui|mgmt|sdk|ui/vropspluginui|api/jsonws|nmaplowercheck|shell|cgi|containers/json|v1\.\d+/containers|aws|metadata|latest/meta-data|computemetadata|_cat|_search|_all_dbs|solr/admin|ws/v1/cluster|level/15|currentsetting\.htm|global-protect|dana-na|sslvpn|vpn|mifs|webtools|developmentserver|evox|cf_scripts)(?:[/.?]|$)`)},
	{SecretsFile, regexp.MustCompile(`(?i)/\.(?:env|aws|ssh|npmrc|pypirc|docker|dockercfg|htpasswd|htaccess|bash_history|git-credentials|netrc|s3cfg|kube|vscode|idea|ds_store)(?:[./_-]|$)|/(?:wp-config|config|configuration|settings|secrets?|credentials|database|db|parameters|local\.settings|appsettings)\.(?:php|json|ya?ml|ini|env|xml|js|conf|cfg|txt|py|rb|bak|old|save|orig)\w*(?:\.\w+)?$|/id_(?:rsa|dsa|ecdsa|ed25519)(?:\.pub)?$|/phpinfo\.php$|/(?:\.?env|\.?secrets?)(?:\.\w+)?$|/web\.config$|/server\.key$`)},
	{BackupFile, regexp.MustCompile(`(?i)\.(?:sql|sqlite3?|db|bak|backup|old|orig|save|swp|tar|tgz|tar\.gz|sql\.gz|zip|7z|rar|dump)$`)},
	{WordPress, regexp.MustCompile(`(?i)/(?:wp-(?:admin|login|content|includes|json|cron|signup|trackback)|xmlrpc\.php|wordpress/|wp/)`)},
	{AdminPanel, regexp.MustCompile(`(?i)/(?:phpmyadmin|pma|myadmin|mysqladmin|adminer|admin|administrator|manager/html|jenkins|login\.action|cpanel|webmail|plesk|grafana|kibana|solr|druid|nacos|console|portal|panel|dashboard/login|user/login|_profiler|telescope|horizon|webui)(?:[/.]|$)`)},
	{ScriptProbe, regexp.MustCompile(`(?i)\.(?:php\d?|phtml|aspx?|ashx|asmx|axd|jsp|jspx|do|action|cgi|pl|cfm)$`)},
}

// scannerAgents are attack tools that name themselves.
var scannerAgents = regexp.MustCompile(`(?i)\b(?:sqlmap|nikto|nuclei|zgrab|masscan|nmap|wpscan|dirbuster|gobuster|ffuf|feroxbuster|dirsearch|acunetix|nessus|openvas|netsparker|qualys|zaproxy|owasp\s*zap|burp|jaeles|xray|arachni|w3af|whatweb|wfuzz|hydra|medusa|commix|joomscan|fuzz\s*faster|l9explore|l9tcpid|leakix|morfeus|zmeu|jorgee)\b`)

// headersNeverQuoted are headers whose values are never searched: they hold
// credentials or the caller's address, which must not end up in a snippet.
var headersNeverQuoted = map[string]bool{
	"Authorization": true, "Proxy-Authorization": true, "Cookie": true, "X-Forwarded-For": true,
	"X-Forwarded-Host": true, "X-Forwarded-Proto": true, "X-Real-Ip": true, "True-Client-Ip": true,
	"Forwarded": true, "Cdn-Loop": true, "Cf-Connecting-Ip": true, "Cf-Connecting-Ipv6": true,
	"Cf-Ipcountry": true, "Cf-Ray": true, "Cf-Visitor": true, "Cf-Worker": true, "Cf-Ew-Via": true,
}

// Check names the first attack it finds in r.
func Check(r Request) Finding {
	path := decode(r.Path, false)
	query := decode(r.RawQuery, true)
	body := decode(string(r.Body), true)

	// Log4Shell is looked for everywhere: it is sent in any header a server might log.
	if f, ok := find(Log4Shell, log4shell, InPath, path); ok {
		return f
	}
	if f, ok := find(Log4Shell, log4shell, InQuery, query); ok {
		return f
	}
	for _, name := range headerNames(r.Header) {
		for _, value := range r.Header[name] {
			if f, ok := find(Log4Shell, log4shell, InHeader, name+": "+value); ok {
				return f
			}
		}
	}
	if f, ok := find(Log4Shell, log4shell, InBody, body); ok {
		return f
	}

	if r.Payload {
		for _, rule := range payloadRules {
			if f, ok := find(rule.name, rule.pattern, InPath, path); ok {
				return f
			}
			if f, ok := find(rule.name, rule.pattern, InQuery, query); ok {
				return f
			}
		}
	}
	// A body is only given for a path the API does not have, so what it holds is the
	// attack itself: code to run, or an injection.
	if body != "" {
		if f, ok := find(WebShell, webShellBody, InBody, body); ok {
			return f
		}
		for _, rule := range payloadRules {
			if f, ok := find(rule.name, rule.pattern, InBody, body); ok {
				return f
			}
		}
	}
	if r.Probe {
		for _, rule := range pathRules {
			if f, ok := find(rule.name, rule.pattern, InPath, path); ok {
				return f
			}
		}
	}
	if agent := r.Header.Get("User-Agent"); agent != "" {
		if f, ok := find(ScannerTool, scannerAgents, InUserAgent, agent); ok {
			return f
		}
	}
	return Finding{}
}

func find(name string, pattern *regexp.Regexp, in, text string) (Finding, bool) {
	if text == "" {
		return Finding{}, false
	}
	at := pattern.FindStringIndex(text)
	if at == nil {
		return Finding{}, false
	}
	return Finding{Type: name, In: in, Snippet: snippet(text, at[0], at[1])}, true
}

// decode undoes percent-encoding up to three times, as attackers encode twice to get
// past filters. A query also turns + into a space.
func decode(text string, query bool) string {
	for range 3 {
		var next string
		var err error
		if query {
			next, err = url.QueryUnescape(text)
		} else {
			next, err = url.PathUnescape(text)
		}
		if err != nil || next == text {
			break
		}
		text = next
	}
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, " ")
	}
	return text
}

// snippet cuts the text around a match to at most SnippetLength characters, with
// control characters and line breaks turned into spaces.
func snippet(text string, start, end int) string {
	all := []rune(text)
	from := utf8.RuneCountInString(text[:start])
	to := from + utf8.RuneCountInString(text[start:end])
	if to-from >= SnippetLength {
		to = from + SnippetLength
	} else {
		from = max(0, from-(SnippetLength-(to-from))/2)
		to = min(len(all), from+SnippetLength)
		from = max(0, to-SnippetLength)
	}
	out := all[from:to]
	for i, r := range out {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			out[i] = ' '
		}
	}
	return string(out)
}

func headerNames(header http.Header) []string {
	names := make([]string, 0, len(header))
	for name := range header {
		if !headersNeverQuoted[http.CanonicalHeaderKey(name)] {
			names = append(names, name)
		}
	}
	sort.Strings(names) // a fixed order, so a request is always labelled the same way
	return names
}
