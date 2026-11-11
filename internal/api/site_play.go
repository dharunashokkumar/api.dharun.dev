package api

// The playground (/play, growth plan Part 3 section 2): one bench for every endpoint.
// Find, build, send; share a request through the part of the address after #, which a
// browser never sends; history kept in the browser only; ready-made chains; a
// simulate switch that answers from the documented samples without calling the server.

import (
	"html/template"
	"net/http"
	"strings"
)

// playChains are the ready-made chains: each step names an endpoint and how to fill
// it from the answers before it (the "from" functions live in play.js).
var playChains = []map[string]any{
	{"id": "pincode-sky", "title": "Pincode → sunrise and panchang → share image",
		"about": "Look up a pincode, use its place for the sun and moon and the day's panchang, then draw a share card with what came back.",
		"steps": []map[string]string{
			{"path": "/v1/pincode/{pincode}", "set": "pincode=600001"},
			{"path": "/v1/sun-moon/{date}", "from": "pincodeToSky"},
			{"path": "/v1/panchang/{date}", "from": "pincodeToSky"},
			{"path": "/v1/og-image", "from": "skyToCard"},
		}},
	{"id": "gstin-holidays", "title": "GSTIN → format check → state → that state's bank holidays",
		"about": "A GSTIN starts with its state's code. Check the number, read the state, and list the bank holidays there this year.",
		"steps": []map[string]string{
			{"path": "/v1/validate/{kind}/{value}", "set": "kind=gstin&value=27AAPFU0939F1ZV"},
			{"path": "/v1/states/{code}", "from": "gstinToState"},
			{"path": "/v1/holidays/{year}", "from": "stateToHolidays"},
		}},
	{"id": "invoice-pdf", "title": "Invoice JSON → PDF → page image",
		"about": "Make a GST invoice PDF from JSON, then draw its first page as a picture.",
		"steps": []map[string]string{
			{"path": "/v1/invoice"},
			{"path": "/v1/pdf/render", "from": "blobToNext"},
		}},
	{"id": "domain", "title": "A domain → DNS → TLS → whois",
		"about": "Everything public about one domain: its records from four resolvers, its certificate, and who registered it.",
		"steps": []map[string]string{
			{"path": "/v1/dns/{name}", "set": "name=dharun.dev"},
			{"path": "/v1/tls/{host}", "from": "domainOn"},
			{"path": "/v1/whois/{query}", "from": "domainOn"},
		}},
	{"id": "notice", "title": "Photo of a Tamil notice → OCR → translation → speech",
		"about": "Read the text in a picture, translate it to English, and read it out. Uses model endpoints, which will need a free key.",
		"steps": []map[string]string{
			{"path": "/v1/ocr", "set": "lang=eng+tam"},
			{"path": "/v1/translate", "from": "ocrToTranslate"},
			{"path": "/v1/tts", "from": "translateToSpeech"},
		}},
}

const playHTML = `
<noscript><p class="aside">The playground runs in your browser and needs JavaScript. Every endpoint is also described, with a working example address, in the <a href="/docs">API reference</a>.</p></noscript>
<div class="play" id="play">
<div class="finder">
<label class="field"><span>Find an endpoint</span><input type="search" id="play-find" placeholder="pincode, pdf, tamil…" autocomplete="off"></label>
<ul id="play-list"></ul>
</div>
<div>
<div class="card" id="play-bench"><p class="note">Pick an endpoint on the left, or start with a chain below.</p></div>
<div class="card">
<h2>Settings</h2>
<div class="row"><label class="toggle"><input type="checkbox" id="play-simulate"> Simulate: answer from the documented samples, without calling the server</label></div>
<div id="play-allowance" class="note"></div>
<p class="note" id="play-key"></p>
<label class="field"><span>Extra request headers, one per line (Name: value)</span><textarea id="play-headers" rows="2" spellcheck="false"></textarea></label>
</div>
<div class="card"><h2>Chains</h2><p class="note">Send one answer into the next request.</p><div id="play-chains"></div></div>
<div class="card"><h2>History</h2><p class="note">Your last requests, kept in this browser only. Requests to private endpoints are never kept.</p><ul class="keylist" id="play-history"></ul></div>
</div>
</div>`

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	s.sendPage(w, r, "play", func(origin string) []byte {
		p := page{Origin: origin, Path: "/play", Nav: "play", Markdown: "/play.md", Title: "API playground: try every endpoint | " + productName, Heading: "Playground",
			Description: "Try every India API endpoint in your browser: search, build a request, share it, chain answers into the next request, or simulate the rate limit.",
			Body: appPage{Wide: true, Heading: "Playground", Lead: "Every endpoint on one bench. Find one, change its values, send it from your browser, share the request, or chain one answer into the next.",
				HTML: template.HTML(playHTML), Scripts: []string{"pow.js", "try.js", "play.js"},
				Data: pageData(map[string]any{"specs": s.allSpecs(), "chains": playChains, "limit": s.cfg.RateLimitPerMinute, "keyLimit": s.cfg.KeyRateLimitPerMinute})}}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Playground", "/play"}))
		return s.renderHTML("app", p)
	}, func(origin string) []byte {
		var out strings.Builder
		out.WriteString("# Playground\n\nThe playground at " + origin + "/play runs in a browser: it builds a request for any endpoint from the documented example, sends it, shows the answer and the rate-limit headers, and can chain one answer into the next request. Every endpoint is also described in " + origin + "/docs.md and " + origin + "/llms.txt.\n\n## Chains\n\n")
		for _, chain := range playChains {
			out.WriteString("- " + chain["title"].(string) + ": " + chain["about"].(string) + "\n")
		}
		return []byte(out.String())
	})
}

// handlePlaySamples serves the documented sample answers for the simulate switch.
func (s *Server) handlePlaySamples(w http.ResponseWriter, r *http.Request) {
	body := s.cached(r, "play-samples", func(origin string) []byte {
		host := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
		samples := map[string]string{}
		for _, group := range catalog {
			for _, e := range group.Endpoints {
				if found := s.sample(host, e); found != nil {
					samples[specOf(group, e, 0).ID] = found.JSON
				}
			}
		}
		return []byte(string(pageData(samples)))
	})
	writeBody(w, r, http.StatusOK, body, typeJSON, cacheShort, true)
}
