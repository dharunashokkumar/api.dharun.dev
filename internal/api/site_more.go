package api

// More pages of growth plan Part 3: company logo pages (/logos, /logos/{symbol}), the
// demos (/demos), and the routes for the playground and the tools. The changelog and
// the API keys page are Markdown templates like privacy and about.

import (
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/traffic"
)

func (s *Server) registerSiteMore() {
	s.handle("docs", costFree, "GET /play", s.handlePlay)
	s.handle("docs", costFree, "GET /play.md", s.handlePlay)
	s.handle("docs", costFree, "GET /play/samples.json", s.handlePlaySamples)
	s.handle("docs", costFree, "GET /tools", s.handleTools)
	s.handle("docs", costFree, "GET /tools.md", s.handleTools)
	s.handle("docs", costFree, "GET /tools/{slug}", s.handleTool)
	s.handle("docs", costFree, "GET /demos", s.handleDemos)
	s.handle("docs", costFree, "GET /demos.md", s.handleDemos)
	s.handle("docs", costFree, "GET /logos.md", s.handleLogoIndex)
	s.handle("docs", costFree, "GET /demos/{slug}", s.handleDemo)
	s.handle("docs", costFree, "GET /logos", s.handleLogoIndex)
	s.handle("docs", costFree, "GET /logos/{symbol}", s.handleLogoPage)
}

// ---- company logo pages ----

// logoPageSymbol is the address of a company's logo page: the ticker, with the exchange
// only for a company listed on BSE alone.
func logoPageSymbol(c *company) string {
	if c.Exchange == "NSE" {
		return escapeSymbol(c.Ticker)
	}
	return symbolRef(c)
}

// escapeSymbol escapes a symbol for a path the way symbolRef does: & as %26, : kept.
func escapeSymbol(symbol string) string {
	return strings.NewReplacer("%3A", ":", "&", "%26").Replace(url.PathEscape(symbol))
}

func (s *Server) handleLogoIndex(w http.ResponseWriter, r *http.Request) {
	s.sendPage(w, r, "logos-index", func(origin string) []byte {
		index := s.companies.Load()
		var leaders []*company
		for _, c := range index.all {
			if c.Listed && c.HasLogo && c.Exchange == "NSE" && containsFold(c.Indexes, "NIFTY 50") {
				leaders = append(leaders, c)
			}
		}
		if len(leaders) == 0 {
			for _, c := range index.all {
				if c.Listed && c.HasLogo && len(leaders) < 60 {
					leaders = append(leaders, c)
				}
			}
		}
		sort.Slice(leaders, func(i, j int) bool { return leaders[i].marketCap > leaders[j].marketCap })
		var html strings.Builder
		html.WriteString(`<form class="card form" action="/logos" method="get" id="logo-find"><label>Company or ticker <input name="q" placeholder="TCS, Infosys, HDFC Bank" autocomplete="off"></label><div class="row"><button class="key key--primary" type="submit">Find the logo</button></div></form><div id="logo-results"></div>`)
		html.WriteString(`<h2 style="margin-block:var(--space-xl) var(--space-md)">The largest listed companies</h2><div class="logo-wall">`)
		for _, c := range leaders {
			html.WriteString(`<figure><a href="/logos/` + logoPageSymbol(c) + `"><img src="/v1/logos/` + symbolRef(c) + `" alt="` + template.HTMLEscapeString(c.Company) + ` logo" loading="lazy" width="48" height="48"></a><figcaption>` + template.HTMLEscapeString(c.Ticker) + `</figcaption></figure>`)
		}
		html.WriteString(`</div><p class="aside">Every logo here is one address: <code>` + origin + `/v1/logos/TCS</code>. See <a href="` + traffic.Tag("/docs/logos", "logos") + `">the logo API</a>. Logos and trademarks belong to their owners.</p>`)
		p := page{Origin: origin, Path: "/logos", Markdown: "/logos.md", Title: "Logos of Indian listed companies: SVG and PNG | " + productName, Heading: "Company logos",
			Description: "Logos of every company listed on NSE and BSE, free as SVG and PNG, with an image address to put in any app. Search by company name or ticker.",
			Body: appPage{Heading: "Company logos", Lead: "The logo of every company listed on NSE and BSE, as SVG and PNG, at one address each.", HTML: template.HTML(html.String()),
				Scripts: []string{"logos.js"}}}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Logos", "/logos"}))
		return s.renderHTML("app", p)
	}, func(origin string) []byte {
		return []byte("# Company logos\n\nThe logo of every company listed on NSE and BSE, as SVG and PNG, at one address each: " + origin + "/v1/logos/TCS (SVG) or " + origin + "/v1/logos/TCS?format=png&size=256. Search companies with " + origin + "/v1/companies?q=tata. Documentation: " + origin + "/docs/logos.md. Logos and trademarks belong to their owners.\n")
	})
}

func containsFold(list []string, want string) bool {
	for _, item := range list {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}

func (s *Server) handleLogoPage(w http.ResponseWriter, r *http.Request) {
	symbol, markdown := strings.CutSuffix(r.PathValue("symbol"), ".md")
	c, err := s.companies.Load().resolve(symbol, "")
	if err != nil || c == nil {
		s.notFound(w, r, "No listed company with that ticker. Search at /logos.")
		return
	}
	// One address per company: the canonical one.
	if canonical := logoPageSymbol(c); canonical != escapeSymbol(symbol) {
		suffix := ""
		if markdown {
			suffix = ".md"
		}
		http.Redirect(w, r, "/logos/"+canonical+suffix, http.StatusMovedPermanently)
		return
	}
	s.sendPage(w, r, "logo-"+c.ID, func(origin string) []byte {
		ref := symbolRef(c)
		svg, png := origin+"/v1/logos/"+ref, origin+"/v1/logos/"+ref+"?format=png&size=256"
		var html strings.Builder
		html.WriteString(`<div class="logo-hero"><img src="/v1/logos/` + ref + `" alt="` + template.HTMLEscapeString(c.Company) + ` logo" width="192" height="192"><div>`)
		html.WriteString(`<dl class="facts-list"><dt>Company</dt><dd>` + template.HTMLEscapeString(c.Company) + `</dd><dt>Ticker</dt><dd>` + c.Exchange + `: ` + template.HTMLEscapeString(c.Ticker) + `</dd>`)
		if c.ISIN != "" {
			html.WriteString(`<dt>ISIN</dt><dd>` + c.ISIN + `</dd>`)
		}
		if c.Sector != "" {
			html.WriteString(`<dt>Sector</dt><dd>` + template.HTMLEscapeString(c.Sector) + `</dd>`)
		}
		if c.Industry != "" {
			html.WriteString(`<dt>Industry</dt><dd>` + template.HTMLEscapeString(c.Industry) + `</dd>`)
		}
		if len(c.Indexes) > 0 {
			html.WriteString(`<dt>In indices</dt><dd>` + template.HTMLEscapeString(strings.Join(c.Indexes[:min(len(c.Indexes), 8)], ", ")) + `</dd>`)
		}
		if !c.Listed {
			html.WriteString(`<dt>Status</dt><dd>Delisted</dd>`)
		}
		html.WriteString(`</dl><div class="actions"><a class="key key--primary" href="` + svg + `" download="` + c.Ticker + `.svg">Download SVG</a><a class="key" href="` + png + `" download="` + c.Ticker + `.png">Download PNG</a></div></div></div>`)
		if !c.HasLogo {
			html.WriteString(`<p class="aside">No published logo is known for this company yet, so the address serves an initials tile. It is replaced by the real logo once one is added.</p>`)
		}
		if c.OneLine != "" {
			html.WriteString(`<div class="prose" style="margin-top:var(--space-xl)"><p>` + template.HTMLEscapeString(c.OneLine) + `</p></div>`)
		}
		html.WriteString(`<div class="prose" style="margin-top:var(--space-xl)"><h2>Use it in a page or app</h2><div class="codebox" data-label="HTML"><pre><code>` +
			template.HTMLEscapeString(`<img src="`+svg+`" alt="`+c.Company+` logo" width="64" height="64">`) + `</code></pre></div>` +
			`<div class="codebox" data-label="Company details as JSON"><pre><code>curl ` + origin + `/v1/companies/` + ref + `</code></pre></div>` +
			`<p class="note">The PNG takes <code>size</code> from 16 to 1024. Free, no key, cached for a day. ` +
			`See <a href="` + traffic.Tag("/docs/logos", "logos") + `">the logo API</a>. The logo and trademark belong to ` + template.HTMLEscapeString(c.Company) + `; it is shown here to identify the company.</p></div>`)
		title := c.Company + " logo (" + c.Ticker + "): SVG and PNG | " + productName
		p := page{Origin: origin, Path: "/logos/" + logoPageSymbol(c), Markdown: "/logos/" + logoPageSymbol(c) + ".md", Title: title, Heading: c.Company + " logo",
			Description: "Download the " + c.Company + " (" + c.Exchange + ": " + c.Ticker + ") logo as SVG or PNG, or link it with one image address. Free, no key.",
			Body:        appPage{Crumbs: [][2]string{{"Logos", "/logos"}}, Heading: c.Company + " logo", HTML: template.HTML(html.String())}}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Logos", "/logos"}, [2]string{c.Company, "/logos/" + logoPageSymbol(c)}),
			map[string]any{"@type": "ImageObject", "contentUrl": svg, "name": c.Company + " logo", "encodingFormat": "image/svg+xml",
				"about": map[string]any{"@type": "Corporation", "name": c.Company, "tickerSymbol": c.Exchange + ":" + c.Ticker}})
		return s.renderHTML("app", p)
	}, func(origin string) []byte {
		ref := symbolRef(c)
		var out strings.Builder
		out.WriteString("# " + c.Company + " logo\n\n")
		out.WriteString("- Ticker: " + c.Exchange + ": " + c.Ticker + "\n")
		if c.ISIN != "" {
			out.WriteString("- ISIN: " + c.ISIN + "\n")
		}
		if c.Sector != "" {
			out.WriteString("- Sector: " + c.Sector + "\n")
		}
		out.WriteString("- SVG: " + origin + "/v1/logos/" + ref + "\n- PNG: " + origin + "/v1/logos/" + ref + "?format=png&size=256\n- Details as JSON: " + origin + "/v1/companies/" + ref + "\n\n")
		out.WriteString("The logo and trademark belong to " + c.Company + ". Free, no key. Documentation: " + origin + "/docs/logos.md\n")
		return []byte(out.String())
	})
}

// logoPages lists the logo pages for the sitemap: listed companies, one each.
func (s *Server) logoPages() []string {
	var paths []string
	seen := map[string]bool{}
	for _, c := range s.companies.Load().all {
		if !c.Listed {
			continue
		}
		path := "/logos/" + logoPageSymbol(c)
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

// ---- demos ----

type demo struct{ Slug, Title, Heading, Lead, HTML string }

var demos = []demo{
	{"market-clock", "Is the Indian stock market open? Market clock | " + productName, "Market clock",
		"NSE open or closed right now, the time to the next bell, and the next market holiday.",
		`<div class="card"><p class="big" id="clock-state">…</p><p class="lead" id="clock-next"></p><p class="note" id="clock-holiday"></p></div><p class="aside">Runs on <a href="/docs/working-days">/v1/market/status</a> and <a href="/docs/holidays">/v1/holidays</a>.</p>`},
	{"script-switcher", "Type once, see nine Indian scripts | " + productName, "Script switcher",
		"Type in any Indian script, or in Roman letters, and see it in the others side by side.",
		`<div class="card form"><label>Text <textarea id="scripts-text" rows="2">नमस्ते भारत</textarea></label></div><div class="scripts" id="scripts-out"></div><p class="aside">Runs on <a href="/docs/transliterate">/v1/transliterate</a>.</p>`},
	{"logo-wall", "Nifty 50 logo wall | " + productName, "Logo wall",
		"The companies of an index as a wall of logos, grouped by sector. Pick one to download its logo.",
		`<div class="card form"><label>Index <select id="wall-index"><option>Nifty 50</option><option>Nifty Next 50</option><option>Nifty Bank</option><option>Nifty IT</option><option>Nifty Pharma</option><option>Nifty Auto</option><option>Nifty FMCG</option></select></label></div><div id="wall"></div><p class="aside">Runs on <a href="/docs/logos">/v1/companies and /v1/logos</a>.</p>`},
	{"under-the-hood", "What happens to one API request | " + productName, "Under the hood",
		"One request followed through the service: what it cost, which copy of the data answered, how long it took, and the cache headers.",
		`<div class="card form"><label>Address <input id="hood-url" value="/v1/pincode/637001"></label><div class="row"><button class="key key--primary" type="button" id="hood-go">Follow it</button></div></div><ol class="chain" id="hood-steps"></ol><p class="aside">Every answer carries these headers; <a href="/healthz">/healthz</a> lists the data copies in use.</p>`},
}

func (s *Server) handleDemos(w http.ResponseWriter, r *http.Request) {
	s.sendPage(w, r, "demos", func(origin string) []byte {
		var html strings.Builder
		html.WriteString(`<ul class="tiles">`)
		for _, d := range demos {
			html.WriteString(`<li><a href="/demos/` + d.Slug + `"><b>` + template.HTMLEscapeString(d.Heading) + `</b><span>` + template.HTMLEscapeString(d.Lead) + `</span></a></li>`)
		}
		html.WriteString(`</ul>`)
		p := page{Origin: origin, Path: "/demos", Markdown: "/demos.md", Title: "Demos: what the India API can do | " + productName, Heading: "Demos",
			Description: "Small live demos built on India API: a market clock, a script switcher for nine Indian scripts, an index logo wall and a request followed under the hood.",
			Body:        appPage{Heading: "Demos", Lead: "Small things made with the API, to look at and share.", HTML: template.HTML(html.String())}}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Demos", "/demos"}))
		return s.renderHTML("app", p)
	}, func(origin string) []byte {
		var out strings.Builder
		out.WriteString("# Demos\n\nSmall things made with the API, to look at and share.\n\n")
		for _, d := range demos {
			out.WriteString("- [" + d.Heading + "](" + origin + "/demos/" + d.Slug + "): " + d.Lead + "\n")
		}
		return []byte(out.String())
	})
}

func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSuffix(r.PathValue("slug"), ".md")
	for _, d := range demos {
		if d.Slug != slug {
			continue
		}
		s.sendPage(w, r, "demo-"+slug, func(origin string) []byte {
			p := page{Origin: origin, Path: "/demos/" + d.Slug, Markdown: "/demos/" + d.Slug + ".md", Title: d.Title, Heading: d.Heading, Description: d.Lead,
				Body: appPage{Crumbs: [][2]string{{"Demos", "/demos"}}, Heading: d.Heading, Lead: d.Lead,
					HTML: template.HTML(`<div data-demo="` + d.Slug + `">` + d.HTML + `<noscript><p class="aside">This demo needs JavaScript.</p></noscript></div>`), Scripts: []string{"demos.js"}}}
			p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Demos", "/demos"}, [2]string{d.Heading, "/demos/" + d.Slug}))
			return s.renderHTML("app", p)
		}, func(origin string) []byte {
			return []byte("# " + d.Heading + "\n\n" + d.Lead + " A live page at " + origin + "/demos/" + d.Slug + "; it needs a browser with JavaScript.\n")
		})
		return
	}
	s.notFound(w, r, "No such demo. The list is at /demos.")
}
