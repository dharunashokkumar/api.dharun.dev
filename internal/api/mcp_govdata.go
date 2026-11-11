package api

import "net/url"

// MCP tools for mandi prices and air quality.

func init() {
	changingTools["get_mandi_prices"] = true
	changingTools["get_air_quality"] = true
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "get_mandi_prices",
			Description: "Today's wholesale prices of a farm commodity (Tomato, Onion, Potato, rice, pulses...) in India's markets, from Agmarknet: min, max and modal price in rupees a quintal, by market. Filter by state, district or market.",
			InputSchema: schema(nil, map[string]any{
				"commodity": text("As Agmarknet names it, such as Tomato or Onion."), "state": text("Such as Tamil Nadu."),
				"district": text("Such as Dharmapuri."), "market": text("Part of a market's name."),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				q := url.Values{}
				for _, k := range []string{"commodity", "state", "district", "market"} {
					if v := arg(args, k); v != "" {
						q.Set(k, v)
					}
				}
				return toolRequest{path: "/v1/mandi?" + q.Encode()}, nil
			},
		},
		mcpTool{
			Name:        "get_air_quality",
			Description: "The air quality index (AQI) at CPCB's monitoring stations in an Indian city or state, or the five nearest a point, with each pollutant (PM2.5, PM10, NO2, ozone...). Updated hourly.",
			InputSchema: schema(nil, map[string]any{
				"city": text("Such as Delhi, Chennai, Bengaluru."), "state": text("Such as Tamil Nadu."),
				"near": text("latitude,longitude, such as 13.08,80.27: the five nearest stations."),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				q := url.Values{}
				for _, k := range []string{"city", "state", "near"} {
					if v := arg(args, k); v != "" {
						q.Set(k, v)
					}
				}
				return toolRequest{path: "/v1/air-quality?" + q.Encode()}, nil
			},
		},
	)
}
