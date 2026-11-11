package api

import "encoding/json"

// MCP tool for running code.

func init() {
	privateTools["run_code"] = true
	changingTools["run_code"] = true
	mcpTools = append(mcpTools, mcpTool{
		Name:        "run_code",
		Description: "Run a short Python or JavaScript program once in a throwaway microVM with no network (30 seconds, 256 MB) and get its output, for a calculation or to check code. Needs an API key in the client configuration.",
		InputSchema: schema([]string{"language", "code"}, map[string]any{
			"language": oneOf("The language.", "python", "javascript"), "code": text("The program."), "stdin": text("Input for the program."),
		}),
		send: func(args map[string]any) (toolRequest, error) {
			body, _ := json.Marshal(map[string]string{"language": arg(args, "language"), "code": arg(args, "code"), "stdin": arg(args, "stdin")})
			return toolRequest{path: "/v1/run", contentType: "application/json", body: body}, nil
		},
	})
}
