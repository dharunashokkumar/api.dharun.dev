package api

// The catalog of set B, markets and company data (setb.go).

func withSetB(groups []apiGroup) []apiGroup {
	for i := range groups {
		if groups[i].ID == "ifsc" {
			groups[i].Endpoints = append(groups[i].Endpoints,
				endpoint{Path: "/v1/micr/{code}", Summary: "Branch from a MICR code", Example: "/v1/micr/400240003",
					Doc:    "The nine-digit code at the foot of a cheque, to the bank branch and its IFSC.",
					Params: []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "Nine digits."}}},
				endpoint{Path: "/v1/swift/{code}", Summary: "Branches with a SWIFT code", Example: "/v1/swift/HDFCINBB",
					Params: []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "8 or 11 characters, such as HDFCINBB."}}},
			)
		}
		if groups[i].ID == "logos" {
			groups[i].Endpoints = append(groups[i].Endpoints,
				endpoint{Path: "/v1/sectors", Summary: "Sectors and industries of listed companies", Example: "/v1/sectors",
					Doc:    "Every sector and industry of the NSE and BSE listed companies with how many companies each has; give sector to list its industries and largest companies.",
					Params: []param{{Name: "sector", In: "query", Type: "string", Doc: "One sector, as listed."}}},
			)
		}
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
			ID: "rates", Category: "money", Title: "RBI rates and exchange rates",
			Tagline:   "Repo rate, CRR and SLR, FBIL reference exchange rates, lending and deposit rates, and bond yields.",
			Intro:     "What the Reserve Bank of India shows as its current rates, checked every three hours: the policy rates, reserve ratios, FBIL reference rates for the dollar, pound, euro, yen, dirham, rupiah and rouble, the range of base rates and MCLR, deposit rates, treasury bill cut-offs and government bond yields. Exchange rate history builds up from the day this server began recording.",
			SourceKey: "rbi-rates",
			Endpoints: []endpoint{
				{Path: "/v1/rates", Summary: "Current RBI rates", Live: true, Example: "/v1/rates"},
				{Path: "/v1/fx", Summary: "Reference exchange rates", Live: true, Example: "/v1/fx?currency=USD",
					Params: []param{{Name: "currency", In: "query", Type: "string", Enum: []string{"USD", "GBP", "EUR", "JPY", "AED", "IDR", "RUB"}, Doc: "One currency. Default all."},
						{Name: "days", In: "query", Type: "integer", Doc: "History for this many days, 1 to 3650, instead of the latest."}}},
			},
		},
		apiGroup{
			ID: "expiries", Category: "money", Title: "Derivative expiries",
			Tagline:   "NSE and BSE futures and options expiry dates, moved for holidays.",
			Intro:     "Every weekly and monthly expiry of a year from the exchange rules in force since 1 September 2025: NSE on Tuesdays (Nifty 50 weekly), BSE on Thursdays (Sensex weekly), monthly contracts on the month's last one; an expiry on a holiday moves to the trading day before.",
			SourceKey: "holidays-market",
			Endpoints: []endpoint{
				{Path: "/v1/expiries", Summary: "Expiry dates", Example: "/v1/expiries?exchange=nse&year=2026&month=3",
					Params: []param{{Name: "exchange", In: "query", Type: "string", Enum: []string{"nse", "bse"}, Doc: "Default nse."},
						{Name: "year", In: "query", Type: "integer", Doc: "From 2025. Default this year."}, {Name: "month", In: "query", Type: "integer", Doc: "1 to 12."},
						{Name: "weekly", In: "query", Type: "boolean", Doc: "false lists only the monthly expiries."}}},
			},
		},
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
