package api

// The catalog of set B, markets and company data (setb.go).

func withSetB(groups []apiGroup) []apiGroup {
	return append(groups,
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
