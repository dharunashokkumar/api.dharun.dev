package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Files for crawlers, AI agents and security researchers. Each is built from the
// catalog and the guides, like the pages.

func base64Std(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// ---- llms.txt ----

// llmsIntro is the opening shared by llms.txt and llms-full.txt.
func (s *Server) llmsIntro(origin string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s (%s)\n\n> %s\n\n", productName, siteName, siteSummary)
	fmt.Fprintf(&out, "Base URL: %s. Every endpoint is a plain GET under `/v1`; endpoints that take text also accept it as a POST body (text/plain, or JSON `{\"text\": ...}`). No API key and no sign-up. CORS is open, so browsers can call it directly.\n\n", origin)
	out.WriteString("JSON responses share one shape: `{ \"data\": ..., \"meta\": { ... } }`. `meta.source` names the dataset, its licence, version and the credit it asks for. Lists add `meta.total`, `meta.limit` and `meta.offset`. Errors are `{ \"error\": { \"code\", \"message\" } }` with a matching HTTP status: 400 for bad input, 404 when nothing matches, 429 over the rate limit, 503 while a dataset is not loaded.\n\n")
	fmt.Fprintf(&out, "Rate limit: %d requests per minute per client; endpoints that do heavy work (images, astronomy, live DNS) count as 10. Image endpoints return the image itself.\n\n", s.cfg.RateLimitPerMinute)
	return out.String()
}

func (s *Server) llmsIndex(origin string) []byte {
	var out strings.Builder
	out.WriteString(s.llmsIntro(origin))
	for _, category := range categorised() {
		fmt.Fprintf(&out, "## %s\n\n", category.Title)
		for _, group := range category.Groups {
			fmt.Fprintf(&out, "- [%s](%s/docs/%s.md): %s Example: `GET %s`\n", group.Title, origin, group.ID, group.Tagline, group.Endpoints[0].Example)
		}
		out.WriteString("\n")
	}
	out.WriteString("## Guides\n\n")
	for _, g := range guides {
		fmt.Fprintf(&out, "- [%s](%s/guides/%s.md): %s\n", g.Title, origin, g.Slug, g.Description)
	}
	out.WriteString("\n## For AI agents\n\n")
	fmt.Fprintf(&out, "- [MCP server](%s/mcp): Streamable HTTP, stateless, no authentication; %d tools that return exactly what the REST endpoints return. Setup for each client: [AI tools](%s/ai.md)\n", origin, len(mcpTools), origin)
	fmt.Fprintf(&out, "- [Agent Skill](%s/.well-known/agent-skills/%s/SKILL.md): teaches a coding agent the API; `npx skills add %s`\n", origin, mcpName, repoSlug)
	fmt.Fprintf(&out, "- [OpenAPI 3.1 description](%s/openapi.json)\n", origin)
	fmt.Fprintf(&out, "- [Machine-readable index of every API](%s/v1)\n", origin)
	out.WriteString("\n## Optional\n\n")
	fmt.Fprintf(&out, "- [Everything above in one file, with sample responses](%s/llms-full.txt)\n", origin)
	fmt.Fprintf(&out, "- [Privacy](%s/privacy.md): what is logged; IP addresses never are\n", origin)
	fmt.Fprintf(&out, "- [Security](%s/security.md): reporting issues, verifying releases\n", origin)
	fmt.Fprintf(&out, "- [About](%s/about.md): licences (code AGPL-3.0, examples MIT-0) and every data source\n", origin)
	fmt.Fprintf(&out, "- [Source code](%s)\n", repoURL)
	return []byte(out.String())
}

func (s *Server) handleLLMs(w http.ResponseWriter, r *http.Request) {
	writeBody(w, r, http.StatusOK, s.cached(r, "llms", s.llmsIndex), "text/plain; charset=utf-8", cacheAPI, true)
}

func (s *Server) handleLLMsFull(w http.ResponseWriter, r *http.Request) {
	body := s.cached(r, "llms-full", func(origin string) []byte {
		var out strings.Builder
		out.WriteString(s.llmsIntro(origin))
		for _, category := range categorised() {
			for _, group := range category.Groups {
				out.Write(renderMarkdown("group", s.docGroup(origin, group, true)))
				out.WriteString("\n---\n\n")
			}
		}
		for _, g := range guides {
			fmt.Fprintf(&out, "# Guide: %s\n\n> %s\n\n%s\n%s\n\n---\n\n", g.Title, g.Description, g.Markdown, guideLicenceLine)
		}
		out.Write(renderMarkdown("ai", s.aiPageData(origin)))
		for _, name := range []string{"privacy", "security", "about"} {
			out.WriteString("\n---\n\n")
			out.Write(renderMarkdown(name, s.staticPageData(origin)))
		}
		return []byte(out.String())
	})
	writeBody(w, r, http.StatusOK, body, "text/plain; charset=utf-8", cacheAPI, true)
}

// ---- robots.txt and the sitemap ----

// Crawlers that fetch on behalf of a person asking right now may also call the API;
// crawlers that build an index or a training set read the pages, not /v1, whose answers
// the pages already show.
var (
	indexCrawlers = []string{"Googlebot", "Bingbot", "GPTBot", "OAI-SearchBot", "ClaudeBot", "Claude-SearchBot",
		"PerplexityBot", "Google-Extended", "Applebot", "Applebot-Extended", "CCBot", "DuckAssistBot", "meta-externalagent"}
	userFetchers = []string{"ChatGPT-User", "Claude-User", "Perplexity-User", "MistralAI-User"}
)

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	rules := "Content-Signal: search=yes, ai-input=yes, ai-train=yes\nAllow: /\nDisallow: /v1/\nAllow: /v1$\nAllow: /v1/og-image\n\n"
	var out strings.Builder
	out.WriteString("# " + productName + ": " + siteTagline + ".\n# Search engines and AI crawlers are welcome to read and use every page.\n\n")
	out.WriteString("User-agent: *\n" + rules)
	for _, agent := range indexCrawlers {
		out.WriteString("User-agent: " + agent + "\n")
	}
	out.WriteString(rules)
	for _, agent := range userFetchers {
		out.WriteString("User-agent: " + agent + "\n")
	}
	out.WriteString("Allow: /\n\n")
	out.WriteString("Sitemap: " + s.origin(r) + "/sitemap.xml\n")
	writeBody(w, r, http.StatusOK, []byte(out.String()), "text/plain; charset=utf-8", cacheAPI, false)
}

type sitemapEntry struct{ Path, Modified string }

// siteMap lists every page. An API page's date is when its data was last retrieved.
func (s *Server) siteMap() []sitemapEntry {
	entries := []sitemapEntry{{Path: "/"}, {Path: "/docs"}}
	for _, category := range categorised() {
		for _, group := range category.Groups {
			modified := ""
			for _, key := range append([]string{group.SourceKey}, group.MoreSources...) {
				if source := s.source(key); source != nil && len(source.Retrieved) >= 10 && source.Retrieved[:10] > modified {
					modified = source.Retrieved[:10]
				}
			}
			entries = append(entries, sitemapEntry{"/docs/" + group.ID, modified})
		}
	}
	entries = append(entries, sitemapEntry{Path: "/ai"}, sitemapEntry{Path: "/guides"})
	for _, g := range guides {
		entries = append(entries, sitemapEntry{"/guides/" + g.Slug, g.Updated})
	}
	return append(entries, sitemapEntry{Path: "/privacy"}, sitemapEntry{Path: "/security"}, sitemapEntry{Path: "/about"})
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	origin := s.origin(r)
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, entry := range s.siteMap() {
		out.WriteString("  <url><loc>" + origin + entry.Path + "</loc>")
		if entry.Modified != "" {
			out.WriteString("<lastmod>" + entry.Modified + "</lastmod>")
		}
		out.WriteString("</url>\n")
	}
	out.WriteString("</urlset>\n")
	writeBody(w, r, http.StatusOK, []byte(out.String()), "application/xml; charset=utf-8", cacheAPI, true)
}

const favicon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="14" fill="#1d1d1f"/><path d="M27 18c-5 0-7 2-7 7v3c0 3-1 4-4 4 3 0 4 1 4 4v3c0 5 2 7 7 7M37 18c5 0 7 2 7 7v3c0 3 1 4 4 4-3 0-4 1-4 4v3c0 5-2 7-7 7" fill="none" stroke="#fff" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/></svg>`

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	writeBody(w, r, http.StatusOK, []byte(favicon), typeSVG, cacheLong, false)
}

// The IndexNow key file proves to Bing and the other IndexNow engines that pings about
// these pages come from their owner. The key is public by design.
func (s *Server) handleIndexNowKey(w http.ResponseWriter, r *http.Request) {
	writeBody(w, r, http.StatusOK, []byte(indexNowKey), "text/plain; charset=utf-8", cacheLong, false)
}

// ---- security.txt (RFC 9116) ----

func (s *Server) handleSecurityTxt(w http.ResponseWriter, r *http.Request) {
	origin := s.origin(r)
	// Always about six months ahead, as the RFC asks for less than a year.
	expires := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 6, 0).Format(time.RFC3339)
	body := "Contact: " + repoURL + "/security/advisories/new\n" +
		"Contact: mailto:me@dharun.dev\n" +
		"Contact: mailto:dharun.dak@gmail.com\n" +
		"Expires: " + expires + "\n" +
		"Preferred-Languages: en\n" +
		"Canonical: " + origin + "/.well-known/security.txt\n" +
		"Policy: " + origin + "/security\n"
	writeBody(w, r, http.StatusOK, []byte(body), "text/plain; charset=utf-8", cacheShort, false)
}

// ---- discovery documents ----

// handleAPICatalog is the RFC 9727 API catalog: where the API's description, its
// documentation and its status live.
func (s *Server) handleAPICatalog(w http.ResponseWriter, r *http.Request) {
	origin := s.origin(r)
	document := map[string]any{"linkset": []any{map[string]any{
		"anchor":       origin + "/v1",
		"service-desc": []any{map[string]string{"href": origin + "/openapi.json", "type": "application/openapi+json"}},
		"service-doc":  []any{map[string]string{"href": origin + "/docs", "type": "text/html"}},
		"service-meta": []any{map[string]string{"href": origin + "/llms.txt", "type": "text/plain"}},
		"status":       []any{map[string]string{"href": origin + "/healthz", "type": "application/json"}},
	}}}
	encoded, _ := json.MarshalIndent(document, "", "  ")
	writeBody(w, r, http.StatusOK, encoded, `application/linkset+json; profile="https://www.rfc-editor.org/info/rfc9727"`, cacheAPI, false)
}

const registryDescription = "Free Indian data: pincode, IFSC, GSTIN checks, holidays, RTO, logos, UPI QR. No key."

// serverCard describes the MCP server before anyone connects (MCP Server Cards, SEP-2127).
func (s *Server) serverCard(origin string) map[string]any {
	return map[string]any{
		"$schema":     "https://static.modelcontextprotocol.io/schemas/v1/server-card.schema.json",
		"name":        registryName,
		"version":     s.cfg.Version,
		"title":       productName,
		"description": registryDescription,
		"websiteUrl":  origin + "/ai",
		"repository":  map[string]string{"url": repoURL, "source": "github"},
		"icons":       []any{map[string]string{"src": origin + "/favicon.svg", "mimeType": "image/svg+xml"}},
		"remotes": []any{map[string]any{
			"type": "streamable-http", "url": origin + "/mcp", "supportedProtocolVersions": mcpProtocolVersions,
		}},
	}
}

func (s *Server) handleServerCard(w http.ResponseWriter, r *http.Request) {
	card := s.serverCard(s.origin(r))
	contentType := "application/mcp-server-card+json"
	if strings.HasPrefix(r.URL.Path, "/.well-known/") {
		// Older discovery drafts read the server's name, its endpoint and what it offers
		// from the well-known copy.
		card["serverInfo"] = map[string]string{"name": mcpName, "title": productName, "version": s.cfg.Version}
		card["endpoint"] = "/mcp"
		card["capabilities"] = map[string]any{"tools": map[string]any{}}
		contentType = typeJSON
	}
	encoded, _ := json.MarshalIndent(card, "", "  ")
	writeBody(w, r, http.StatusOK, encoded, contentType, cacheAPI, false)
}

// handleAICatalog lists what this host offers to AI agents (AI Catalog).
func (s *Server) handleAICatalog(w http.ResponseWriter, r *http.Request) {
	origin := s.origin(r)
	host := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
	document := map[string]any{
		"specVersion": "1.0",
		"host":        map[string]string{"displayName": productName, "identifier": "did:web:" + host},
		"entries": []any{map[string]any{
			"identifier":  "urn:air:" + host + ":mcp:" + mcpName,
			"displayName": productName,
			"type":        "application/mcp-server-card+json",
			"url":         origin + "/mcp/server-card",
			"representativeQueries": []string{
				"Which district and state is pincode 637001 in?",
				"Which bank branch is IFSC SBIN0000001?",
				"Is 26 January 2026 a bank holiday in Tamil Nadu?",
				"Is GSTIN 27AAPFU0939F1ZV well formed?",
			},
		}},
	}
	encoded, _ := json.MarshalIndent(document, "", "  ")
	writeBody(w, r, http.StatusOK, encoded, typeJSON, cacheAPI, false)
}

// ---- the Agent Skill, served from the site as well as from the repository ----

func (s *Server) handleSkillIndex(w http.ResponseWriter, r *http.Request) {
	files := skillFiles()
	sum := sha256.Sum256(files["SKILL.md"])
	document := map[string]any{
		"$schema": "https://schemas.agentskills.io/discovery/0.2.0/schema.json",
		"skills": []any{map[string]string{
			"name": mcpName, "type": "skill-md", "description": skillDescription,
			"url":    "/.well-known/agent-skills/" + mcpName + "/SKILL.md",
			"digest": "sha256:" + hex.EncodeToString(sum[:]),
		}},
	}
	encoded, _ := json.MarshalIndent(document, "", "  ")
	writeBody(w, r, http.StatusOK, encoded, typeJSON, cacheAPI, false)
}

func (s *Server) handleSkillFile(w http.ResponseWriter, r *http.Request) {
	body, ok := skillFiles()[r.PathValue("file")]
	if !ok {
		s.notFound(w, r, "No such file in the skill.")
		return
	}
	writeBody(w, r, http.StatusOK, body, "text/markdown; charset=utf-8", cacheAPI, true)
}

func (s *Server) handleSkillZip(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", `attachment; filename="`+mcpName+`.zip"`)
	writeBody(w, r, http.StatusOK, skillZip(), "application/zip", cacheAPI, false)
}
