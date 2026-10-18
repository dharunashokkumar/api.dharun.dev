package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// The browser tools: a screenshot of a page, a page or posted HTML as a PDF, and the text
// of a page after its scripts ran. Chromium runs in a microVM of its own in the sandbox VM
// and reaches only public addresses. The page text tool reads one page a caller names,
// never follows links, and honours robots.txt for its agent token; section 8 of the
// roadmap ("no open scraping") stands.

const (
	// browserAgent is the token the browser names itself by and robots.txt is read for.
	browserAgent = "india-api-browser"
	// htmlLimit bounds HTML posted for a PDF or a screenshot (images inline as data: URLs).
	htmlLimit = 4 << 20
)

type browserBody struct {
	URL        string `json:"url"`
	HTML       string `json:"html"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FullPage   bool   `json:"fullPage"`
	Format     string `json:"format"`
	Paper      string `json:"paper"`
	Landscape  bool   `json:"landscape"`
	Background bool   `json:"background"`
	WaitMillis int    `json:"waitMs"`
	Mobile     bool   `json:"mobile"`
	Markdown   bool   `json:"markdown"`
}

// fromQuery reads the options of a GET request.
func (b *browserBody) fromQuery(query url.Values) {
	b.URL, b.Format, b.Paper = query.Get("url"), query.Get("format"), query.Get("paper")
	b.Width, _ = strconv.Atoi(query.Get("width"))
	b.Height, _ = strconv.Atoi(query.Get("height"))
	b.WaitMillis, _ = strconv.Atoi(query.Get("waitMs"))
	b.FullPage = query.Get("fullPage") == "true"
	b.Landscape = query.Get("landscape") == "true"
	b.Background = query.Get("background") == "true"
	b.Mobile = query.Get("mobile") == "true"
	b.Markdown = query.Get("format") == "markdown" || query.Get("markdown") == "true"
}

func (s *Server) handleBrowser(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		info(r).private = true
		service := s.sandboxService(w, r)
		if service == nil {
			return
		}
		var body browserBody
		if r.Method == http.MethodPost {
			raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, htmlLimit))
			if err != nil {
				s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The body is over "+itoa(htmlLimit>>20)+" MB.")
				return
			}
			if err := json.Unmarshal(raw, &body); err != nil {
				s.badRequest(w, r, `Send JSON such as {"url": "https://..."} or {"html": "<h1>...</h1>"}.`)
				return
			}
		} else {
			body.fromQuery(r.URL.Query())
		}
		if action == sandbox.BrowserText && body.HTML != "" {
			s.badRequest(w, r, "The text tool reads a page at a URL; HTML you hold can be read without a browser.")
			return
		}
		if (body.URL == "") == (body.HTML == "") {
			s.badRequest(w, r, "Give url, or for a screenshot or PDF html, but not both.")
			return
		}
		if body.URL != "" {
			target, ok := s.checkBrowserTarget(w, r, body.URL, action)
			if !ok {
				return
			}
			body.URL = target
		}
		contentType, answer, err := service.client.Browser(r.Context(), sandbox.BrowserRequest{
			Action: action, URL: body.URL, HTML: body.HTML, Width: body.Width, Height: body.Height, FullPage: body.FullPage,
			Format: body.Format, Paper: body.Paper, Landscape: body.Landscape, Background: body.Background,
			WaitMillis: body.WaitMillis, Mobile: body.Mobile, Markdown: body.Markdown && action == sandbox.BrowserText,
		})
		if err != nil {
			s.sandboxFailed(w, r, "browser", err)
			return
		}
		s.metrics.sandboxEvent("browser", "ok")
		if action == sandbox.BrowserText {
			var text sandbox.PageText
			if err := json.Unmarshal(answer, &text); err != nil {
				s.fail(w, r, http.StatusBadGateway, "sandbox_error", "The page's text could not be read.")
				return
			}
			s.ok(w, r, text, meta{Note: "Read once, as the page stood, by a browser that names itself " + browserAgent + "."}, cacheNone)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.Write(answer)
	}
}

// checkBrowserTarget checks a URL: http or https, a public host on the usual port, within
// the per-site limit, and for the text tool allowed by robots.txt. It returns the URL to
// open, or answers the refusal itself.
func (s *Server) checkBrowserTarget(w http.ResponseWriter, r *http.Request, raw, action string) (string, bool) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	target, err := url.Parse(raw)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" || target.User != nil {
		s.badRequest(w, r, "url must be an http:// or https:// address.")
		return "", false
	}
	host, err := hostOf(target.Hostname())
	if err != nil {
		s.failNetwork(w, r, target.Hostname(), err)
		return "", false
	}
	if port := target.Port(); port != "" && !(port == "443" && target.Scheme == "https") && !(port == "80" && target.Scheme == "http") {
		s.fail(w, r, http.StatusUnprocessableEntity, "port_not_allowed", "Only the usual web ports (80 for http, 443 for https) are reached.")
		return "", false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err := s.guard(80, 443).Resolve(ctx, host); err != nil {
		s.failNetwork(w, r, host, err)
		return "", false
	}
	if !s.extras.network.targets.Allow(host) {
		w.Header().Set("Retry-After", "60")
		s.fail(w, r, http.StatusTooManyRequests, "target_limited",
			"This site has been opened "+itoa(targetPerMinute)+" times in the last minute by callers of this API. Try again in a minute.")
		return "", false
	}
	if action == sandbox.BrowserText && !s.robotsAllow(ctx, target) {
		s.fail(w, r, http.StatusForbidden, "robots_disallowed", host+"'s robots.txt does not allow "+browserAgent+" (or every agent) to read this page.")
		return "", false
	}
	return target.String(), true
}

// robots caches each site's robots.txt rules for our agent for ten minutes.
var robots = struct {
	sync.Mutex
	rules map[string]robotsEntry
}{rules: map[string]robotsEntry{}}

type robotsEntry struct {
	rules   *robotsRules
	fetched time.Time
}

func (s *Server) robotsAllow(ctx context.Context, target *url.URL) bool {
	origin := target.Scheme + "://" + target.Host
	robots.Lock()
	entry, ok := robots.rules[origin]
	robots.Unlock()
	if !ok || time.Since(entry.fetched) > 10*time.Minute {
		entry = robotsEntry{rules: s.fetchRobots(ctx, origin), fetched: time.Now()}
		robots.Lock()
		if len(robots.rules) > 5000 {
			robots.rules = map[string]robotsEntry{}
		}
		robots.rules[origin] = entry
		robots.Unlock()
	}
	path := target.EscapedPath()
	if path == "" {
		path = "/"
	}
	if target.RawQuery != "" {
		path += "?" + target.RawQuery
	}
	return entry.rules.allows(path)
}

// fetchRobots reads a site's robots.txt as Google does: a missing file (4xx) allows
// everything; one the site refuses or fails to serve (401, 403, 5xx, no answer) allows
// nothing.
func (s *Server) fetchRobots(ctx context.Context, origin string) *robotsRules {
	client := s.guard(80, 443).Client(netguard.ClientOptions{Timeout: 8 * time.Second, Redirects: 5,
		UserAgent: "Mozilla/5.0 (compatible; " + browserAgent + "; +https://api.dharun.dev/docs/browser)"})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/robots.txt", nil)
	if err != nil {
		return &robotsRules{disallowAll: true}
	}
	response, err := client.Do(req)
	if err != nil {
		return &robotsRules{disallowAll: true}
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden || response.StatusCode >= 500:
		return &robotsRules{disallowAll: true}
	case response.StatusCode >= 400:
		return &robotsRules{}
	case response.StatusCode >= 300:
		return &robotsRules{disallowAll: true}
	}
	return parseRobots(io.LimitReader(response.Body, 512<<10), browserAgent)
}

type robotsRule struct {
	allow   bool
	pattern *regexp.Regexp
	length  int
}

type robotsRules struct {
	disallowAll bool
	rules       []robotsRule
}

// parseRobots keeps the rules of the group naming agent, or else of the * group.
func parseRobots(r io.Reader, agent string) *robotsRules {
	type group struct {
		agents []string
		rules  []robotsRule
	}
	var groups []*group
	var current *group
	lastWasAgent := false
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), 64<<10)
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		field, value = strings.ToLower(strings.TrimSpace(field)), strings.TrimSpace(value)
		switch field {
		case "user-agent":
			if !lastWasAgent || current == nil {
				current = &group{}
				groups = append(groups, current)
			}
			current.agents = append(current.agents, strings.ToLower(value))
			lastWasAgent = true
		case "allow", "disallow":
			lastWasAgent = false
			if current == nil || (value == "" && field == "disallow") {
				continue
			}
			current.rules = append(current.rules, robotsRule{allow: field == "allow", pattern: robotsPattern(value), length: len(value)})
		default:
			lastWasAgent = false
		}
	}
	var chosen, star *group
	for _, g := range groups {
		for _, name := range g.agents {
			if name == "*" && star == nil {
				star = g
			} else if name != "*" && strings.Contains(agent, name) && chosen == nil {
				chosen = g
			}
		}
	}
	if chosen == nil {
		chosen = star
	}
	if chosen == nil {
		return &robotsRules{}
	}
	return &robotsRules{rules: chosen.rules}
}

// robotsPattern turns a robots.txt path (with * and a final $) into an anchored regexp.
func robotsPattern(path string) *regexp.Regexp {
	anchored := strings.HasSuffix(path, "$")
	path = strings.TrimSuffix(path, "$")
	parts := strings.Split(path, "*")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}
	expression := "^" + strings.Join(parts, ".*")
	if anchored {
		expression += "$"
	}
	return regexp.MustCompile(expression)
}

// allows applies the longest matching rule; an allow wins a tie.
func (rr *robotsRules) allows(path string) bool {
	if rr.disallowAll {
		return false
	}
	best, allowed := -1, true
	for _, rule := range rr.rules {
		if rule.pattern.MatchString(path) && (rule.length > best || (rule.length == best && rule.allow)) {
			best, allowed = rule.length, rule.allow
		}
	}
	return allowed
}
