package api

import (
	"encoding/json"
	"errors"
	"net/url"
)

// MCP tools for round 5: the code sandbox, the browser tools, the request bin and
// throwaway Postgres. The sandbox tools need the caller's sandbox token, sent as the
// MCP connection's Authorization header or as the sandbox_token argument; the rest are
// keyless. Unlike the other tools, these do something rather than look something up.

// actingTools change state somewhere (a microVM, a bin, a database): not read-only.
var actingTools = map[string]bool{
	"run_code": true, "start_sandbox_session": true, "run_in_sandbox_session": true, "stop_sandbox_session": true,
	"create_request_bin": true, "create_postgres_database": true,
}

func init() {
	for name := range actingTools {
		privateTools[name] = true
		changingTools[name] = true
	}
	for _, name := range []string{"screenshot_web_page", "web_page_to_pdf", "read_web_page", "read_request_bin"} {
		privateTools[name] = true
		openWorldTools[name] = true
	}
	openWorldTools["start_sandbox_session"] = true
	token := text("The sandbox token from POST /v1/sandbox/login, when the MCP connection does not send it as Authorization: Bearer.")
	jsonBody := func(path string, body any) (toolRequest, error) {
		encoded, err := json.Marshal(body)
		return toolRequest{path: path, contentType: "application/json", body: encoded}, err
	}
	mcpTools = append(mcpTools,
		mcpTool{
			Name: "run_code",
			Description: "Run code once in a throwaway Linux microVM and get stdout, stderr and the exit code: Python (numpy, pandas, matplotlib included), JavaScript (Node.js), Go, C, C++, Ruby, PHP or Bash. 30 seconds, 256 MB, no network. " +
				"Files can be sent along. Needs a free sandbox account (GitHub sign-in at POST /v1/sandbox/login); 200 runs a day.",
			InputSchema: schema([]string{"language", "code"}, map[string]any{
				"language":      oneOf("The language.", "python", "javascript", "go", "c", "cpp", "ruby", "php", "bash"),
				"code":          text("The program, up to 64 KB."),
				"stdin":         text("Standard input."),
				"files":         map[string]any{"type": "array", "description": "Files for the working directory: [{path, content, encoding: text or base64}].", "items": map[string]any{"type": "object"}},
				"sandbox_token": token,
			}),
			send: func(args map[string]any) (toolRequest, error) {
				return jsonBody("/v1/sandbox/run", map[string]any{"language": arg(args, "language"), "code": arg(args, "code"),
					"stdin": arg(args, "stdin"), "files": args["files"]})
			},
		},
		mcpTool{
			Name:        "start_sandbox_session",
			Description: "Start a sandbox kept for five minutes (ten for a distribution), to run several commands, keep files and, with network true, install packages from PyPI, npm, the Go proxy, RubyGems or Packagist (nothing else is reachable). image: code (default), debian, alpine, arch or fedora. Returns its id. Needs a sandbox account.",
			InputSchema: schema(nil, map[string]any{
				"image":         oneOf("The machine.", "code", "debian", "alpine", "arch", "fedora"),
				"network":       map[string]any{"type": "boolean", "description": "Reach the package registries (code image only)."},
				"sandbox_token": token,
			}),
			send: func(args map[string]any) (toolRequest, error) {
				network, _ := args["network"].(bool)
				return jsonBody("/v1/sandbox/sessions", map[string]any{"image": arg(args, "image"), "network": network})
			},
		},
		mcpTool{
			Name:        "run_in_sandbox_session",
			Description: "Run a shell command in a sandbox session (from start_sandbox_session): its files and installed packages stay between commands. Up to 60 seconds each.",
			InputSchema: schema([]string{"id", "command"}, map[string]any{
				"id":            text("The session's id."),
				"command":       text("A shell command line, such as pip install requests && python main.py."),
				"stdin":         text("Standard input."),
				"sandbox_token": token,
			}),
			send: func(args map[string]any) (toolRequest, error) {
				if arg(args, "id") == "" {
					return toolRequest{}, errors.New("id is required: the session from start_sandbox_session.")
				}
				return jsonBody("/v1/sandbox/sessions/"+url.PathEscape(arg(args, "id"))+"/exec",
					map[string]any{"command": arg(args, "command"), "stdin": arg(args, "stdin")})
			},
		},
		mcpTool{
			Name:        "stop_sandbox_session",
			Description: "Stop a sandbox session now; its machine and files are destroyed.",
			InputSchema: schema([]string{"id"}, map[string]any{"id": text("The session's id."), "sandbox_token": token}),
			send: func(args map[string]any) (toolRequest, error) {
				return jsonBody("/v1/sandbox/sessions/"+url.PathEscape(arg(args, "id"))+"/stop", map[string]any{})
			},
		},
		mcpTool{
			Name:        "screenshot_web_page",
			Description: "A screenshot of a public web page in a real Chromium, as an image: the window or the whole page, desktop or phone size.",
			InputSchema: schema([]string{"url"}, map[string]any{
				"url":      text("The page, http or https."),
				"fullPage": map[string]any{"type": "boolean", "description": "The whole page rather than the window."},
				"mobile":   map[string]any{"type": "boolean", "description": "A phone's window."},
				"width":    map[string]any{"type": "integer", "description": "Window width, 320 to 3840."},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				full, _ := args["fullPage"].(bool)
				mobile, _ := args["mobile"].(bool)
				width, _ := args["width"].(float64)
				return jsonBody("/v1/browser/screenshot", map[string]any{"url": arg(args, "url"), "fullPage": full, "mobile": mobile, "width": int(width)})
			},
		},
		mcpTool{
			Name:        "web_page_to_pdf",
			Description: "A public web page, or HTML you give, as a PDF from a real Chromium (A4 by default). Good for invoices and reports written in HTML and CSS.",
			InputSchema: schema(nil, map[string]any{
				"url":   text("The page, http or https."),
				"html":  text("HTML to print instead of a URL."),
				"paper": oneOf("Paper size.", "a4", "a3", "a5", "letter", "legal"),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				return jsonBody("/v1/browser/pdf", map[string]any{"url": arg(args, "url"), "html": arg(args, "html"), "paper": arg(args, "paper")})
			},
		},
		mcpTool{
			Name:        "read_web_page",
			Description: "The text of one public web page after its scripts ran, as plain text or Markdown, from a real Chromium that honours robots.txt. Reads only the page given; follows no links.",
			InputSchema: schema([]string{"url"}, map[string]any{
				"url":      text("The page, http or https."),
				"markdown": map[string]any{"type": "boolean", "description": "Markdown with headings, lists, links and tables, rather than plain text."},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				markdown, _ := args["markdown"].(bool)
				return jsonBody("/v1/browser/text", map[string]any{"url": arg(args, "url"), "markdown": markdown})
			},
		},
		mcpTool{
			Name:        "create_request_bin",
			Description: "Make a request bin: a URL that records every request sent to it for 24 hours (the last 100), for testing webhooks. Returns its url, id and token; read it with read_request_bin.",
			InputSchema: schema(nil, map[string]any{}),
			send:        func(map[string]any) (toolRequest, error) { return jsonBody("/v1/hooks", map[string]any{}) },
		},
		mcpTool{
			Name:        "read_request_bin",
			Description: "What a request bin received, newest first: method, path, query, headers and body of each request.",
			InputSchema: schema([]string{"id", "token"}, map[string]any{
				"id":    text("The bin's id."),
				"token": text("The bin's token, from create_request_bin."),
			}),
			path: func(args map[string]any) (string, error) {
				if arg(args, "id") == "" || arg(args, "token") == "" {
					return "", errors.New("id and token are required.")
				}
				return "/v1/hooks/" + url.PathEscape(arg(args, "id")) + "?token=" + url.QueryEscape(arg(args, "token")), nil
			},
		},
		mcpTool{
			Name:        "create_postgres_database",
			Description: "Make a throwaway Postgres 17 database for an hour (100 MB): returns its name, user, password and how to connect (a WebSocket for serverless drivers, or psql through the client's db bridge). Needs a sandbox account.",
			InputSchema: schema(nil, map[string]any{"sandbox_token": token}),
			send:        func(map[string]any) (toolRequest, error) { return jsonBody("/v1/db", map[string]any{}) },
		},
	)
}
