package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["count_tokens"] = `{"text":"hello world","model":"gpt-4"}`
	mcpTestCalls["redact_indian_identifiers"] = `{"text":"PAN ABCPK1234F","style":"partial"}`
	mcpTestCalls["detect_language"] = `{"text":"भारत एक विशाल देश है"}`
}

func TestTextChunk(t *testing.T) {
	document := "# Setup\n\nInstall Go. Clone the repository and build it.\n\n# Deploy\n\nPush to main. The server installs it."
	posted := request(t, http.MethodPost, "/v1/text/chunk?size=200&overlap=0", document, "Content-Type", "text/plain")
	if posted.status != http.StatusOK {
		t.Fatalf("POST chunk: %d %s", posted.status, posted.body)
	}
	data := posted.data(t)
	chunks := data["chunks"].([]any)
	if data["format"] != "markdown" || len(chunks) != 2 {
		t.Fatalf("chunks: %v", data)
	}
	if chunks[1].(map[string]any)["heading"] != "Deploy" || !strings.HasPrefix(chunks[1].(map[string]any)["text"].(string), "# Deploy") {
		t.Errorf("second chunk: %v", chunks[1])
	}
	if posted.header.Get("Cache-Control") != "private, no-store" {
		t.Error("a posted document must not be cached")
	}
	asJSON := request(t, http.MethodPost, "/v1/text/chunk", `{"text":"one two three","size":100,"format":"text"}`, "Content-Type", "application/json")
	if asJSON.status != http.StatusOK || asJSON.data(t)["format"] != "text" {
		t.Errorf("JSON body: %d %s", asJSON.status, asJSON.body)
	}
	expectStatus(t, "/v1/text/chunk?text=x&size=50", http.StatusBadRequest)
	expectStatus(t, "/v1/text/chunk?text=x&size=200&overlap=150", http.StatusBadRequest)
	expectStatus(t, "/v1/text/chunk?text=x&format=html", http.StatusBadRequest)
	expectStatus(t, "/v1/text/chunk", http.StatusBadRequest)
}

func TestTextRedact(t *testing.T) {
	text := "Ravi, PAN ABCPK1234F, phone +91 98765 43210, Aadhaar 2341 2341 2346."
	result := expectStatus(t, "/v1/text/redact?style=partial&text="+url.QueryEscape(text), http.StatusOK).data(t)
	if result["text"] != "Ravi, PAN XXXXXXXX4F, phone +XX XXXXX XXX10, Aadhaar XXXX XXXX 2346." {
		t.Errorf("partial: %q", result["text"])
	}
	counts := result["counts"].(map[string]any)
	if counts["pan"].(float64) != 1 || counts["phone"].(float64) != 1 || counts["aadhaar"].(float64) != 1 {
		t.Errorf("counts: %v", counts)
	}
	only := expectStatus(t, "/v1/text/redact?types=pan&text="+url.QueryEscape(text), http.StatusOK).data(t)
	if !strings.Contains(only["text"].(string), "98765") || !strings.Contains(only["text"].(string), "[PAN]") {
		t.Errorf("types=pan: %q", only["text"])
	}
	expectStatus(t, "/v1/text/redact?types=names&text=x", http.StatusBadRequest)
	expectStatus(t, "/v1/text/redact?style=hash&text=x", http.StatusBadRequest)
}

func TestTextTokens(t *testing.T) {
	count := func(path string) map[string]any { return expectStatus(t, path, http.StatusOK).data(t) }
	if hello := count("/v1/text/tokens?text=hello%20world"); hello["tokens"].(float64) != 2 || hello["encoding"] != "o200k_base" || hello["words"].(float64) != 2 {
		t.Errorf("hello world: %v", hello)
	}
	if gpt4 := count("/v1/text/tokens?model=gpt-4&text=hello%20world"); gpt4["encoding"] != "cl100k_base" || gpt4["tokens"].(float64) != 2 {
		t.Errorf("gpt-4: %v", gpt4)
	}
	tamil := count("/v1/text/tokens?text=" + url.QueryEscape("வணக்கம்"))
	if tamil["characters"].(float64) != 7 || tamil["tokens"].(float64) < 2 || tamil["bytes"].(float64) != 21 {
		t.Errorf("Tamil: %v", tamil)
	}
	expectStatus(t, "/v1/text/tokens?model=claude-3&text=hi", http.StatusBadRequest)
}

func TestTextLanguage(t *testing.T) {
	if hindi := expectStatus(t, "/v1/text/language?text="+url.QueryEscape("भारत एक विशाल देश है जहाँ अनेक भाषाएँ बोली जाती हैं"), http.StatusOK).data(t); hindi["code"] != "hin" || hindi["code2"] != "hi" {
		t.Errorf("Hindi: %v", hindi)
	}
	expectStatus(t, "/v1/text/language?text=12345", http.StatusUnprocessableEntity)
}

// A tool call through /mcp costs what its endpoint costs, not one lookup.
func TestMCPPaysFullCost(t *testing.T) {
	server := newTestServer(Config{Version: "t", RateLimitPerMinute: 25})
	defer server.Close()
	call := func() string {
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"count_tokens","arguments":{"text":"hi"}}}`))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "203.0.113.50:4000"
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		return recorder.Body.String()
	}
	// 25 a minute: each call costs 1 for /mcp and 9 more for the tool.
	results := []bool{}
	for i := 0; i < 3; i++ {
		results = append(results, strings.Contains(call(), `"isError":true`))
	}
	if results[0] || results[1] || !results[2] {
		t.Errorf("the third call should be refused by the rate limit: %v", results)
	}
}
