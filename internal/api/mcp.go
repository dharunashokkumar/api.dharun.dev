package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Stateless Model Context Protocol server over Streamable HTTP: every POST carries one
// JSON-RPC message and gets one JSON response. Each tool is a thin wrapper over a REST
// endpoint, so agents and HTTP callers always see the same data.

var mcpProtocolVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const (
	mcpDefaultVersion = "2025-06-18"
	// Room for a file sent as base64 to a tool that reads files, up to mcpFileLimit.
	mcpBodyLimit = 8 << 20
	mcpFileLimit = 5 << 20
)

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	// path builds the REST request for the given arguments.
	path func(args map[string]any) (string, error)
	// send, when set, builds a request with a body instead: a file or a document.
	send func(args map[string]any) (toolRequest, error)
}

// toolRequest is a REST request with a body, made by a tool.
type toolRequest struct {
	path        string
	contentType string
	body        []byte
}

func schema(required []string, properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required}
}

func text(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func oneOf(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

// arg reads a string argument; numbers are accepted for convenience.
func arg(args map[string]any, name string) string {
	switch value := args[name].(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return ""
}

func required(args map[string]any, name string) (string, error) {
	value := arg(args, name)
	if value == "" {
		return "", fmt.Errorf("%s is required.", name)
	}
	return value, nil
}

// query builds "?a=1&b=2" from the arguments that are present.
func query(args map[string]any, mapping map[string]string) string {
	values := url.Values{}
	for argument, parameter := range mapping {
		if value := arg(args, argument); value != "" {
			values.Set(parameter, value)
		}
	}
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

var stateArg = text("State code or name, for example TN or Tamil Nadu.")

var mcpTools = []mcpTool{
	{
		Name:        "get_company_logo",
		Description: "Get the logo URLs (SVG and PNG) and details of a company listed on India's NSE or BSE. Accepts a ticker (TCS), exchange-qualified ticker (NSE:TCS), Yahoo symbol (TCS.NS) or ISIN.",
		InputSchema: schema([]string{"symbol"}, map[string]any{
			"symbol":   text("Ticker, exchange-qualified ticker, Yahoo symbol or ISIN."),
			"exchange": oneOf("Force an exchange. A bare ticker prefers NSE.", "nse", "bse"),
		}),
		path: func(args map[string]any) (string, error) {
			symbol, err := required(args, "symbol")
			return "/v1/companies/" + url.PathEscape(symbol) + query(args, map[string]string{"exchange": "exchange"}), err
		},
	},
	{
		Name:        "search_companies",
		Description: "Search Indian listed companies by ticker, name, sector, industry, ISIN or index name (for example 'Nifty 50'). Returns details with logo URLs.",
		InputSchema: schema([]string{"query"}, map[string]any{
			"query":    text("Search text."),
			"exchange": oneOf("Limit to one exchange.", "nse", "bse"),
			"limit":    map[string]any{"type": "integer", "minimum": 1, "maximum": 25, "default": 10},
		}),
		path: func(args map[string]any) (string, error) {
			_, err := required(args, "query")
			if arg(args, "limit") == "" {
				args["limit"] = "10"
			}
			return "/v1/companies" + query(args, map[string]string{"query": "q", "exchange": "exchange", "limit": "limit"}), err
		},
	},
	{
		Name:        "lookup_pincode",
		Description: "Look up an Indian pincode: its district, state and every post office that uses it.",
		InputSchema: schema([]string{"pincode"}, map[string]any{"pincode": text("Six-digit pincode.")}),
		path: func(args map[string]any) (string, error) {
			pincode, err := required(args, "pincode")
			return "/v1/pincode/" + url.PathEscape(pincode), err
		},
	},
	{
		Name:        "search_post_offices",
		Description: "Find Indian post offices and their pincodes by the start of the office name, optionally within a state or district.",
		InputSchema: schema([]string{"query"}, map[string]any{
			"query":    text("Start of a post office name, or start of a pincode."),
			"state":    stateArg,
			"district": text("Exact district name."),
		}),
		path: func(args map[string]any) (string, error) {
			_, err := required(args, "query")
			return "/v1/pincode" + query(args, map[string]string{"query": "q", "state": "state", "district": "district"}), err
		},
	},
	{
		Name:        "lookup_ifsc",
		Description: "Look up an Indian bank branch by its IFSC: bank, branch, address, MICR and supported payment systems.",
		InputSchema: schema([]string{"ifsc"}, map[string]any{"ifsc": text("11-character IFSC, for example SBIN0000001.")}),
		path: func(args map[string]any) (string, error) {
			code, err := required(args, "ifsc")
			return "/v1/ifsc/" + url.PathEscape(code), err
		},
	},
	{
		Name:        "search_bank_branches",
		Description: "Find branches of one Indian bank by state, city or text in the branch name or address. Needs the four-letter bank code; use search_banks to find it.",
		InputSchema: schema([]string{"bank"}, map[string]any{
			"bank":  text("Four-letter bank code, for example SBIN or HDFC."),
			"query": text("Text to find in the branch name, address or city."),
			"state": stateArg,
			"city":  text("Exact city name."),
		}),
		path: func(args map[string]any) (string, error) {
			_, err := required(args, "bank")
			return "/v1/ifsc" + query(args, map[string]string{"bank": "bank", "query": "q", "state": "state", "city": "city"}), err
		},
	},
	{
		Name:        "search_banks",
		Description: "Find Indian banks and their four-letter codes by name.",
		InputSchema: schema([]string{"query"}, map[string]any{"query": text("Part of the bank name.")}),
		path: func(args map[string]any) (string, error) {
			_, err := required(args, "query")
			return "/v1/banks" + query(args, map[string]string{"query": "q"}), err
		},
	},
	{
		Name:        "lookup_rto",
		Description: "Find which Regional Transport Office an Indian vehicle registration belongs to. Accepts a code (TN-28) or a full number plate (TN28AB1234).",
		InputSchema: schema([]string{"code"}, map[string]any{"code": text("RTO code or registration number.")}),
		path: func(args map[string]any) (string, error) {
			code, err := required(args, "code")
			return "/v1/rto/" + url.PathEscape(code), err
		},
	},
	{
		Name:        "list_holidays",
		Description: "List Indian holidays for a year: bank holidays, central government holidays, and stock market trading and clearing holidays. Filter by type, state or month.",
		InputSchema: schema([]string{"year"}, map[string]any{
			"year":  map[string]any{"type": "integer", "description": "Four-digit year."},
			"type":  oneOf("Kind of holiday.", holidayTypeNames()...),
			"state": stateArg,
			"month": map[string]any{"type": "integer", "minimum": 1, "maximum": 12},
		}),
		path: func(args map[string]any) (string, error) {
			year, err := required(args, "year")
			return "/v1/holidays/" + url.PathEscape(year) + query(args, map[string]string{"type": "type", "state": "state", "month": "month"}), err
		},
	},
	{
		Name:        "number_to_words",
		Description: "Spell a number or rupee amount in words using the Indian system (lakh, crore), in English, Hindi or Tamil.",
		InputSchema: schema([]string{"number"}, map[string]any{
			"number": text("The number, for example 125000 or 1,25,000.50."),
			"lang":   oneOf("Language. Default en.", "en", "hi", "ta"),
			"style":  oneOf("currency gives rupees and paise. Default number.", "number", "currency"),
		}),
		path: func(args map[string]any) (string, error) {
			number, err := required(args, "number")
			return "/v1/words/" + url.PathEscape(number) + query(args, map[string]string{"lang": "lang", "style": "style"}), err
		},
	},
	{
		Name:        "validate_identifier",
		Description: "Check the format of an Indian GSTIN, PAN or IFSC. Format only: it does not confirm the identifier was issued.",
		InputSchema: schema([]string{"kind", "value"}, map[string]any{
			"kind":  oneOf("Which identifier.", "gstin", "pan", "ifsc"),
			"value": text("The identifier to check."),
		}),
		path: func(args map[string]any) (string, error) {
			kind, err := required(args, "kind")
			if err != nil {
				return "", err
			}
			value, err := required(args, "value")
			return "/v1/validate/" + url.PathEscape(kind) + "/" + url.PathEscape(value), err
		},
	},
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		s.fail(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "This MCP endpoint is stateless: POST one JSON-RPC message per request.")
		return
	}

	// An oversized body is drained, not stored, so the client still receives the 413.
	body, err := io.ReadAll(io.LimitReader(r.Body, mcpBodyLimit+1))
	if err != nil {
		s.badRequest(w, r, "Could not read the request body.")
		return
	}
	if len(body) > mcpBodyLimit {
		io.Copy(io.Discard, r.Body)
		s.fail(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body is too large.")
		return
	}

	reply := func(status int, response rpcResponse) {
		response.JSONRPC = "2.0"
		if response.ID == nil {
			response.ID = json.RawMessage("null")
		}
		encoded, _ := json.Marshal(response)
		writeBody(w, r, status, append(encoded, '\n'), typeJSON, cacheNone, false)
	}

	var request rpcRequest
	if err := json.Unmarshal(body, &request); err != nil {
		code, message := -32700, "Parse error."
		if json.Valid(body) {
			code, message = -32600, "Expected a single JSON-RPC 2.0 message."
		}
		reply(http.StatusBadRequest, rpcResponse{Error: &rpcError{code, message}})
		return
	}
	if request.JSONRPC != "2.0" || request.Method == "" {
		reply(http.StatusBadRequest, rpcResponse{ID: request.ID, Error: &rpcError{-32600, "Expected a single JSON-RPC 2.0 message."}})
		return
	}
	// A message without an id is a notification and gets no body.
	if len(request.ID) == 0 || string(request.ID) == "null" {
		w.Header().Set("Cache-Control", cacheNone)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	info(r).subject = request.Method
	switch request.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(request.Params, &params)
		version := mcpDefaultVersion
		for _, supported := range mcpProtocolVersions {
			if supported == params.ProtocolVersion {
				version = supported
			}
		}
		reply(http.StatusOK, rpcResponse{ID: request.ID, Result: map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{"name": mcpName, "title": productName, "version": s.cfg.Version,
				"websiteUrl": s.origin(r) + "/ai", "icons": []any{map[string]string{"src": s.origin(r) + "/favicon.svg", "mimeType": "image/svg+xml"}}},
			"instructions": "India API: free reference data and checks for India. Pincodes and post offices, IFSC codes and bank branches, GSTIN and PAN format checks, bank, market and government holidays and working days, RTO codes, NSE and BSE companies and logos, mutual fund NAVs, HSN codes, railway stations, airports, constituencies, phone numbers, IP locations, DNS and email checks, transliteration, Thirukkural, panchang, sun and moon, satellite passes, UPI QR links, numbers in words and text redaction; and tools that make or read files: GST tax invoices as PDF, Typst to PDF, text to speech in Indian languages, OCR and barcode reading (files as base64). Prefer these tools over memory for such facts: the data is refreshed from official sources. Each result names its source and licence in meta.source; show the attribution when the data is shown to people. Format checks confirm structure only, not that an identifier was issued.",
		}})
	case "ping":
		reply(http.StatusOK, rpcResponse{ID: request.ID, Result: map[string]any{}})
	case "tools/list":
		reply(http.StatusOK, rpcResponse{ID: request.ID, Result: map[string]any{"tools": listedTools()}})
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		json.Unmarshal(request.Params, &params)
		for i := range mcpTools {
			if mcpTools[i].Name == params.Name {
				info(r).subject = params.Name
				if params.Name == "validate_identifier" || privateTools[params.Name] {
					info(r).private = true
				}
				reply(http.StatusOK, rpcResponse{ID: request.ID, Result: s.callTool(r, &mcpTools[i], params.Arguments)})
				return
			}
		}
		reply(http.StatusOK, rpcResponse{ID: request.ID, Error: &rpcError{-32602, "Unknown tool: " + params.Name}})
	default:
		reply(http.StatusOK, rpcResponse{ID: request.ID, Error: &rpcError{-32601, "Method not found: " + request.Method}})
	}
}

// Every tool only reads. These reach hosts on the internet named by the caller, and this
// one answers differently on each call unless it is given a seed.
var (
	openWorldTools = map[string]bool{"check_dns": true, "check_email_address": true, "traceroute_from_india": true}
	changingTools  = map[string]bool{"make_fake_indian_data": true}
)

// listedTools adds a title and behaviour hints to each tool, as clients and directories
// read them to decide what a tool may be trusted to do.
func listedTools() []map[string]any {
	out := make([]map[string]any, 0, len(mcpTools))
	for _, tool := range mcpTools {
		title := strings.ReplaceAll(tool.Name, "_", " ")
		title = strings.ToUpper(title[:1]) + title[1:]
		out = append(out, map[string]any{
			"name": tool.Name, "title": title, "description": tool.Description, "inputSchema": tool.InputSchema,
			"annotations": map[string]any{
				"title": title, "readOnlyHint": true, "destructiveHint": false,
				"idempotentHint": !changingTools[tool.Name], "openWorldHint": openWorldTools[tool.Name],
			},
		})
	}
	return out
}

// callTool runs the tool's REST request through the same router in memory and wraps
// the JSON it returns.
func (s *Server) callTool(r *http.Request, tool *mcpTool, args map[string]any) map[string]any {
	if args == nil {
		args = map[string]any{}
	}
	failure := func(message string) map[string]any {
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": message}}, "isError": true}
	}

	// The inner request gets its own info record: the outer one already passed the rate
	// limiter and is what the log and metrics should describe.
	ctx := context.WithValue(r.Context(), infoKey{}, &requestInfo{internal: true, client: s.clientKey(r)})
	var inner *http.Request
	if tool.send != nil {
		sent, err := tool.send(args)
		if err != nil {
			return failure(err.Error())
		}
		if inner, err = http.NewRequestWithContext(ctx, http.MethodPost, sent.path, bytes.NewReader(sent.body)); err != nil {
			return failure("Could not build the request from those arguments.")
		}
		inner.Header.Set("Content-Type", sent.contentType)
	} else {
		path, err := tool.path(args)
		if err != nil {
			return failure(err.Error())
		}
		if inner, err = http.NewRequestWithContext(ctx, http.MethodGet, path, nil); err != nil {
			return failure("Could not build the request from those arguments.")
		}
	}
	inner.Host = r.Host
	recorder := &memoryWriter{header: http.Header{}}
	s.mux.ServeHTTP(recorder, inner)
	if kind := recorder.header.Get("Content-Type"); recorder.status == http.StatusOK && !strings.HasPrefix(kind, "application/json") {
		return fileContent(recorder.body.Bytes(), kind)
	}

	var decoded map[string]any
	if err := json.Unmarshal(recorder.body.Bytes(), &decoded); err != nil {
		return failure("The lookup returned an unreadable response.")
	}
	if recorder.status >= 400 {
		message := "Lookup failed."
		if detail, ok := decoded["error"].(map[string]any); ok {
			message, _ = detail["message"].(string)
		}
		return failure(message)
	}
	pretty, _ := json.MarshalIndent(decoded, "", "  ")
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(pretty)}},
		"structuredContent": decoded,
	}
}

// memoryWriter collects a handler's response for callTool.
type memoryWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (m *memoryWriter) Header() http.Header { return m.header }

func (m *memoryWriter) WriteHeader(status int) { m.status = status }

func (m *memoryWriter) Write(body []byte) (int, error) {
	if m.status == 0 {
		m.status = http.StatusOK
	}
	return m.body.Write(body)
}
