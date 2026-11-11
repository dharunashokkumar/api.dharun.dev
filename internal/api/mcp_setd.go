package api

import (
	"encoding/json"
	"net/url"
)

// MCP tools for set D, the developer utilities.

func init() {
	for _, name := range []string{"convert_data_format", "compare_texts", "decode_jwt", "hash_text", "preview_link", "read_web_page"} {
		privateTools[name] = true
	}
	openWorldTools["preview_link"] = true
	openWorldTools["read_web_page"] = true
	changingTools["make_ids"] = true
	mcpTools = append(mcpTools,
		textTool("convert_data_format", "Convert a document between JSON, YAML, TOML, CSV and XML. The input format is guessed when from is not given.",
			"/v1/convert", "text", "The document.",
			map[string]any{"from": oneOf("Input format.", "json", "yaml", "toml", "csv", "xml"), "to": oneOf("Output format. Default json.", "json", "yaml", "toml", "csv", "xml")},
			map[string]string{"from": "from", "to": "to"}),
		mcpTool{
			Name:        "compare_texts",
			Description: "Compare two texts line by line (a unified patch with counts), or two JSON documents (a list of JSON Patch changes with old values).",
			InputSchema: schema([]string{"a", "b"}, map[string]any{
				"a": text("The first text or JSON document."), "b": text("The second."),
				"mode": oneOf("text or json. Default: json when both parse as JSON objects or arrays, else text.", "text", "json"),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				a, err := required(args, "a")
				if err != nil {
					return toolRequest{}, err
				}
				b, err := required(args, "b")
				if err != nil {
					return toolRequest{}, err
				}
				mode := arg(args, "mode")
				if mode == "" {
					var x, y any
					if json.Unmarshal([]byte(a), &x) == nil && json.Unmarshal([]byte(b), &y) == nil {
						_, objA := x.(map[string]any)
						_, listA := x.([]any)
						if objA || listA {
							mode = "json"
						}
					}
				}
				if mode == "" {
					mode = "text"
				}
				body, _ := json.Marshal(map[string]any{"a": a, "b": b, "mode": mode})
				return toolRequest{path: "/v1/diff", contentType: "application/json", body: body}, nil
			},
		},
		mcpTool{
			Name:        "explain_cron",
			Description: "Explain a five-field cron expression in words and list its next run times (India time by default, or a named zone).",
			InputSchema: schema([]string{"expression"}, map[string]any{
				"expression": text("Such as 30 9 * * 1-5, or @daily."), "timezone": text("IANA zone, short name or offset. Default Asia/Kolkata."),
				"count": map[string]any{"type": "integer", "description": "Next runs, 1 to 50. Default 5."},
			}),
			path: func(args map[string]any) (string, error) {
				expr, err := required(args, "expression")
				return "/v1/cron?expr=" + url.QueryEscape(expr) + func() string {
					q := query(args, map[string]string{"timezone": "tz", "count": "count"})
					if q == "" {
						return ""
					}
					return "&" + q[1:]
				}(), err
			},
		},
		mcpTool{
			Name:        "convert_time_zone",
			Description: "Show a time in other time zones, with offsets and the day difference: for example 3 pm IST in Berlin, New York and Singapore. Reads 2026-11-11 15:00, 3pm, 15:30 or a Unix time.",
			InputSchema: schema([]string{"to"}, map[string]any{
				"at": text("The time. Default now."), "from": text("Its zone. Default India (IST)."), "to": text("Zones to show it in, separated by commas: IANA names, short names (PST, CET), cities or offsets."),
			}),
			path: func(args map[string]any) (string, error) {
				return "/v1/time" + query(args, map[string]string{"at": "at", "from": "from", "to": "to"}), nil
			},
		},
		mcpTool{
			Name:        "make_ids",
			Description: "Make new UUIDs (version 4 or 7), ULIDs or nano ids.",
			InputSchema: schema(nil, map[string]any{"kind": oneOf("Default uuid4.", "uuid4", "uuid7", "ulid", "nanoid"), "count": map[string]any{"type": "integer", "description": "1 to 100. Default 1."}}),
			path: func(args map[string]any) (string, error) {
				return "/v1/uuid" + query(args, map[string]string{"kind": "kind", "count": "count"}), nil
			},
		},
		textTool("decode_jwt", "Decode a JSON Web Token's header and payload and say when it was issued and expires. The signature is not checked.",
			"/v1/jwt/decode", "token", "The token.", nil, nil),
		textTool("hash_text", "Hash a text with MD5, SHA-1, SHA-2, SHA-3, BLAKE2b or CRC-32, in hex and base64; with key, an HMAC.",
			"/v1/hash", "text", "The text.", map[string]any{"algo": text("Comma-separated algorithms. Default md5,sha1,sha256,sha512."), "key": text("HMAC key.")},
			map[string]string{"algo": "algo", "key": "key"}),
		mcpTool{
			Name:        "check_color_contrast",
			Description: "WCAG contrast ratio between a text colour and its background, which levels it passes (AA, AAA), and a nearby text colour that passes AA.",
			InputSchema: schema([]string{"fg", "bg"}, map[string]any{"fg": text("Text colour: hex, rgb(), hsl() or a name."), "bg": text("Background colour.")}),
			path: func(args map[string]any) (string, error) {
				return "/v1/color/contrast" + query(args, map[string]string{"fg": "fg", "bg": "bg"}), nil
			},
		},
		mcpTool{
			Name:        "preview_link",
			Description: "Title, description, image, icon and site name of a web page, as a chat app shows it (Open Graph and Twitter tags), fetched from India.",
			InputSchema: schema([]string{"url"}, map[string]any{"url": text("An http or https address.")}),
			path: func(args map[string]any) (string, error) {
				u, err := required(args, "url")
				return "/v1/unfurl?url=" + url.QueryEscape(u), err
			},
		},
		mcpTool{
			Name:        "read_web_page",
			Description: "Fetch one web page and return its main content as clean Markdown, with its title. Respects robots.txt. Needs a key once keys are required.",
			InputSchema: schema([]string{"url"}, map[string]any{"url": text("The page's address."), "full": map[string]any{"type": "boolean", "description": "Keep navigation, headers and footers."}}),
			path: func(args map[string]any) (string, error) {
				u, err := required(args, "url")
				address := "/v1/read?url=" + url.QueryEscape(u)
				if full, _ := args["full"].(bool); full {
					address += "&full=true"
				}
				return address, err
			},
		},
	)
}
