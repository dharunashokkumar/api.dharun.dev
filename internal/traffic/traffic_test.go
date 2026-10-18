package traffic

import "testing"

const (
	chrome    = "Mozilla/5.0 (Linux; Android 14; CUBOT X30) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36"
	firefox   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:158.0) Gecko/20100101 Firefox/158.0"
	chatgpt   = "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot"
	googlebot = "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Googlebot/2.1; +http://www.google.com/bot.html) Chrome/141.0.0.0 Safari/537.36"
	gptbot    = "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; GPTBot/1.2; +https://openai.com/gptbot"
)

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		name                      string
		request                   Request
		source, known, kind, medi string
	}{
		{"browser", Request{UserAgent: firefox}, "browser", "browser", Person, "none"},
		{"phone named CUBOT is not a bot", Request{UserAgent: chrome}, "browser", "browser", Person, "none"},
		{"docs link in a browser", Request{UserAgent: firefox, UTMMedium: "docs"}, "browser", "browser", Person, "docs"},
		{"agent names itself", Request{UserAgent: "curl/8.7.1", UTMSource: "claude", UTMMedium: "llms"}, "claude", "claude", AIAssistant, "llms"},
		{"agent with a version", Request{UserAgent: "node", UTMSource: "Claude Code 2.1"}, "claude-code-2.1", "claude", AIAssistant, "none"},
		{"link in a ChatGPT answer", Request{UserAgent: chrome, UTMSource: "chatgpt.com"}, "chatgpt.com", "chatgpt", Person, "none"},
		{"ChatGPT fetching", Request{UserAgent: chatgpt}, "chatgpt", "chatgpt", AIAssistant, "none"},
		{"from a Claude conversation", Request{UserAgent: firefox, Referer: "https://claude.ai/chat/abc"}, "claude", "claude", Person, "none"},
		{"from Google India", Request{UserAgent: firefox, Referer: "https://www.google.co.in/"}, "google", "google", Person, "none"},
		{"from our own page", Request{UserAgent: firefox, Referer: "https://api.dharun.dev/docs/logos", UTMMedium: "docs-preview"}, "browser", "browser", Person, "docs-preview"},
		{"MCP client", Request{UserAgent: "python-httpx/0.28", Client: "claude-ai", MCP: true}, "claude-ai", "claude", AIAssistant, "mcp"},
		{"MCP unknown client", Request{UserAgent: "node", Client: "my-agent", MCP: true}, "my-agent", "other", Program, "mcp"},
		{"Googlebot", Request{UserAgent: googlebot}, "googlebot", "googlebot", SearchCrawler, "none"},
		{"GPTBot", Request{UserAgent: gptbot}, "gptbot", "gptbot", AICrawler, "none"},
		{"crawler claiming an assistant", Request{UserAgent: gptbot, UTMSource: "claude"}, "claude", "claude", AICrawler, "none"},
		{"own monitor", Request{UserAgent: "Uptime-Kuma/2.5.5"}, "own", "own", Own, "none"},
		{"own check", Request{UserAgent: "india-api-check/1 (+https://github.com/dharunashokkumar/api.dharun.dev)"}, "own", "own", Own, "none"},
		{"tagged own", Request{UserAgent: "curl/8.21.0", UTMSource: "own"}, "own", "own", Own, "none"},
		{"unknown tag", Request{UserAgent: "curl/8.5.0", UTMSource: "my newsletter!", UTMMedium: "Email"}, "my-newsletter", "other", Program, "other"},
		{"nothing at all", Request{}, "", "none", Unknown, "none"},
		{"unnamed bot", Request{UserAgent: "Mozilla/5.0 (compatible; Thing/1.0; +http://thing.example)"}, "bot", "bot", Bot, "none"},
		{"python", Request{UserAgent: "python-requests/2.32.3"}, "python", "python", Program, "none"},
		{"our python client", Request{UserAgent: "india-api-python/3f2a9c1b0d4e"}, "india-api-python", "india-api-python", Program, "none"},
		{"our js client", Request{UserAgent: "india-api-js/3f2a9c1b0d4e"}, "india-api-js", "india-api-js", Program, "none"},
		{"compatible form", Request{UserAgent: "Mozilla/5.0 (compatible; Thing/1.0)"}, "bot", "bot", Bot, "none"},
		{"MCP directory", Request{UserAgent: "mcpbeat/0.1 (+https://mcpbeat.com/bot/; liveness check)", MCP: true}, "mcp-directory", "mcp-directory", Bot, "mcp"},
		{"MCP directory without a link", Request{UserAgent: "io.verifymcp/probe"}, "mcp-directory", "mcp-directory", Bot, "none"},
		{"MCP proxy", Request{UserAgent: "mcp-remote/0.1.29", Client: "claude-ai", MCP: true}, "claude-ai", "claude", AIAssistant, "mcp"},
		{"bare Mozilla", Request{UserAgent: "Mozilla/5.0 (read-only documentation audit)"}, "bot", "bot", Bot, "none"},
		{"Telegram preview", Request{UserAgent: "TelegramBot (like TwitterBot)"}, "telegrambot", "telegrambot", Bot, "none"},
		{"reduced Android Chrome", Request{UserAgent: "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Mobile Safari/537.36"}, "browser", "browser", Person, "none"},
	} {
		got := Classify(c.request)
		if got.Source != c.source || got.Known != c.known || got.Kind != c.kind || got.KnownMedium != c.medi {
			t.Errorf("%s: got source %q known %q kind %q medium %q, want %q %q %q %q",
				c.name, got.Source, got.Known, got.Kind, got.KnownMedium, c.source, c.known, c.kind, c.medi)
		}
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"Claude":                    "claude",
		"  ChatGPT.com ":            "chatgpt.com",
		"claude_code":               "claude-code",
		"<script>alert(1)</script>": "scriptalert1-script",
		"a@b.example":               "ab.example",
		"--x--":                     "x",
		"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz": "abcdefghijklmnopqrstuvwxyzabcdefghijklmn",
	} {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTag(t *testing.T) {
	for _, c := range [][3]string{
		{"https://api.dharun.dev/v1/pincode/637001", "docs", "https://api.dharun.dev/v1/pincode/637001?utm_medium=docs"},
		{"/v1/logos/TCS?format=png", "docs", "/v1/logos/TCS?format=png&utm_medium=docs"},
		{"/v1/logos/TCS?utm_medium=guide", "docs", "/v1/logos/TCS?utm_medium=guide"},
		{"/v1/rto/TN-28", "", "/v1/rto/TN-28"},
	} {
		if got := Tag(c[0], c[1]); got != c[2] {
			t.Errorf("Tag(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// Every known name the metrics can carry stays a short, fixed list.
func TestKnownNamesAreBounded(t *testing.T) {
	names := map[string]bool{"other": true, "none": true, Own: true}
	for name := range assistants {
		names[name] = true
	}
	for _, name := range knownAgents {
		names[name] = true
	}
	for _, name := range aliases {
		if !names[name] {
			t.Errorf("alias target %q is not a known name", name)
		}
	}
	if len(names) > 150 {
		t.Errorf("%d known source names; the metric must stay small", len(names))
	}
}
