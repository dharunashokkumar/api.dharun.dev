package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["test_regex"] = `{"pattern":"(\\d+)","text":"a 12 b 345","replace":"<$1>"}`
}

func TestRegex(t *testing.T) {
	got := expectStatus(t, "/v1/regex?flags=i&pattern="+url.QueryEscape(regexPatternExample)+"&text="+url.QueryEscape(regexTextExample), http.StatusOK)
	if got.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("a regex request must not be cached: %q", got.header.Get("Cache-Control"))
	}
	data := got.data(t)
	if data["flavor"] != "js" || data["count"].(float64) != 3 {
		t.Fatalf("data: %v", data)
	}
	second := data["matches"].([]any)[1].(map[string]any)
	groups := second["groups"].([]any)
	if second["text"] != "ka-05" || groups[0].(map[string]any)["name"] != "state" || groups[0].(map[string]any)["text"] != "ka" || groups[1].(map[string]any)["text"] != "05" {
		t.Errorf("second match: %v", second)
	}
	if len(data["explanation"].([]any)) < 5 {
		t.Errorf("explanation: %v", data["explanation"])
	}

	// POST as JSON, with a replacement, in RE2.
	body := `{"pattern":"(\\w+)@(\\w+)","text":"mail ravi@example now","flavor":"re2","replace":"${2}:${1}"}`
	posted := request(t, http.MethodPost, "/v1/regex", body, "Content-Type", "application/json")
	if posted.status != http.StatusOK || posted.data(t)["replaced"] != "mail example:ravi now" {
		t.Errorf("POST: %d %s", posted.status, posted.body)
	}

	invalid := expectStatus(t, "/v1/regex?flavor=re2&pattern="+url.QueryEscape("a(?=b)")+"&text=ab", http.StatusUnprocessableEntity)
	if !strings.Contains(string(invalid.body), "invalid_pattern") {
		t.Errorf("lookahead in RE2: %s", invalid.body)
	}
	slow := expectStatus(t, "/v1/regex?pattern="+url.QueryEscape("(a+)+$")+"&text="+strings.Repeat("a", 40)+"b", http.StatusUnprocessableEntity)
	if !strings.Contains(string(slow.body), `"timeout"`) {
		t.Errorf("catastrophic pattern: %s", slow.body)
	}
	expectStatus(t, "/v1/regex?text=abc", http.StatusBadRequest)
	expectStatus(t, "/v1/regex?pattern=a&text=a&flags=x", http.StatusBadRequest)
	expectStatus(t, "/v1/regex?pattern=a&text=a&flavor=pcre", http.StatusBadRequest)
}
