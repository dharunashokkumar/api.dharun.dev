package api

import (
	"errors"
	"net/url"
)

// MCP tools for Census 2011, STD codes, card prefixes and the classics.

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "census_2011",
			Description: "Census of India 2011 figures (population, households, literacy rate, sex ratio, SC/ST, workers; total, rural and urban) for India, a state, district, sub-district or town. Give q to find a place by name, or level and code for its figures and the places inside it.",
			InputSchema: schema(nil, map[string]any{
				"q":     text("A place's name, such as Coimbatore."),
				"level": oneOf("With code: which kind of place.", "india", "state", "district", "subdistrict", "town"),
				"code":  text("The 2011 census code from a search (33 is Tamil Nadu; India is 0)."),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				if code := arg(args, "code"); code != "" {
					level := arg(args, "level")
					if level == "" {
						return toolRequest{}, errors.New("level is needed with code")
					}
					return toolRequest{path: "/v1/census/" + url.PathEscape(level) + "/" + url.PathEscape(code)}, nil
				}
				path := "/v1/census?q=" + url.QueryEscape(arg(args, "q"))
				if level := arg(args, "level"); level != "" {
					path += "&level=" + url.QueryEscape(level)
				}
				return toolRequest{path: path}, nil
			},
		},
		mcpTool{
			Name:        "find_std_code",
			Description: "Indian landline STD codes: the area of a code or landline number (0422 gives Coimbatore), or the code of a town by name.",
			InputSchema: schema(nil, map[string]any{
				"code": text("An STD code or landline number, such as 0422 or 044 2345 6789."),
				"q":    text("A town's name, such as Pollachi."),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				if code := arg(args, "code"); code != "" {
					return toolRequest{path: "/v1/std-codes/" + url.PathEscape(code)}, nil
				}
				return toolRequest{path: "/v1/std-codes?q=" + url.QueryEscape(arg(args, "q"))}, nil
			},
		},
		mcpTool{
			Name:        "card_bin_lookup",
			Description: "The card network (Visa, Mastercard, RuPay...), debit or credit, product and issuing bank of a payment card's first 6 to 8 digits. Never pass a whole card number.",
			InputSchema: schema([]string{"prefix"}, map[string]any{"prefix": text("The first 6 to 8 digits of the card.")}),
			send: func(args map[string]any) (toolRequest, error) {
				prefix, err := required(args, "prefix")
				if err != nil {
					return toolRequest{}, err
				}
				return toolRequest{path: "/v1/bin/" + url.PathEscape(prefix)}, nil
			},
		},
		mcpTool{
			Name:        "indian_classics",
			Description: "Verses of public domain Indian classics with transliteration: the Bhagavad Gita in Sanskrit (ref 2.47), Avvaiyar's Aathichoodi in Tamil (ref 1 to 109), Kabir's couplets in Hindi (ref 1.3); ref random for a random one, or a chapter number for all of it.",
			InputSchema: schema([]string{"text", "ref"}, map[string]any{
				"text": oneOf("Which text.", "gita", "aathichoodi", "kabir"),
				"ref":  text("2.47, 2, random, or a line number for Aathichoodi."),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				textID, err := required(args, "text")
				if err != nil {
					return toolRequest{}, err
				}
				ref, err := required(args, "ref")
				if err != nil {
					return toolRequest{}, err
				}
				return toolRequest{path: "/v1/classics/" + url.PathEscape(textID) + "/" + url.PathEscape(ref)}, nil
			},
		},
	)
}
