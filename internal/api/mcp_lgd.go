package api

import "net/url"

// MCP tool for the Local Government Directory.

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "find_villages",
		Description: "Find Indian villages with their LGD codes, sub-district (taluk), district and state: by the start of a name, by PIN code, or all villages of a sub-district.",
		InputSchema: schema(nil, map[string]any{
			"q": text("The start of a village's name."), "pincode": text("Six digits."),
			"subdistrict": map[string]any{"type": "integer", "description": "A sub-district's LGD code."},
			"state":       map[string]any{"type": "integer", "description": "With q: a state's LGD code, such as 33 for Tamil Nadu."},
		}),
		send: func(args map[string]any) (toolRequest, error) {
			q := url.Values{}
			for _, k := range []string{"q", "pincode", "subdistrict", "state"} {
				if v := arg(args, k); v != "" {
					q.Set(k, v)
				}
			}
			return toolRequest{path: "/v1/lgd/villages?" + q.Encode()}, nil
		},
	})
}
