package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["transliterate_indian_text"] = `{"text":"நன்றி","to":"hi"}`
}

func TestTransliterate(t *testing.T) {
	get := func(path string) map[string]any { return expectStatus(t, path, http.StatusOK).data(t) }
	if tamil := get("/v1/transliterate?to=ta&text=" + url.QueryEscape("नमस्ते भारत")); tamil["text"] != "நமஸ்தே பாரத" || tamil["from"] != "devanagari" || tamil["to"] != "tamil" {
		t.Errorf("to Tamil: %v", tamil)
	}
	if roman := get("/v1/transliterate?to=ascii&text=" + url.QueryEscape("வணக்கம் தமிழ்")); roman["text"] != "vanakkam tamizh" {
		t.Errorf("to ASCII: %v", roman)
	}
	if back := get("/v1/transliterate?from=iast&to=devanagari&text=" + url.QueryEscape("saṃskṛtam")); back["text"] != "संस्कृतम्" {
		t.Errorf("from IAST: %v", back)
	}
	posted := request(t, http.MethodPost, "/v1/transliterate?to=kannada", "हिन्दी", "Content-Type", "text/plain")
	if posted.status != http.StatusOK || posted.data(t)["text"] != "ಹಿನ್ದೀ" {
		t.Errorf("POST: %d %s", posted.status, posted.body)
	}
	for _, bad := range []string{
		"/v1/transliterate?text=abc",                          // no target
		"/v1/transliterate?to=klingon&text=abc",               // unknown target
		"/v1/transliterate?to=tamil&text=vanakkam",            // Roman text without a scheme
		"/v1/transliterate?to=tamil&from=ascii&text=vanakkam", // casual spelling refused with a reason
		"/v1/transliterate?to=iast&from=iso15919&text=x",      // roman to roman
	} {
		got := expectStatus(t, bad, http.StatusBadRequest)
		if !strings.Contains(string(got.body), "message") {
			t.Errorf("%s: no explanation", bad)
		}
	}
}
