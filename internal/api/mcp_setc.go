package api

import (
	"net/url"
	"strconv"
)

// MCP tools for set C.

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "list_indian_festivals",
			Description: "Dates of Indian festivals (Hindu, Tamil, Sikh, Jain, Buddhist, Islamic, Christian and national days) for a year from 1950 to 2100, computed from the panchang, each with the rule that fixes it. Islamic dates are marked expected (moon sighting).",
			InputSchema: schema([]string{"year"}, map[string]any{
				"year":  map[string]any{"type": "integer", "description": "The year."},
				"month": map[string]any{"type": "integer", "description": "Only this month, 1 to 12."},
				"kind":  oneOf("Only one kind.", festivalKinds...),
				"state": text("Leave out regional festivals not kept in this state, such as TN."),
			}),
			path: func(args map[string]any) (string, error) {
				year, err := required(args, "year")
				if err != nil {
					return "", err
				}
				if _, err := strconv.Atoi(year); err != nil {
					return "", err
				}
				return "/v1/festivals/" + url.PathEscape(year) + query(args, map[string]string{"month": "month", "kind": "kind", "state": "state"}), nil
			},
		},
		textTool("write_bharati_braille", "Write text in any Indian script as Bharati braille (Unicode braille cells, Grade 1).", "/v1/braille", "text", "The text.", nil, nil),
		textTool("convert_indian_keyboard", "Fix text typed with the wrong keyboard on: what QWERTY keys give on the InScript (Hindi) or Tamil99 layout; or, with direction keys, which keys type a text.",
			"/v1/keyboard/convert", "text", "The keys typed, or the text.",
			map[string]any{"layout": oneOf("Default inscript-hindi.", "inscript-hindi", "tamil99"), "direction": oneOf("type (keys to text, default) or keys.", "type", "keys")},
			map[string]string{"layout": "layout", "direction": "direction"}),
		textTool("count_indian_letters", "Split Indian-script text into aksharas (the letters a reader counts, conjuncts as one) and count them, with code points, words and bytes.",
			"/v1/indic/aksharas", "text", "The text.", nil, nil),
	)
}
