package devtools

import (
	"bytes"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// MarkdownToHTML renders GitHub-flavoured Markdown. Raw HTML in the input is left out,
// so the output is safe to show.
func MarkdownToHTML(source string) (string, error) {
	var b bytes.Buffer
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	if err := md.Convert([]byte(source), &b); err != nil {
		return "", err
	}
	return b.String(), nil
}

// Page is a web page turned into Markdown.
type Page struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"`
	Markdown    string `json:"markdown"`
	Words       int    `json:"words"`
}

// HTMLToMarkdown converts HTML to Markdown. With main, only the page's main content is
// kept (its <main> or <article>, without navigation, headers, footers and forms). Links
// and images are made absolute against base when it is given.
func HTMLToMarkdown(source string, base *url.URL, main bool) (Page, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return Page{}, err
	}
	var page Page
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Title:
				if page.Title == "" {
					page.Title = strings.TrimSpace(textOf(n))
				}
			case atom.Html:
				page.Language = attr(n, "lang")
			case atom.Meta:
				name := strings.ToLower(attr(n, "name") + attr(n, "property"))
				if (name == "description" || name == "og:description") && page.Description == "" {
					page.Description = strings.TrimSpace(attr(n, "content"))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	root := doc
	if main {
		if found := find(doc, atom.Main); found != nil {
			root = found
		} else if found := find(doc, atom.Article); found != nil {
			root = found
		} else if found := find(doc, atom.Body); found != nil {
			root = found
		}
	}
	w := &mdWriter{base: base, main: main}
	w.block(root)
	page.Markdown = cleanMarkdown(w.b.String())
	page.Words = len(strings.Fields(page.Markdown))
	return page, nil
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func find(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := find(c, a); found != nil {
			return found
		}
	}
	return nil
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

type mdWriter struct {
	b     strings.Builder
	base  *url.URL
	main  bool
	lists []listState
	pre   int
}

type listState struct {
	ordered bool
	n       int
}

func (w *mdWriter) link(href string) string {
	href = strings.TrimSpace(href)
	if w.base == nil || href == "" {
		return href
	}
	u, err := w.base.Parse(href)
	if err != nil {
		return href
	}
	return u.String()
}

var skipped = map[atom.Atom]bool{atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Template: true, atom.Svg: true, atom.Iframe: true,
	atom.Head: true, atom.Button: true, atom.Select: true, atom.Input: true, atom.Textarea: true, atom.Canvas: true, atom.Object: true}

var chrome = map[atom.Atom]bool{atom.Nav: true, atom.Footer: true, atom.Header: true, atom.Aside: true, atom.Form: true, atom.Dialog: true}

func (w *mdWriter) blank() {
	s := w.b.String()
	if s == "" || strings.HasSuffix(s, "\n\n") {
		return
	}
	if strings.HasSuffix(s, "\n") {
		w.b.WriteString("\n")
	} else {
		w.b.WriteString("\n\n")
	}
}

func (w *mdWriter) block(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w.node(c)
	}
}

func (w *mdWriter) node(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		if w.pre > 0 {
			w.b.WriteString(n.Data)
			return
		}
		text := collapse(n.Data)
		if text == " " && (w.b.Len() == 0 || strings.HasSuffix(w.b.String(), " ") || strings.HasSuffix(w.b.String(), "\n")) {
			return
		}
		w.b.WriteString(escapeMarkdown(text))
		return
	case html.ElementNode:
	default:
		w.block(n)
		return
	}
	if skipped[n.DataAtom] || attr(n, "hidden") != "" || strings.EqualFold(attr(n, "aria-hidden"), "true") {
		return
	}
	if w.main && chrome[n.DataAtom] {
		return
	}
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		w.blank()
		level := int(n.Data[1] - '0')
		w.b.WriteString(strings.Repeat("#", level) + " " + strings.TrimSpace(collapse(textOf(n))))
		w.blank()
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Main, atom.Figure, atom.Header, atom.Footer, atom.Nav, atom.Aside, atom.Form, atom.Details, atom.Summary, atom.Address:
		w.blank()
		w.block(n)
		w.blank()
	case atom.Br:
		w.b.WriteString("  \n")
	case atom.Hr:
		w.blank()
		w.b.WriteString("---")
		w.blank()
	case atom.Strong, atom.B:
		inner := strings.TrimSpace(w.inline(n))
		if inner != "" {
			w.b.WriteString("**" + inner + "**")
		}
	case atom.Em, atom.I:
		inner := strings.TrimSpace(w.inline(n))
		if inner != "" {
			w.b.WriteString("*" + inner + "*")
		}
	case atom.Del, atom.S, atom.Strike:
		w.b.WriteString("~~" + strings.TrimSpace(w.inline(n)) + "~~")
	case atom.Code:
		if w.pre > 0 {
			w.block(n)
			return
		}
		code := textOf(n)
		fence := "`"
		if strings.Contains(code, "`") {
			fence = "``"
		}
		w.b.WriteString(fence + code + fence)
	case atom.Pre:
		w.blank()
		lang := ""
		if code := find(n, atom.Code); code != nil {
			for _, class := range strings.Fields(attr(code, "class")) {
				if l, ok := strings.CutPrefix(class, "language-"); ok {
					lang = l
				}
			}
		}
		w.b.WriteString("```" + lang + "\n")
		w.pre++
		start := w.b.Len()
		w.block(n)
		w.pre--
		if !strings.HasSuffix(w.b.String()[start:], "\n") {
			w.b.WriteString("\n")
		}
		w.b.WriteString("```")
		w.blank()
	case atom.A:
		href := attr(n, "href")
		inner := strings.TrimSpace(w.inline(n))
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") || inner == "" {
			w.b.WriteString(inner)
			return
		}
		w.b.WriteString("[" + inner + "](" + w.link(href) + ")")
	case atom.Img:
		src := attr(n, "src")
		if src == "" || strings.HasPrefix(src, "data:") {
			return
		}
		w.b.WriteString("![" + strings.ReplaceAll(attr(n, "alt"), "]", "") + "](" + w.link(src) + ")")
	case atom.Ul, atom.Ol:
		w.blank()
		w.lists = append(w.lists, listState{ordered: n.DataAtom == atom.Ol})
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.DataAtom == atom.Li {
				w.item(c)
			}
		}
		w.lists = w.lists[:len(w.lists)-1]
		w.blank()
	case atom.Blockquote:
		w.blank()
		inner := &mdWriter{base: w.base, main: w.main}
		inner.block(n)
		for _, line := range strings.Split(cleanMarkdown(inner.b.String()), "\n") {
			w.b.WriteString("> " + line + "\n")
		}
		w.blank()
	case atom.Table:
		w.blank()
		w.table(n)
		w.blank()
	default:
		w.block(n)
	}
}

func (w *mdWriter) inline(n *html.Node) string {
	inner := &mdWriter{base: w.base, main: w.main, pre: w.pre}
	inner.block(n)
	return strings.ReplaceAll(inner.b.String(), "\n", " ")
}

func (w *mdWriter) item(li *html.Node) {
	state := &w.lists[len(w.lists)-1]
	state.n++
	marker := "- "
	if state.ordered {
		marker = strconv.Itoa(state.n) + ". "
	}
	indent := strings.Repeat("  ", len(w.lists)-1)
	inner := &mdWriter{base: w.base, main: w.main, lists: w.lists}
	inner.block(li)
	lines := strings.Split(strings.TrimSpace(cleanMarkdown(inner.b.String())), "\n")
	if !strings.HasSuffix(w.b.String(), "\n") && w.b.Len() > 0 {
		w.b.WriteString("\n")
	}
	w.b.WriteString(indent + marker + lines[0] + "\n")
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		w.b.WriteString(indent + "  " + strings.TrimLeft(line, " ") + "\n")
	}
}

func (w *mdWriter) table(t *html.Node) {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Tr {
			var row []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
					row = append(row, strings.ReplaceAll(strings.TrimSpace(w.inline(c)), "|", `\|`))
				}
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(t)
	if len(rows) == 0 {
		return
	}
	width := 0
	for _, r := range rows {
		width = max(width, len(r))
	}
	for i, r := range rows {
		for len(r) < width {
			r = append(r, "")
		}
		w.b.WriteString("| " + strings.Join(r, " | ") + " |\n")
		if i == 0 {
			w.b.WriteString("|" + strings.Repeat("---|", width) + "\n")
		}
	}
}

var spaces = regexp.MustCompile(`[ \t\r\n\f]+`)

func collapse(s string) string { return spaces.ReplaceAllString(s, " ") }

var markdownSpecial = regexp.MustCompile("([\\\\`*_\\[\\]])")

func escapeMarkdown(s string) string { return markdownSpecial.ReplaceAllString(s, `\$1`) }

var manyBlank = regexp.MustCompile(`\n{3,}`)

func cleanMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.HasSuffix(line, "  ") {
			lines[i] = strings.TrimRight(line, " ") + "  "
		} else {
			lines[i] = strings.TrimRight(line, " \t")
		}
	}
	return strings.TrimSpace(manyBlank.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}
