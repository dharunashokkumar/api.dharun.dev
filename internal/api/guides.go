package api

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// Guides are short articles that solve one real task with the API. Each is a Markdown
// file in guides/ with a small header:
//
//	---
//	title: Fill city and state from a pincode
//	description: One sentence for search results.
//	apis: pincode, states
//	published: 2026-10-14
//	updated: 2026-10-14
//	---
//
// The code in them is MIT-0, and every API address in them is requested by a test.

//go:embed guides/*.md
var guideFiles embed.FS

type guide struct {
	Slug        string
	Title       string
	Description string
	APIs        []string
	Published   string
	Updated     string
	Markdown    string // the body, without the header
	HTML        template.HTML
}

var guides = loadGuides()

var guideRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

func loadGuides() []guide {
	entries, err := guideFiles.ReadDir("guides")
	if err != nil {
		panic(err)
	}
	var out []guide
	for _, entry := range entries {
		raw, err := guideFiles.ReadFile(path.Join("guides", entry.Name()))
		if err != nil {
			panic(err)
		}
		g := parseGuide(strings.TrimSuffix(entry.Name(), ".md"), string(raw))
		var html bytes.Buffer
		if err := guideRenderer.Convert([]byte(g.Markdown), &html); err != nil {
			panic(entry.Name() + ": " + err.Error())
		}
		g.HTML = template.HTML(html.String()) // goldmark drops raw HTML unless told otherwise
		out = append(out, g)
	}
	// Newest first, then by title.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Published != out[j].Published {
			return out[i].Published > out[j].Published
		}
		return out[i].Title < out[j].Title
	})
	return out
}

func parseGuide(slug, raw string) guide {
	g := guide{Slug: slug}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	header, body, ok := strings.Cut(strings.TrimPrefix(raw, "---\n"), "\n---\n")
	if !strings.HasPrefix(raw, "---\n") || !ok {
		panic("guide " + slug + " has no header")
	}
	for _, line := range strings.Split(header, "\n") {
		key, value, _ := strings.Cut(line, ":")
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "title":
			g.Title = value
		case "description":
			g.Description = value
		case "published":
			g.Published = value
		case "updated":
			g.Updated = value
		case "apis":
			for _, id := range strings.Split(value, ",") {
				if id = strings.TrimSpace(id); id != "" {
					g.APIs = append(g.APIs, id)
				}
			}
		}
	}
	if g.Updated == "" {
		g.Updated = g.Published
	}
	g.Markdown = strings.TrimLeft(body, "\n")
	return g
}

func guideBySlug(slug string) (guide, bool) {
	for _, g := range guides {
		if g.Slug == slug {
			return g, true
		}
	}
	return guide{}, false
}

func (s *Server) handleGuides(w http.ResponseWriter, r *http.Request) {
	s.sendPage(w, r, "guides", func(origin string) []byte {
		p := page{
			Origin: origin, Path: "/guides", Nav: "guides", Markdown: "/guides.md",
			Title:       "Guides: real tasks with free Indian data APIs | " + productName,
			Heading:     "Guides",
			Description: "Short, tested guides: fill city and state from a pincode, look up IFSC codes, check GSTIN and PAN, count working days, make UPI QR codes and more.",
			Body:        guides,
		}
		var items []any
		for i, g := range guides {
			items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "url": origin + "/guides/" + g.Slug, "name": g.Title})
		}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Guides", "/guides"}), map[string]any{"@type": "ItemList", "itemListElement": items})
		return s.renderHTML("guides", p)
	}, func(origin string) []byte {
		var out strings.Builder
		out.WriteString("# Guides\n\nShort guides for common tasks with India API. Every request in them is tested; their code is MIT-0.\n\n")
		for _, g := range guides {
			fmt.Fprintf(&out, "- [%s](%s/guides/%s.md): %s\n", g.Title, origin, g.Slug, g.Description)
		}
		return []byte(out.String())
	})
}

func (s *Server) handleGuide(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSuffix(r.PathValue("slug"), ".md")
	g, ok := guideBySlug(slug)
	if !ok {
		s.notFound(w, r, "No such guide.")
		return
	}
	s.sendPage(w, r, "guide:"+slug, func(origin string) []byte {
		var related []apiGroup
		for _, id := range g.APIs {
			if group, ok := groupByID(id); ok {
				related = append(related, group)
			}
		}
		p := page{
			Origin: origin, Path: "/guides/" + slug, Nav: "guides", Markdown: "/guides/" + slug + ".md",
			Title: g.Title + " | " + productName, Heading: g.Title, Description: g.Description,
			Body: map[string]any{"Guide": g, "Related": related},
		}
		p.JSONLD = jsonLD(
			breadcrumbs(origin, [2]string{"Guides", "/guides"}, [2]string{g.Title, "/guides/" + slug}),
			map[string]any{"@type": "TechArticle", "headline": g.Title, "description": g.Description, "url": origin + "/guides/" + slug,
				"datePublished": g.Published, "dateModified": g.Updated, "inLanguage": "en-IN",
				"author": author(), "isPartOf": map[string]any{"@id": origin + "/#site"},
				"license": "https://spdx.org/licenses/MIT-0.html"},
		)
		return s.renderHTML("guide", p)
	}, func(origin string) []byte {
		return []byte("# " + g.Title + "\n\n> " + g.Description + "\n\n" + g.Markdown + "\n" + guideLicenceLine + "\n")
	})
}

const guideLicenceLine = "The code in this guide is MIT-0: copy it into anything, no credit needed."
