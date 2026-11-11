package api

// The interactive parts of the site (growth plan Parts 2 and 3): the account pages,
// the try-it panel, the playground, the tool pages and the demos. Each page is
// rendered here like the rest of the site; its behaviour is a plain JavaScript file
// from internal/api/assets, served from this binary. No framework, no build step,
// nothing from other hosts. Every page explains itself and shows the API call
// without JavaScript.

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"path"
	"strings"
)

//go:embed assets/*.js
var assetFiles embed.FS

// handleAsset serves a script from assets/.
func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if path.Ext(name) != ".js" || strings.Contains(name, "/") {
		s.notFound(w, r, "No such file.")
		return
	}
	body, err := assetFiles.ReadFile("assets/" + name)
	if err != nil {
		s.notFound(w, r, "No such file.")
		return
	}
	writeBody(w, r, http.StatusOK, body, "text/javascript; charset=utf-8", cacheShort, true)
}

// appPage is the Body of an interactive page.
type appPage struct {
	Crumbs  [][2]string // name, address
	Heading string
	Lead    string
	HTML    template.HTML // the page's own markup
	Scripts []string      // files in assets/, loaded at the end
	Data    template.JS   // JSON the scripts read, from <script id="page-data">
	Wide    bool
}

// pageData encodes a value for <script type="application/json">: json.Marshal
// escapes <, > and &, so it cannot close the element.
func pageData(value any) template.JS {
	encoded, _ := json.Marshal(value)
	return template.JS(encoded)
}

const htmlApp = `
{{define "app"}}{{template "top" .}}{{with .Body}}
<div class="container page{{if .Wide}} wide{{end}}">
<header class="pagehead">
<p class="crumbs"><a href="/">` + productName + `</a>{{range .Crumbs}} / <a href="{{index . 1}}">{{index . 0}}</a>{{end}}</p>
<h1>{{.Heading}}</h1>
{{with .Lead}}<p class="lead">{{.}}</p>{{end}}
</header>
{{.HTML}}
{{if .Data}}<script type="application/json" id="page-data">{{.Data}}</script>{{end}}
{{range .Scripts}}<script src="/assets/{{.}}" defer></script>
{{end}}</div>
{{end}}{{template "bottom" .}}{{end}}
`

// cssApp styles forms, panels and the playground, in the site's own tokens.
const cssApp = `
.wide .pagehead { max-width: none; }
.app-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: var(--gap-columns); align-items: start; }
.card { padding: var(--space-lg); border: 1px solid var(--color-rule); border-radius: var(--radius-card); background: var(--color-paper); box-shadow: var(--shadow-card); }
.card + .card { margin-top: var(--space-lg); }
.card h2 { font-size: var(--text-title); max-width: none; margin-bottom: var(--space-md); }
.form { display: grid; gap: var(--space-md); max-width: 36rem; }
.form label, .field > span { display: grid; gap: var(--space-2xs); font: 500 var(--text-sm)/1.3 var(--font-display); color: var(--color-ink-2); }
.form input, .form select, .form textarea, .field input, .field select, .field textarea { width: 100%; min-height: 44px; padding: var(--space-xs) var(--space-sm); border: 1px solid var(--color-rule-2); border-radius: var(--radius-input); background: var(--color-key-face); color: var(--color-ink); font: 400 var(--text-md)/1.4 var(--font-body); }
.form textarea, .field textarea { min-height: 8rem; font-family: var(--font-outlier); font-size: var(--text-sm); }
.form input:focus-visible, .form select:focus-visible, .form textarea:focus-visible, .field input:focus-visible, .field select:focus-visible, .field textarea:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 1px; }
.fields { display: grid; grid-template-columns: repeat(auto-fill, minmax(14rem, 1fr)); gap: var(--space-sm) var(--space-md); }
.field { display: grid; gap: var(--space-2xs); }
.field small, .form small { color: var(--color-muted); font-size: var(--text-xs); font-weight: 400; }
.row { display: flex; flex-wrap: wrap; gap: var(--space-sm); align-items: center; }
.status { min-height: 1.5em; color: var(--color-ink-2); font-size: var(--text-sm); }
.status.error { color: var(--color-error); }
.status.good { color: var(--color-success); }
.keybox { display: flex; gap: var(--space-sm); align-items: center; flex-wrap: wrap; padding: var(--space-md); border-radius: var(--radius-card); background: var(--color-graphite); color: var(--color-graphite-ink); font-family: var(--font-outlier); overflow-wrap: anywhere; }
.keylist { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--space-sm); }
.keylist li { display: flex; flex-wrap: wrap; gap: var(--space-sm); align-items: center; justify-content: space-between; padding: var(--space-sm) 0; border-top: 1px solid var(--color-rule); }
.key--small { min-height: 36px; padding: 0 var(--space-md); font-size: var(--text-sm); }
.key--danger { color: var(--color-error); }
.meter { height: 8px; border-radius: 4px; background: var(--color-paper-3); overflow: hidden; }
.meter > span { display: block; height: 100%; background: var(--color-accent); }
.tryit { margin-top: var(--space-md); border: 1px solid var(--color-rule); border-radius: var(--radius-card); background: var(--color-paper-2); }
.tryit > summary { cursor: pointer; padding: var(--space-sm) var(--space-md); font: 500 var(--text-sm)/1.3 var(--font-display); list-style: none; display: flex; align-items: center; gap: var(--space-xs); min-height: 44px; }
.tryit > summary::before { content: "▶"; font-size: 0.7em; color: var(--color-accent); transition: transform var(--dur-short) var(--ease-out); }
.tryit[open] > summary::before { transform: rotate(90deg); }
.tryit .inner { display: grid; gap: var(--space-md); padding: 0 var(--space-md) var(--space-md); }
.answer { display: grid; gap: var(--space-xs); }
.pairs { display: grid; grid-template-columns: minmax(8rem, max-content) 1fr; gap: var(--space-2xs) var(--space-md); margin: 0; }
.pairs dt { color: var(--color-muted); }
.pairs dd { margin: 0; overflow-wrap: anywhere; }
.answer .meta { display: flex; flex-wrap: wrap; gap: var(--space-xs) var(--space-md); color: var(--color-muted); font: 500 var(--text-xs)/1.3 var(--font-outlier); }
.answer .meta b { color: var(--color-ink); }
.answer pre { max-height: 28rem; overflow: auto; }
.answer img, .answer iframe, .answer audio { max-width: 100%; border-radius: var(--radius-card); background: var(--color-paper); border: 1px solid var(--color-rule); }
.answer iframe { width: 100%; height: 28rem; }
.tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr)); gap: var(--space-md); list-style: none; margin: 0; padding: 0; }
.tiles a { display: grid; gap: var(--space-2xs); height: 100%; padding: var(--space-md) var(--space-lg); border: 1px solid var(--color-rule); border-radius: var(--radius-card); background: var(--color-paper); text-decoration: none; box-shadow: var(--shadow-card); transition: border-color var(--dur-short) var(--ease-out), transform var(--dur-short) var(--ease-out); }
.tiles a:hover { border-color: var(--color-accent); transform: translateY(-1px); }
.tiles b { font: 600 var(--text-lg)/1.2 var(--font-display); color: var(--color-ink); }
.tiles span { color: var(--color-ink-2); font-size: var(--text-sm); }
.play { display: grid; grid-template-columns: 18rem minmax(0, 1fr); gap: var(--gap-columns); align-items: start; }
.play .finder { position: sticky; top: calc(var(--nav-h) + var(--space-xl)); display: grid; gap: var(--space-sm); max-height: calc(100vh - var(--nav-h) - var(--space-2xl)); overflow: auto; }
.play .finder ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 2px; }
.play .finder button { width: 100%; text-align: left; padding: var(--space-xs) var(--space-sm); border: 0; border-radius: var(--radius-input); background: none; color: var(--color-ink-2); cursor: pointer; font-size: var(--text-sm); min-height: 36px; }
.play .finder button:hover, .play .finder button[aria-current] { background: var(--color-accent-wash); color: var(--color-accent-deep); }
.play .finder code { background: none; padding: 0; font-size: var(--text-xs); color: var(--color-muted); }
.toggle { display: inline-flex; align-items: center; gap: var(--space-xs); font-size: var(--text-sm); }
.chain { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--space-sm); }
.chain li { padding: var(--space-sm) var(--space-md); border-left: 3px solid var(--color-accent); background: var(--color-paper-2); border-radius: 0 var(--radius-input) var(--radius-input) 0; }
.big { font: 600 var(--text-display-s)/1 var(--font-display); letter-spacing: var(--tracking-display); }
.logo-wall { display: grid; grid-template-columns: repeat(auto-fill, minmax(6rem, 1fr)); gap: var(--space-sm); }
.logo-wall figure { margin: 0; display: grid; gap: var(--space-2xs); justify-items: center; padding: var(--space-sm); border: 1px solid var(--color-rule); border-radius: var(--radius-card); background: var(--color-paper); }
.logo-wall img { width: 48px; height: 48px; object-fit: contain; }
.logo-wall figcaption { font: 500 var(--text-xs)/1.2 var(--font-outlier); color: var(--color-muted); text-align: center; overflow-wrap: anywhere; }
.logo-hero { display: grid; grid-template-columns: 12rem minmax(0, 1fr); gap: var(--gap-columns); align-items: center; }
.logo-hero img { width: 12rem; height: 12rem; object-fit: contain; padding: var(--space-lg); border: 1px solid var(--color-rule); border-radius: var(--radius-card); background: var(--color-paper); }
.facts-list { display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: var(--space-2xs) var(--space-lg); margin: 0; }
.facts-list dt { color: var(--color-muted); font-size: var(--text-sm); }
.facts-list dd { margin: 0; }
.scripts { display: grid; grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr)); gap: var(--space-sm); }
.scripts div { padding: var(--space-sm) var(--space-md); border: 1px solid var(--color-rule); border-radius: var(--radius-card); background: var(--color-paper); }
.scripts small { display: block; color: var(--color-muted); font-size: var(--text-xs); }
.scripts p { font-size: var(--text-lg); overflow-wrap: anywhere; }
@media (max-width: 60rem) {
  .app-grid, .play, .logo-hero { grid-template-columns: minmax(0, 1fr); }
  .play .finder { position: static; max-height: 22rem; }
  .bar nav { overflow-x: auto; scrollbar-width: none; min-width: 0; }
  .bar nav a { flex-shrink: 0; }
}
`

// ---- account pages ----

var accountPages = map[string]struct{ Title, Heading, Lead, View string }{
	"/signup":  {"Get a free API key | " + productName, "Get a free API key", "An account is free and holds up to three keys. A key raises your limits on every endpoint and will be needed for the model endpoints (OCR, speech, translation, embeddings and the rest). No password: we mail you a code.", "code"},
	"/login":   {"Log in | " + productName, "Log in", "Type your email address and we mail you a six-digit code. No password to remember.", "code"},
	"/account": {"Your account and API keys | " + productName, "Your API keys", "", "account"},
}

const accountForms = `
<noscript><p class="aside">The account pages need JavaScript: they talk to the account service from your browser. Everything else on this site works without it.</p></noscript>
<div id="account-app" data-view="{{.View}}">
{{if eq .View "code"}}
<div class="app-grid">
<div class="card">
<form class="form" id="email-form">
<h2>Your email address</h2>
<label>Email <input type="email" name="email" autocomplete="email" required></label>
<p class="status" id="code-status" role="status"></p>
<div class="row"><button class="key key--primary" type="submit">Send me a code</button></div>
<p class="note">One step for both: a new address gets an account, a known one logs in. A short check runs in your browser first (a small sum it works out, instead of a puzzle for you).</p>
</form>
<form class="form" id="code-form" hidden>
<h2>The code from the mail</h2>
<label>Six-digit code <input name="code" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6" required></label>
<p class="status" id="code-status-2" role="status"></p>
<div class="row"><button class="key key--primary" type="submit">Log in</button><button class="key key--small" type="button" id="code-again">Use another address</button></div>
<p class="note">It works once, for 10 minutes. Not there? Look in spam, or ask again in a minute.</p>
</form>
</div>
<div class="card"><h2>What a key gives you</h2><ul>
<li>{{.KeyLimit}} units a minute per key instead of {{.Limit}} per address, on every endpoint, logos included.</li>
<li>The model endpoints: {{.Quota}} requests a day per key, one at a time.</li>
<li>Your own allowance: apps on a shared address no longer share one.</li></ul>
<p class="note">How to send a key, and what changes when: <a href="/keys">API keys</a>. Login with Google or GitHub is planned.</p></div>
</div>
{{else if eq .View "account"}}
<p class="status" id="account-status" role="status">Loading your account…</p>
<div id="account-body" hidden>
<div class="card"><h2>Keys</h2>
<div id="new-key" hidden><p><b>Your new key.</b> This is the only time it is shown: copy it now.</p><div class="keybox"><code id="new-key-value"></code><button class="copy" type="button" id="copy-key">Copy</button></div></div>
<ul class="keylist" id="keys"></ul>
<form class="row" id="key-form"><label class="field"><span>Name for a new key</span><input name="name" maxlength="60" placeholder="my app"></label><button class="key key--primary" type="submit">Make a key</button></form>
<p class="note">Send a key as <code>Authorization: Bearer &lt;key&gt;</code> or <code>X-API-Key: &lt;key&gt;</code>, never in the address. <a href="/keys">How keys work</a>.</p>
</div>
<div class="card"><h2>Use</h2><div id="usage"><p class="note">Counted per key and day (India time), by API group.</p></div></div>
<div class="card"><h2>Account</h2>
<p id="account-email"></p>
<div class="row" style="margin-top:var(--space-lg)"><button class="key" type="button" id="logout">Log out</button><button class="key" type="button" id="logout-all">Log out everywhere</button></div>
<details style="margin-top:var(--space-lg)"><summary>Delete my account</summary><form class="form" id="delete-form"><p>This removes your account, every key and their usage counts at once. It cannot be undone.</p><label>Type your email address to confirm <input name="confirm" autocomplete="off"></label><div class="row"><button class="key key--danger" type="submit">Delete everything</button></div></form></details>
</div>
</div>
{{end}}
</div>`

var accountFormsTemplate = template.Must(template.New("forms").Parse(accountForms))

func (s *Server) handleAccountPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/signup.md" || r.URL.Path == "/signup" && wantsMarkdown(r) {
		addVary(w.Header(), "Accept")
		body := "# Get a free API key\n\n" + accountPages["/signup"].Lead + " Sign up in a browser at " + s.origin(r) + "/signup; how keys work: " + s.origin(r) + "/keys.md\n"
		writeBody(w, r, http.StatusOK, []byte(body), "text/markdown; charset=utf-8", cacheShort, false)
		return
	}
	meta, ok := accountPages[r.URL.Path]
	if !ok {
		s.notFound(w, r, "Not found.")
		return
	}
	var forms strings.Builder
	accountFormsTemplate.Execute(&forms, map[string]any{"View": meta.View, "Limit": s.cfg.RateLimitPerMinute,
		"KeyLimit": s.cfg.KeyRateLimitPerMinute, "Quota": s.cfg.ModelQuotaPerDay})
	origin := s.origin(r)
	p := page{Origin: origin, Path: r.URL.Path, Title: meta.Title, Heading: meta.Heading, Nav: "account",
		Description: "Free API keys for India API: higher limits on every endpoint and access to the model endpoints.",
		Body: appPage{Heading: meta.Heading, Lead: meta.Lead, HTML: template.HTML(forms.String()), Scripts: []string{"pow.js", "account.js"},
			Data: pageData(map[string]any{"view": meta.View, "accounts": s.cfg.AccountsURL != ""})}}
	if r.URL.Path != "/signup" {
		p.Robots = "noindex"
	} else {
		p.Markdown = "/signup.md"
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Get a free API key", "/signup"}))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeBody(w, r, http.StatusOK, s.renderHTML("app", p), "text/html; charset=utf-8", cacheNone, true)
}
