package api

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// MCP tools for growth plan Part 4: the calculators, today in India, batch lookups and
// charts. Each one runs its REST endpoint, as every tool does.

func init() {
	changingTools["today_in_india"] = true
	mcpTools = append(mcpTools,
		mcpTool{
			Name: "calculate_indian_money",
			Description: "Indian money calculators with the rules written in: sip (monthly, rate, years, stepup), lumpsum (amount, rate, years), emi (amount, rate, years or months, schedule), " +
				"gst (amount, rate, inclusive, interstate), income-tax (income, year, regime new/old/both, age, salaried, deductions), hra (basic, hra, rent, metro), gratuity (salary, years, covered), " +
				"ppf (yearly, years, rate), capital-gains (asset, bought, sold, cost, price, expenses) and trade (buy, sell, quantity, kind, brokerage, dp). Rates are percent a year, amounts rupees. Each answer names the rule and year used.",
			InputSchema: schema([]string{"kind", "inputs"}, map[string]any{
				"kind":   oneOf("Which calculator.", calcKinds...),
				"inputs": map[string]any{"type": "object", "description": "The calculator's inputs by name, as listed for its kind, for example {\"income\": 1500000, \"regime\": \"both\"}.", "additionalProperties": true},
			}),
			path: func(args map[string]any) (string, error) {
				kind, err := required(args, "kind")
				if err != nil {
					return "", err
				}
				inputs, _ := args["inputs"].(map[string]any)
				values := url.Values{}
				for name, value := range inputs {
					switch v := value.(type) {
					case bool:
						if v {
							values.Set(name, "true")
						} else {
							values.Set(name, "false")
						}
					default:
						values.Set(name, arg(inputs, name))
					}
				}
				address := "/v1/calc/" + url.PathEscape(kind)
				if encoded := values.Encode(); encoded != "" {
					address += "?" + encoded
				}
				return address, nil
			},
		},
		mcpTool{
			Name:        "today_in_india",
			Description: "India's \"now\": the date and time in IST, financial year, quarter and assessment year, whether today is a bank (for a state), NSE market and central government working day, today's holidays, whether the market is open, and with a pincode the sunrise and the day's panchang in brief.",
			InputSchema: schema(nil, map[string]any{
				"state":   text("State code or name, for bank holidays, such as TN."),
				"pincode": text("Six digits, for sunrise and the panchang."),
				"date":    text("Another day, YYYY-MM-DD, instead of today."),
			}),
			path: func(args map[string]any) (string, error) {
				return "/v1/today" + query(args, map[string]string{"state": "state", "pincode": "pincode", "date": "date"}), nil
			},
		},
		mcpTool{
			Name:        "batch_lookup",
			Description: "Run up to 1,000 lookups of one kind in one call: " + strings.Join(batchKindNames(), ", ") + ". Each answer is what the single lookup returns, with its status, in order. Use instead of many single calls.",
			InputSchema: schema([]string{"kind", "values"}, map[string]any{
				"kind":   oneOf("What to look up.", batchKindNames()...),
				"values": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Up to 1,000 values."},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				kind, err := required(args, "kind")
				values, ok := args["values"].([]any)
				if err != nil || !ok || len(values) == 0 {
					return toolRequest{}, errors.New("kind and values (a list) are required.")
				}
				encoded, _ := json.Marshal(map[string]any{"kind": kind, "values": values})
				return toolRequest{path: "/v1/batch", contentType: "application/json", body: encoded}, nil
			},
		},
		mcpTool{
			Name:        "make_chart_image",
			Description: "Draw a bar, line, pie or sparkline chart as a PNG picture from numbers, for an email, chat message or README where a JavaScript chart cannot run. Axis values are written the Indian way (K, L, Cr).",
			InputSchema: schema([]string{"series"}, map[string]any{
				"type":   oneOf("Default bar.", "bar", "line", "pie", "sparkline"),
				"title":  text("Title above the chart."),
				"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "A label per value."},
				"series": map[string]any{"type": "array", "description": "1 to 8 series.", "items": map[string]any{"type": "object", "properties": map[string]any{
					"name": text("Series name."), "values": map[string]any{"type": "array", "items": map[string]any{"type": "number"}}, "color": text("Hex colour, optional.")}}},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				if _, ok := args["series"].([]any); !ok {
					return toolRequest{}, errors.New("series is required: a list of {name, values}.")
				}
				body := map[string]any{"type": arg(args, "type"), "title": arg(args, "title"), "labels": args["labels"], "series": args["series"], "format": "png"}
				encoded, _ := json.Marshal(body)
				return toolRequest{path: "/v1/chart?format=png", contentType: "application/json", body: encoded}, nil
			},
		},
	)
}
