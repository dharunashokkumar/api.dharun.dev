package api

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestTamilFontConvert(t *testing.T) {
	get := func(path string) map[string]any {
		t.Helper()
		return expectStatus(t, path, http.StatusOK).data(t)
	}
	// As pasted from a page that lost its font.
	toUnicode := get("/v1/tamil-font/convert?from=tscii&text=" + url.QueryEscape("¾Á¢ú Å½ì¸õ"))
	if toUnicode["text"] != "தமிழ் வணக்கம்" || toUnicode["from"] != "tscii" || toUnicode["to"] != "unicode" {
		t.Errorf("to Unicode: %v", toUnicode)
	}
	// And back: the same characters, and the bytes behind them.
	back := get("/v1/tamil-font/convert?from=unicode&to=tscii&text=" + url.QueryEscape("தமிழ் கொடு ₹"))
	raw, _ := base64.StdEncoding.DecodeString(back["base64"].(string))
	if string(raw) != "\xBE\xC1\xA2\xFA \xA6\xB8\xA1\xCE ?" || back["text"] != "¾Á¢ú ¦¸¡Î ?" {
		t.Errorf("to TSCII: %v (% X)", back, raw)
	}
	if unmapped, _ := back["unmapped"].([]any); len(unmapped) != 1 || unmapped[0] != "₹" {
		t.Errorf("unmapped: %v", back["unmapped"])
	}

	// Raw bytes, as read from an old file; 0xFF is not a TSCII character.
	posted := request(t, http.MethodPost, "/v1/tamil-font/convert?from=tscii", "\xA7\xB8\xA1\xFF", "Content-Type", "application/octet-stream")
	if data := posted.data(t); posted.status != http.StatusOK || data["text"] != "கோ�" || data["undefined"] != float64(1) {
		t.Errorf("bytes: %d %s", posted.status, posted.body)
	}
	asJSON := request(t, http.MethodPost, "/v1/tamil-font/convert", `{"text":"¾Á¢ú","from":"tscii"}`, "Content-Type", "application/json")
	if asJSON.status != http.StatusOK || asJSON.data(t)["text"] != "தமிழ்" {
		t.Errorf("JSON: %d %s", asJSON.status, asJSON.body)
	}
	if cache := posted.header.Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Errorf("conversions are not cached: %q", cache)
	}

	already := expectStatus(t, "/v1/tamil-font/convert?from=tscii&text="+url.QueryEscape("தமிழ்"), http.StatusBadRequest)
	if !strings.Contains(string(already.body), "already Unicode") {
		t.Errorf("Unicode given as TSCII: %s", already.body)
	}
	for _, bad := range []string{
		"/v1/tamil-font/convert?text=abc",                         // no from
		"/v1/tamil-font/convert?from=klingon&text=abc",            // unknown encoding
		"/v1/tamil-font/convert?from=unicode&text=abc",            // nothing to convert to
		"/v1/tamil-font/convert?from=unicode&to=unicode&text=abc", // nothing to do
		"/v1/tamil-font/convert?from=tscii&to=tscii&text=abc",     // neither side is Unicode
		"/v1/tamil-font/convert?from=tscii",                       // no text
	} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
	if empty := request(t, http.MethodPost, "/v1/tamil-font/convert?from=tscii", "", "Content-Type", "application/octet-stream"); empty.status != http.StatusBadRequest {
		t.Errorf("an empty body = %d", empty.status)
	}
}

func TestTamilFontEncodings(t *testing.T) {
	got := expectStatus(t, "/v1/tamil-font/encodings", http.StatusOK).json(t)
	list, _ := got["data"].([]any)
	if len(list) == 0 || list[0].(map[string]any)["id"] != "tscii" {
		t.Errorf("encodings: %v", got["data"])
	}
}
