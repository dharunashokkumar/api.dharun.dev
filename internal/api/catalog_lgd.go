package api

// The Local Government Directory (lgd.go).

func withLGD(groups []apiGroup) []apiGroup {
	return append(groups, apiGroup{
		ID: "lgd", Category: "places", Title: "Villages and LGD codes",
		Tagline:   "Every state, district, sub-district and village of India with its LGD and Census 2011 codes, and village PIN codes.",
		Intro:     "The Local Government Directory (LGD) of the Ministry of Panchayati Raj gives every administrative unit a code that government systems use. Walk down from states to districts, sub-districts and villages, find a village by name or by PIN code, or look one up by its code. From data.gov.in, refreshed monthly; names in English and the local script.",
		SourceKey: "lgd",
		Endpoints: []endpoint{
			{Path: "/v1/lgd/states", Summary: "States with their LGD codes", Live: true, Example: "/v1/lgd/states"},
			{Path: "/v1/lgd/districts", Summary: "Districts of a state", Live: true, Example: "/v1/lgd/districts?state=33",
				Params: []param{{Name: "state", In: "query", Type: "integer", Required: true, Doc: "The state's LGD code, such as 33 for Tamil Nadu."}}},
			{Path: "/v1/lgd/subdistricts", Summary: "Sub-districts (taluks, tehsils, mandals) of a district", Live: true, Example: "/v1/lgd/subdistricts?district=580",
				Params: []param{{Name: "district", In: "query", Type: "integer", Required: true, Doc: "The district's LGD code."}}},
			{Path: "/v1/lgd/villages", Summary: "Villages by sub-district, PIN code or name", Live: true, Example: "/v1/lgd/villages?pincode=637001",
				Doc: "One of subdistrict, pincode or q. Each village with its sub-district, district and state.",
				Params: []param{{Name: "subdistrict", In: "query", Type: "integer", Doc: "A sub-district's LGD code: all its villages."},
					{Name: "pincode", In: "query", Type: "string", Doc: "Six digits: the villages LGD lists under it."},
					{Name: "q", In: "query", Type: "string", Doc: "The start of a village's name, 3 to 80 characters; at most 100 answers."},
					{Name: "state", In: "query", Type: "integer", Doc: "With q: only this state (LGD code)."}}},
			{Path: "/v1/lgd/villages/{code}", Summary: "A village by its LGD code", Live: true, Example: "/v1/lgd/villages/634584",
				Params: []param{{Name: "code", In: "path", Type: "integer", Required: true, Doc: "The village's LGD code."}}},
		},
	})
}
