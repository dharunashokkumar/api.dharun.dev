package api

import (
	"encoding/json"
	"net/url"
)

// MCP tools for set E: an agent's notes. Document search needs several calls with a
// secret, which suits code better than a tool.

func init() {
	privateTools["agent_notes"] = true
	changingTools["agent_notes"] = true
	mcpTools = append(mcpTools, mcpTool{
		Name:        "agent_notes",
		Description: "Read and write notes that persist between conversations, in a private store made with POST /v1/kv (id and secret). Give key and value to write, key alone to read, neither to list.",
		InputSchema: schema([]string{"store", "secret"}, map[string]any{
			"store": text("The store's id."), "secret": text("The store's secret."), "key": text("The note's name."), "value": text("Text to write (JSON is fine)."),
		}),
		send: func(args map[string]any) (toolRequest, error) {
			store, err := required(args, "store")
			if err != nil {
				return toolRequest{}, err
			}
			secret, err := required(args, "secret")
			if err != nil {
				return toolRequest{}, err
			}
			address := "/v1/kv/" + url.PathEscape(store)
			if key := arg(args, "key"); key != "" {
				address += "/" + url.PathEscape(key)
			}
			address += "?secret=" + url.QueryEscape(secret)
			if value, ok := args["value"]; ok && arg(args, "key") != "" {
				body, isText := value.(string)
				if !isText {
					encoded, _ := json.Marshal(value)
					body = string(encoded)
				}
				return toolRequest{path: address, contentType: "text/plain", body: []byte(body)}, nil
			}
			return toolRequest{path: address}, nil
		},
	})
}
