package api

import (
	"image/color"
	"net/url"
	"strings"
	"testing"
)

// The network tools read hosts and URLs that callers send, and icon files that strangers'
// sites send. None may panic, and what the URL reader accepts must stay within the rules.
func FuzzProbeURL(f *testing.F) {
	for _, seed := range []string{"https://site.in/", "site.in", "http://user:pw@site.in", "https://site.in:8443/", "HTTPS://Site.IN./a?b#c",
		"https://[::1]/", "https://भारत.in/", "ftp://x", "https://%zz/", "http://1.2.3.4:80/x"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		parsed, host, err := probeURL(raw)
		if err != nil {
			return
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" {
			t.Errorf("probeURL(%q) accepted %s", raw, parsed)
		}
		if port := parsed.Port(); port != "" && port != "80" && port != "443" {
			t.Errorf("probeURL(%q) accepted port %s", raw, port)
		}
		if again, err := hostOf(host); err != nil || again != host {
			t.Errorf("probeURL(%q) gave host %q, which hostOf reads as %q, %v", raw, host, again, err)
		}
		if reparsed, err := url.Parse(parsed.String()); err != nil || reparsed.Hostname() != strings.Trim(parsed.Host, "[]") && reparsed.Hostname() != host {
			t.Errorf("probeURL(%q) = %s does not parse back: %v", raw, parsed, err)
		}
	})
}

func FuzzDecodeICO(f *testing.F) {
	f.Add(icoWithBitmap(16, color.NRGBA{R: 1, A: 255}))
	f.Add(icoFile(0, 32, solidPNG(8, color.White)))
	f.Add([]byte{0, 0, 1, 0, 1, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if img, err := decodeICO(data); err == nil && (img.Bounds().Dx() <= 0 || img.Bounds().Dx() > 4096) {
			t.Errorf("decoded a %v picture", img.Bounds())
		}
		decodeIcon(data)
	})
}

func FuzzIconLinks(f *testing.F) {
	f.Add(`<link rel="icon" href="/a.png" sizes="16x16 32x32"><link rel=manifest href=m.json>`)
	f.Add(`<link rel="apple-touch-icon" href="javascript:alert(1)">`)
	base, _ := url.Parse("https://site.in/")
	f.Fuzz(func(t *testing.T, page string) {
		for _, link := range iconLinks([]byte(page), base) {
			if !strings.HasPrefix(link.URL, "http://") && !strings.HasPrefix(link.URL, "https://") {
				t.Errorf("iconLinks(%q) gave %q", page, link.URL)
			}
		}
		manifestIcons([]byte(page), base)
		largestSize(page)
	})
}
