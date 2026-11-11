package api

import "net/url"

// MCP tool for hospitals.

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "find_hospitals",
		Description: "Find hospitals in India near a point (latitude,longitude), by PIN code, by district or by name, with phone numbers and specialties. From the National Hospital Directory of May 2016: tell the user to call before going; for an emergency, 112 or 108.",
		InputSchema: schema(nil, map[string]any{
			"near": text("latitude,longitude."), "pincode": text("Six digits."), "district": text("A district."), "state": text("A state, with district or q."), "q": text("Part of a hospital's name."),
		}),
		send: func(args map[string]any) (toolRequest, error) {
			q := url.Values{}
			for _, k := range []string{"near", "pincode", "district", "state", "q"} {
				if v := arg(args, k); v != "" {
					q.Set(k, v)
				}
			}
			return toolRequest{path: "/v1/hospitals?" + q.Encode()}, nil
		},
	})
}
