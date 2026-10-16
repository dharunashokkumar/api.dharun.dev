package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

var updateGenerated = flag.Bool("update", false, "rewrite the generated skill, plugin and registry files")

// Every page in the sitemap answers as HTML with what search engines and link previews
// read, and as Markdown where it has a Markdown form.
func TestSitePages(t *testing.T) {
	jsonLD := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	for _, entry := range testServer.siteMap() {
		page := request(t, http.MethodGet, entry.Path, "", "Accept", "text/html")
		if page.status != http.StatusOK || !strings.HasPrefix(page.header.Get("Content-Type"), "text/html") {
			t.Errorf("%s = %d %s", entry.Path, page.status, page.header.Get("Content-Type"))
			continue
		}
		body := string(page.body)
		for _, want := range []string{
			`<link rel="canonical" href="` + testOrigin + entry.Path + `">`,
			`<meta name="description" content="`, `<meta property="og:image" content="` + testOrigin + `/v1/og-image?`,
			`<title>`, `<h1`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %s", entry.Path, want)
			}
		}
		if strings.Contains(body, "template error") || strings.Contains(body, "markdown error") {
			t.Errorf("%s: %s", entry.Path, clip(body[strings.Index(body, "error"):], 300))
		}
		// Nothing is loaded from elsewhere: no scripts, styles or fonts from other hosts.
		if regexp.MustCompile(`<script[^>]+src=|<link[^>]+rel="stylesheet"|@import|fonts\.googleapis`).MatchString(body) {
			t.Errorf("%s loads something from elsewhere", entry.Path)
		}
		match := jsonLD.FindStringSubmatch(body)
		if match == nil {
			t.Errorf("%s has no structured data", entry.Path)
		} else if !json.Valid([]byte(match[1])) {
			t.Errorf("%s: structured data is not JSON: %s", entry.Path, clip(match[1], 200))
		}
		title := regexp.MustCompile(`<title>(.*?)</title>`).FindStringSubmatch(body)
		description := regexp.MustCompile(`<meta name="description" content="(.*?)">`).FindStringSubmatch(body)
		if title != nil && utf8.RuneCountInString(title[1]) > 80 {
			t.Errorf("%s: title is %d characters", entry.Path, utf8.RuneCountInString(title[1]))
		}
		if description != nil && (utf8.RuneCountInString(description[1]) > 200 || len(description[1]) < 50) {
			t.Errorf("%s: description is %d characters", entry.Path, utf8.RuneCountInString(description[1]))
		}

		// Every page has a Markdown form: the site and llms.txt say so.
		if !strings.Contains(body, `<link rel="alternate" type="text/markdown"`) {
			t.Errorf("%s has no Markdown form", entry.Path)
			continue
		}
		for _, markdown := range []result{
			request(t, http.MethodGet, entry.Path, "", "Accept", "text/markdown"),
			get(t, strings.TrimSuffix(entry.Path, "/")+map[bool]string{true: "/index.md", false: ".md"}[entry.Path == "/"]),
		} {
			if markdown.status != http.StatusOK || !strings.HasPrefix(markdown.header.Get("Content-Type"), "text/markdown") ||
				!bytes.HasPrefix(markdown.body, []byte("# ")) || bytes.Contains(markdown.body, []byte("template error")) {
				t.Errorf("%s as Markdown = %d %s: %s", entry.Path, markdown.status, markdown.header.Get("Content-Type"), clip(string(markdown.body), 120))
			}
		}
	}
	// A browser's ordinary Accept header gets HTML.
	browser := request(t, http.MethodGet, "/docs/pincode", "", "Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if !strings.HasPrefix(browser.header.Get("Content-Type"), "text/html") {
		t.Errorf("browser got %s", browser.header.Get("Content-Type"))
	}
	// A page varies on Accept, and a compressed one on Accept-Encoding as well; a shared
	// cache that saw only one of them could hand Markdown to a browser.
	for _, accept := range []string{"text/html", "text/markdown"} {
		page := request(t, http.MethodGet, "/docs/pincode", "", "Accept", accept, "Accept-Encoding", "gzip")
		if vary := varyTokens(page.header); !vary["accept"] || !vary["accept-encoding"] {
			t.Errorf("Accept %s: Vary %q", accept, page.header.Values("Vary"))
		}
	}
	// Quality values decide, and q=0 rules a type out.
	for accept, want := range map[string]string{
		"text/markdown;q=0, text/html;q=1":     "text/html",
		"text/html;q=0.1, text/markdown;q=1":   "text/markdown",
		"text/markdown, text/html":             "text/markdown",
		"text/html, text/markdown":             "text/html",
		"text/markdown;q=0.5, text/html;q=0.9": "text/html",
		"*/*":                                  "text/html",
	} {
		page := request(t, http.MethodGet, "/docs/pincode", "", "Accept", accept)
		if !strings.HasPrefix(page.header.Get("Content-Type"), want) {
			t.Errorf("Accept %q got %s, want %s", accept, page.header.Get("Content-Type"), want)
		}
	}
	// The Markdown form names its HTML page as canonical.
	for path, page := range map[string]string{"/docs/pincode.md": "/docs/pincode", "/index.md": "/", "/docs.md": "/docs"} {
		if link := get(t, path).header.Values("Link"); !slices.Contains(link, "<"+testOrigin+page+`>; rel="canonical"`) {
			t.Errorf("%s Link header %q", path, link)
		}
	}
	if home := get(t, "/"); !strings.Contains(home.header.Get("Link"), `rel="api-catalog"`) {
		t.Errorf("home page Link header: %q", home.header.Get("Link"))
	}
	expectStatus(t, "/guides/nothing", http.StatusNotFound)

	// The directory shows how each endpoint is really called and what comes back.
	for _, path := range []string{"/docs", "/docs.md"} {
		page := string(get(t, path).body)
		for _, want := range []string{"POST /v1/invoice", "GET or POST /v1/typst", "GET /v1/pincode/{pincode}", "(returns PDF)", "(returns image)"} {
			if !strings.Contains(page, want) {
				t.Errorf("%s does not show %q", path, want)
			}
		}
		if strings.Contains(page, "GET /v1/invoice") || strings.Contains(page, "Every endpoint is a GET") {
			t.Errorf("%s still calls every endpoint a GET", path)
		}
	}
}

// varyTokens lists the tokens of every Vary header, in lower case.
func varyTokens(header http.Header) map[string]bool {
	tokens := map[string]bool{}
	for _, value := range header.Values("Vary") {
		for _, token := range strings.Split(value, ",") {
			tokens[strings.ToLower(strings.TrimSpace(token))] = true
		}
	}
	return tokens
}

// API pages show what the examples really return.
func TestAPIPagesShowSamples(t *testing.T) {
	page := string(get(t, "/docs/pincode").body)
	if !strings.Contains(page, "&#34;district&#34;: &#34;Namakkal&#34;") || !strings.Contains(page, "Department of Posts") {
		t.Errorf("pincode page lacks its sample response or source:\n%s", clip(page[strings.Index(page, "<main>"):], 600))
	}
	markdown := string(get(t, "/docs/ifsc.md").body)
	if !strings.Contains(markdown, "```json\n{\n  \"data\": {") || !strings.Contains(markdown, "Example: "+testOrigin+"/v1/ifsc/SBIN0000001") {
		t.Errorf("IFSC Markdown lacks its sample:\n%s", clip(markdown, 800))
	}
	// Endpoints that depend on the network never run for the documentation.
	if mtr := string(get(t, "/docs/mtr.md").body); strings.Contains(mtr, "```json") {
		t.Error("the traceroute page ran its example")
	}

	// Shortening keeps key order and cuts long lists.
	tree, _ := readNode(json.NewDecoder(strings.NewReader(`{"b":1,"a":[1,2,3,4,5],"c":"` + strings.Repeat("x", 200) + `"}`)))
	var out strings.Builder
	if !tree.write(&out, "") || !strings.HasPrefix(out.String(), "{\n  \"b\": 1,\n  \"a\": [\n    1,\n    2,\n    3\n  ],") || !strings.Contains(out.String(), "x…\"") {
		t.Errorf("shortened sample:\n%s", out.String())
	}
}

func TestCategoriesCoverEveryGroup(t *testing.T) {
	seen := 0
	for _, category := range categorised() {
		if len(category.Groups) == 0 {
			t.Errorf("category %s is empty", category.ID)
		}
		seen += len(category.Groups)
	}
	if seen != len(catalog) {
		for _, group := range catalog {
			if categoryTitle(group.Category) == "" {
				t.Errorf("group %s has unknown category %q", group.ID, group.Category)
			}
		}
	}
}

// Every API address in a guide is requested, so a guide cannot show a call that fails.
func TestGuides(t *testing.T) {
	if len(guides) == 0 {
		t.Fatal("no guides")
	}
	address := regexp.MustCompile("https://api\\.dharun\\.dev(/v1/[^\\s\"'`)<>]*)")
	for _, g := range guides {
		if g.Title == "" || g.Published == "" || len(g.Description) < 50 || len(g.APIs) == 0 {
			t.Errorf("guide %s: incomplete header %+v", g.Slug, g)
		}
		for _, id := range g.APIs {
			if _, ok := groupByID(id); !ok {
				t.Errorf("guide %s names unknown API %s", g.Slug, id)
			}
		}
		tested := 0
		for _, match := range address.FindAllStringSubmatch(g.Markdown, -1) {
			path := strings.ReplaceAll(match[1], "&amp;", "&")
			if strings.Contains(path, "${") || strings.Contains(path, "{") {
				continue // built in code from a variable
			}
			tested++
			if strings.HasSuffix(path, "/v1/text/redact") {
				posted := request(t, http.MethodPost, path, "Call 98765 43210, PAN ABCPK1234F", "Content-Type", "text/plain")
				if posted.status != http.StatusOK || !strings.Contains(string(posted.body), "[PAN]") {
					t.Errorf("guide %s: POST %s = %d", g.Slug, path, posted.status)
				}
				continue
			}
			got := request(t, http.MethodGet, path, "", "CF-Connecting-IP", "49.204.0.1")
			// PNG needs the renderer, and PNG logos are drawn when a release is built (the
			// smoke test checks them there), so a 501 for either is not the guide's fault.
			if got.status != http.StatusOK && got.status != http.StatusNotImplemented {
				t.Errorf("guide %s: GET %s = %d: %s", g.Slug, path, got.status, clip(string(got.body), 160))
			}
		}
		if tested == 0 {
			t.Errorf("guide %s calls no API address that can be tested", g.Slug)
		}
	}
}

func TestDiscoveryDocuments(t *testing.T) {
	robots := string(get(t, "/robots.txt").body)
	for _, want := range []string{"Content-Signal: search=yes, ai-input=yes, ai-train=yes", "User-agent: ClaudeBot", "User-agent: Claude-User\nUser-agent: Perplexity-User", "Disallow: /v1/"} {
		if !strings.Contains(robots, want) && !strings.Contains(robots, strings.Split(want, "\n")[0]) {
			t.Errorf("robots.txt lacks %q", want)
		}
	}
	security := string(get(t, "/.well-known/security.txt").body)
	for _, want := range []string{"Contact: mailto:me@dharun.dev", "Expires: ", "Policy: " + testOrigin + "/security"} {
		if !strings.Contains(security, want) {
			t.Errorf("security.txt lacks %q", want)
		}
	}
	if got := string(get(t, "/"+indexNowKey+".txt").body); got != indexNowKey {
		t.Errorf("IndexNow key file: %q", got)
	}

	decode := func(path, contentType string) map[string]any {
		got := get(t, path)
		var document map[string]any
		if got.status != http.StatusOK || !strings.HasPrefix(got.header.Get("Content-Type"), contentType) || json.Unmarshal(got.body, &document) != nil {
			t.Errorf("%s = %d %s: %s", path, got.status, got.header.Get("Content-Type"), clip(string(got.body), 200))
		}
		return document
	}
	card := decode("/mcp/server-card", "application/mcp-server-card+json")
	if card["name"] != registryName || card["version"] != "1.2.3" || card["remotes"].([]any)[0].(map[string]any)["url"] != testOrigin+"/mcp" {
		t.Errorf("server card: %v", card)
	}
	if wellKnown := decode("/.well-known/mcp/server-card.json", "application/json"); wellKnown["endpoint"] != "/mcp" {
		t.Errorf("well-known server card: %v", wellKnown)
	}
	decode("/.well-known/ai-catalog.json", "application/json")
	if catalog := decode("/.well-known/api-catalog", "application/linkset+json"); catalog["linkset"] == nil {
		t.Error("API catalog has no linkset")
	}

	index := decode("/.well-known/agent-skills/index.json", "application/json")
	entry := index["skills"].([]any)[0].(map[string]any)
	skill := get(t, entry["url"].(string))
	sum := sha256.Sum256(skill.body)
	if entry["digest"] != "sha256:"+hex.EncodeToString(sum[:]) || !bytes.HasPrefix(skill.body, []byte("---\nname: "+mcpName+"\n")) {
		t.Errorf("skill index entry %v does not match the skill served", entry)
	}
	expectStatus(t, "/.well-known/agent-skills/"+mcpName+"/references/pincode.md", http.StatusOK)
	expectStatus(t, "/.well-known/agent-skills/"+mcpName+"/nothing.md", http.StatusNotFound)
	if zip := get(t, "/skills/"+mcpName+".zip"); zip.status != http.StatusOK || !bytes.HasPrefix(zip.body, []byte("PK")) {
		t.Errorf("skill zip = %d", zip.status)
	}
}

// The skill follows the Agent Skills format and covers every API.
func TestSkill(t *testing.T) {
	files := skillFiles()
	skill := string(files["SKILL.md"])
	header, body, _ := strings.Cut(strings.TrimPrefix(skill, "---\n"), "\n---\n")
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(mcpName) || len(mcpName) > 64 {
		t.Errorf("skill name %q breaks the naming rules", mcpName)
	}
	if !strings.Contains(header, "name: "+mcpName+"\n") || len(skillDescription) > 1024 || strings.Contains(skillDescription, "\n") {
		t.Errorf("skill header:\n%s", header)
	}
	if lines := strings.Count(body, "\n"); lines > 500 {
		t.Errorf("SKILL.md has %d lines; the format asks for under 500", lines)
	}
	for _, group := range catalog {
		if files["references/"+group.ID+".md"] == nil || !strings.Contains(skill, "references/"+group.ID+".md") {
			t.Errorf("skill does not cover %s", group.ID)
		}
		for _, e := range group.Endpoints {
			if !strings.Contains(skill, "`GET "+e.Path+"`") {
				t.Errorf("skill does not list %s", e.Path)
			}
		}
	}
	if strings.Contains(skill, testOrigin) || utf8.RuneCountInString(registryDescription) > 100 {
		t.Error("skill points at the test server, or the registry description is over 100 characters")
	}
}

// The committed skill, plugin, marketplace and registry entry are exactly what the
// catalog makes. After changing an API, run:
//
//	go test ./internal/api -run TestGeneratedFilesAreCurrent -update
func TestGeneratedFilesAreCurrent(t *testing.T) {
	root := filepath.Join("..", "..")
	files := repoFiles()
	for path, want := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if *updateGenerated {
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, want, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(full)
		if err != nil || !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
			t.Errorf("%s is out of date: run go test ./internal/api -run TestGeneratedFilesAreCurrent -update", path)
		}
	}
	// A reference file for an API that no longer exists is removed too.
	for _, dir := range []string{"skills/" + mcpName + "/references", "plugins/" + mcpName + "/skills/" + mcpName + "/references"} {
		entries, _ := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		for _, entry := range entries {
			path := dir + "/" + entry.Name()
			if _, ok := files[path]; !ok {
				if *updateGenerated {
					os.Remove(filepath.Join(root, filepath.FromSlash(path)))
				} else {
					t.Errorf("%s is no longer generated; delete it", path)
				}
			}
		}
	}
	var manifest map[string]any
	if json.Unmarshal(files["plugins/"+mcpName+"/.claude-plugin/plugin.json"], &manifest) != nil || manifest["name"] != mcpName {
		t.Error("plugin manifest is not valid")
	}
	if !slices.Contains(fileNames(files), "server.json") {
		t.Error("no registry entry")
	}
}
