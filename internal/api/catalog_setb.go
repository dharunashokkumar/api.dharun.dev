package api

// The catalog of set B, markets and company data (setb.go).

func withSetB(groups []apiGroup) []apiGroup {
	for i := range groups {
		if groups[i].ID == "mutual-funds" {
			groups[i].MoreSources = append(groups[i].MoreSources, "nav-history")
			groups[i].Endpoints = append(groups[i].Endpoints,
				endpoint{Path: "/v1/mutual-funds/{code}/returns", Summary: "Returns and SIP returns",
					Doc:     "Point-to-point returns over 1, 3 and 6 months and 1, 3, 5 and 10 years (CAGR from a year), and what a monthly SIP of 1,000 would be worth now (XIRR), from AMFI's NAV history.",
					Example: "/v1/mutual-funds/122639/returns",
					Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "AMFI scheme code or ISIN."}}},
				endpoint{Path: "/v1/mutual-funds/{code}/history", Summary: "NAV history",
					Doc:     "Month-end NAVs for up to ten years and every NAV of the last 45 days.",
					Example: "/v1/mutual-funds/122639/history?from=2026-01-01",
					Params: []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "AMFI scheme code or ISIN."},
						{Name: "from", In: "query", Type: "string", Doc: "From this date, YYYY-MM-DD."}, {Name: "to", In: "query", Type: "string", Doc: "Up to this date."}}},
			)
		}
	}
	return append(groups,
		apiGroup{
			ID: "bullion", Category: "money", Title: "Gold and silver rates",
			Tagline:   "Today's gold (24K, 22K, 18K...), silver and platinum rates from IBJA, and their history.",
			Intro:     "The India Bullion and Jewellers Association's benchmark rates, published each working day in two sessions, checked every two hours. Before GST and making charges. Gold and platinum per 10 grams, silver per kilogram, with the price per gram.",
			SourceKey: "bullion",
			Endpoints: []endpoint{
				{Path: "/v1/bullion", Summary: "Today's rates", Live: true, Example: "/v1/bullion"},
				{Path: "/v1/bullion/history", Summary: "Rates over time", Live: true, Example: "/v1/bullion/history?metal=gold&purity=916&days=30",
					Params: []param{{Name: "metal", In: "query", Type: "string", Enum: []string{"gold", "silver", "platinum"}, Doc: "Default gold."},
						{Name: "purity", In: "query", Type: "string", Enum: []string{"999", "995", "916", "750", "585"}, Doc: "Default 999."},
						{Name: "days", In: "query", Type: "integer", Doc: "1 to 3650. Default 30."}}},
			},
		},
		apiGroup{
			ID: "indices", Category: "money", Title: "Index constituents",
			Tagline:   "Which companies are in Nifty 50, Bank Nifty, Nifty 500 and 28 more NSE indices, and which indices a stock is in.",
			Intro:     "The member lists of 31 NSE indices (broad, sector and thematic) as NSE Indices Ltd publishes them, refreshed weekly, each member with its industry, ISIN and logo address. Weights are not published here.",
			SourceKey: "indices",
			Endpoints: []endpoint{
				{Path: "/v1/indices", Summary: "List indices, or the indices of a stock", Example: "/v1/indices?symbol=HDFCBANK",
					Params: []param{{Name: "symbol", In: "query", Type: "string", Doc: "An NSE symbol or ISIN: only the indices it is in."}}},
				{Path: "/v1/indices/{id}", Summary: "Constituents of an index", Example: "/v1/indices/nifty-bank",
					Params: []param{{Name: "id", In: "path", Type: "string", Required: true, Doc: "Such as nifty-50, nifty-bank, nifty-it, nifty-midcap-150."},
						{Name: "industry", In: "query", Type: "string", Doc: "Only members in this industry, as the list names it."}}},
			},
		},
	)
}
