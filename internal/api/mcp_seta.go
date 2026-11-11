package api

import (
	"net/url"
)

// MCP tools for the rest of set A (seta.go).

func init() {
	privateTools["read_payment_qr_text"] = true
	privateTools["parse_indian_address"] = true
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "format_indian_number",
			Description: "Write a number the Indian way: lakh and crore grouping (12,34,567), international grouping, rupees with paise, short forms (1.25 Cr, 3.4 L), in English words, and with the digits of nine Indian scripts.",
			InputSchema: schema([]string{"number"}, map[string]any{
				"number":   text("The number, such as 123456789.5."),
				"decimals": map[string]any{"type": "integer", "description": "Round to this many decimals, 0 to 10."},
			}),
			path: func(args map[string]any) (string, error) {
				n, err := required(args, "number")
				return "/v1/format/number/" + url.PathEscape(n) + query(args, map[string]string{"decimals": "decimals"}), err
			},
		},
		mcpTool{
			Name:        "indian_date_info",
			Description: "For a date: the Indian financial year, quarter, half and assessment year, ISO week, the national (Saka) date, Tamil solar date, Vikram Samvat year and tabular Hijri date. With from (and optional to) instead, the span between two dates in years, months and days, for ages and tenures.",
			InputSchema: schema(nil, map[string]any{
				"date": text("A date, YYYY-MM-DD, or today."),
				"from": text("For a span or an age: the start date, YYYY-MM-DD."),
				"to":   text("For a span: the end date. Default today."),
			}),
			path: func(args map[string]any) (string, error) {
				if arg(args, "from") != "" {
					return "/v1/dates/diff" + query(args, map[string]string{"from": "from", "to": "to"}), nil
				}
				date := arg(args, "date")
				if date == "" {
					date = "today"
				}
				return "/v1/dates/" + url.PathEscape(date), nil
			},
		},
		mcpTool{
			Name:        "lookup_upi_handle",
			Description: "Which app and partner bank issued a UPI handle (the part after @ in a UPI ID), such as okhdfcbank (Google Pay, HDFC Bank) or ybl (PhonePe, Yes Bank).",
			InputSchema: schema([]string{"handle"}, map[string]any{"handle": text("The handle, or a whole UPI ID.")}),
			path: func(args map[string]any) (string, error) {
				h, err := required(args, "handle")
				return "/v1/upi-handles/" + url.PathEscape(h), err
			},
		},
		textTool("read_payment_qr_text", "Split the text inside a payment QR code (a upi://pay link, or an EMVCo or Bharat QR string) into payee, amount, merchant category, city and reference, and check its checksum.",
			"/v1/qr/parse", "text", "The QR code's text.", nil, nil),
		textTool("parse_indian_address", "Split a free-text Indian address into house, street, landmark, locality, city, district, state and pincode, and check the pincode against the post office list.",
			"/v1/address/parse", "address", "The address as written.", nil, nil),
	)
}
