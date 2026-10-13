package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

const (
	siteName    = "api.dharun.dev"
	siteTagline = "Free, open APIs for Indian developers"
	siteSummary = "Free JSON APIs for Indian data: company logos, pincodes, IFSC codes, RTO codes, holidays, number to words and format checks. No API key, no sign-up, CORS enabled."
)

var aiCrawlers = []string{
	"GPTBot", "OAI-SearchBot", "ChatGPT-User", "ClaudeBot", "Claude-User", "Claude-SearchBot",
	"PerplexityBot", "Perplexity-User", "Google-Extended", "Applebot-Extended", "CCBot",
}

func (s *Server) registerRoutes() {
	// Data endpoints: counted against the per-client rate limit.
	s.handle("logo", true, "GET /v1/logos/{symbol}", s.handleLogo)
	s.handle("company", true, "GET /v1/companies/{symbol}", s.handleCompany)
	s.handle("company_search", true, "GET /v1/companies", s.handleCompanySearch)
	s.handle("pincode", true, "GET /v1/pincode/{pincode}", s.handlePincode)
	s.handle("pincode_search", true, "GET /v1/pincode", s.handlePincodeSearch)
	s.handle("ifsc", true, "GET /v1/ifsc/{code}", s.handleIFSC)
	s.handle("ifsc_search", true, "GET /v1/ifsc", s.handleIFSCSearch)
	s.handle("bank", true, "GET /v1/banks/{code}", s.handleBank)
	s.handle("bank_search", true, "GET /v1/banks", s.handleBankSearch)
	s.handle("rto", true, "GET /v1/rto/{code}", s.handleRTO)
	s.handle("rto_search", true, "GET /v1/rto", s.handleRTOSearch)
	s.handle("holidays", true, "GET /v1/holidays/{year}", s.handleHolidays)
	s.handle("words", true, "GET /v1/words/{number}", s.handleWords)
	s.handle("validate", true, "GET /v1/validate/{kind}/{value}", s.handleValidate)
	s.handle("states", true, "GET /v1/states", s.handleStates)
	s.handle("states", true, "GET /v1/states/{code}", s.handleState)
	s.handle("mcp", true, "/mcp", s.handleMCP)

	// Documentation and plumbing.
	s.handle("docs", false, "GET /v1", s.handleIndex)
	s.handle("docs", false, "GET /{$}", s.handleHome)
	s.handle("docs", false, "GET /docs/{group}", s.handleGroupPage)
	s.handle("docs", false, "GET /openapi.json", s.handleOpenAPI)
	s.handle("docs", false, "GET /llms.txt", s.handleLLMs)
	s.handle("docs", false, "GET /robots.txt", s.handleRobots)
	s.handle("docs", false, "GET /sitemap.xml", s.handleSitemap)
	s.handle("docs", false, "GET /favicon.svg", s.handleFavicon)
	s.handle("health", false, "GET /healthz", s.handleHealth)

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.notFound(w, r, "Not found. The API index is at /v1 and the documentation at /.")
	})
}

// cached renders a document once per public origin.
func (s *Server) cached(r *http.Request, name string, build func(origin string) []byte) []byte {
	origin := s.origin(r)
	key := name + "|" + origin
	if value, ok := s.documents.Load(key); ok {
		return value.([]byte)
	}
	document := build(origin)
	// Only a configured origin is cached: a Host header chosen by the client must not
	// be able to fill memory.
	if s.cfg.PublicOrigin != "" {
		s.documents.Store(key, document)
	}
	return document
}

// ---- machine-readable index ----

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	origin := s.origin(r)
	type endpointView struct {
		Path    string `json:"path"`
		Summary string `json:"summary"`
		Example string `json:"example"`
	}
	type groupView struct {
		ID        string         `json:"id"`
		Title     string         `json:"title"`
		Summary   string         `json:"summary"`
		Docs      string         `json:"docs"`
		Source    *sourceRef     `json:"source,omitempty"`
		Endpoints []endpointView `json:"endpoints"`
	}
	groups := make([]groupView, 0, len(catalog))
	for _, group := range catalog {
		view := groupView{ID: group.ID, Title: group.Title, Summary: group.Tagline, Docs: origin + "/docs/" + group.ID, Source: s.source(group.SourceKey)}
		for _, e := range group.Endpoints {
			view.Endpoints = append(view.Endpoints, endpointView{Path: e.Path, Summary: e.Summary, Example: origin + e.Example})
		}
		groups = append(groups, view)
	}
	data := map[string]any{
		"name":    siteName,
		"summary": siteTagline,
		"version": s.cfg.Version,
		"records": map[string]int{
			"logos": s.counts.Logos, "pincodes": s.counts.Pincodes, "postOffices": s.counts.PostOffices,
			"banks": s.counts.Banks, "bankBranches": s.counts.Branches, "rtoCodes": s.counts.RTOs, "holidays": s.counts.Holidays,
		},
		"holidayYears": s.years,
		"apis":         groups,
		"links": map[string]string{
			"docs": origin + "/", "openapi": origin + "/openapi.json", "llms": origin + "/llms.txt", "mcp": origin + "/mcp",
		},
	}
	s.ok(w, r, data, meta{}, cacheAPI)
}

// ---- OpenAPI ----

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	body := s.cached(r, "openapi", func(origin string) []byte {
		paths := map[string]any{}
		for _, group := range catalog {
			for _, e := range group.Endpoints {
				parameters := []any{}
				for _, p := range e.Params {
					schema := map[string]any{"type": p.Type}
					if len(p.Enum) > 0 {
						schema["enum"] = p.Enum
					}
					parameters = append(parameters, map[string]any{
						"name": p.Name, "in": p.In, "required": p.Required, "description": p.Doc, "schema": schema,
					})
				}
				success := map[string]any{
					"description": "Success",
					"content":     map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Envelope"}}},
				}
				if e.Image {
					success["content"] = map[string]any{"image/svg+xml": map[string]any{}, "image/png": map[string]any{}}
				}
				errorResponse := map[string]any{"$ref": "#/components/responses/Error"}
				paths[e.Path] = map[string]any{"get": map[string]any{
					"operationId": operationID(e.Path),
					"tags":        []string{group.Title},
					"summary":     e.Summary,
					"description": e.Doc,
					"parameters":  parameters,
					"responses":   map[string]any{"200": success, "400": errorResponse, "404": errorResponse, "429": errorResponse},
				}}
			}
		}
		document := map[string]any{
			"openapi": "3.1.0",
			"info": map[string]any{
				"title":       siteName + ": " + siteTagline,
				"version":     s.cfg.Version,
				"description": siteSummary,
			},
			"servers": []any{map[string]any{"url": origin}},
			"paths":   paths,
			"components": map[string]any{
				"schemas": map[string]any{
					"Envelope": map[string]any{
						"type":     "object",
						"required": []string{"data", "meta"},
						"properties": map[string]any{
							"data": map[string]any{"description": "The result: an object, or an array for list endpoints."},
							"meta": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"total":  map[string]any{"type": "integer"},
									"limit":  map[string]any{"type": "integer"},
									"offset": map[string]any{"type": "integer"},
									"note":   map[string]any{"type": "string"},
									"source": map[string]any{
										"type":        "object",
										"description": "Where the data came from and the credit its licence asks for.",
										"properties": map[string]any{
											"name": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"},
											"licence": map[string]any{"type": "string"}, "version": map[string]any{"type": "string"},
											"retrieved": map[string]any{"type": "string"}, "attribution": map[string]any{"type": "string"},
										},
									},
								},
							},
						},
					},
					"Error": map[string]any{
						"type": "object",
						"properties": map[string]any{"error": map[string]any{
							"type":       "object",
							"properties": map[string]any{"code": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"}},
						}},
					},
				},
				"responses": map[string]any{"Error": map[string]any{
					"description": "Error",
					"content":     map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Error"}}},
				}},
			},
		}
		encoded, _ := json.MarshalIndent(document, "", "  ")
		return encoded
	})
	writeBody(w, r, http.StatusOK, body, typeJSON, cacheAPI, true)
}

func operationID(path string) string {
	replacer := strings.NewReplacer("/v1/", "", "{", "by_", "}", "", "/", "_")
	return replacer.Replace(path)
}

// ---- llms.txt, robots.txt, sitemap ----

func (s *Server) handleLLMs(w http.ResponseWriter, r *http.Request) {
	body := s.cached(r, "llms", func(origin string) []byte {
		var out strings.Builder
		fmt.Fprintf(&out, "# %s\n\n> %s\n\n", siteName, siteSummary)
		out.WriteString("Every endpoint is a plain GET under `/v1`. JSON responses share one shape: `{ \"data\": ..., \"meta\": { ... } }`, where `meta.source` credits the dataset. Errors are `{ \"error\": { \"code\", \"message\" } }` with a matching HTTP status. Limit: " +
			fmt.Sprint(s.cfg.RateLimitPerMinute) + " requests per minute per client.\n\n")
		for _, group := range catalog {
			fmt.Fprintf(&out, "## %s\n\n%s\n\n", group.Title, group.Intro)
			for _, e := range group.Endpoints {
				fmt.Fprintf(&out, "- [%s](%s%s): `GET %s`", e.Summary, origin, e.Example, e.Path)
				var options []string
				for _, p := range e.Params {
					if p.In == "query" {
						option := "`" + p.Name + "`"
						if len(p.Enum) > 0 {
							option += " (" + strings.Join(p.Enum, ", ") + ")"
						}
						options = append(options, option)
					}
				}
				if len(options) > 0 {
					out.WriteString(". Query: " + strings.Join(options, ", "))
				}
				out.WriteString(".\n")
			}
			out.WriteString("\n")
		}
		out.WriteString("## More\n\n")
		fmt.Fprintf(&out, "- [OpenAPI description](%s/openapi.json)\n", origin)
		fmt.Fprintf(&out, "- [MCP server](%s/mcp): Streamable HTTP, no authentication. One tool per lookup.\n", origin)
		fmt.Fprintf(&out, "- [Documentation](%s/)\n", origin)
		out.WriteString("- Company logos and trademarks belong to their respective owners.\n")
		return []byte(out.String())
	})
	writeBody(w, r, http.StatusOK, body, "text/plain; charset=utf-8", cacheAPI, true)
}

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	var out strings.Builder
	out.WriteString("User-agent: *\nAllow: /\n\n")
	for _, agent := range aiCrawlers {
		out.WriteString("User-agent: " + agent + "\nAllow: /\n\n")
	}
	out.WriteString("Sitemap: " + s.origin(r) + "/sitemap.xml\n")
	writeBody(w, r, http.StatusOK, []byte(out.String()), "text/plain; charset=utf-8", cacheAPI, false)
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	origin := s.origin(r)
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	out.WriteString("  <url><loc>" + origin + "/</loc></url>\n")
	for _, group := range catalog {
		out.WriteString("  <url><loc>" + origin + "/docs/" + group.ID + "</loc></url>\n")
	}
	out.WriteString("</urlset>\n")
	writeBody(w, r, http.StatusOK, []byte(out.String()), "application/xml; charset=utf-8", cacheAPI, false)
}

const favicon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" fill="#171512"/><path d="M14 44V16h9l18 28h-9L14 16v28z" fill="#f5f0e6"/><path d="M43 16h7v32h-7zM14 44h36v7H14z" fill="#c8ff3d"/><path d="M14 13h36v5H14z" fill="#e23d31"/></svg>`

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	writeBody(w, r, http.StatusOK, []byte(favicon), typeSVG, cacheLong, false)
}

// ---- documentation site ----

type pageData struct {
	Origin      string
	Title       string
	Description string
	Canonical   string
	Groups      []pageGroup
	Single      bool
	Limit       int
	Counts      datasetCounts
}

type pageGroup struct {
	apiGroup
	Source *sourceRef
}

var pageTemplate = template.Must(template.New("page").Funcs(template.FuncMap{
	"queryParams": func(e endpoint) []param {
		var out []param
		for _, p := range e.Params {
			if p.In == "query" {
				out = append(out, p)
			}
		}
		return out
	},
	"join": strings.Join,
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.Canonical}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.Canonical}}">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="alternate" type="text/plain" href="/llms.txt" title="API guide for language models">
<link rel="service-desc" type="application/json" href="/openapi.json">
<style>
:root{--ink:#171512;--soft:#4c463d;--paper:#f3efe7;--card:#fffaf0;--line:#252018;--blue:#2248ff;--gold:#ffb627}
*{box-sizing:border-box}
body{margin:0;font:16px/1.55 system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--ink);background:var(--paper)}
main{max-width:920px;margin:0 auto;padding:32px 16px 72px}
a{color:var(--blue)}
h1{font:900 clamp(2rem,6vw,3.4rem)/1 Georgia,"Times New Roman",serif;margin:8px 0 12px}
h2{font:800 1.6rem/1.2 Georgia,serif;margin:0 0 6px}
h3{font-size:1rem;margin:22px 0 6px}
.kicker{font-size:.8rem;letter-spacing:.12em;text-transform:uppercase;color:var(--soft);margin:0}
.lead{font-size:1.1rem;color:var(--soft);max-width:60ch}
nav{display:flex;flex-wrap:wrap;gap:8px;margin:20px 0 28px}
nav a{padding:6px 12px;border:1px solid var(--line);background:var(--card);text-decoration:none;color:var(--ink);font-size:.92rem}
section{background:var(--card);border:1px solid var(--line);box-shadow:6px 6px 0 var(--gold);padding:20px;margin:0 0 28px}
code{font:.9em ui-monospace,SFMono-Regular,Consolas,monospace;overflow-wrap:anywhere}
pre{margin:6px 0;padding:10px 12px;background:var(--ink);color:#f5f0e6;overflow-x:auto;font:.88rem/1.5 ui-monospace,Consolas,monospace}
pre a{color:#c8ff3d}
table{border-collapse:collapse;width:100%;font-size:.92rem;margin:6px 0}
td,th{text-align:left;vertical-align:top;padding:5px 10px 5px 0;border-bottom:1px solid #d9d2c3}
th{font-weight:600;white-space:nowrap}
.source{font-size:.86rem;color:var(--soft);margin-top:14px}
footer{color:var(--soft);font-size:.9rem;margin-top:36px}
</style>
</head>
<body>
<main>
<p class="kicker"><a href="/">` + siteName + `</a></p>
{{if .Single}}{{with index .Groups 0}}<h1>{{.Title}} API</h1><p class="lead">{{.Tagline}} Free, no API key.</p>{{end}}
{{else}}<h1>` + siteTagline + `</h1>
<p class="lead">JSON APIs for Indian data. No API key, no sign-up. Open CORS, so they work straight from a browser.</p>
<pre>curl <a href="{{.Origin}}/v1/pincode/637001">{{.Origin}}/v1/pincode/637001</a></pre>
<p>Every response has the same shape: <code>{ "data": ..., "meta": { ... } }</code>. <code>meta.source</code> names the dataset and its licence.
Errors are <code>{ "error": { "code", "message" } }</code> with a matching status code. The limit is {{.Limit}} requests a minute per client.</p>
<p>For tools and AI agents: <a href="/openapi.json">OpenAPI</a>, <a href="/llms.txt">llms.txt</a>, an MCP server at <code>{{.Origin}}/mcp</code>, and a machine-readable index at <a href="/v1">/v1</a>.</p>
<nav>{{range .Groups}}<a href="/docs/{{.ID}}">{{.Title}}</a>{{end}}</nav>{{end}}
{{range .Groups}}
<section id="{{.ID}}">
{{if not $.Single}}<h2><a href="/docs/{{.ID}}">{{.Title}}</a></h2>{{end}}
<p>{{.Intro}}</p>
{{range .Endpoints}}
<h3>{{.Summary}}</h3>
<pre>GET {{.Path}}</pre>
{{if .Doc}}<p>{{.Doc}}</p>{{end}}
{{with queryParams .}}<table>{{range .}}<tr><th><code>{{.Name}}</code></th><td>{{.Doc}}{{if .Enum}} One of: {{join .Enum ", "}}.{{end}}{{if .Required}} Required.{{end}}</td></tr>{{end}}</table>{{end}}
<p>Try it: <a href="{{$.Origin}}{{.Example}}">{{$.Origin}}{{.Example}}</a></p>
{{end}}
{{with .Source}}<p class="source">Data: <a href="{{.URL}}">{{.Name}}</a>{{if .Version}} ({{.Version}}){{end}}, retrieved {{.Retrieved}}. Licence: {{.Licence}}.{{if .Attribution}} {{.Attribution}}{{end}}</p>{{end}}
</section>
{{end}}
<footer>
<p>{{.Counts.Logos}} logos, {{.Counts.PostOffices}} post offices, {{.Counts.Branches}} bank branches, {{.Counts.RTOs}} RTO codes.
Company logos and trademarks belong to their respective owners.</p>
<p>Built by <a href="https://dharun.dev">Dharun Ashokkumar</a>. Source on <a href="https://github.com/dharunashokkumar/api.dharun.dev">GitHub</a>.</p>
</footer>
</main>
</body>
</html>
`))

func (s *Server) renderPage(origin string, groups []apiGroup, single bool) []byte {
	data := pageData{
		Origin:      origin,
		Title:       siteName + ": " + siteTagline,
		Description: siteSummary,
		Canonical:   origin + "/",
		Single:      single,
		Limit:       s.cfg.RateLimitPerMinute,
		Counts:      s.counts,
	}
	for _, group := range groups {
		data.Groups = append(data.Groups, pageGroup{apiGroup: group, Source: s.source(group.SourceKey)})
	}
	if single {
		data.Title = groups[0].Title + " API for India, free and open | " + siteName
		data.Description = groups[0].Tagline + " " + groups[0].Intro
		data.Canonical = origin + "/docs/" + groups[0].ID
	}
	var buffer bytes.Buffer
	if err := pageTemplate.Execute(&buffer, data); err != nil {
		return []byte("template error: " + err.Error())
	}
	return buffer.Bytes()
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	body := s.cached(r, "home", func(origin string) []byte { return s.renderPage(origin, catalog, false) })
	writeBody(w, r, http.StatusOK, body, "text/html; charset=utf-8", cacheShort, true)
}

func (s *Server) handleGroupPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("group")
	for _, group := range catalog {
		if group.ID == id {
			body := s.cached(r, "docs:"+id, func(origin string) []byte { return s.renderPage(origin, []apiGroup{group}, true) })
			writeBody(w, r, http.StatusOK, body, "text/html; charset=utf-8", cacheShort, true)
			return
		}
	}
	s.notFound(w, r, "No such documentation page.")
}
