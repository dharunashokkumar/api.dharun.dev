package api

// The round 5 groups: things that run or keep something for a while on the caller's
// behalf. They get their own category after the developer tools.

var round5Catalog = []apiGroup{
	{
		ID:       "hooks",
		Category: "sandbox",
		Title:    "Request bin",
		Tagline:  "A URL that records every request sent to it, for testing webhooks.",
		Intro:    "Make a bin and get a URL. Point a webhook at it (Razorpay, GitHub, a form, your own code) or call it with any method and any path below it; each request is kept with its method, path, query, headers and body (the first 64 KB). Read them back as JSON with the bin's token. A bin keeps the last 100 requests and lasts 24 hours, then it and everything in it is deleted. Headers that carry the sender's address are dropped before anything is stored, nothing is logged, and what a bin holds is only ever returned as JSON to the token's holder. Making a bin counts as 10 requests against the rate limit; each request a bin records counts as one for its sender, and a bin records up to 60 a minute.",
		Endpoints: []endpoint{
			{
				Path: "/v1/hooks", Summary: "Make a request bin", Private: true,
				Doc:         "Returns the bin's id, url (where to send requests), readUrl and token. The token is shown only in this answer; only its hash is kept.",
				Example:     "/v1/hooks",
				Upload:      &upload{Accepts: "Nothing: an empty body or {}.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: "{}",
			},
			{
				Path: "/v1/hook/{id}", Summary: "Send a request to a bin", Private: true, Post: true, Live: true,
				Doc:         "Any method (GET, HEAD, POST, PUT, PATCH, DELETE) and any path after the id (/v1/hook/{id}/payments?attempt=1) is recorded. The answer is always the same small JSON: a bin does not imitate an API. The documentation's bin, example, accepts requests and keeps none.",
				Body:        "Anything, up to 64 KB kept (the rest is counted, not kept).",
				Example:     "/v1/hook/" + hookExample,
				ExampleBody: `{"event":"payment.captured"}`,
				Params: []param{
					{Name: "id", In: "path", Type: "string", Required: true, Doc: "The bin's id."},
				},
			},
			{
				Path: "/v1/hooks/{id}", Summary: "Read what a bin received", Private: true,
				Doc:     "Newest first. body is text when it is valid UTF-8, otherwise base64 (bodyEncoding says which); truncated is true when the body was over 64 KB. Poll with since=<the highest seq you have> to get only new requests. POST /v1/hooks/{id}/delete with the same token deletes the bin at once. The example bin's token is example.",
				Example: "/v1/hooks/" + hookExample + "?token=" + hookExample,
				Params: []param{
					{Name: "id", In: "path", Type: "string", Required: true, Doc: "The bin's id."},
					{Name: "token", In: "query", Type: "string", Doc: "The bin's token, if not sent as Authorization: Bearer <token>."},
					{Name: "since", In: "query", Type: "integer", Doc: "Only requests with a higher seq."},
					{Name: "limit", In: "query", Type: "integer", Doc: "At most this many, 1 to 100. Default 100."},
				},
			},
			{
				Path: "/v1/hooks/{id}/delete", Summary: "Delete a bin now", Private: true, Live: true,
				Doc:         "Deletes the bin and everything it received. Needs the bin's token.",
				Example:     "/v1/hooks/" + hookExample + "/delete",
				Upload:      &upload{Accepts: "Nothing: an empty body or {}.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: "{}",
				Params: []param{
					{Name: "id", In: "path", Type: "string", Required: true, Doc: "The bin's id."},
				},
			},
		},
	},
}

const (
	runExample     = `{"language": "python", "code": "import sys\nprint(sum(range(10)), sys.version.split()[0])"}`
	sessionExample = `{"image": "code", "network": true}`
	execExample    = `{"command": "pip install pyfiglet && python -m pyfiglet hi"}`
	sandboxSession = "/v1/sandbox/sessions/ses0f1e2d3c4b5a6978"
	tokenNote      = " Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login)."
)

var sessionID = param{Name: "id", In: "path", Type: "string", Required: true, Doc: "The session's id."}

var sandboxCatalog = []apiGroup{
	{
		ID:       "sandbox",
		Category: "sandbox",
		Title:    "Code sandbox",
		Tagline:  "Run Python, JavaScript, Go, C, C++, Ruby, PHP or Bash in a throwaway microVM.",
		Intro: "Code in, output out. Each run gets a Firecracker microVM of its own, with its own Linux kernel, which is destroyed afterwards: nothing from one run reaches the next. A run has 30 seconds, 256 MB of memory, one processor, no network and 64 KB of each of stdout and stderr; files can be sent along and the code can read them. " +
			"A session keeps a microVM for five minutes (ten for a distribution): run several commands, write and read files, and with network on install packages from each language's own registry (PyPI, npm, the Go module proxy, RubyGems, Packagist), and nothing else. Distributions to try out: Debian, Alpine, Arch and Fedora, in a session or in the terminal page. " +
			"The sandbox services need a free sign-in with GitHub (POST /v1/sandbox/login, which works from a terminal or an agent), so they cannot be used to mine or spam anonymously. Each account has 200 runs and 30 minutes of processor time a day (50 and 10 for GitHub accounts younger than 30 days), reset at midnight India time. What ran is kept as a hash in an audit log for a year, never the code. The rest of the API stays keyless.",
		Endpoints: []endpoint{
			{
				Path: "/v1/sandbox/languages", Summary: "Languages, versions and limits",
				Doc:     "The languages the code image runs, with the version of each, the session images, and every limit. Needs no token.",
				Example: "/v1/sandbox/languages",
			},
			{
				Path: "/v1/sandbox/run", Summary: "Run code once", Private: true, Live: true,
				Doc:         "language (python, javascript, go, c, cpp, ruby, php, bash, or an alias such as py, js, node, golang, c++, sh), code (up to 64 KB), and optionally stdin, args and files ([{path, content, encoding}] with encoding text or base64; up to 20 files of 1 MB each, relative to the working directory /work). Returns stdout, stderr, exitCode, durationMs, cpuMs and limitsHit (time, memory, output)." + tokenNote,
				Example:     "/v1/sandbox/run",
				Upload:      &upload{Accepts: "JSON: language, code, and optionally stdin, args and files.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: runExample,
			},
			{
				Path: "/v1/sandbox/sessions", Summary: "Start a session", Private: true, Live: true,
				Doc:         "image is code (default) or a distribution: debian, alpine, arch, fedora. network: true lets a code session reach its languages' package registries through a proxy (500 MB of downloads), and nothing else; distributions have no network. Returns the session's id, when it expires and links to exec, run, files, stop and tty. An account may have two sessions at once." + tokenNote,
				Example:     "/v1/sandbox/sessions",
				Upload:      &upload{Accepts: "JSON: image and network, both optional.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: sessionExample,
			},
			{
				Path: "/v1/sandbox/sessions/{id}", Summary: "Describe a session", Private: true, Live: true,
				Doc:     "Its image, network and expiry." + tokenNote,
				Example: sandboxSession,
				Params:  []param{sessionID},
			},
			{
				Path: "/v1/sandbox/sessions/{id}/exec", Summary: "Run a command in a session", Private: true, Live: true,
				Doc:         "command (a shell line) or argv (a list), and optionally stdin, cwd (default /work), env and timeoutMs (up to 60 seconds). Returns the same as a run. Counts as a run against the daily quota." + tokenNote,
				Example:     sandboxSession + "/exec",
				Upload:      &upload{Accepts: "JSON: command or argv, and optionally stdin, cwd, env and timeoutMs.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: execExample,
				Params:      []param{sessionID},
			},
			{
				Path: "/v1/sandbox/sessions/{id}/run", Summary: "Run code in a session", Private: true, Live: true,
				Doc:         "As /v1/sandbox/run, in the session: files written before and packages installed stay. Code sessions only." + tokenNote,
				Example:     sandboxSession + "/run",
				Upload:      &upload{Accepts: "JSON: language, code, and optionally stdin, args and files.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: runExample,
				Params:      []param{sessionID},
			},
			{
				Path: "/v1/sandbox/sessions/{id}/upload", Summary: "Put a file in a session", Private: true, Live: true,
				Doc:     "Writes the body, or the first file of a multipart form, to path: relative to /work, or absolute. Up to 1 MB." + tokenNote,
				Example: sandboxSession + "/upload?path=receipt.png",
				Upload:  &upload{Accepts: "The file, as the body or as multipart/form-data, up to 1 MB.", Types: []string{"application/octet-stream"}, Sample: "receipt.png"},
				Params: []param{sessionID,
					{Name: "path", In: "query", Type: "string", Required: true, Doc: "Where to write it."}},
			},
			{
				Path: "/v1/sandbox/sessions/{id}/download", Summary: "Read a file from a session", Private: true, Live: true,
				Doc:     "Returns the file at path, up to 1 MB. A directory (list=true, a path ending in /, or no path for /work) is listed as JSON: name, size, dir and mode of each entry." + tokenNote,
				Example: sandboxSession + "/download?path=out.png",
				Params: []param{sessionID,
					{Name: "path", In: "query", Type: "string", Doc: "The file or directory."},
					{Name: "list", In: "query", Type: "string", Enum: []string{"true"}, Doc: "List the directory at path."}},
			},
			{
				Path: "/v1/sandbox/sessions/{id}/stop", Summary: "Stop a session now", Private: true, Live: true,
				Doc:         "Ends the session and destroys its microVM and everything in it." + tokenNote,
				Example:     sandboxSession + "/stop",
				Upload:      &upload{Accepts: "Nothing: an empty body or {}.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: "{}",
				Params:      []param{sessionID},
			},
			{
				Path: "/v1/sandbox/sessions/{id}/tty", Summary: "Terminal of a session (WebSocket)", Private: true, Live: true,
				Doc:     "A WebSocket: binary messages carry the terminal both ways, and a text message {\"cols\": 120, \"rows\": 32} resizes it. Only for sessions started from the terminal page at /terminal, which passes a browser check; programs use exec. The token may be given as ?token=, as browsers cannot set headers on a WebSocket." + tokenNote,
				Example: sandboxSession + "/tty",
				Params: []param{sessionID,
					{Name: "cols", In: "query", Type: "integer", Doc: "Columns, default 80."},
					{Name: "rows", In: "query", Type: "integer", Doc: "Rows, default 24."},
					{Name: "token", In: "query", Type: "string", Doc: "The sandbox token, for browsers."}},
			},
		},
	},
	{
		ID:       "sandbox-account",
		Category: "sandbox",
		Title:    "Sandbox account",
		Tagline:  "Sign in with GitHub for the sandbox services, and see today's quota.",
		Intro:    "The sandbox, the terminal and the throwaway databases need an account; nothing else in the API does. Sign-in uses GitHub's device flow, which works where there is no browser to redirect: POST /v1/sandbox/login gives a short code, the person enters it at github.com/login/device, and POST /v1/sandbox/login/poll with the deviceCode then returns a token for 30 days. The API asks GitHub for no permissions and reads only the public profile; it keeps the account's numeric ID, login and creation date, its usage per day and an audit log of what ran for a year, and forgets GitHub's own token at once.",
		Endpoints: []endpoint{
			{
				Path: "/v1/sandbox/login", Summary: "Start a GitHub sign-in", Private: true, Live: true,
				Doc:         "Returns userCode, verificationUri, deviceCode, interval (seconds between polls) and expiresIn.",
				Example:     "/v1/sandbox/login",
				Upload:      &upload{Accepts: "Nothing: an empty body or {}.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: "{}",
			},
			{
				Path: "/v1/sandbox/login/poll", Summary: "Finish a GitHub sign-in", Private: true, Live: true,
				Doc:         "status is pending until the person approves, slow_down when polled too often, or ok with the token, its expiry and the account's quota. 403 when the sign-in was refused or the code expired.",
				Example:     "/v1/sandbox/login/poll",
				Upload:      &upload{Accepts: `JSON: {"deviceCode": "..."}.`, Types: []string{"application/json"}, JSON: true},
				ExampleBody: `{"deviceCode": "3584d83530557fdd1f46af8289938c8ef79f9dc5"}`,
			},
			{
				Path: "/v1/sandbox/account", Summary: "Today's quota", Private: true, Live: true,
				Doc:     "The login, the daily quota, what has been used today and what is left, and when it resets." + tokenNote,
				Example: "/v1/sandbox/account",
			},
			{
				Path: "/v1/sandbox/logout", Summary: "Revoke a token", Private: true, Live: true,
				Doc:         "The token stops working at once." + tokenNote,
				Example:     "/v1/sandbox/logout",
				Upload:      &upload{Accepts: "Nothing: an empty body or {}.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: "{}",
			},
		},
	},
	{
		ID:       "browser",
		Category: "sandbox",
		Title:    "Browser tools",
		Tagline:  "Screenshots, PDFs and the text of web pages, from a real Chromium.",
		Intro:    "A page opened in headless Chromium, which runs its scripts: a screenshot (the window or the whole page, PNG, JPEG or WebP), the page as a PDF (or HTML you send, for invoices and reports), or its text as plain text or Markdown for an LLM. Each page gets a microVM of its own that reaches only public addresses, on ports 80 and 443. The browser names itself india-api-browser. The text tool reads one page you name, never follows links, and honours robots.txt for that name (or for every agent); a site's own refusal is passed on. No account is needed; each page counts as 30 requests against the rate limit, and a site can be opened 10 times a minute by all callers together. Nothing is kept or logged.",
		Endpoints: []endpoint{
			{
				Path: "/v1/browser/screenshot", Summary: "Screenshot of a page", Image: true, Private: true, Live: true, Post: true,
				Doc:         "Returns the image. GET with the options in the query, or POST JSON with url or html (up to 4 MB) and the same options.",
				Body:        "JSON: url or html, and the options.",
				Example:     "/v1/browser/screenshot?url=https://example.com&width=1280&height=800",
				ExampleBody: `{"url": "https://example.com", "fullPage": true}`,
				Params: []param{
					{Name: "url", In: "query", Type: "string", Doc: "The page, http or https."},
					{Name: "width", In: "query", Type: "integer", Doc: "Window width, 320 to 3840. Default 1280 (390 with mobile)."},
					{Name: "height", In: "query", Type: "integer", Doc: "Window height, 200 to 2160. Default 800."},
					{Name: "fullPage", In: "query", Type: "string", Enum: []string{"true"}, Doc: "The whole page, up to 16,384 pixels tall."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"png", "jpeg", "webp"}, Doc: "Default png."},
					{Name: "mobile", In: "query", Type: "string", Enum: []string{"true"}, Doc: "A phone's window and user agent."},
					{Name: "waitMs", In: "query", Type: "integer", Doc: "Extra time after the page loads, up to 10,000 ms. Default 500."},
				},
			},
			{
				Path: "/v1/browser/pdf", Summary: "A page or HTML as PDF", Output: "pdf", Private: true, Live: true, Post: true,
				Doc:         "Returns the PDF. The page's print styles apply. POST html to make a PDF from your own HTML and CSS (images inline or at public URLs).",
				Body:        "JSON: url or html, and the options.",
				Example:     "/v1/browser/pdf?url=https://example.com",
				ExampleBody: `{"html": "<h1>வணக்கம்</h1><p>Invoice 42</p>", "paper": "a4"}`,
				Params: []param{
					{Name: "url", In: "query", Type: "string", Doc: "The page, http or https."},
					{Name: "paper", In: "query", Type: "string", Enum: []string{"a4", "a3", "a5", "letter", "legal"}, Doc: "Default a4; a page's own @page size wins."},
					{Name: "landscape", In: "query", Type: "string", Enum: []string{"true"}, Doc: "Landscape pages."},
					{Name: "background", In: "query", Type: "string", Enum: []string{"true"}, Doc: "Print background colours and images."},
					{Name: "waitMs", In: "query", Type: "integer", Doc: "Extra time after the page loads, up to 10,000 ms."},
				},
			},
			{
				Path: "/v1/browser/text", Summary: "Text of a page", Private: true, Live: true, Post: true,
				Doc:         "url, title, language, the HTTP status and text: what a reader sees (innerText), or with format=markdown the page as Markdown with headings, lists, links, code and tables. 403 robots_disallowed when the site's robots.txt does not allow it.",
				Body:        "JSON: url and the options.",
				Example:     "/v1/browser/text?url=https://example.com&format=markdown",
				ExampleBody: `{"url": "https://example.com", "markdown": true}`,
				Params: []param{
					{Name: "url", In: "query", Type: "string", Required: true, Doc: "The page, http or https."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"text", "markdown"}, Doc: "Default text."},
					{Name: "waitMs", In: "query", Type: "integer", Doc: "Extra time after the page loads, up to 10,000 ms."},
				},
			},
		},
	},
	{
		ID:       "db",
		Category: "sandbox",
		Title:    "Throwaway Postgres",
		Tagline:  "A real Postgres database for an hour, for tests and demos.",
		Intro:    "Ask for a database and get one: its own user and password, 100 MB, five connections, deleted after an hour. It is real Postgres 17, reached over a WebSocket at /v1/db/ws, which serverless drivers such as Neon's speak (no port to open); the India API clients include a small bridge that lets psql and any ordinary driver connect through localhost. A user can reach only its own database. Queries stop after 30 seconds. A database over 100 MB is locked (it can still be dropped). Needs a sandbox token; an account may have three at once.",
		Endpoints: []endpoint{
			{
				Path: "/v1/db", Summary: "Make a database", Private: true, Live: true,
				Doc:         "Returns name, user, password (shown only now), websocket, a connectionString for the bridge, and expiresAt." + tokenNote,
				Example:     "/v1/db",
				Upload:      &upload{Accepts: "Nothing: an empty body or {}.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: "{}",
			},
			{
				Path: "/v1/db/{name}", Summary: "Describe a database", Private: true, Live: true,
				Doc:     "Its size, limit and expiry." + tokenNote,
				Example: "/v1/db/sb_0f1e2d3c4b5a6978",
				Params:  []param{{Name: "name", In: "path", Type: "string", Required: true, Doc: "The database's name."}},
			},
			{
				Path: "/v1/db/{name}/drop", Summary: "Delete a database now", Private: true, Live: true,
				Doc:         "Cuts off its connections and deletes it." + tokenNote,
				Example:     "/v1/db/sb_0f1e2d3c4b5a6978/drop",
				Upload:      &upload{Accepts: "Nothing: an empty body or {}.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: "{}",
				Params:      []param{{Name: "name", In: "path", Type: "string", Required: true, Doc: "The database's name."}},
			},
			{
				Path: "/v1/db/ws", Summary: "Postgres over WebSocket", Private: true, Live: true,
				Doc:     "A WebSocket carrying the Postgres protocol to the server, as Neon's serverless driver expects (set its wsProxy to this address). Postgres checks the user and password; no token is needed here.",
				Example: "/v1/db/ws",
			},
		},
	},
}

// withRound5 adds the round 5 groups at the end.
func withRound5(base []apiGroup) []apiGroup {
	return append(append(append([]apiGroup{}, base...), sandboxCatalog...), round5Catalog...)
}
