package api

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	textTemplate "text/template"
)

// The website: a short landing page, a directory of the APIs, one page per API, guides,
// a page for AI tools and three trust pages. Every page also exists as Markdown, at its
// .md address or with Accept: text/markdown, and everything is generated from the
// catalog, so a page cannot describe an endpoint differently from the OpenAPI document.

const (
	productName = "India API"
	siteName    = "api.dharun.dev"
	siteTagline = "Free, open source APIs for Indian data"
	siteSummary = "Free JSON APIs for Indian data: pincodes, IFSC and banks, GSTIN and PAN checks, holidays and working days, RTO codes, company logos, UPI QR codes, number to words in lakh and crore, GST invoices as PDF, OCR in Indian languages, PDF and image tools, translation and speech to text in Indian languages, embeddings, moderation and more. No API key, no sign-up, open CORS, an MCP server for AI assistants. Open source under AGPL-3.0."

	publicOrigin = "https://api.dharun.dev" // where the skill and the registry entry point
	repoURL      = "https://github.com/dharunashokkumar/api.dharun.dev"
	repoSlug     = "dharunashokkumar/api.dharun.dev"
	authorName   = "Dharun Ashokkumar"
	authorURL    = "https://dharun.dev"
	mcpName      = "india-api"
	registryName = "io.github.dharunashokkumar/india-api"
	marketName   = "dharun-dev"
	indexNowKey  = "8156672c00aa8ecc9b5f73bac62a783d"

	// What the skill states, as it is written once for every server: the defaults of
	// RATE_LIMIT_PER_MIN and MAX_BODY_KB.
	defaultRateLimit = 600
	defaultBodyBytes = 256 << 10
)

// siteCategories orders the API groups on the landing page and in the docs sidebar.
// Every catalog group names one of these IDs.
var siteCategories = []struct{ ID, Title string }{
	{"money", "Money and business"},
	{"places", "Places and transport"},
	{"dates", "Dates and calendars"},
	{"language", "Language and text"},
	{"network", "Network and contact checks"},
	{"files", "Files, documents and media"},
	{"developer", "Developer tools"},
	{"sky", "Sky and radio"},
}

type siteCategory struct {
	ID, Title string
	Groups    []apiGroup
}

func categorised() []siteCategory {
	out := make([]siteCategory, 0, len(siteCategories))
	for _, c := range siteCategories {
		category := siteCategory{ID: c.ID, Title: c.Title}
		for _, group := range catalog {
			if group.Category == c.ID {
				category.Groups = append(category.Groups, group)
			}
		}
		out = append(out, category)
	}
	return out
}

func groupByID(id string) (apiGroup, bool) {
	for _, group := range catalog {
		if group.ID == id {
			return group, true
		}
	}
	return apiGroup{}, false
}

func categoryTitle(id string) string {
	for _, c := range siteCategories {
		if c.ID == id {
			return c.Title
		}
	}
	return ""
}

func (s *Server) registerSite() {
	s.handle("docs", costFree, "GET /{$}", s.handleHome)
	s.handle("docs", costFree, "GET /index.md", s.handleHome)
	s.handle("docs", costFree, "GET /docs", s.handleDocsIndex)
	s.handle("docs", costFree, "GET /docs.md", s.handleDocsIndex)
	s.handle("docs", costFree, "GET /docs/{group}", s.handleGroupPage)
	s.handle("docs", costFree, "GET /guides", s.handleGuides)
	s.handle("docs", costFree, "GET /guides.md", s.handleGuides)
	s.handle("docs", costFree, "GET /guides/{slug}", s.handleGuide)
	s.handle("docs", costFree, "GET /ai", s.handleAIPage)
	s.handle("docs", costFree, "GET /ai.md", s.handleAIPage)
	for _, name := range []string{"privacy", "security", "about"} {
		s.handle("docs", costFree, "GET /"+name, s.handleStaticPage)
		s.handle("docs", costFree, "GET /"+name+".md", s.handleStaticPage)
	}
	s.handle("docs", costFree, "GET /llms.txt", s.handleLLMs)
	s.handle("docs", costFree, "GET /llms-full.txt", s.handleLLMsFull)
	s.handle("docs", costFree, "GET /robots.txt", s.handleRobots)
	s.handle("docs", costFree, "GET /sitemap.xml", s.handleSitemap)
	s.handle("docs", costFree, "GET /favicon.svg", s.handleFavicon)
	s.handle("docs", costFree, "GET /favicon.ico", s.handleFavicon)
	s.handle("docs", costFree, "GET /"+indexNowKey+".txt", s.handleIndexNowKey)
	s.handle("docs", costFree, "GET /.well-known/security.txt", s.handleSecurityTxt)
	s.handle("docs", costFree, "GET /.well-known/api-catalog", s.handleAPICatalog)
	s.handle("docs", costFree, "GET /.well-known/ai-catalog.json", s.handleAICatalog)
	s.handle("docs", costFree, "GET /.well-known/mcp/server-card.json", s.handleServerCard)
	s.handle("docs", costFree, "GET /mcp/server-card", s.handleServerCard)
	s.handle("docs", costFree, "GET /.well-known/agent-skills/index.json", s.handleSkillIndex)
	s.handle("docs", costFree, "GET /.well-known/agent-skills/"+mcpName+"/{file...}", s.handleSkillFile)
	s.handle("docs", costFree, "GET /skills/"+mcpName+".zip", s.handleSkillZip)
}

// ---- pages ----

// page is what the layout needs, plus the page's own data in Body.
type page struct {
	Origin      string
	Path        string // "/docs/pincode"
	Title       string // the <title>
	Heading     string // for the share image
	Description string
	Nav         string // which header link is current: docs, guides, ai
	Markdown    string // path of the Markdown copy, if any
	JSONLD      template.JS
	Body        any
}

func (p page) Canonical() string { return p.Origin + p.Path }

// OGImage is a share card drawn by our own /v1/og-image.
func (p page) OGImage() string {
	values := url.Values{}
	values.Set("title", p.Heading)
	values.Set("subtitle", shorten(p.Description, 140))
	values.Set("site", siteName)
	values.Set("tag", productName)
	values.Set("theme", "light")
	values.Set("bg", "ffffff")
	values.Set("fg", "1d1d1f")
	values.Set("accent", "0071e3")
	return p.Origin + "/v1/og-image?" + values.Encode()
}

func (s *Server) renderHTML(name string, p page) []byte {
	var buffer bytes.Buffer
	if err := siteTemplates.ExecuteTemplate(&buffer, name, p); err != nil {
		return []byte("template error: " + err.Error())
	}
	return buffer.Bytes()
}

func renderMarkdown(name string, data any) []byte {
	var buffer bytes.Buffer
	if err := markdownTemplates.ExecuteTemplate(&buffer, name, data); err != nil {
		return []byte("template error: " + err.Error())
	}
	return buffer.Bytes()
}

// wantsMarkdown is true for a .md address, or when the client prefers Markdown to HTML:
// a higher quality value, or the same one listed first. q=0 rules a type out.
func wantsMarkdown(r *http.Request) bool {
	if strings.HasSuffix(r.URL.Path, ".md") {
		return true
	}
	markdown, html := -1.0, -1.0
	for _, part := range strings.Split(r.Header.Get("Accept"), ",") {
		fields := strings.Split(part, ";")
		media := strings.ToLower(strings.TrimSpace(fields[0]))
		quality := 1.0
		for _, field := range fields[1:] {
			name, value, _ := strings.Cut(strings.TrimSpace(field), "=")
			if strings.EqualFold(strings.TrimSpace(name), "q") {
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && parsed >= 0 && parsed <= 1 {
					quality = parsed
				}
			}
		}
		switch media {
		case "text/markdown", "text/x-markdown":
			markdown = max(markdown, quality)
		case "text/html", "application/xhtml+xml":
			// The first listed wins a tie, so HTML only counts once it outranks Markdown.
			if quality > markdown {
				html = max(html, quality)
			}
		}
	}
	return markdown > 0 && markdown > html
}

// markdownPage gives the HTML address a Markdown address stands for: /docs/ifsc.md is
// /docs/ifsc, /index.md is /.
func markdownPage(path string) string {
	if path == "/index.md" {
		return "/"
	}
	return strings.TrimSuffix(path, ".md")
}

// addVary adds a token to the Vary header unless it is there already, keeping the
// others: a page varies on Accept and, when compressed, on Accept-Encoding too.
func addVary(header http.Header, token string) {
	for _, value := range header.Values("Vary") {
		for _, existing := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), token) {
				return
			}
		}
	}
	header.Add("Vary", token)
}

// sendPage answers with the HTML or the Markdown form of a page. Both are rendered once
// per origin and kept until a dataset changes. The Markdown form names the HTML page as
// canonical, so search engines index one of the two.
func (s *Server) sendPage(w http.ResponseWriter, r *http.Request, key string, html func(origin string) []byte, markdown func(origin string) []byte) {
	addVary(w.Header(), "Accept")
	if markdown != nil && wantsMarkdown(r) {
		body := s.cached(r, key+".md", markdown)
		w.Header().Add("Link", "<"+s.origin(r)+markdownPage(r.URL.Path)+`>; rel="canonical"`)
		writeBody(w, r, http.StatusOK, body, "text/markdown; charset=utf-8", cacheShort, true)
		return
	}
	body := s.cached(r, key, html)
	writeBody(w, r, http.StatusOK, body, "text/html; charset=utf-8", cacheShort, true)
}

// jsonLD encodes structured data for a <script type="application/ld+json"> block.
// json.Marshal escapes <, > and &, so the result cannot close the script element.
func jsonLD(graph ...map[string]any) template.JS {
	document := map[string]any{"@context": "https://schema.org", "@graph": graph}
	encoded, _ := json.Marshal(document)
	return template.JS(encoded)
}

func breadcrumbs(origin string, trail ...[2]string) map[string]any {
	items := []any{map[string]any{"@type": "ListItem", "position": 1, "name": productName, "item": origin + "/"}}
	for i, step := range trail {
		items = append(items, map[string]any{"@type": "ListItem", "position": i + 2, "name": step[0], "item": origin + step[1]})
	}
	return map[string]any{"@type": "BreadcrumbList", "itemListElement": items}
}

func author() map[string]any {
	return map[string]any{"@type": "Person", "@id": authorURL + "/#person", "name": authorName, "url": authorURL,
		"sameAs": []string{"https://github.com/dharunashokkumar"}}
}

// ---- landing page ----

type faq struct{ Question, Answer string }

func (s *Server) faqs() []faq {
	return []faq{
		{"Is it free?", fmt.Sprintf("Yes. There is no API key, no sign-up and no paid plan. Each client can make %d requests a minute; a few endpoints that do heavy work count as more than one.", s.cfg.RateLimitPerMinute)},
		{"Can I use it in a commercial app?", "Yes. Calling the API puts no licence on your app. The data keeps the licence of its source, named in every response under meta.source; a few sources ask for a credit line. The code examples on this site are MIT-0, so you can paste them anywhere."},
		{"Where does the data come from?", "From official publications such as the Reserve Bank of India, India Post, NSE and the GST council, and from open datasets such as OurAirports and Wikipedia. Every response says which source it used, its version and when it was retrieved."},
		{"What do you log?", "One line per request with the path, the status, the time taken and roughly where it came from (country, state, city and internet provider, looked up from the address). IP addresses are never written to the log, and endpoints that take personal values (phone numbers, email addresses, text you send) leave their input out of the log entirely."},
		{"Is my text sent to an AI company?", "No. Translation, speech to text, moderation, name detection, embeddings and the other model-based APIs run open models on this server itself. What you send is not passed to any other service, not kept and not used to train anything."},
		{"Can I run it myself?", "Yes. The whole service is open source under AGPL-3.0: one Go program and its datasets. The README explains how to build and run it."},
		{"Does it work with AI assistants?", "Yes. There is an MCP server at /mcp with no authentication, an Agent Skill, a Claude Code plugin, an OpenAPI description and llms.txt."},
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	// Where agents find the API's description, documentation and catalog (RFC 8288).
	w.Header().Set("Link", `</.well-known/api-catalog>; rel="api-catalog", </openapi.json>; rel="service-desc"; type="application/openapi+json", </docs>; rel="service-doc", </llms.txt>; rel="describedby"; type="text/plain"`)
	s.sendPage(w, r, "home", func(origin string) []byte {
		pincode, _ := groupByID("pincode")
		faqs := s.faqs()
		var faqItems []any
		for _, f := range faqs {
			faqItems = append(faqItems, map[string]any{"@type": "Question", "name": f.Question,
				"acceptedAnswer": map[string]any{"@type": "Answer", "text": f.Answer}})
		}
		p := page{
			Origin: origin, Path: "/",
			Title:       productName + ": free pincode, IFSC, GSTIN and holiday APIs",
			Heading:     "Free APIs for India",
			Description: fmt.Sprintf("Free, open source JSON APIs for Indian data: pincode, IFSC, GSTIN and PAN checks, holidays, RTO codes, UPI QR and %d more. No key, no sign-up. MCP server for AI.", max(len(catalog)-6, 1)),
			Markdown:    "/index.md",
			Body: map[string]any{
				"Categories": categorised(),
				"Count":      len(catalog),
				"Example":    origin + pincode.Endpoints[0].Example,
				"Sample":     s.sample(strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://"), pincode.Endpoints[0]),
				"Guides":     guides,
				"FAQs":       faqs,
				"Limit":      s.cfg.RateLimitPerMinute,
			},
		}
		p.JSONLD = jsonLD(
			map[string]any{"@type": "WebSite", "@id": origin + "/#site", "name": productName, "alternateName": siteName, "url": origin + "/",
				"description": siteSummary, "inLanguage": "en-IN", "publisher": map[string]any{"@id": authorURL + "/#person"}},
			author(),
			map[string]any{"@type": "WebAPI", "@id": origin + "/#api", "name": productName, "description": siteSummary,
				"url": origin + "/", "documentation": origin + "/docs", "provider": map[string]any{"@id": authorURL + "/#person"},
				"isAccessibleForFree": true, "termsOfService": origin + "/about", "license": "https://www.gnu.org/licenses/agpl-3.0.html",
				"availableChannel": map[string]any{"@type": "ServiceChannel", "serviceUrl": origin + "/v1"}},
			map[string]any{"@type": "SoftwareSourceCode", "name": productName, "codeRepository": repoURL, "programmingLanguage": "Go",
				"license": "https://www.gnu.org/licenses/agpl-3.0.html", "author": map[string]any{"@id": authorURL + "/#person"}},
			map[string]any{"@type": "FAQPage", "mainEntity": faqItems},
		)
		return s.renderHTML("home", p)
	}, func(origin string) []byte { return s.llmsIndex(origin) })
}

// ---- API directory and API pages ----

func (s *Server) handleDocsIndex(w http.ResponseWriter, r *http.Request) {
	s.sendPage(w, r, "docs", func(origin string) []byte {
		p := page{
			Origin: origin, Path: "/docs", Nav: "docs", Markdown: "/docs.md",
			Title:       "API reference: every endpoint | " + productName,
			Heading:     "API reference",
			Description: fmt.Sprintf("All %d APIs of %s with their endpoints: pincodes, IFSC, GSTIN, holidays, RTO codes, logos, UPI QR and more. Free, no key.", len(catalog), productName),
			Body:        map[string]any{"Categories": categorised(), "Limit": s.cfg.RateLimitPerMinute},
		}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Docs", "/docs"}))
		return s.renderHTML("docs", p)
	}, s.docsIndexMarkdown)
}

// docsIndexMarkdown is the Markdown form of /docs: every API with each of its endpoints.
func (s *Server) docsIndexMarkdown(origin string) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "# API reference\n\nAll %d APIs of %s, with every endpoint. Each name links to its full page: parameters, a real response and code in curl, JavaScript and Python. Base URL: %s. No API key; %d requests a minute per client.\n\n", len(catalog), productName, origin, s.cfg.RateLimitPerMinute)
	for _, category := range categorised() {
		fmt.Fprintf(&out, "## %s\n\n", category.Title)
		for _, group := range category.Groups {
			fmt.Fprintf(&out, "### [%s](%s/docs/%s.md)\n\n%s\n\n", group.Title, origin, group.ID, group.Tagline)
			for _, e := range group.Endpoints {
				returns := ""
				if kind := e.Kind(); kind != "" {
					returns = " (returns " + kind + ")"
				}
				fmt.Fprintf(&out, "- `%s %s`: %s%s\n", e.Methods(), e.Path, e.Summary, returns)
			}
			out.WriteString("\n")
		}
	}
	return []byte(out.String())
}

type docEndpoint struct {
	endpoint
	Anchor     string
	URL        string
	Samples    []codeSample
	Sample     *sample
	PathParams []param
	Query      []param
}

type codeSample struct{ Lang, Label, Code string }

type docGroup struct {
	apiGroup
	CategoryTitle string
	Endpoints     []docEndpoint
	Sources       []*sourceRef
	Guides        []guide
	Categories    []siteCategory
	Limit         int
	BodyLimit     string
	Origin        string
}

// staticDocGroup is the part of an API's page that depends only on the catalog: what
// the skill's reference files are made from.
func staticDocGroup(origin string, group apiGroup) docGroup {
	view := docGroup{apiGroup: group, CategoryTitle: categoryTitle(group.Category), Categories: categorised(),
		Limit: defaultRateLimit, BodyLimit: byteSize(defaultBodyBytes), Origin: origin}
	for _, e := range group.Endpoints {
		d := docEndpoint{endpoint: e, Anchor: anchor(e.Summary), URL: origin + e.Example}
		for _, p := range e.Params {
			if p.In == "path" {
				d.PathParams = append(d.PathParams, p)
			} else {
				d.Query = append(d.Query, p)
			}
		}
		d.Samples = codeSamples(e, d.URL)
		view.Endpoints = append(view.Endpoints, d)
	}
	for _, g := range guides {
		if slices.Contains(g.APIs, group.ID) {
			view.Guides = append(view.Guides, g)
		}
	}
	return view
}

// docGroup adds what this server knows: its limits, sample responses and the sources of
// the data in use.
func (s *Server) docGroup(origin string, group apiGroup, samples bool) docGroup {
	view := staticDocGroup(origin, group)
	view.Limit, view.BodyLimit = s.cfg.RateLimitPerMinute, byteSize(s.cfg.MaxBodyBytes)
	if samples {
		host := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
		for i := range view.Endpoints {
			view.Endpoints[i].Sample = s.sample(host, view.Endpoints[i].endpoint)
		}
	}
	for _, key := range append([]string{group.SourceKey}, group.MoreSources...) {
		if source := s.source(key); source != nil {
			view.Sources = append(view.Sources, source)
		}
	}
	return view
}

// shorten cuts text to at most limit characters at a word boundary.
func shorten(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := string(runes[:limit-1])
	if space := strings.LastIndex(cut, " "); space > limit/2 {
		cut = cut[:space]
	}
	return strings.TrimRight(cut, " ,;:.") + "…"
}

func anchor(text string) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(text) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
			dash = false
		} else if !dash && out.Len() > 0 {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(out.String(), "-")
}

// codeSamples are the ready-to-paste calls shown for an endpoint.
func codeSamples(e endpoint, address string) []codeSample {
	if e.Upload != nil || e.Output != "" {
		return fileSamples(e, address)
	}
	if e.Image {
		extension := "svg"
		if strings.Contains(address, "format=png") || (strings.Contains(e.Path, "image") && !strings.Contains(address, "format=svg")) {
			extension = "png"
		}
		return []codeSample{
			{"curl", "curl", fmt.Sprintf("curl -o picture.%s \"%s\"", extension, address)},
			{"html", "HTML", fmt.Sprintf("<img src=\"%s\" alt=\"%s\">", template.HTMLEscapeString(address), e.Summary)},
		}
	}
	return []codeSample{
		{"curl", "curl", fmt.Sprintf("curl \"%s\"", address)},
		{"js", "JavaScript", fmt.Sprintf("const response = await fetch(\"%s\");\nconst body = await response.json();\nif (!response.ok) throw new Error(body.error.message);\nconsole.log(body.data);", address)},
		{"python", "Python", fmt.Sprintf("import requests\n\nresponse = requests.get(\"%s\", timeout=10)\nbody = response.json()\nif not response.ok:\n    raise RuntimeError(body[\"error\"][\"message\"])\nprint(body[\"data\"])", address)},
	}
}

func (s *Server) handleGroupPage(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("group"), ".md")
	group, ok := groupByID(id)
	if !ok {
		s.notFound(w, r, "No such documentation page.")
		return
	}
	s.sendPage(w, r, "docs:"+id, func(origin string) []byte {
		view := s.docGroup(origin, group, true)
		p := page{
			Origin: origin, Path: "/docs/" + id, Nav: "docs",
			Title:       group.Title + " API for India, free and open | " + productName,
			Heading:     group.Title + " API",
			Description: shorten(group.Tagline+" "+group.Intro, 158),
			Markdown:    "/docs/" + id + ".md",
			Body:        view,
		}
		p.JSONLD = jsonLD(
			breadcrumbs(origin, [2]string{"Docs", "/docs"}, [2]string{group.Title, "/docs/" + id}),
			map[string]any{"@type": "TechArticle", "headline": group.Title + " API", "description": group.Tagline,
				"url": origin + "/docs/" + id, "inLanguage": "en-IN", "author": map[string]any{"@id": authorURL + "/#person"},
				"isPartOf": map[string]any{"@id": origin + "/#site"}, "about": map[string]any{"@id": origin + "/#api"}},
		)
		return s.renderHTML("group", p)
	}, func(origin string) []byte {
		return renderMarkdown("group", s.docGroup(origin, group, true))
	})
}

// ---- the AI page ----

type aiClient struct {
	Name, Note, Code, Button, ButtonURL string
}

func aiClients(origin string) []aiClient {
	endpoint := origin + "/mcp"
	vscode, _ := json.Marshal(map[string]string{"type": "http", "url": endpoint})
	cursor, _ := json.Marshal(map[string]string{"url": endpoint})
	return []aiClient{
		{Name: "Claude Code", Note: "Add the server, or install the plugin, which brings the skill as well.",
			Code: "claude mcp add --transport http " + mcpName + " " + endpoint + "\n\n# or the plugin (skill and MCP server together)\n/plugin marketplace add " + repoSlug + "\n/plugin install " + mcpName + "@" + marketName},
		{Name: "Claude (claude.ai and Claude Desktop)", Note: "Settings, Connectors, Add custom connector. Name it " + productName + " and paste the address. No authentication is needed.",
			Code: endpoint},
		{Name: "ChatGPT", Note: "Where your plan allows custom connectors: turn on developer mode in Settings, then create a connector with this address and no authentication.",
			Code: endpoint},
		{Name: "VS Code (GitHub Copilot)", Note: "Use the button, or add this to .vscode/mcp.json.",
			Code:   "{\n  \"servers\": {\n    \"" + mcpName + "\": { \"type\": \"http\", \"url\": \"" + endpoint + "\" }\n  }\n}",
			Button: "Install in VS Code", ButtonURL: "https://vscode.dev/redirect/mcp/install?name=" + mcpName + "&config=" + url.QueryEscape(string(vscode))},
		{Name: "Cursor", Note: "Use the button, or add this to ~/.cursor/mcp.json.",
			Code:   "{\n  \"mcpServers\": {\n    \"" + mcpName + "\": { \"url\": \"" + endpoint + "\" }\n  }\n}",
			Button: "Install in Cursor", ButtonURL: "https://cursor.com/en/install-mcp?name=" + mcpName + "&config=" + base64Std(cursor)},
		{Name: "Windsurf", Note: "Add this to ~/.codeium/windsurf/mcp_config.json.",
			Code: "{\n  \"mcpServers\": {\n    \"" + mcpName + "\": { \"serverUrl\": \"" + endpoint + "\" }\n  }\n}"},
		{Name: "Gemini CLI", Note: "Add this to ~/.gemini/settings.json.",
			Code: "{\n  \"mcpServers\": {\n    \"" + mcpName + "\": { \"httpUrl\": \"" + endpoint + "\" }\n  }\n}"},
		{Name: "OpenAI Codex", Note: "Add this to ~/.codex/config.toml.",
			Code: "[mcp_servers." + mcpName + "]\nurl = \"" + endpoint + "\""},
	}
}

func (s *Server) aiPageData(origin string) map[string]any {
	return map[string]any{
		"Endpoint": origin + "/mcp",
		"Clients":  aiClients(origin),
		"Tools":    mcpTools,
		"Origin":   origin,
		"Repo":     repoSlug,
		"Name":     mcpName,
		"Market":   marketName,
	}
}

func (s *Server) handleAIPage(w http.ResponseWriter, r *http.Request) {
	s.sendPage(w, r, "ai", func(origin string) []byte {
		p := page{
			Origin: origin, Path: "/ai", Nav: "ai", Markdown: "/ai.md",
			Title:       "Use " + productName + " with Claude, ChatGPT, Cursor and VS Code (MCP)",
			Heading:     "Use it from your AI assistant",
			Description: "Connect the free India API MCP server to Claude, ChatGPT, Cursor, VS Code, Gemini CLI or Codex, or install the Agent Skill. No key, no sign-up.",
			Body:        s.aiPageData(origin),
		}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"AI tools", "/ai"}),
			map[string]any{"@type": "TechArticle", "headline": p.Heading, "description": p.Description, "url": origin + "/ai",
				"author": map[string]any{"@id": authorURL + "/#person"}})
		return s.renderHTML("ai", p)
	}, func(origin string) []byte { return renderMarkdown("ai", s.aiPageData(origin)) })
}

// ---- privacy, security, about ----

func (s *Server) staticPageData(origin string) map[string]any {
	seen := map[string]bool{}
	var sources []*sourceRef
	for _, group := range catalog {
		for _, key := range append([]string{group.SourceKey}, group.MoreSources...) {
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			if source := s.source(key); source != nil {
				sources = append(sources, source)
			}
		}
	}
	var private []string
	for _, group := range catalog {
		for _, e := range group.Endpoints {
			if e.Private {
				private = append(private, e.Path)
			}
		}
	}
	return map[string]any{"Origin": origin, "Repo": repoURL, "RepoSlug": repoSlug, "Sources": sources, "Private": private,
		"Limit": s.cfg.RateLimitPerMinute, "BodyLimit": byteSize(s.cfg.MaxBodyBytes), "Author": authorName, "AuthorURL": authorURL}
}

var staticPages = map[string]struct{ Title, Heading, Description string }{
	"privacy":  {"Privacy: what " + productName + " logs and keeps", "Privacy", "What India API logs, what it never logs, and what happens to the values you send. No cookies, no tracking, no IP addresses in logs."},
	"security": {"Security: reporting issues and verifying releases | " + productName, "Security", "How to report a vulnerability in India API, how the service limits what it does, and how to verify that a release came from the public source."},
	"about":    {"About " + productName + ": who builds it, licences and data sources", "About", "Who builds India API, why, the licences of the code and the examples, and every data source with its licence."},
}

// handleStaticPage serves privacy, security and about. Each is written once, as a
// Markdown template; the HTML page is that Markdown rendered.
func (s *Server) handleStaticPage(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ".md")
	meta := staticPages[name]
	markdown := func(origin string) []byte { return renderMarkdown(name, s.staticPageData(origin)) }
	s.sendPage(w, r, name, func(origin string) []byte {
		var html bytes.Buffer
		if err := guideRenderer.Convert(markdown(origin), &html); err != nil {
			return []byte("markdown error: " + err.Error())
		}
		p := page{Origin: origin, Path: "/" + name, Title: meta.Title, Heading: meta.Heading, Description: meta.Description,
			Markdown: "/" + name + ".md", Body: template.HTML(html.String())}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{meta.Heading, "/" + name}))
		return s.renderHTML("prose", p)
	}, markdown)
}

// ---- shared template helpers ----

var templateFuncs = map[string]any{
	"join":     strings.Join,
	"category": categoryTitle,
	"lower":    strings.ToLower,
	"add":      func(a, b int) int { return a + b },
	"fence": func(code string) string { // a Markdown code fence that the code cannot close
		fence := "```"
		for strings.Contains(code, fence) {
			fence += "`"
		}
		return fence
	},
}

// The Markdown forms live in templates/, since they are full of backticks.
//
//go:embed templates/*.md
var markdownFiles embed.FS

var (
	siteTemplates     = template.Must(template.New("site").Funcs(templateFuncs).Parse(htmlLayout + htmlPages))
	markdownTemplates = textTemplate.Must(textTemplate.New("md").Funcs(templateFuncs).ParseFS(markdownFiles, "templates/*.md"))
)
