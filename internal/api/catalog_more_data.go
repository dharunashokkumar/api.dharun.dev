package api

// Census 2011, STD codes, card prefixes and the classics (census.go, stdcodes.go,
// bins.go, classics.go).

func withMoreData(groups []apiGroup) []apiGroup {
	return append(groups,
		apiGroup{
			ID: "census", Category: "places", Title: "Census 2011 figures",
			Tagline:   "Population, households, literacy, sex ratio and workers for India, every state, district, sub-district and town, from the 2011 census.",
			Intro:     "The Primary Census Abstract of the Census of India 2011, the latest count published: households, people by sex, children under seven, Scheduled Castes and Tribes, literacy and work, each as a total and split into rural and urban, with the literacy rate, sex ratio and other rates worked out. 35 states and union territories, 640 districts, 5,923 sub-districts and 7,933 towns, as they were in 2011.",
			SourceKey: "census",
			Endpoints: []endpoint{
				{Path: "/v1/census/{level}/{code}", Summary: "Figures for a place, and the places inside it", Example: "/v1/census/district/632",
					Doc: "figures holds total, rural and urban (a town has urban only); rates the literacy rate (of people aged 7 and over), sex ratio (females per 1,000 males), child sex ratio, work participation and the urban share. children lists the places one level down with their population. India is /v1/census/india/0.",
					Params: []param{{Name: "level", In: "path", Type: "string", Required: true, Enum: []string{"india", "state", "district", "subdistrict", "town"}, Doc: "Which kind of place."},
						{Name: "code", In: "path", Type: "string", Required: true, Doc: "The 2011 census code: 33 for Tamil Nadu, 632 for Coimbatore district (from a search)."}}},
				{Path: "/v1/census", Summary: "Find a place by name", Example: "/v1/census?q=coimbatore",
					Doc: "Places whose names contain the words, exact names first, then the largest; at most 50, each with its code and population.",
					Params: []param{{Name: "q", In: "query", Type: "string", Required: true, Doc: "3 to 60 letters, such as madurai."},
						{Name: "level", In: "query", Type: "string", Enum: []string{"state", "district", "subdistrict", "town"}, Doc: "Only this kind of place."}}},
			},
		},
		apiGroup{
			ID: "std-codes", Category: "network", Title: "STD codes",
			Tagline:   "The landline STD code of every town and area in India, and which area a landline number belongs to.",
			Intro:     "All 2,645 Short Distance Charging Areas of the Department of Telecommunications' National Numbering Plan, each with its STD (trunk) code, the long distance area and the telecom circle. Look a code or a landline number up, or find a town's code. Circles are the 2003 service areas, which are not always states.",
			SourceKey: "std-codes",
			Endpoints: []endpoint{
				{Path: "/v1/std-codes/{code}", Summary: "The area of an STD code or a landline number", Example: "/v1/std-codes/0422", Private: true,
					Doc:    "A code (44, 0422) must match whole; a landline number (0422 2345678, +91 44 2345 6789) is matched by its longest code. A whole number is kept out of the log.",
					Params: []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "An STD code or a landline number, with or without 0 or +91."}}},
				{Path: "/v1/std-codes", Summary: "Find the STD code of a town", Example: "/v1/std-codes?q=pollachi",
					Doc: "Areas whose names (or long distance area) contain the words, or every area of a circle; shortest codes first.",
					Params: []param{{Name: "q", In: "query", Type: "string", Doc: "3 to 60 letters of a town's name."},
						{Name: "circle", In: "query", Type: "string", Doc: "A circle code such as TN, KT, MH, UPE, NE."}}},
			},
		},
		apiGroup{
			ID: "bin", Category: "money", Title: "Card BIN lookup",
			Tagline:   "Which network, card type and bank a card's first six digits belong to: Visa, Mastercard, RuPay, debit or credit.",
			Intro:     "The first six to eight digits of a card (the BIN or IIN) name its network, whether it is debit, credit or prepaid, its product and the bank that issued it. About 374,000 prefixes worldwide, about 12,000 of them Indian, from an open BIN list (CC BY 4.0). Use it to show the right card logo or to route a payment; never send a whole card number.",
			SourceKey: "bins",
			Endpoints: []endpoint{
				{Path: "/v1/bin/{prefix}", Summary: "Network, type and bank of a card prefix", Example: "/v1/bin/605067", Private: true, Live: true,
					Doc:    "The longest known prefix of the digits given. More than eight digits is refused, so a card number is never looked up. Quality varies: the list is not the networks' own.",
					Params: []param{{Name: "prefix", In: "path", Type: "string", Required: true, Doc: "6 to 8 digits."}}},
			},
		},
		apiGroup{
			ID: "classics", Category: "language", Title: "Indian classics",
			Tagline:   "Aathichoodi, the Bhagavad Gita in Sanskrit and Kabir's couplets, a line or verse at a time, with transliteration.",
			Intro:     "Public domain classics beside the Thirukkural: Avvaiyar's Aathichoodi (109 Tamil maxims), the Bhagavad Gita (700 Sanskrit verses) and 284 of Kabir's sakhis (Hindi couplets) from the Kabir Granthavali, as transcribed on Wikisource. Each line comes with a Roman transliteration (ISO 15919). No translations: those still under copyright are not used.",
			SourceKey: "classics",
			Endpoints: []endpoint{
				{Path: "/v1/classics", Summary: "The texts", Example: "/v1/classics"},
				{Path: "/v1/classics/{text}", Summary: "A text and its parts", Example: "/v1/classics/gita",
					Params: []param{{Name: "text", In: "path", Type: "string", Required: true, Enum: []string{"aathichoodi", "gita", "kabir"}, Doc: "Which text."}}},
				{Path: "/v1/classics/{text}/{ref}", Summary: "A verse, a whole part, or a random one", Example: "/v1/classics/gita/2.47",
					Doc: "ref is part.number (gita 2.47, kabir 1.3), a part alone for all of it (gita 2), random, or for Aathichoodi a line number (1 to 109).",
					Params: []param{{Name: "text", In: "path", Type: "string", Required: true, Enum: []string{"aathichoodi", "gita", "kabir"}, Doc: "Which text."},
						{Name: "ref", In: "path", Type: "string", Required: true, Doc: "2.47, 2, random, or 7 for Aathichoodi."}}},
			},
		},
	)
}
