// Package traffic sorts requests by who sent them and where the link came from, for the
// request log, the metrics and the traffic report. Nothing here keeps state: a request's
// labels come only from its own user agent, referring page and utm_source / utm_medium
// parameters.
package traffic

import (
	"net/url"
	"strings"
)

// Kinds of client.
const (
	Own           = "own"            // this project's own monitoring and checks
	Person        = "person"         // a browser
	AIAssistant   = "ai-assistant"   // an AI tool acting for someone right now
	AICrawler     = "ai-crawler"     // collects pages for an AI index or training set
	SearchCrawler = "search-crawler" // a search engine's crawler
	Bot           = "bot"            // link previews, SEO tools, uptime services, scanners
	Program       = "program"        // an HTTP library or command line tool
	Unknown       = "unknown"        // no user agent, or one nothing matches
)

// Kinds lists every kind, in the order a report shows them.
var Kinds = []string{Person, AIAssistant, Program, AICrawler, SearchCrawler, Bot, Own, Unknown}

// Media are the places our own links carry in utm_medium. Any other value is kept in
// the log but counted as "other".
var Media = []string{
	"docs",         // code on an API page
	"docs-preview", // the picture an API page shows, loaded by whoever opens the page
	"docs-md",      // the Markdown form of an API page
	"home",         // the landing page
	"guide",        // a guide
	"llms",         // llms.txt and llms-full.txt
	"skill",        // the Agent Skill and the plugin
	"index",        // the /v1 index
	"mcp",          // the MCP server (set by the server, not by a link)
}

// Origin is what a request is labelled with.
type Origin struct {
	// Source is who sent the request or led to it: a known name (claude, chatgpt,
	// googlebot, curl, browser, own, ...), the cleaned utm_source value, or "".
	Source string
	// Known is Source when it is in the fixed list, otherwise "other" (or "none"): the
	// form that is safe for a metric label.
	Known string
	// Medium is the cleaned utm_medium value, "mcp" for the MCP server, or "".
	Medium string
	// KnownMedium is Medium when it is one of Media, otherwise "other" (or "none").
	KnownMedium string
	Kind        string
}

// Request is what Classify reads from a request.
type Request struct {
	UserAgent string
	Referer   string
	UTMSource string
	UTMMedium string
	// Client is the name an MCP client gave itself on initialize, if any.
	Client string
	// MCP is true for a request to the MCP server.
	MCP bool
}

// Classify labels one request. An explicit utm_source wins, then the MCP client's own
// name, then the referring site, then the user agent.
func Classify(r Request) Origin {
	agent := matchAgent(r.UserAgent)
	o := Origin{Kind: agent.kind}

	switch {
	case r.UTMSource != "":
		o.Source = Clean(r.UTMSource)
	case r.Client != "":
		o.Source = Clean(r.Client)
	}
	if o.Source != "" {
		if name, ok := alias(o.Source); ok {
			o.Known = name
		}
	}
	if o.Source == "" {
		if name := refererSource(r.Referer); name != "" {
			o.Source, o.Known = name, name
		}
	}
	if o.Source == "" {
		o.Source, o.Known = agent.name, agent.name
	}

	switch {
	case agent.kind == Own || o.Known == Own:
		o.Kind = Own
	case assistants[o.Known] && agent.kind != Person && agent.kind != AICrawler && agent.kind != SearchCrawler && agent.kind != Bot:
		// An agent that named itself, or a request whose user agent is an AI tool's. A
		// browser with such a source is a person who followed a link the assistant gave.
		o.Kind = AIAssistant
	}

	o.Medium = Clean(r.UTMMedium)
	if o.Medium == "" && r.MCP {
		o.Medium = "mcp"
	}
	o.KnownMedium = known(o.Medium, mediaSet)
	if o.Known == "" {
		o.Known = "other"
	}
	if o.Source == "" {
		o.Known = "none"
	}
	return o
}

// Clean keeps a caller's label short and plain: lower case letters, digits, dots and
// dashes, at most 40 characters. Spaces and underscores become dashes.
func Clean(value string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if out.Len() >= 40 {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-':
			out.WriteRune(r)
		case r == ' ', r == '_', r == '/':
			out.WriteByte('-')
		}
	}
	return strings.Trim(out.String(), "-.")
}

var mediaSet = func() map[string]bool {
	set := map[string]bool{}
	for _, m := range Media {
		set[m] = true
	}
	return set
}()

func known(value string, set map[string]bool) string {
	switch {
	case value == "":
		return "none"
	case set[value]:
		return value
	}
	return "other"
}

// assistants are the AI tools a source may name.
var assistants = map[string]bool{
	"claude": true, "chatgpt": true, "gemini": true, "perplexity": true, "copilot": true, "cursor": true,
	"windsurf": true, "codex": true, "mistral": true, "deepseek": true, "grok": true, "meta-ai": true,
	"cline": true, "zed": true, "kiro": true,
}

// aliases maps what agents and sites call themselves to one name per assistant. A value
// that is already a known name maps to itself.
var aliases = map[string]string{
	"claude.ai": "claude", "anthropic": "claude", "claude-code": "claude", "claude-ai": "claude",
	"claude-desktop": "claude", "claude-user": "claude",
	"chatgpt.com": "chatgpt", "chat.openai.com": "chatgpt", "openai": "chatgpt", "gpt": "chatgpt",
	"openai-mcp": "chatgpt", "chatgpt-user": "chatgpt",
	"gemini.google.com": "gemini", "gemini-cli": "gemini", "bard": "gemini", "google-gemini": "gemini",
	"perplexity.ai": "perplexity", "perplexity-user": "perplexity",
	"copilot.microsoft.com": "copilot", "github-copilot": "copilot", "visual-studio-code": "copilot", "vscode": "copilot",
	"cursor-vscode": "cursor", "codex-cli": "codex", "codex-mcp-client": "codex", "mistralai-user": "mistral",
	"chat.mistral.ai": "mistral", "chat.deepseek.com": "deepseek", "grok.com": "grok", "meta.ai": "meta-ai",
}

// alias finds the known name for a source, matching a version suffix too
// ("claude-code-2.1" is claude).
func alias(source string) (string, bool) {
	if source == Own {
		return Own, true
	}
	if assistants[source] {
		return source, true
	}
	if name, ok := aliases[source]; ok {
		return name, true
	}
	for from, name := range aliases {
		if strings.HasPrefix(source, from+"-") || strings.HasPrefix(source, from+".") {
			return name, true
		}
	}
	for name := range assistants {
		if strings.HasPrefix(source, name+"-") {
			return name, true
		}
	}
	if name, ok := knownAgents[source]; ok {
		return name, true
	}
	return "", false
}

// referers are sites whose links lead here, by host.
var referers = map[string]string{
	"chatgpt.com": "chatgpt", "chat.openai.com": "chatgpt", "claude.ai": "claude", "perplexity.ai": "perplexity",
	"gemini.google.com": "gemini", "copilot.microsoft.com": "copilot", "chat.mistral.ai": "mistral",
	"chat.deepseek.com": "deepseek", "grok.com": "grok", "meta.ai": "meta-ai",
	"google.com": "google", "bing.com": "bing", "duckduckgo.com": "duckduckgo", "search.yahoo.com": "yahoo",
	"github.com": "github", "news.ycombinator.com": "hackernews", "reddit.com": "reddit", "x.com": "x",
	"t.co": "x", "linkedin.com": "linkedin", "lnkd.in": "linkedin",
}

// refererSource names the site a link was followed from, or "" for none, this site, or
// one not in the list. Google's country domains count as google.
func refererSource(referer string) string {
	if referer == "" {
		return ""
	}
	parsed, err := url.Parse(referer)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host == "" {
		return ""
	}
	for {
		if name, ok := referers[host]; ok {
			return name
		}
		if strings.HasPrefix(host, "google.") {
			return "google"
		}
		dot := strings.IndexByte(host, '.')
		if dot < 0 || !strings.Contains(host[dot+1:], ".") {
			return ""
		}
		host = host[dot+1:]
	}
}

type agentRule struct {
	token string // lower case, found anywhere in the user agent
	name  string
	kind  string
}

// agentRules are tried in order; the first token found in the user agent wins. AI and
// crawler tokens come before the browser rule because their user agents also start
// with Mozilla.
var agentRules = []agentRule{
	{"uptime-kuma", "own", Own}, {"beacon/", "own", Own}, {"india-api-check", "own", Own},

	{"chatgpt-user", "chatgpt", AIAssistant}, {"claude-user", "claude", AIAssistant},
	{"perplexity-user", "perplexity", AIAssistant}, {"mistralai-user", "mistral", AIAssistant},
	{"claude-code", "claude", AIAssistant}, {"claude-desktop", "claude", AIAssistant},
	{"openai-mcp", "chatgpt", AIAssistant}, {"cursor/", "cursor", AIAssistant}, {"windsurf", "windsurf", AIAssistant},
	{"codex", "codex", AIAssistant}, {"gemini-cli", "gemini", AIAssistant}, {"google-gemini", "gemini", AIAssistant},
	{"copilot", "copilot", AIAssistant}, {"duckassistbot", "duckassistbot", AIAssistant},

	{"gptbot", "gptbot", AICrawler}, {"oai-searchbot", "oai-searchbot", AICrawler},
	{"claudebot", "claudebot", AICrawler}, {"claude-searchbot", "claude-searchbot", AICrawler},
	{"anthropic-ai", "claudebot", AICrawler}, {"perplexitybot", "perplexitybot", AICrawler},
	{"ccbot", "ccbot", AICrawler}, {"bytespider", "bytespider", AICrawler}, {"amazonbot", "amazonbot", AICrawler},
	{"meta-externalagent", "meta-externalagent", AICrawler}, {"cohere-ai", "cohere-ai", AICrawler},
	{"google-cloudvertexbot", "google-vertex", AICrawler}, {"applebot-extended", "applebot", AICrawler},
	{"mistralai", "mistralai", AICrawler}, {"youbot", "youbot", AICrawler}, {"timpibot", "timpibot", AICrawler},
	{"diffbot", "diffbot", AICrawler}, {"ai2bot", "ai2bot", AICrawler},

	{"googlebot", "googlebot", SearchCrawler}, {"google-inspectiontool", "googlebot", SearchCrawler},
	{"bingbot", "bingbot", SearchCrawler}, {"applebot", "applebot", SearchCrawler},
	{"duckduckbot", "duckduckbot", SearchCrawler}, {"yandex", "yandexbot", SearchCrawler},
	{"baiduspider", "baiduspider", SearchCrawler}, {"petalbot", "petalbot", SearchCrawler},
	{"seznambot", "seznambot", SearchCrawler},

	// An MCP proxy that real clients run, before the directories' probes below.
	{"mcp-remote", "mcp-remote", Program},
	// MCP and agent directories checking the server is up: the largest class of bot here.
	{"mcp", "mcp-directory", Bot}, {"agentrank", "mcp-directory", Bot}, {"agent-collector", "mcp-directory", Bot},
	{"agent-tools", "mcp-directory", Bot}, {"agentic-web", "mcp-directory", Bot}, {"aiagentboard", "mcp-directory", Bot},
	{"agenstry", "mcp-directory", Bot}, {"proofbench", "mcp-directory", Bot},

	{"facebookexternalhit", "facebook", Bot}, {"facebookcatalog", "facebook", Bot}, {"telegrambot", "telegrambot", Bot},
	{"twitterbot", "twitterbot", Bot}, {"slackbot", "slackbot", Bot}, {"discordbot", "discordbot", Bot},
	{"whatsapp", "whatsapp", Bot}, {"linkedinbot", "linkedinbot", Bot}, {"ahrefsbot", "ahrefsbot", Bot},
	{"semrushbot", "semrushbot", Bot}, {"mj12bot", "mj12bot", Bot}, {"dotbot", "dotbot", Bot},
	{"uptimerobot", "uptimerobot", Bot}, {"censys", "censys", Bot}, {"expanse", "expanse", Bot},
	{"zgrab", "zgrab", Bot}, {"nuclei", "nuclei", Bot}, {"headlesschrome", "headless-browser", Bot},
	// Unnamed bots. A bare "bot" would also match phone models in browser user agents
	// (CUBOT), so only the forms bots write are matched.
	{"bot/", "bot", Bot}, {"bot;", "bot", Bot}, {"bot)", "bot", Bot}, {"+http", "bot", Bot},
	{"crawler", "bot", Bot}, {"spider", "bot", Bot}, {"scanner", "bot", Bot}, {"probe", "bot", Bot},
	{"collector", "bot", Bot}, {"survey", "bot", Bot}, {"audit", "bot", Bot}, {"seo-", "bot", Bot},
	{"health check", "bot", Bot}, {"monitor", "bot", Bot},

	// Our own client packages, before the generic python and node rules.
	{"india-api-js", "india-api-js", Program}, {"india-api-python", "india-api-python", Program},
	{"curl/", "curl", Program}, {"wget", "wget", Program}, {"python-requests", "python", Program},
	{"python-httpx", "python", Program}, {"aiohttp", "python", Program}, {"python-urllib", "python", Program},
	{"python", "python", Program}, {"node-fetch", "node", Program}, {"undici", "node", Program},
	{"axios", "node", Program}, {"node", "node", Program}, {"deno", "deno", Program}, {"bun/", "bun", Program},
	{"go-http-client", "go", Program}, {"okhttp", "java", Program}, {"java", "java", Program},
	{"apache-httpclient", "java", Program}, {"dart", "dart", Program}, {"guzzle", "php", Program},
	{"php", "php", Program}, {"ruby", "ruby", Program}, {"faraday", "ruby", Program},
	{"reqwest", "rust", Program}, {"postmanruntime", "postman", Program}, {"insomnia", "insomnia", Program},
	{"httpie", "httpie", Program}, {"powershell", "powershell", Program}, {"libwww-perl", "perl", Program},

	// "Mozilla/5.0 (compatible; Name/1.0)" is how bots write themselves; browsers have not
	// since Internet Explorer.
	{"(compatible;", "bot", Bot},

	// A browser names its engine. "Mozilla/5.0" alone, or with only a comment, is a
	// script dressed up as one.
	{"applewebkit/", "browser", Person}, {"gecko/", "browser", Person}, {"opera/", "browser", Person},
	{"mozilla/", "bot", Bot},
}

// knownAgents is every name the rules give, so a utm_source naming one is known too.
var knownAgents = func() map[string]string {
	names := map[string]string{}
	for _, rule := range agentRules {
		names[rule.name] = rule.name
	}
	for _, name := range referers {
		names[name] = name
	}
	return names
}()

type agentMatch struct{ name, kind string }

func matchAgent(userAgent string) agentMatch {
	lower := strings.ToLower(userAgent)
	if strings.TrimSpace(lower) == "" {
		return agentMatch{"", Unknown}
	}
	for _, rule := range agentRules {
		if strings.Contains(lower, rule.token) {
			return agentMatch{rule.name, rule.kind}
		}
	}
	return agentMatch{"", Unknown}
}

// Tag adds utm_medium to one of our own addresses, keeping what is there.
func Tag(address, medium string) string {
	if medium == "" || strings.Contains(address, "utm_medium=") {
		return address
	}
	separator := "?"
	if strings.Contains(address, "?") {
		separator = "&"
	}
	return address + separator + "utm_medium=" + medium
}
