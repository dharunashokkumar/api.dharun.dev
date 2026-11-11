package api

import "net/url"

// MCP tool for the new criminal laws.

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "find_law_section",
		Description: "Map a section between India's old and new criminal laws: IPC to BNS (and back), CrPC to BNSS, Indian Evidence Act to BSA. Give law and section (IPC 302 gives BNS 103), or law and q to search section titles.",
		InputSchema: schema([]string{"law"}, map[string]any{
			"law":     oneOf("The law the section belongs to.", "bns", "bnss", "bsa", "ipc", "crpc", "iea"),
			"section": text("Such as 302, 498A or 103(1)."), "q": text("Words of a section title, such as cheating."),
		}),
		send: func(args map[string]any) (toolRequest, error) {
			law, err := required(args, "law")
			if err != nil {
				return toolRequest{}, err
			}
			if section := arg(args, "section"); section != "" {
				return toolRequest{path: "/v1/law/" + url.PathEscape(law) + "/" + url.PathEscape(section)}, nil
			}
			return toolRequest{path: "/v1/law/" + url.PathEscape(law) + "?q=" + url.QueryEscape(arg(args, "q"))}, nil
		},
	})
}
