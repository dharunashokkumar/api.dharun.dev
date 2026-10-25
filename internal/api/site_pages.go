package api

// The HTML of each page. Data comes from site.go; the layout from site_layout.go.

const htmlPages = `
{{define "home"}}{{template "top" .}}{{$origin := .Origin}}{{with .Body}}
<section class="container hero" aria-labelledby="hero-title">
<div>
<h1 id="hero-title">Free APIs for India.</h1>
<p class="lead">Pincodes, IFSC, GSTIN, holidays and Indian-language tools, with no key or sign-up.</p>
<div class="actions"><a class="key key--primary" href="/docs">Browse the APIs</a><a class="key" href="/ai">Use with AI</a></div>
</div>
<figure class="console" aria-label="A request to this server and its answer">
<div class="codebox" data-label="Request"><pre><code>curl {{.Example}}</code></pre></div>
{{with .Sample}}<div class="codebox" data-label="Response"><pre class="sample" tabindex="0"><code>{{.JSON}}</code></pre></div>{{end}}
</figure>
</section>

<section class="container facts">
<div><h2 class="t">No key, no sign-up</h2><p>Call it from your server or straight from a browser. CORS is open, and every answer has the same <code>{ data, meta }</code> shape.</p></div>
<div><h2 class="t">Open source</h2><p>The code is on GitHub under AGPL-3.0. Every response names its data source, its version and its licence.</p></div>
<div><h2 class="t">Private by default</h2><p>No cookies, no tracking, no IP addresses in the logs. Personal values you send are never logged. <a href="/privacy">Details</a></p></div>
</section>

<section class="sec" id="apis"><div class="container">
<div class="head head-split"><h2>Every API.</h2><p class="lead">{{.Count}} APIs in one shape. Each has its own page with parameters, a real response and code to copy.</p></div>
{{range .Categories}}<div class="cat"><h3>{{.Title}}</h3><ul>{{range .Groups}}<li><a href="/docs/{{.ID}}"><strong>{{.Title}}</strong><span>{{.Tagline}}</span></a></li>{{end}}</ul></div>
{{end}}</div></section>

<section class="band"><div class="container split">
<div class="split-text">
<h2>Ready for AI assistants.</h2>
<p class="lead">Connect the MCP server and Claude, ChatGPT, Cursor or VS Code can look up Indian data on their own. Add the Agent Skill and your coding agent knows the whole API.</p>
<a class="key" href="/ai">Set it up</a>
</div>
<div class="split-visual">
<div class="codebox" data-label="MCP server"><pre><code>claude mcp add --transport http ` + mcpName + ` {{$origin}}/mcp</code></pre></div>
<div class="codebox" data-label="Agent Skill"><pre><code>npx skills add {{.Repo}}</code></pre></div>
</div>
</div></section>

<section class="sec-l"><div class="container">
<div class="head head-split"><h2>Guides.</h2><p class="lead">Real tasks, solved with code you can paste.</p></div>
<ul class="guides">{{range .Guides}}<li><a href="/guides/{{.Slug}}"><strong class="t">{{.Title}}</strong><span>{{.Description}}</span></a></li>{{end}}</ul>
</div></section>

<section class="sec"><div class="container faqs">
<h2>Questions.</h2>
<div class="faq-list">{{range .FAQs}}<details class="faq"><summary>{{.Question}}</summary><p>{{.Answer}}</p></details>
{{end}}</div>
</div></section>
{{end}}{{template "bottom" .}}{{end}}

{{define "docs"}}{{template "top" .}}{{with .Body}}
<div class="container page">
<header class="pagehead">
<h1>API reference</h1>
<p class="lead">Every endpoint is under <code>/v1</code>. Lookups use GET; endpoints that take a file or a document use POST. JSON answers come as <code>{ "data": ..., "meta": ... }</code>; image, PDF and audio endpoints return the file itself. No key; {{.Limit}} requests a minute per client.</p>
<nav class="chips" aria-label="Categories">{{range .Categories}}<a href="#{{.ID}}">{{.Title}}</a>{{end}}</nav>
</header>
{{range .Categories}}<section class="dircat" id="{{.ID}}"><h2>{{.Title}}</h2><div class="dirs">
{{range .Groups}}<div class="dir"><h3><a href="/docs/{{.ID}}">{{.Title}}</a></h3><p>{{.Tagline}}</p>
<ul>{{range .Endpoints}}<li><code>{{.Methods}} {{.Path}}</code><span>{{.Summary}}{{with .Kind}} (returns {{.}}){{end}}</span></li>{{end}}</ul></div>
{{end}}</div></section>
{{end}}
</div>
{{end}}{{template "bottom" .}}{{end}}

{{define "group"}}{{template "top" .}}{{with .Body}}
<div class="container docs">
<article class="article">
<p class="crumbs"><a href="/docs">Docs</a> / {{.CategoryTitle}}</p>
<h1>{{.Title}} API</h1>
<p class="lead">{{.Tagline}}</p>
<p>{{.Intro}}</p>
<div class="facts-row"><span>Base URL <code>{{.Origin}}</code></span><span>No API key</span><span>{{.Limit}} requests a minute</span><span><a href="/docs/{{.ID}}.md">This page as Markdown</a></span></div>
{{if gt (len .Endpoints) 1}}<nav class="chips" aria-label="Endpoints">{{range .Endpoints}}<a href="#{{.Anchor}}">{{.Summary}}</a>{{end}}</nav>{{end}}
{{range .Endpoints}}
<section class="endpoint" id="{{.Anchor}}">
<h2>{{.Summary}}</h2>
<p class="route"><span class="method">{{.Method}}</span><span>{{.Path}}</span>{{if .Post}}<span class="method">POST</span>{{end}}</p>
{{if .Doc}}<p>{{.Doc}}</p>{{end}}
{{if .Post}}<p>Also takes POST: {{.Body}} Send it as <code>text/plain</code>, or as JSON <code>{"text": "..."}</code> with the other options beside it, up to {{$.Body.BodyLimit}}.</p>{{end}}
{{with .Upload}}<p>Send: {{.Accepts}}{{if not .JSON}} Post the file as the request body, or as <code>multipart/form-data</code>{{if gt .Files 1}} with one field per file{{end}}.{{end}}{{if .Sample}} Try it with <a href="/samples/{{.Sample}}">this sample</a>.{{end}}</p>{{end}}
{{if .Returns}}<p>The answer {{.Returns}}; errors are JSON as everywhere else.</p>{{end}}
{{if .Private}}<p class="note">What you send here is never written to the log, and the answer is not cached.</p>{{end}}
{{if or .PathParams .Query}}<table class="params"><thead><tr><th>Parameter</th><th>Description</th></tr></thead><tbody>
{{range .PathParams}}<tr><th><code>{{.Name}}</code><small>path, required</small></th><td>{{.Doc}}{{if .Enum}} One of: {{join .Enum ", "}}.{{end}}</td></tr>
{{end}}{{range .Query}}<tr><th><code>{{.Name}}</code><small>query{{if .Required}}, required{{end}}</small></th><td>{{.Doc}}{{if .Enum}} One of: {{join .Enum ", "}}.{{end}}</td></tr>
{{end}}</tbody></table>{{end}}
<div class="example">
<div class="tabs" role="tablist">{{range .Samples}}<button type="button" role="tab" data-lang="{{.Lang}}">{{.Label}}</button>{{end}}</div>
{{range .Samples}}<div class="panel" data-lang="{{.Lang}}"><p class="panel-label">{{.Label}}</p><div class="codebox" data-label="{{.Label}}"><pre><code>{{.Code}}</code></pre></div></div>
{{end}}</div>
{{if and .Upload .Upload.JSON}}<p class="label">invoice.json</p><div class="codebox" data-label="invoice.json"><pre><code>{{.ExampleBody}}</code></pre></div>{{end}}
{{if and .Image (not .Upload)}}<p class="label">Result</p><div class="preview"><img src="{{.Preview}}" alt="Example: {{.Summary}}" loading="lazy"></div>
{{else if .Sample}}<p class="label">Response{{if .Sample.Shortened}}, with lists and long text shortened{{end}}</p><div class="codebox" data-label="JSON"><pre class="sample" tabindex="0"><code>{{.Sample.JSON}}</code></pre></div>
{{end}}</section>
{{end}}
{{if .Sources}}<section class="sources"><h2>Data source</h2>{{range .Sources}}<p><a href="{{.URL}}">{{.Name}}</a>{{if .Version}}, {{.Version}}{{end}}. Retrieved {{.Retrieved}}. Licence: {{.Licence}}.{{if .Attribution}} {{.Attribution}}{{end}}</p>{{end}}</section>{{end}}
{{if .Guides}}<section class="sources"><h2>Guides</h2>{{range .Guides}}<p><a href="/guides/{{.Slug}}">{{.Title}}</a></p>{{end}}</section>{{end}}
<p class="aside">Using an AI assistant? <a href="/ai">Connect the MCP server</a> and it can call this API by itself.</p>
</article>
<aside class="side" aria-label="All APIs">{{range .Categories}}<h2>{{.Title}}</h2>{{range .Groups}}<a href="/docs/{{.ID}}"{{if eq .ID $.Body.ID}} aria-current="page"{{end}}>{{.Title}}</a>{{end}}{{end}}</aside>
</div>
{{end}}{{template "bottom" .}}{{end}}

{{define "ai"}}{{template "top" .}}{{with .Body}}
<div class="container page">
<header class="pagehead">
<h1>Use ` + productName + ` from your AI assistant</h1>
<p class="lead">Connect the MCP server and your assistant can look up pincodes, IFSC codes, holidays and more by itself. No key, no sign-up, nothing to install.</p>
</header>
<div class="article wide">
<div class="codebox" data-label="MCP server address"><pre><code>{{.Endpoint}}</code></pre></div>
<p class="note">Streamable HTTP, stateless, no authentication. Each of the {{len .Tools}} tools returns exactly what the matching REST endpoint returns.</p>
<div class="clients">{{range .Clients}}<section class="client"><h2 class="t">{{.Name}}</h2><p>{{.Note}}</p><div class="codebox" data-label="{{.Name}}"><pre><code>{{.Code}}</code></pre></div>{{if .Button}}<a class="key" href="{{.ButtonURL}}">{{.Button}}</a>{{end}}</section>
{{end}}</div>
<div class="measure">
<h2>Teach your coding agent the API</h2>
<p>The Agent Skill tells Claude Code, Codex, Copilot, Cursor, Gemini CLI and other agents when to use this API and how to call it. It is generated from the same catalog as these pages.</p>
<div class="codebox" data-label="Agent Skill"><pre><code>npx skills add {{.Repo}}</code></pre></div>
<p>Or download it and upload it where your assistant takes skills, such as claude.ai: <a href="/skills/{{.Name}}.zip">{{.Name}}.zip</a>. Read it first at <a href="/.well-known/agent-skills/{{.Name}}/SKILL.md">SKILL.md</a>. The skill is MIT-0.</p>
<h2>Try asking</h2>
<ul><li>Which district and state is pincode 637001 in?</li><li>Which bank branch is IFSC SBIN0000001, and does it support RTGS?</li><li>Is 26 January 2026 a bank holiday in Tamil Nadu? What is the next working day?</li><li>Write 1,25,000.50 rupees in words in Hindi.</li><li>Is GSTIN 27AAPFU0939F1ZV well formed, and which state is it from?</li><li>Make a GST invoice PDF for 4 bags of rice at ₹2,150 each, 5% GST, from my Tamil Nadu shop to a buyer in Bengaluru.</li><li>Read the Tamil text in this photo.</li><li>Pull the text out of pages 2 to 4 of this PDF.</li><li>Remove the background from this product photo.</li><li>Translate this notice into Tamil and Hindi.</li><li>Is this Hinglish comment abusive?</li></ul>
</div>
<h2>Tools</h2>
<table class="params"><thead><tr><th>Tool</th><th>What it does</th></tr></thead><tbody>
{{range .Tools}}<tr><th><code>{{.Name}}</code></th><td>{{.Description}}</td></tr>
{{end}}</tbody></table>
<div class="measure">
<h2>For other programs</h2>
<ul>
<li><a href="/openapi.json">OpenAPI 3.1 description</a> for code generators and API clients</li>
<li><a href="/llms.txt">llms.txt</a> and <a href="/llms-full.txt">llms-full.txt</a>: the documentation for language models</li>
<li><a href="/mcp/server-card">MCP server card</a>, <a href="/.well-known/ai-catalog.json">AI catalog</a>, <a href="/.well-known/api-catalog">API catalog</a> and <a href="/.well-known/agent-skills/index.json">skills index</a> for automatic discovery</li>
<li>Every page is also Markdown: add <code>.md</code> to its address or send <code>Accept: text/markdown</code></li>
</ul>
</div>
</div>
</div>
{{end}}{{template "bottom" .}}{{end}}

{{define "guides"}}{{template "top" .}}
<div class="container page">
<header class="pagehead">
<h1>Guides</h1>
<p class="lead">Real tasks solved with ` + productName + `. Every request in them is tested, and the code is MIT-0: paste it anywhere.</p>
</header>
<ul class="rows">{{range .Body}}<li><a href="/guides/{{.Slug}}"><strong class="t">{{.Title}}</strong><span>{{.Description}}</span></a></li>{{end}}</ul>
</div>
{{template "bottom" .}}{{end}}

{{define "guide"}}{{template "top" .}}{{with .Body}}
<div class="container page">
<div class="article prose">
<p class="crumbs"><a href="/guides">Guides</a></p>
<h1>{{.Guide.Title}}</h1>
<p class="lead">{{.Guide.Description}}</p>
{{.Guide.HTML}}
<p class="note">` + guideLicenceLine + ` Updated {{.Guide.Updated}}.</p>
{{if .Related}}<p class="aside">APIs used: {{range $i, $g := .Related}}{{if $i}}, {{end}}<a href="/docs/{{$g.ID}}">{{$g.Title}}</a>{{end}}.</p>{{end}}
</div>
</div>
{{end}}{{template "bottom" .}}{{end}}

{{define "prose"}}{{template "top" .}}
<div class="container page">
<div class="article prose">
<p class="crumbs"><a href="/">` + productName + `</a></p>
{{.Body}}
</div>
</div>
{{template "bottom" .}}{{end}}
`
