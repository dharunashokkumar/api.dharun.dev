package api

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Tool sets: a client that wants fewer tools in its context connects to /mcp?tools=
// with set names (money,dates), and find_tool returns the few tools for a task. Every
// tool can still be called by name whatever set was asked for.

var toolSetNames = []string{"files", "money", "network", "text", "dates", "places"}

// toolSetRules decide a tool's set from words in its name, first match wins.
var toolSetRules = []struct {
	set   string
	words []string
}{
	{"network", []string{"dns", "email", "traceroute", "whois", "tls", "ipv6", "probe", "icon", "lookup_ip", "preview_link", "read_web_page"}},
	{"files", []string{"image", "pdf", "barcode", "qr_code", "invoice", "typst", "speech", "audio", "transcribe", "noise", "background", "chart", "code_image", "read_text_in_image", "document", "aadhaar"}},
	{"dates", []string{"holiday", "today", "panchang", "sun_moon", "festival", "date", "cron", "time_zone", "working", "satellite"}},
	{"money", []string{"calculate", "gst", "rbi", "gold", "mutual", "expiries", "index", "compan", "logo", "ifsc", "bank", "upi", "payment", "words", "indian_number", "hsn"}},
	{"text", []string{"translat", "moderate", "embed", "rerank", "chunk", "redact", "tamil", "braille", "keyboard", "letters", "compare_texts", "data_format", "hash", "jwt", "regex", "thirukkural", "fake", "ids", "color"}},
}

func toolSet(name string) string {
	for _, rule := range toolSetRules {
		for _, w := range rule.words {
			if strings.Contains(name, w) {
				return rule.set
			}
		}
	}
	return "places"
}

// filterToolSets keeps the tools of the named sets, and find_tool always.
func filterToolSets(tools []map[string]any, sets string) []map[string]any {
	if strings.TrimSpace(sets) == "" {
		return tools
	}
	want := map[string]bool{}
	for _, set := range strings.Split(strings.ToLower(sets), ",") {
		want[strings.TrimSpace(set)] = true
	}
	var out []map[string]any
	for _, tool := range tools {
		name := tool["name"].(string)
		if name == "find_tool" || want[toolSet(name)] {
			out = append(out, tool)
		}
	}
	return out
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "find_tool",
		Description: "Find the right India API tools for a task in a few words (such as \"gold rate\", \"merge pdf\", \"bank holiday\"): returns the best matching tools with their descriptions. Useful when connected to only some tool sets.",
		InputSchema: schema([]string{"task"}, map[string]any{"task": text("What you want to do.")}),
		path: func(args map[string]any) (string, error) {
			task, err := required(args, "task")
			return "/v1/mcp-tools?q=" + url.QueryEscape(task), err
		},
	})
}

func (s *Server) handleMCPTools(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	set := strings.ToLower(r.URL.Query().Get("set"))
	words := strings.FieldsFunc(q, func(r rune) bool { return r == ' ' || r == ',' || r == '-' || r == '_' })
	type found struct {
		Name        string `json:"name"`
		Set         string `json:"set"`
		Description string `json:"description"`
		score       int
	}
	var out []found
	for _, tool := range mcpTools {
		if tool.Name == "find_tool" {
			continue
		}
		f := found{Name: tool.Name, Set: toolSet(tool.Name), Description: tool.Description}
		if set != "" && f.Set != set {
			continue
		}
		name, description := strings.ToLower(tool.Name), strings.ToLower(tool.Description)
		for _, word := range words {
			if len(word) < 2 {
				continue
			}
			if strings.Contains(name, word) {
				f.score += 5
			}
			if strings.Contains(description, word) {
				f.score += 2
			}
			// A plural or a shorter stem: "pdfs", "holidays".
			if stem := strings.TrimSuffix(word, "s"); stem != word && strings.Contains(description, stem) {
				f.score++
			}
		}
		if q == "" || f.score > 0 {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	if q != "" && len(out) > 6 {
		out = out[:6]
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Note: "The MCP server's tools that match, best first. Connect to /mcp?tools=" + strings.Join(toolSetNames, ",") + " (any of them) to list fewer tools."}, cacheAPI)
}
