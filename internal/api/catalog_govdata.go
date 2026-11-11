package api

// Mandi prices and air quality from data.gov.in (govdata.go).

func withGovData(groups []apiGroup) []apiGroup {
	return append(groups,
		apiGroup{
			ID: "mandi", Category: "money", Title: "Mandi prices",
			Tagline:   "Today's wholesale prices of vegetables, fruit, grain and pulses in India's markets, from Agmarknet.",
			Intro:     "The prices that agricultural markets (mandis) report to Agmarknet each day: minimum, maximum and modal price in rupees a quintal, by market, commodity, variety and grade. From data.gov.in, checked every three hours, the last 30 days kept.",
			SourceKey: "mandi",
			Endpoints: []endpoint{
				{Path: "/v1/mandi", Summary: "Prices by commodity, state, district or market", Live: true, Example: "/v1/mandi?commodity=Tomato&state=Tamil%20Nadu",
					Doc: "The latest day's prices unless date is given, 100 rows unless limit says otherwise.",
					Params: []param{{Name: "commodity", In: "query", Type: "string", Doc: "As Agmarknet names it: Tomato, Onion, Potato, Paddy(Dhan)(Common) ... (see /v1/mandi/commodities)."},
						{Name: "state", In: "query", Type: "string", Doc: "Such as Tamil Nadu."}, {Name: "district", In: "query", Type: "string", Doc: "Such as Dharmapuri."},
						{Name: "market", In: "query", Type: "string", Doc: "Part of a market's name."}, {Name: "date", In: "query", Type: "string", Doc: "YYYY-MM-DD, within the last 30 days. Default the latest."},
						{Name: "limit", In: "query", Type: "integer", Doc: "1 to 1000. Default 100."}}},
				{Path: "/v1/mandi/commodities", Summary: "What was reported on the latest day", Live: true, Example: "/v1/mandi/commodities?state=Karnataka",
					Doc:    "Each commodity with how many markets reported it and the range of their modal prices.",
					Params: []param{{Name: "state", In: "query", Type: "string", Doc: "Only this state's markets."}}},
			},
		},
		apiGroup{
			ID: "air-quality", Category: "places", Title: "Air quality",
			Tagline:   "The air quality index at CPCB's monitoring stations across India, updated hourly.",
			Intro:     "The latest readings of the Central Pollution Control Board's continuous monitoring stations (PM2.5, PM10, NO2, SO2, CO, ozone, NH3), from data.gov.in, refreshed hourly, with the National Air Quality Index and its band (Good to Severe) worked out per CPCB's rule. Find stations by city, state, name, or nearest to a point.",
			SourceKey: "air-quality",
			Endpoints: []endpoint{
				{Path: "/v1/air-quality", Summary: "Stations with their AQI and pollutants", Live: true, Example: "/v1/air-quality?city=Chennai",
					Doc: "Every station that matches, each with its AQI, category, the pollutant that sets it, and each pollutant's sub-index. With near, the five nearest stations and their distance.",
					Params: []param{{Name: "city", In: "query", Type: "string", Doc: "As CPCB names it: Delhi, Mumbai, Bengaluru, Chennai ..."},
						{Name: "state", In: "query", Type: "string", Doc: "Such as Tamil Nadu."}, {Name: "station", In: "query", Type: "string", Doc: "Part of a station's name."},
						{Name: "near", In: "query", Type: "string", Doc: "latitude,longitude: the five nearest stations."}}},
			},
		},
	)
}
