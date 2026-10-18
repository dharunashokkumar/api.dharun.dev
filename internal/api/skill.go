package api

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"
)

// The Agent Skill, the Claude Code plugin and the MCP Registry entry are generated from
// the catalog, like the site. They are committed to the repository (where installers
// fetch them) and served by the API; TestGeneratedFilesAreCurrent fails when the
// committed copies differ from what this code makes, and rewrites them with -update.
// They depend on nothing a server holds, so every server makes the same files.

const skillDescription = "Look up Indian reference data with the free India API (api.dharun.dev): pincodes and post offices, IFSC codes and bank branches, GSTIN and PAN format checks, bank, market and government holidays and working days, RTO vehicle codes, NSE and BSE company logos, mutual fund NAVs, HSN and SAC codes, UPI QR codes, numbers in words (lakh, crore), transliteration between Indian scripts, panchang, GST tax invoices as PDF, OCR in Indian languages, PDF and image tools, text to speech and speech to text, translation between English and 22 Indian languages, Roman typing to Indian scripts (vanakkam to வணக்கம்), moderation including Hinglish and Tanglish, multilingual embeddings and reranking, whois, TLS and speed checks from India, page screenshots and text, a code sandbox, Postgres and more. Use when building an app that needs Indian data, Indian-language AI or document tools, or when the user asks about an Indian pincode, IFSC, GSTIN, PAN, holiday, vehicle registration or listed company. No API key needed."

const mit0 = `MIT No Attribution

Copyright 2026 ` + authorName + `

Permission is hereby granted, free of charge, to any person obtaining a copy of this
software and associated documentation files (the "Software"), to deal in the Software
without restriction, including without limitation the rights to use, copy, modify,
merge, publish, distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED,
INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A
PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
`

// skillFiles are the skill's files by path inside the skill directory.
var skillFiles = sync.OnceValue(func() map[string][]byte {
	files := map[string][]byte{
		"SKILL.md": renderMarkdown("skill", map[string]any{
			"Name": mcpName, "Description": skillDescription, "Origin": publicOrigin, "Repo": repoURL,
			"Limit": defaultRateLimit, "Categories": categorised(),
		}),
		"LICENSE.txt": []byte(mit0),
	}
	for _, group := range catalog {
		files["references/"+group.ID+".md"] = renderMarkdown("group", staticDocGroup(publicOrigin, group, "skill"))
	}
	return files
})

// skillZip is the skill as one folder in a zip, the form claude.ai takes.
var skillZip = sync.OnceValue(func() []byte {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	files := skillFiles()
	for _, name := range fileNames(files) {
		// A fixed time, so the same skill always makes the same zip.
		writer, err := archive.CreateHeader(&zip.FileHeader{Name: mcpName + "/" + name, Method: zip.Deflate,
			Modified: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			panic(err)
		}
		writer.Write(files[name])
	}
	archive.Close()
	return buffer.Bytes()
})

func fileNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// contentHash is a short hash of every file under prefix and of extra.
func contentHash(files map[string][]byte, prefix string, extra []byte) string {
	sum := sha256.New()
	for _, name := range fileNames(files) {
		if strings.HasPrefix(name, prefix) {
			sum.Write([]byte(name))
			sum.Write(files[name])
		}
	}
	sum.Write(extra)
	return hex.EncodeToString(sum.Sum(nil))[:12]
}

func indentedJSON(value any) []byte {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	encoder.Encode(value)
	return buffer.Bytes()
}

var pluginKeywords = []string{"india", "pincode", "ifsc", "gstin", "pan", "holidays", "rto", "upi", "mcp", "api"}

// repoFiles are the generated files of the repository, by path from its root.
func repoFiles() map[string][]byte {
	files := clientFiles()
	for name, body := range skillFiles() {
		files["skills/"+mcpName+"/"+name] = body
		files["plugins/"+mcpName+"/skills/"+mcpName+"/"+name] = body
	}
	pluginDescription := "Indian reference data for Claude Code: the India API skill plus its MCP server (pincodes, IFSC, GSTIN and PAN checks, holidays, RTO codes, logos, UPI QR and more). Free, no key."
	files["plugins/"+mcpName+"/README.md"] = renderMarkdown("plugin-readme", map[string]any{
		"Count": len(catalog), "Origin": publicOrigin, "Repo": repoSlug, "RepoURL": repoURL, "Name": mcpName, "Market": marketName,
	})
	files["plugins/"+mcpName+"/LICENSE.txt"] = []byte(mit0)
	manifest := map[string]any{
		"name":             mcpName,
		"displayName":      productName,
		"description":      pluginDescription,
		"author":           map[string]string{"name": authorName, "url": authorURL},
		"homepage":         publicOrigin + "/ai",
		"repository":       repoURL,
		"license":          "MIT-0",
		"keywords":         pluginKeywords,
		"documentationUrl": publicOrigin + "/docs",
		"supportUrl":       repoURL + "/issues",
		"privacyPolicyUrl": publicOrigin + "/privacy",
		"mcpServers":       map[string]any{mcpName: map[string]string{"type": "http", "url": publicOrigin + "/mcp"}},
	}
	// The version follows the plugin's content, so installs update exactly when it changes.
	manifest["version"] = "1.0.0-" + contentHash(files, "plugins/"+mcpName+"/", indentedJSON(manifest))
	files["plugins/"+mcpName+"/.claude-plugin/plugin.json"] = indentedJSON(manifest)
	files[".claude-plugin/marketplace.json"] = indentedJSON(map[string]any{
		"name":        marketName,
		"description": "Plugins from " + siteName + ": free Indian data for AI agents.",
		"owner":       map[string]string{"name": authorName, "url": authorURL},
		"plugins": []any{map[string]any{
			"name":        mcpName,
			"source":      "./plugins/" + mcpName,
			"description": pluginDescription,
			"category":    "development",
			"keywords":    pluginKeywords,
			"relevance": map[string]any{
				"topic":   "Indian data",
				"signals": map[string]any{"hosts": []string{"api.dharun.dev"}},
			},
		}},
	})
	// The registry entry. CI sets "version" to the release before publishing it.
	files["server.json"] = indentedJSON(map[string]any{
		"$schema":     "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json",
		"name":        registryName,
		"title":       productName,
		"description": registryDescription,
		"version":     "1.1.0",
		"websiteUrl":  publicOrigin + "/ai",
		"repository":  map[string]string{"url": repoURL, "source": "github"},
		"remotes":     []any{map[string]string{"type": "streamable-http", "url": publicOrigin + "/mcp"}},
	})
	return files
}
