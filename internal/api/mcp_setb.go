package api

import "net/url"

// MCP tools for set B.

func init() {
	for _, name := range []string{"get_rbi_rates", "get_gold_silver_rates", "get_mutual_fund_returns"} {
		changingTools[name] = true
	}
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "get_rbi_rates",
			Description: "India's current repo rate, reverse repo, SDF, MSF, bank rate, CRR, SLR, base rate and MCLR ranges, deposit rates, T-bill and government bond yields from the RBI; with currency, the FBIL reference exchange rate (USD, GBP, EUR, JPY, AED, IDR, RUB) and optional history.",
			InputSchema: schema(nil, map[string]any{"currency": oneOf("Only this exchange rate.", "USD", "GBP", "EUR", "JPY", "AED", "IDR", "RUB"), "days": map[string]any{"type": "integer", "description": "Exchange rate history for this many days."}}),
			path: func(args map[string]any) (string, error) {
				if arg(args, "currency") != "" {
					return "/v1/fx" + query(args, map[string]string{"currency": "currency", "days": "days"}), nil
				}
				return "/v1/rates", nil
			},
		},
		mcpTool{
			Name:        "get_gold_silver_rates",
			Description: "Today's IBJA benchmark rates for gold (24K 999 to 14K 585), silver and platinum in India, per 10 grams (silver per kg) and per gram, before GST; or the history of one.",
			InputSchema: schema(nil, map[string]any{"metal": oneOf("For history.", "gold", "silver", "platinum"), "purity": oneOf("For history.", "999", "995", "916", "750", "585"), "days": map[string]any{"type": "integer", "description": "History for this many days."}}),
			path: func(args map[string]any) (string, error) {
				if arg(args, "days") != "" || arg(args, "metal") != "" {
					return "/v1/bullion/history" + query(args, map[string]string{"metal": "metal", "purity": "purity", "days": "days"}), nil
				}
				return "/v1/bullion", nil
			},
		},
		mcpTool{
			Name:        "get_mutual_fund_returns",
			Description: "A mutual fund scheme's returns over 1, 3 and 6 months and 1, 3, 5 and 10 years (CAGR) and the XIRR of a monthly SIP, from AMFI NAV history. Scheme codes are in AMFI's list (the REST search /v1/mutual-funds?q=).",
			InputSchema: schema([]string{"code"}, map[string]any{"code": text("AMFI scheme code or ISIN.")}),
			path: func(args map[string]any) (string, error) {
				code, err := required(args, "code")
				return "/v1/mutual-funds/" + url.PathEscape(code) + "/returns", err
			},
		},
		mcpTool{
			Name:        "list_derivative_expiries",
			Description: "NSE (Tuesday) and BSE (Thursday) weekly and monthly futures and options expiry dates, moved to the previous trading day for holidays.",
			InputSchema: schema(nil, map[string]any{"exchange": oneOf("Default nse.", "nse", "bse"), "year": map[string]any{"type": "integer", "description": "From 2025."}, "month": map[string]any{"type": "integer", "description": "1 to 12."}}),
			path: func(args map[string]any) (string, error) {
				return "/v1/expiries" + query(args, map[string]string{"exchange": "exchange", "year": "year", "month": "month"}), nil
			},
		},
		mcpTool{
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
