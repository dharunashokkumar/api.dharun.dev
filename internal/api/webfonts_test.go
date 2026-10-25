package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Every font file matches the checksum recorded with its source, and every font the
// styles name is served.
func TestWebFonts(t *testing.T) {
	list, err := webFonts.ReadFile("webfonts/SHA256SUMS")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(string(list), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.HasPrefix(line, "#") {
			continue
		}
		listed[fields[1]] = true
		raw, err := webFonts.ReadFile("webfonts/" + fields[1])
		if err != nil {
			t.Errorf("%s is listed but missing", fields[1])
			continue
		}
		if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != fields[0] {
			t.Errorf("%s does not match SHA256SUMS", fields[1])
		}
	}
	for _, match := range regexp.MustCompile(`url\(/fonts/([^)]+)\)`).FindAllStringSubmatch(siteCSS, -1) {
		if !listed[match[1]] {
			t.Errorf("%s is used by the styles but not in SHA256SUMS", match[1])
		}
		got := get(t, "/fonts/"+match[1])
		if got.status != http.StatusOK || got.header.Get("Content-Type") != "font/woff2" {
			t.Errorf("/fonts/%s: status %d, type %q", match[1], got.status, got.header.Get("Content-Type"))
		}
	}
	expectStatus(t, "/fonts/nothing.woff2", http.StatusNotFound)
	expectStatus(t, "/fonts/SHA256SUMS", http.StatusNotFound)
}
