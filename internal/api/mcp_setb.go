package api

import "net/url"

// MCP tools for set B.

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "list_index_constituents",
		Description: "Members of an NSE index (Nifty 50, Nifty Bank, Nifty IT, Nifty 500 and 27 more) with industry and ISIN; or, with symbol, the indices a stock belongs to.",
		InputSchema: schema(nil, map[string]any{"index": text("Index id, such as nifty-50 or nifty-bank. Leave out to list the indices."), "symbol": text("An NSE symbol: the indices it is in.")}),
		path: func(args map[string]any) (string, error) {
			if index := arg(args, "index"); index != "" {
				return "/v1/indices/" + url.PathEscape(index), nil
			}
			return "/v1/indices" + query(args, map[string]string{"symbol": "symbol"}), nil
		},
	})
}
