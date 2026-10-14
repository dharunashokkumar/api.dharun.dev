package api

// The layout every HTML page shares: head, header, footer, styles and the small script
// for code tabs and copy buttons. Plain white, system fonts, nothing loaded from
// elsewhere: no web fonts, no analytics, no cookies.

const siteCSS = `
:root{--text:#1d1d1f;--muted:#6e6e73;--faint:#86868b;--line:#d2d2d7;--hair:#e8e8ed;--soft:#f5f5f7;--link:#0066cc;--blue:#0071e3;--green:#1a7f37}
*{box-sizing:border-box}
html{-webkit-text-size-adjust:100%;scroll-padding-top:72px}
body{margin:0;background:#fff;color:var(--text);font:17px/1.52 -apple-system,BlinkMacSystemFont,"SF Pro Text","Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif;-webkit-font-smoothing:antialiased}
a{color:var(--link);text-decoration:none}
a:hover{text-decoration:underline}
img{max-width:100%;height:auto}
.wrap{max-width:1080px;margin:0 auto;padding:0 22px}
.narrow{max-width:760px}
header.top{position:sticky;top:0;z-index:10;background:rgba(255,255,255,.88);-webkit-backdrop-filter:saturate(180%) blur(20px);backdrop-filter:saturate(180%) blur(20px);border-bottom:1px solid var(--hair)}
header.top .wrap{display:flex;align-items:center;gap:20px;height:52px}
.brand{display:flex;align-items:center;gap:9px;color:var(--text);font-weight:600;letter-spacing:-.01em}
.brand:hover{text-decoration:none}
.brand svg{width:24px;height:24px;flex:none}
header.top nav{display:flex;gap:22px;margin-left:auto;font-size:14px}
header.top nav a{color:var(--text);opacity:.78}
header.top nav a:hover{opacity:1;text-decoration:none}
header.top nav a[aria-current]{opacity:1;font-weight:600}
h1,h2,h3{letter-spacing:-.02em}
.hero{text-align:center;padding:84px 0 64px}
.eyebrow{color:var(--muted);font-weight:600;margin:0 0 10px}
.hero h1{font-size:clamp(42px,7.4vw,72px);line-height:1.04;font-weight:700;margin:0 0 18px;letter-spacing:-.03em}
.lead{font-size:clamp(19px,2.3vw,23px);line-height:1.42;color:var(--muted);margin:0 0 28px}
.hero .lead{max-width:700px;margin:0 auto 30px}
.actions{display:flex;gap:16px;justify-content:center;align-items:center;flex-wrap:wrap}
.button{display:inline-block;background:var(--blue);color:#fff;border-radius:980px;padding:10px 21px;font-size:16px;line-height:1.4}
.button:hover{background:#0077ed;text-decoration:none}
.more{font-size:16px}
.try{max-width:780px;margin:52px auto 0;text-align:left;display:grid;grid-template-columns:minmax(0,1fr);gap:10px}
.try pre.sample{max-height:300px}
.band{padding:76px 0;border-top:1px solid var(--hair)}
.band.alt{background:var(--soft);border-top:0}
.section{font-size:clamp(30px,4.2vw,44px);line-height:1.1;margin:0 0 12px;font-weight:700}
.section-lead{color:var(--muted);font-size:19px;margin:0 0 40px;max-width:640px}
.facts{display:grid;grid-template-columns:repeat(auto-fit,minmax(230px,1fr));gap:32px}
.fact h3{font-size:20px;margin:0 0 8px}
.fact p{margin:0;color:var(--muted)}
.cats{display:grid;gap:44px}
.cat h3,.side h4{font-size:13px;color:var(--faint);font-weight:600;margin:0 0 12px;letter-spacing:.02em;text-transform:uppercase}
.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(240px,1fr));gap:12px}
.card{display:block;background:#fff;border-radius:18px;padding:18px 20px;color:var(--text);border:1px solid var(--hair);transition:box-shadow .2s}
.band.alt .card{border-color:#fff}
.card:hover{text-decoration:none;box-shadow:0 4px 18px rgba(0,0,0,.07)}
.card strong{display:block;font-size:17px;margin-bottom:3px}
.card span{display:block;color:var(--muted);font-size:15px;line-height:1.42}
.split{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:48px;align-items:center}
.list a{display:block;padding:16px 0;border-top:1px solid var(--hair);color:var(--text)}
.list a:hover{text-decoration:none}
.list a:hover strong{color:var(--link)}
.list strong{display:block;font-size:18px}
.list span{display:block;color:var(--muted);font-size:15px}
.faq{border-top:1px solid var(--hair);padding:16px 0}
.faq:last-child{border-bottom:1px solid var(--hair)}
.faq summary{cursor:pointer;font-weight:600;font-size:18px;list-style:none}
.faq summary::-webkit-details-marker{display:none}
.faq summary::after{content:"+";float:right;color:var(--faint);font-weight:400}
.faq[open] summary::after{content:"\2212"}
.faq p{color:var(--muted);margin:10px 0 0}
code,pre{font-family:"SF Mono",SFMono-Regular,ui-monospace,Menlo,Consolas,monospace}
code{font-size:.86em;background:var(--soft);border-radius:6px;padding:.12em .38em;overflow-wrap:anywhere}
pre{margin:0;background:var(--soft);border-radius:14px;padding:16px 18px;overflow-x:auto;font-size:13.5px;line-height:1.6}
pre code{background:none;padding:0;font-size:inherit;overflow-wrap:normal;border-radius:0}
pre.sample{max-height:400px;overflow:auto}
.codebox{position:relative}
.copy{position:absolute;top:8px;right:8px;font:inherit;font-size:12px;border:1px solid var(--line);background:#fff;color:var(--muted);border-radius:980px;padding:3px 10px;cursor:pointer;opacity:0;transition:opacity .15s}
.codebox:hover .copy,.copy:focus{opacity:1}
.docs{display:grid;grid-template-columns:200px minmax(0,1fr);gap:56px;padding:40px 22px 88px}
.side{order:-1;position:sticky;top:76px;align-self:start;max-height:calc(100vh - 96px);overflow:auto;font-size:14px;padding-bottom:20px}
.side h4{margin:22px 0 6px}
.side h4:first-child{margin-top:0}
.side a{display:block;color:var(--text);opacity:.75;padding:3px 0}
.side a:hover{opacity:1;text-decoration:none}
.side a[aria-current]{opacity:1;font-weight:600}
.article{min-width:0}
.article h1{font-size:clamp(34px,5vw,48px);line-height:1.08;margin:8px 0 12px;font-weight:700}
.article h2{font-size:26px;line-height:1.2;margin:44px 0 12px}
.article h3{font-size:19px;margin:28px 0 8px}
.article p,.article li{max-width:72ch}
.crumbs{font-size:14px;color:var(--muted);margin:0}
.crumbs a{color:var(--muted)}
.facts-row{display:flex;flex-wrap:wrap;gap:6px 22px;font-size:14px;color:var(--muted);padding:14px 0;border-top:1px solid var(--hair);border-bottom:1px solid var(--hair);margin:26px 0 8px}
.toc{display:flex;flex-wrap:wrap;gap:8px;margin:20px 0 0}
.toc a{font-size:14px;background:var(--soft);color:var(--text);border-radius:980px;padding:5px 12px}
.toc a:hover{text-decoration:none;background:var(--hair)}
.endpoint{padding:12px 0 36px;border-bottom:1px solid var(--hair)}
.endpoint h2{margin-top:36px}
.route{display:inline-flex;flex-wrap:wrap;gap:4px 10px;font:14px/1.5 "SF Mono",SFMono-Regular,ui-monospace,Menlo,Consolas,monospace;background:var(--soft);border-radius:10px;padding:7px 12px;margin:0 0 14px;overflow-wrap:anywhere}
.method{font-weight:700;color:var(--green)}
.note{font-size:15px;color:var(--muted)}
table.params{width:100%;border-collapse:collapse;font-size:15px;margin:18px 0 6px}
table.params th,table.params td{text-align:left;vertical-align:top;padding:11px 14px 11px 0;border-top:1px solid var(--hair)}
table.params thead th{font-size:13px;color:var(--faint);font-weight:600;border-top:0;padding-top:0}
table.params tbody th{font-weight:400;white-space:nowrap;width:1%}
table.params small{display:block;color:var(--faint);font-size:12px;margin-top:2px}
.label{font-size:13px;color:var(--faint);font-weight:600;margin:24px 0 8px}
.tabs{display:none;gap:4px;margin:24px 0 8px}
.js .tabs{display:flex}
.tabs button{font:inherit;font-size:13px;border:0;background:none;color:var(--muted);padding:5px 12px;border-radius:980px;cursor:pointer}
.tabs button[aria-selected=true]{background:var(--soft);color:var(--text);font-weight:600}
.panel{margin-top:10px}
.panel-label{font-size:13px;color:var(--faint);font-weight:600;margin:20px 0 8px}
.js .panel{display:none;margin-top:0}
.js .panel.on{display:block}
.js .panel-label{display:none}
.preview{background:var(--soft);border-radius:14px;padding:20px;text-align:center}
.preview img{max-height:320px}
.sources p{font-size:15px;color:var(--muted)}
.aside{margin-top:40px;padding:18px 20px;background:var(--soft);border-radius:14px;font-size:15px}
.prose pre{margin:14px 0}
.prose table{border-collapse:collapse;width:100%;font-size:15px;margin:16px 0}
.prose th,.prose td{text-align:left;padding:9px 12px 9px 0;border-top:1px solid var(--hair);vertical-align:top}
.dir{margin:0 0 44px}
.dir h2{font-size:22px;margin:0 0 4px}
.dir h2 a{color:var(--text)}
.dir p{color:var(--muted);margin:0 0 10px}
.dir ul{list-style:none;padding:0;margin:0;font-size:15px}
.dir li{padding:5px 0;border-top:1px solid var(--hair)}
.dir li code{background:none;padding:0;color:var(--text)}
.dir li span{color:var(--muted)}
.client{padding:24px 0;border-top:1px solid var(--hair)}
.client h3{margin:0 0 6px}
.client .button{margin-top:12px}
.page{padding:48px 22px 88px}
footer.foot{background:var(--soft);color:var(--muted);font-size:13px;padding:40px 0 48px;margin-top:0}
.foot-grid{display:grid;grid-template-columns:2fr 1fr 1fr 1fr;gap:28px}
.foot-grid strong{color:var(--text);font-size:14px}
.foot-grid p{margin:6px 0 0}
.foot-grid h4{color:var(--text);font-size:13px;margin:0 0 8px}
.foot-grid a{display:block;color:var(--muted);padding:2px 0}
.fine{border-top:1px solid var(--line);margin-top:28px;padding-top:16px}
@media (max-width:860px){
 .docs{grid-template-columns:minmax(0,1fr);gap:24px}
 .side{order:0;position:static;max-height:none;border-top:1px solid var(--hair);padding-top:24px}
 .split{grid-template-columns:minmax(0,1fr)}
 .foot-grid{grid-template-columns:1fr 1fr}
}
@media (max-width:520px){
 header.top nav{gap:14px}
 .hero{padding:56px 0 44px}
 .band{padding:56px 0}
 .foot-grid{grid-template-columns:1fr}
}
`

// siteJS adds code tabs (the chosen language is remembered in this browser only) and
// copy buttons. Without it every code sample is simply shown.
const siteJS = `
(function () {
  var key = "india-api-lang", chosen = null;
  try { chosen = localStorage.getItem(key); } catch (e) {}
  function pick(lang) {
    document.querySelectorAll(".example").forEach(function (example) {
      var panels = example.querySelectorAll(".panel");
      var use = example.querySelector('.panel[data-lang="' + lang + '"]') ? lang : panels[0].getAttribute("data-lang");
      panels.forEach(function (panel) { panel.classList.toggle("on", panel.getAttribute("data-lang") === use); });
      example.querySelectorAll(".tabs button").forEach(function (button) {
        button.setAttribute("aria-selected", button.getAttribute("data-lang") === use ? "true" : "false");
      });
    });
  }
  document.querySelectorAll(".codebox").forEach(function (box) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "copy";
    button.textContent = "Copy";
    box.appendChild(button);
  });
  document.addEventListener("click", function (event) {
    var tab = event.target.closest(".tabs button");
    if (tab) {
      pick(tab.getAttribute("data-lang"));
      try { localStorage.setItem(key, tab.getAttribute("data-lang")); } catch (e) {}
    }
    var copy = event.target.closest(".copy");
    if (copy && navigator.clipboard) {
      navigator.clipboard.writeText(copy.parentNode.querySelector("pre").innerText).then(function () {
        copy.textContent = "Copied";
        setTimeout(function () { copy.textContent = "Copy"; }, 1500);
      });
    }
  });
  pick(chosen || "curl");
})();
`

const brandMark = `<svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14" fill="#1d1d1f"/><path d="M27 18c-5 0-7 2-7 7v3c0 3-1 4-4 4 3 0 4 1 4 4v3c0 5 2 7 7 7M37 18c5 0 7 2 7 7v3c0 3 1 4 4 4-3 0-4 1-4 4v3c0 5-2 7-7 7" fill="none" stroke="#fff" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/></svg>`

const htmlLayout = `
{{define "top"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.Canonical}}">
<meta name="theme-color" content="#ffffff">
<meta name="author" content="` + authorName + `">
<meta property="og:type" content="website">
<meta property="og:site_name" content="` + productName + `">
<meta property="og:locale" content="en_IN">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.Canonical}}">
<meta property="og:image" content="{{.OGImage}}">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta name="twitter:card" content="summary_large_image">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
{{if .Markdown}}<link rel="alternate" type="text/markdown" href="{{.Markdown}}">
{{end}}<link rel="alternate" type="text/plain" href="/llms.txt" title="llms.txt">
<link rel="service-desc" type="application/openapi+json" href="/openapi.json">
<script>document.documentElement.className = "js";</script>
<style>` + siteCSS + `</style>
{{if .JSONLD}}<script type="application/ld+json">{{.JSONLD}}</script>
{{end}}</head>
<body>
<header class="top"><div class="wrap">
<a class="brand" href="/">` + brandMark + ` ` + productName + `</a>
<nav aria-label="Main">
<a href="/docs"{{if eq .Nav "docs"}} aria-current="page"{{end}}>Docs</a>
<a href="/guides"{{if eq .Nav "guides"}} aria-current="page"{{end}}>Guides</a>
<a href="/ai"{{if eq .Nav "ai"}} aria-current="page"{{end}}>AI</a>
<a href="` + repoURL + `">GitHub</a>
</nav>
</div></header>
<main>
{{end}}

{{define "bottom"}}</main>
<footer class="foot"><div class="wrap">
<div class="foot-grid">
<div><strong>` + productName + `</strong><p>` + siteTagline + `. Built by <a href="` + authorURL + `" style="display:inline">` + authorName + `</a>.</p></div>
<div><h4>Use it</h4><a href="/docs">API reference</a><a href="/guides">Guides</a><a href="/ai">AI tools</a><a href="/openapi.json">OpenAPI</a><a href="/llms.txt">llms.txt</a></div>
<div><h4>Trust</h4><a href="/privacy">Privacy</a><a href="/security">Security</a><a href="/about">About and licences</a><a href="/healthz">Status</a></div>
<div><h4>Open source</h4><a href="` + repoURL + `">Source code</a><a href="` + repoURL + `/issues">Report a problem</a><a href="` + repoURL + `/blob/main/LICENSE">AGPL-3.0 licence</a></div>
</div>
<p class="fine">Company logos and trademarks belong to their owners. Data belongs to the sources named on each API page.</p>
</div></footer>
<script>` + siteJS + `</script>
</body>
</html>
{{end}}
`
