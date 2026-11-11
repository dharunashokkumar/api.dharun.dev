package api

import (
	"bytes"
	"image/png"
	"net/http"
	"strings"
	"testing"
)

// A brand in the fixtures known only by its site icon.
const siteIconBrand = "cred"

func init() {
	mcpTestCalls["find_logo"] = `{"query":"phone","set":"brand"}`
}

func TestWorldLogoSets(t *testing.T) {
	for path, set := range map[string]string{
		"/v1/brand-logos/phonepe":              "brand",
		"/v1/brand-logos/PhonePe.svg":          "brand",
		"/v1/brand-logos/phonepe.com":          "brand",
		"/v1/brand-logos/google%20pay":         "brand",
		"/v1/brand-logos/gpay":                 "brand",
		"/v1/stock-logos/AAPL":                 "stock",
		"/v1/stock-logos/nasdaq:aapl":          "stock",
		"/v1/stock-logos/US0378331005":         "stock",
		"/v1/stock-logos/apple":                "stock",
		"/v1/stock-logos/VOD.L":                "stock",
		"/v1/stock-logos/0700.HK":              "stock",
		"/v1/stock-logos/AAPL?exchange=nasdaq": "stock",
		"/v1/asset-logos/BTC":                  "asset",
		"/v1/asset-logos/btcusdt":              "asset",
		"/v1/asset-logos/BTC-INR":              "asset",
		"/v1/asset-logos/NIFTY":                "asset",
		"/v1/asset-logos/nifty%2050":           "asset",
		"/v1/asset-logos/GOLD?kind=commodity":  "asset",
		"/v1/asset-logos/NIFTYBEES":            "asset",
	} {
		got := expectStatus(t, path, http.StatusOK)
		if got.header.Get("X-Logo-Set") != set || got.header.Get("X-Logo-Fallback") != "" || !bytes.HasPrefix(got.body, []byte("<svg")) {
			t.Errorf("%s: set %q fallback %q body %s", path, got.header.Get("X-Logo-Set"), got.header.Get("X-Logo-Fallback"), clip(string(got.body), 60))
		}
		if got.header.Get("Cache-Control") != cacheLong || got.header.Get("Content-Security-Policy") != svgPolicy {
			t.Errorf("%s: headers %v", path, got.header)
		}
	}

	// Different spellings of one entry give one picture.
	apple := expectStatus(t, "/v1/stock-logos/AAPL", http.StatusOK).body
	if !bytes.Equal(apple, expectStatus(t, "/v1/stock-logos/apple", http.StatusOK).body) {
		t.Error("AAPL and apple answered different pictures")
	}
	// A brand known only by its site icon is a PNG inside an SVG.
	icon := expectStatus(t, "/v1/brand-logos/"+siteIconBrand, http.StatusOK)
	if !bytes.Contains(icon.body, []byte(`href="data:image/png;base64,`)) {
		t.Errorf("site-icon brand: %s", clip(string(icon.body), 120))
	}
	if picture := expectStatus(t, "/v1/brand-logos/"+siteIconBrand+"?format=png&size=48", http.StatusOK); picture.header.Get("Content-Type") != typePNG {
		t.Errorf("site-icon brand PNG: %v", picture.header)
	} else if decoded, err := png.Decode(bytes.NewReader(picture.body)); err != nil || decoded.Bounds().Dx() != 48 {
		t.Errorf("site-icon brand PNG: %v", err)
	}

	// Unknown names: the tile, a day's cache, written down; fallback=none keeps the 404.
	for _, path := range []string{"/v1/brand-logos/zz-no-brand", "/v1/stock-logos/ZZNOSTOCK", "/v1/asset-logos/ZZNOCOIN", "/v1/stock-logos/AAPL?exchange=lse"} {
		tile := expectStatus(t, path, http.StatusOK)
		if tile.header.Get("X-Logo-Known") != "false" || tile.header.Get("Cache-Control") != cacheUnknownLogo {
			t.Errorf("%s: headers %v", path, tile.header)
		}
		expectStatus(t, path+map[bool]string{true: "&", false: "?"}[strings.Contains(path, "?")]+"fallback=none", http.StatusNotFound)
	}
	for _, path := range []string{
		"/v1/stock-logos/AAPL?exchange=nyse2", "/v1/asset-logos/BTC?kind=bond", "/v1/brand-logos/phonepe?category=x",
		"/v1/stock-logos/AAPL?format=gif", "/v1/asset-logos/" + strings.Repeat("A", maxLogoSymbol+1),
	} {
		expectStatus(t, path, http.StatusBadRequest)
	}
}

func TestLogosFallThrough(t *testing.T) {
	for path, set := range map[string]string{
		"/v1/logos/TCS":                  "india",
		"/v1/logos/phonepe":              "brand",
		"/v1/logos/AAPL":                 "stock",
		"/v1/logos/NASDAQ:AAPL":          "stock",
		"/v1/logos/VOD.L":                "stock",
		"/v1/logos/AAPL?exchange=nasdaq": "stock",
		"/v1/logos/BTC":                  "asset",
		"/v1/logos/SENSEX":               "asset",
	} {
		got := expectStatus(t, path, http.StatusOK)
		if got.header.Get("X-Logo-Set") != set || got.header.Get("X-Logo-Known") != "" {
			t.Errorf("%s: set %q known %q", path, got.header.Get("X-Logo-Set"), got.header.Get("X-Logo-Known"))
		}
	}
	// The same picture as at the set's own address.
	if !bytes.Equal(expectStatus(t, "/v1/logos/AAPL", http.StatusOK).body, expectStatus(t, "/v1/stock-logos/AAPL", http.StatusOK).body) {
		t.Error("/v1/logos/AAPL differs from /v1/stock-logos/AAPL")
	}
	// exchange=nse asks for the Indian listing only.
	if got := expectStatus(t, "/v1/logos/AAPL?exchange=nse", http.StatusOK); got.header.Get("X-Logo-Known") != "false" {
		t.Errorf("AAPL on NSE: %v", got.header)
	}
	expectStatus(t, "/v1/logos/TCS?exchange=auto", http.StatusOK)
}

func TestWorldLogoSearch(t *testing.T) {
	search := func(path string) []map[string]any {
		t.Helper()
		body := expectStatus(t, path, http.StatusOK).json(t)
		list, _ := body["data"].([]any)
		out := make([]map[string]any, len(list))
		for i, item := range list {
			out[i] = item.(map[string]any)
		}
		if source := body["meta"].(map[string]any)["source"]; source == nil {
			t.Errorf("%s: no meta.source", path)
		}
		return out
	}
	brands := search("/v1/brand-logos?q=phone")
	if len(brands) == 0 || brands[0]["symbol"] != "phonepe" || brands[0]["category"] != "payments" {
		t.Fatalf("brand search: %v", brands)
	}
	artwork, _ := brands[0]["artwork"].(map[string]any)
	if artwork == nil || artwork["origin"] != "commons" || artwork["licence"] == "" || !strings.Contains(artwork["url"].(string), "commons.wikimedia.org") {
		t.Errorf("PhonePe artwork: %v", brands[0]["artwork"])
	}
	logo := brands[0]["logo"].(map[string]any)
	if !strings.HasSuffix(logo["svg"].(string), "/v1/brand-logos/phonepe") {
		t.Errorf("logo address: %v", logo)
	}

	stocks := search("/v1/stock-logos?q=app&exchange=nasdaq")
	if len(stocks) == 0 || stocks[0]["symbol"] != "AAPL" || stocks[0]["artwork"] != nil {
		t.Errorf("stock search: %v", stocks)
	}
	indices := search("/v1/asset-logos?kind=index")
	for _, index := range indices {
		if index["kind"] != "index" {
			t.Errorf("kind=index returned %v", index)
		}
	}
	if len(indices) < 2 {
		t.Errorf("indices: %v", indices)
	}
	if all := search("/v1/stock-logos?limit=100"); len(all) < 3 {
		t.Errorf("listing every stock: %d", len(all))
	}
	expectStatus(t, "/v1/stock-logos?q=%25%25", http.StatusBadRequest)
	expectStatus(t, "/v1/asset-logos?kind=bond", http.StatusBadRequest)
}
