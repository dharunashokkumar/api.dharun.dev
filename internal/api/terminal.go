package api

import (
	"bytes"
	"embed"
	"html"
	"mime"
	"net/http"
	"path"
	"strings"
)

// The terminal page (round 5, phase 3): a throwaway machine in the browser. It is an
// app, not documentation, so it stands apart from the site's layout. xterm.js is served
// from here like everything else; the one thing loaded from another host is Cloudflare's
// Turnstile, the browser check the terminal needs.

//go:embed terminal/*
var terminalFiles embed.FS

func (s *Server) handleTerminalPage(w http.ResponseWriter, r *http.Request) {
	raw, _ := terminalFiles.ReadFile("terminal/index.html")
	siteKey := ""
	if service := s.extras.sandbox; service != nil && service.turnstile != nil && s.extras.github != nil {
		siteKey = service.turnstile.siteKey
	}
	page := bytes.Replace(raw, []byte("{{SITEKEY}}"), []byte(html.EscapeString(siteKey)), 1)
	socket := strings.Replace(s.origin(r), "http", "ws", 1)
	header := w.Header()
	header.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://challenges.cloudflare.com; "+
		"frame-src https://challenges.cloudflare.com; connect-src 'self' "+socket+"; style-src 'self' 'unsafe-inline'; "+
		"img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	header.Set("X-Frame-Options", "DENY")
	writeBody(w, r, http.StatusOK, page, "text/html; charset=utf-8", "no-cache", true)
}

func (s *Server) handleTerminalFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if name == "index.html" || strings.Contains(name, "/") {
		s.notFound(w, r, "No such file.")
		return
	}
	data, err := terminalFiles.ReadFile("terminal/" + name)
	if err != nil {
		s.notFound(w, r, "No such file.")
		return
	}
	kind := mime.TypeByExtension(path.Ext(name))
	if kind == "" {
		kind = "text/plain; charset=utf-8"
	}
	writeBody(w, r, http.StatusOK, data, kind, cacheShort, true)
}
