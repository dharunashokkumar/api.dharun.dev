package api

// The HTML of each page. Data comes from site.go; the layout from site_layout.go.

const htmlPages = `
{{define "home"}}{{template "top" .}}{{$origin := .Origin}}{{with .Body}}
<section class="hero"><div class="wrap">
<p class="eyebrow">` + productName + `</p>
<h1>Free APIs for India.</h1>
<p class="lead">Pincodes, IFSC codes, GSTIN checks, holidays, GST invoices, OCR in Indian languages, UPI QR codes and more: {{.Count}} APIs, one simple shape. No key. No sign-up. Open source.</p>
<div class="actions"><a class="button" href="/docs">Browse the APIs</a><a class="more" href="/ai">Use with AI ›</a></div>
<div class="try">
<div class="codebox"><pre><code>curl {{.Example}}</code></pre></div>
{{with .Sample}}<div class="codebox"><pre class="sample"><code>{{.JSON}}</code></pre></div>{{end}}
</div>
</div></section>

<section class="band"><div class="wrap facts">
<div class="fact"><h3>No key, no sign-up</h3><p>Call it from your server or straight from a browser. CORS is open, and every answer has the same <code>{ data, meta }</code> shape.</p></div>
<div class="fact"><h3>Open source</h3><p>The code is on GitHub under AGPL-3.0. Every response names its data source, its version and its licence.</p></div>
<div class="fact"><h3>Private by default</h3><p>No cookies, no tracking, no IP addresses in the logs. Personal values you send are never logged. <a href="/privacy">Details</a></p></div>
</div></section>

<section class="band alt" id="apis"><div class="wrap">
<h2 class="section">Every API.</h2>
<p class="section-lead">Each has its own page with parameters, a real response and code to copy.</p>
<div class="cats">{{range .Categories}}
<div class="cat"><h3>{{.Title}}</h3><div class="cards">{{range .Groups}}<a class="card" href="/docs/{{.ID}}"><strong>{{.Title}}</strong><span>{{.Tagline}}</span></a>{{end}}</div></div>{{end}}
</div>
</div></section>

<section class="band"><div class="wrap split">
<div>
<h2 class="section">Ready for AI assistants.</h2>
<p class="section-lead">Connect the MCP server and Claude, ChatGPT, Cursor or VS Code can look up Indian data on their own. Add the Agent Skill and your coding agent knows the whole API.</p>
<a class="button" href="/ai">Set it up</a>
</div>
<div class="codebox"><pre><code>claude mcp add --transport http ` + mcpName + ` {{$origin}}/mcp</code></pre></div>
</div></section>

<section class="band"><div class="wrap">
<h2 class="section">Guides.</h2>
<p class="section-lead">Real tasks, solved with code you can paste.</p>
<div class="list">{{range .Guides}}<a href="/guides/{{.Slug}}"><strong>{{.Title}}</strong><span>{{.Description}}</span></a>{{end}}</div>
</div></section>

<section class="band"><div class="wrap narrow">
<h2 class="section">Questions.</h2>
{{range .FAQs}}<details class="faq"><summary>{{.Question}}</summary><p>{{.Answer}}</p></details>
{{end}}</div></section>
{{end}}{{template "bottom" .}}{{end}}

{{define "docs"}}{{template "top" .}}{{with .Body}}
<div class="wrap page">
<h1 class="section">API reference</h1>
<p class="section-lead">Every endpoint is a GET under <code>/v1</code> and answers <code>{ "data": ..., "meta": ... }</code>. No key; {{.Limit}} requests a minute per client.</p>
{{range .Categories}}<h2 class="label">{{.Title}}</h2>
{{range .Groups}}<div class="dir"><h2><a href="/docs/{{.ID}}">{{.Title}}</a></h2><p>{{.Tagline}}</p>
<ul>{{range .Endpoints}}<li><code>GET {{.Path}}</code> <span>{{.Summary}}</span></li>{{end}}</ul></div>
{{end}}{{end}}
</div>
{{end}}{{template "bottom" .}}{{end}}

{{define "group"}}{{template "top" .}}{{with .Body}}
<div class="wrap docs">
<article class="article">
<p class="crumbs"><a href="/docs">Docs</a> › {{.CategoryTitle}}</p>
<h1>{{.Title}} API</h1>
<p class="lead">{{.Tagline}}</p>
<p>{{.Intro}}</p>
<div class="facts-row"><span>Base URL <code>{{.Origin}}</code></span><span>No API key</span><span>{{.Limit}} requests a minute</span><span><a href="/docs/{{.ID}}.md">This page as Markdown</a></span></div>
{{if gt (len .Endpoints) 1}}<nav class="toc" aria-label="Endpoints">{{range .Endpoints}}<a href="#{{.Anchor}}">{{.Summary}}</a>{{end}}</nav>{{end}}
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
{{range .Samples}}<div class="panel" data-lang="{{.Lang}}"><p class="panel-label">{{.Label}}</p><div class="codebox"><pre><code>{{.Code}}</code></pre></div></div>
{{end}}</div>
{{if and .Upload .Upload.JSON}}<p class="label">invoice.json</p><div class="codebox"><pre><code>{{.ExampleBody}}</code></pre></div>{{end}}
{{if and .Image (not .Upload)}}<p class="label">Result</p><div class="preview"><img src="{{.URL}}" alt="Example: {{.Summary}}" loading="lazy"></div>
{{else if .Sample}}<p class="label">Response{{if .Sample.Shortened}}, with lists and long text shortened{{end}}</p><div class="codebox"><pre class="sample"><code>{{.Sample.JSON}}</code></pre></div>
{{end}}</section>
{{end}}
{{if .Sources}}<section class="sources"><h2>Data source</h2>{{range .Sources}}<p><a href="{{.URL}}">{{.Name}}</a>{{if .Version}}, {{.Version}}{{end}}. Retrieved {{.Retrieved}}. Licence: {{.Licence}}.{{if .Attribution}} {{.Attribution}}{{end}}</p>{{end}}</section>{{end}}
{{if .Guides}}<section class="sources"><h2>Guides</h2>{{range .Guides}}<p><a href="/guides/{{.Slug}}">{{.Title}}</a></p>{{end}}</section>{{end}}
<p class="aside">Using an AI assistant? <a href="/ai">Connect the MCP server</a> and it can call this API by itself.</p>
</article>
<aside class="side" aria-label="All APIs">{{range .Categories}}<h4>{{.Title}}</h4>{{range .Groups}}<a href="/docs/{{.ID}}"{{if eq .ID $.Body.ID}} aria-current="page"{{end}}>{{.Title}}</a>{{end}}{{end}}</aside>
</div>
{{end}}{{template "bottom" .}}{{end}}

{{define "ai"}}{{template "top" .}}{{with .Body}}
<div class="wrap page narrow article">
<h1>Use ` + productName + ` from your AI assistant</h1>
<p class="lead">Connect the MCP server and your assistant can look up pincodes, IFSC codes, holidays and more by itself. No key, no sign-up, nothing to install.</p>
<div class="codebox"><pre><code>{{.Endpoint}}</code></pre></div>
<p class="note">Streamable HTTP, stateless, no authentication. Each of the {{len .Tools}} tools returns exactly what the matching REST endpoint returns.</p>
{{range .Clients}}<section class="client"><h3>{{.Name}}</h3><p>{{.Note}}</p><div class="codebox"><pre><code>{{.Code}}</code></pre></div>{{if .Button}}<a class="button" href="{{.ButtonURL}}">{{.Button}}</a>{{end}}</section>
{{end}}
<h2>Teach your coding agent the API</h2>
<p>The Agent Skill tells Claude Code, Codex, Copilot, Cursor, Gemini CLI and other agents when to use this API and how to call it. It is generated from the same catalog as these pages.</p>
<div class="codebox"><pre><code>npx skills add {{.Repo}}</code></pre></div>
<p>Or download it and upload it where your assistant takes skills, such as claude.ai: <a href="/skills/{{.Name}}.zip">{{.Name}}.zip</a>. Read it first at <a href="/.well-known/agent-skills/{{.Name}}/SKILL.md">SKILL.md</a>. The skill is MIT-0.</p>
<h2>Try asking</h2>
<ul><li>Which district and state is pincode 637001 in?</li><li>Which bank branch is IFSC SBIN0000001, and does it support RTGS?</li><li>Is 26 January 2026 a bank holiday in Tamil Nadu? What is the next working day?</li><li>Write 1,25,000.50 rupees in words in Hindi.</li><li>Is GSTIN 27AAPFU0939F1ZV well formed, and which state is it from?</li><li>Make a GST invoice PDF for 4 bags of rice at ₹2,150 each, 5% GST, from my Tamil Nadu shop to a buyer in Bengaluru.</li><li>Read the Tamil text in this photo.</li></ul>
<h2>Tools</h2>
<table class="params"><thead><tr><th>Tool</th><th>What it does</th></tr></thead><tbody>
{{range .Tools}}<tr><th><code>{{.Name}}</code></th><td>{{.Description}}</td></tr>
{{end}}</tbody></table>
<h2>For other programs</h2>
<ul>
<li><a href="/openapi.json">OpenAPI 3.1 description</a> for code generators and API clients</li>
<li><a href="/llms.txt">llms.txt</a> and <a href="/llms-full.txt">llms-full.txt</a>: the documentation for language models</li>
<li><a href="/mcp/server-card">MCP server card</a>, <a href="/.well-known/ai-catalog.json">AI catalog</a>, <a href="/.well-known/api-catalog">API catalog</a> and <a href="/.well-known/agent-skills/index.json">skills index</a> for automatic discovery</li>
<li>Every page is also Markdown: add <code>.md</code> to its address or send <code>Accept: text/markdown</code></li>
</ul>
</div>
{{end}}{{template "bottom" .}}{{end}}

{{define "guides"}}{{template "top" .}}
<div class="wrap page narrow">
<h1 class="section">Guides</h1>
<p class="section-lead">Real tasks solved with ` + productName + `. Every request in them is tested, and the code is MIT-0: paste it anywhere.</p>
<div class="list">{{range .Body}}<a href="/guides/{{.Slug}}"><strong>{{.Title}}</strong><span>{{.Description}}</span></a>{{end}}</div>
</div>
{{template "bottom" .}}{{end}}

{{define "guide"}}{{template "top" .}}{{with .Body}}
<div class="wrap page narrow article prose">
<p class="crumbs"><a href="/guides">Guides</a></p>
<h1>{{.Guide.Title}}</h1>
<p class="lead">{{.Guide.Description}}</p>
{{.Guide.HTML}}
<p class="note">` + guideLicenceLine + ` Updated {{.Guide.Updated}}.</p>
{{if .Related}}<p class="aside">APIs used: {{range $i, $g := .Related}}{{if $i}}, {{end}}<a href="/docs/{{$g.ID}}">{{$g.Title}}</a>{{end}}.</p>{{end}}
</div>
{{end}}{{template "bottom" .}}{{end}}

{{define "prose"}}{{template "top" .}}
<div class="wrap page narrow article prose">
{{.Body}}
</div>
{{template "bottom" .}}{{end}}
`
