package api

// The catalog of the rest of set A (seta.go).

func withSetA(groups []apiGroup) []apiGroup {
	return append(groups,
		apiGroup{
			ID: "format", Category: "money", Title: "Indian number formatting",
			Tagline: "Lakh and crore grouping, 1.2 Cr short forms, rupees and Indian-script digits.",
			Intro:   "One number written every way it is written in India: 12,34,567 grouping, the international form, rupees with paise, short forms (12.3 L, 1.25 Cr), in words, and with the digits of nine Indian scripts.",
			Endpoints: []endpoint{{
				Path: "/v1/format/number/{number}", Summary: "Format a number the Indian way", Reference: true,
				Example: "/v1/format/number/123456789.5",
				Params: []param{
					{Name: "number", In: "path", Type: "string", Required: true, Doc: "The number: digits, a point, optional commas or a minus sign. Indian-script digits are read too."},
					{Name: "decimals", In: "query", Type: "integer", Doc: "Round to this many decimals, 0 to 10."},
				},
			}},
		},
		apiGroup{
			ID: "dates", Category: "dates", Title: "Indian date tools",
			Tagline: "Financial and assessment year, quarter, the Saka, Tamil and Hijri dates, and ages.",
			Intro:   "For a date: the financial year (April to March), its quarter and half, the assessment year, the ISO week, the Indian national (Saka) date, the Tamil solar date, the Vikram Samvat year and the tabular Hijri date. And the span between two dates in years, months and days, for ages and tenures.",
			Endpoints: []endpoint{
				{Path: "/v1/dates/{date}", Summary: "One date in Indian calendars", Reference: true,
					Example: "/v1/dates/2026-11-11",
					Params:  []param{{Name: "date", In: "path", Type: "string", Required: true, Doc: "YYYY-MM-DD, or today."}}},
				{Path: "/v1/dates/diff", Summary: "Age and the span between dates", Reference: true, Live: true,
					Doc:     "Whole years, months and days from one date to another, the total days and weeks, and the next anniversary (a birthday, say).",
					Example: "/v1/dates/diff?from=1995-08-15&to=2026-11-11",
					Params: []param{{Name: "from", In: "query", Type: "string", Required: true, Doc: "Start date, YYYY-MM-DD (a date of birth for an age)."},
						{Name: "to", In: "query", Type: "string", Doc: "End date, YYYY-MM-DD. Default today in India."}}},
			},
		},
		apiGroup{
			ID: "upi", Category: "money", Title: "UPI handles and payment QR codes",
			Tagline: "Which app and bank is behind @okaxis, and what a UPI or Bharat QR code says.",
			Intro:   "The handle after the @ in a UPI ID names the app and the partner bank that issued it. The QR reader takes the text inside a payment QR code (a upi://pay link or an EMVCo string, which Bharat QR uses), splits it into payee, amount, merchant category, city and reference, and checks its checksum.",
			Endpoints: []endpoint{
				{Path: "/v1/upi-handles", Summary: "Known UPI handles", Reference: true, Example: "/v1/upi-handles"},
				{Path: "/v1/upi-handles/{handle}", Summary: "Look up a UPI handle", Reference: true,
					Doc:     "Give the handle (okhdfcbank) or a whole UPI ID; only the part after the @ is read.",
					Example: "/v1/upi-handles/okhdfcbank",
					Params:  []param{{Name: "handle", In: "path", Type: "string", Required: true, Doc: "The handle, with or without the name and @ before it."}}},
				{Path: "/v1/qr/parse", Summary: "Read a UPI or Bharat QR code's text", Reference: true, Post: true, Private: true,
					Doc:         "Splits the text of a payment QR code into its fields and checks the EMVCo checksum. To get the text from a picture, POST the picture to /v1/barcode/read first.",
					Body:        "The QR code's text, as text/plain or JSON {\"text\": \"...\"}.",
					Example:     "/v1/qr/parse?text=upi%3A%2F%2Fpay%3Fpa%3Dshop%40okaxis%26pn%3DSri%2520Murugan%2520Stores%26am%3D250%26cu%3DINR%26mc%3D5411",
					ExampleBody: "upi://pay?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250&cu=INR",
					Params:      []param{{Name: "text", In: "query", Type: "string", Doc: "The QR code's text."}}},
			},
		},
		apiGroup{
			ID: "badges", Category: "developer", Title: "README badges",
			Tagline: "Small SVG badges: market open or closed, the next holiday, a dataset's freshness, or your own.",
			Intro:   "Badges in the familiar two-part style, for READMEs, dashboards and status pages. The text is drawn as outlines, so any script shows the same everywhere.",
			Endpoints: []endpoint{
				{Path: "/v1/badge", Summary: "A badge of your own", Image: true, Reference: true,
					Example: "/v1/badge?label=made%20in&message=India&color=orange",
					Params: []param{{Name: "label", In: "query", Type: "string", Doc: "Left part, up to 60 characters. Optional."},
						{Name: "message", In: "query", Type: "string", Required: true, Doc: "Right part, up to 80 characters."},
						{Name: "color", In: "query", Type: "string", Doc: "Message colour: green, red, orange, yellow, blue, grey, purple, teal, or hex. Default blue."},
						{Name: "labelColor", In: "query", Type: "string", Doc: "Label colour, hex. Default 555."}, pictureFormatParam}},
				{Path: "/v1/badge/{kind}", Summary: "Market or holiday badge", Image: true, Live: true,
					Doc:     "market: NSE open, closed or in its pre-open or closing session. holiday: the next holiday (bank by default; type and state as in /v1/holidays).",
					Example: "/v1/badge/market",
					Params: []param{{Name: "kind", In: "path", Type: "string", Required: true, Enum: []string{"market", "holiday"}, Doc: "Which badge."},
						{Name: "label", In: "query", Type: "string", Doc: "Your own label."}, {Name: "type", In: "query", Type: "string", Doc: "holiday: bank (default), market, central."}, stateFilter, pictureFormatParam}},
				{Path: "/v1/badge/dataset/{name}", Summary: "Dataset freshness badge", Image: true, Live: true,
					Doc:     "When one of this API's datasets was last updated; orange when older than 45 days.",
					Example: "/v1/badge/dataset/ifsc",
					Params:  []param{{Name: "name", In: "path", Type: "string", Required: true, Doc: "A dataset name, as /healthz lists them."}, pictureFormatParam}},
			},
		},
		apiGroup{
			ID: "address", Category: "places", Title: "Address reader",
			Tagline:   "A free-text Indian address split into house, street, locality, city, state and pincode, checked.",
			Intro:     "Splits a written address into its parts and checks the pincode against the post office list: a pincode in another state, or a city that is not near the pincode, is flagged. Rules, not a model. What you send is never logged.",
			SourceKey: "pincode",
			Endpoints: []endpoint{{
				Path: "/v1/address/parse", Summary: "Read an Indian address", Post: true, Private: true,
				Body:        "The address, as text/plain or JSON {\"text\": \"...\"}.",
				Example:     "/v1/address/parse?text=12%2C%20Anna%20Salai%2C%20Mount%20Road%2C%20Chennai%2C%20Tamil%20Nadu%20600002",
				ExampleBody: "Flat 4B, Lakshmi Apartments, 3rd Cross Road, Indiranagar, Bengaluru 560038",
				Params:      []param{{Name: "text", In: "query", Type: "string", Doc: "The address."}},
			}},
		},
	)
}
