package api

import "strconv"

// The catalog describes every public endpoint once. The documentation site, the
// OpenAPI document, llms.txt and the sitemap are all generated from it, so they
// cannot drift apart.

type param struct {
	Name     string
	In       string // "path" or "query"
	Type     string // "string", "integer"
	Required bool
	Enum     []string
	Doc      string
}

type endpoint struct {
	Path    string // OpenAPI-style path, e.g. /v1/pincode/{pincode}
	Summary string
	Doc     string
	Params  []param
	Example string // a real request path that returns 200
	Image   bool   // returns an image, not the JSON envelope
	// Post marks an endpoint that also takes its text as a POST body. Body says what the
	// body holds, and ExampleBody is a body that returns 200 when posted to Example.
	Post        bool
	Body        string
	ExampleBody string
	// Private marks an endpoint whose input may be personal: never logged, never cached.
	Private bool
}

type apiGroup struct {
	ID        string
	Title     string
	Tagline   string
	Intro     string
	SourceKey string // the dataset whose source is credited, "" for pure logic
	// MoreSources lists further datasets the group draws on.
	MoreSources []string
	Endpoints   []endpoint
}

var (
	limitParam       = param{Name: "limit", In: "query", Type: "integer", Doc: "Results per page, 1 to 100."}
	offsetParam      = param{Name: "offset", In: "query", Type: "integer", Doc: "Results to skip."}
	stateFilter      = param{Name: "state", In: "query", Type: "string", Doc: "State code or name, for example TN or Tamil Nadu."}
	calendarParamDoc = param{Name: "calendar", In: "query", Type: "string", Enum: workCalendarIDs(),
		Doc: "bank (default; needs state), market (NSE trading), settlement (NSE clearing) or central (central government offices)."}
)

var catalog = []apiGroup{
	{
		ID:        "logos",
		Title:     "Company logos",
		Tagline:   "Logos and details for companies listed on NSE and BSE.",
		Intro:     "Put a ticker in the URL and get the logo back as SVG or PNG. A symbol can be a ticker (TCS), an exchange-qualified ticker (NSE:TCS, NSE_TCS), a Yahoo Finance symbol (TCS.NS, AAIL.BO) or an ISIN. A bare ticker prefers NSE.",
		SourceKey: "logos",
		Endpoints: []endpoint{
			{
				Path: "/v1/logos/{symbol}", Summary: "Logo image for a listed company", Image: true,
				Doc:     "Returns the image itself, ready for an <img> tag. Unknown symbols return 404 unless fallback=monogram is set.",
				Example: "/v1/logos/TCS",
				Params: []param{
					{Name: "symbol", In: "path", Type: "string", Required: true, Doc: "Ticker, exchange-qualified ticker, Yahoo symbol or ISIN. A .svg or .png suffix selects the format."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"svg", "png"}, Doc: "Image format. Default svg."},
					{Name: "size", In: "query", Type: "integer", Doc: "PNG width and height in pixels, " + strconv.Itoa(pngMinSize) + " to " + strconv.Itoa(pngMaxSize) + ". Default " + strconv.Itoa(pngDefaultSize) + "."},
					{Name: "fallback", In: "query", Type: "string", Enum: []string{"monogram"}, Doc: "Return an initials tile instead of 404 for unknown symbols."},
					{Name: "exchange", In: "query", Type: "string", Enum: []string{"nse", "bse"}, Doc: "Force an exchange."},
				},
			},
			{
				Path: "/v1/companies/{symbol}", Summary: "Details of a listed company",
				Doc:     "Name, sector, industry, ISIN, market capitalisation, index memberships and logo URLs.",
				Example: "/v1/companies/RELIANCE",
				Params: []param{
					{Name: "symbol", In: "path", Type: "string", Required: true, Doc: "Ticker, exchange-qualified ticker, Yahoo symbol or ISIN."},
					{Name: "exchange", In: "query", Type: "string", Enum: []string{"nse", "bse"}, Doc: "Force an exchange."},
				},
			},
			{
				Path: "/v1/companies", Summary: "Search listed companies",
				Doc:     "Searches ticker, company name, sector, industry, ISIN and index names. Best match first.",
				Example: "/v1/companies?q=tata&limit=5",
				Params: []param{
					{Name: "q", In: "query", Type: "string", Doc: "Search text."},
					{Name: "exchange", In: "query", Type: "string", Enum: []string{"nse", "bse"}, Doc: "Limit to one exchange."},
					{Name: "sector", In: "query", Type: "string", Doc: "Exact sector name."},
					{Name: "index", In: "query", Type: "string", Doc: "Index name, for example Nifty 50."},
					limitParam, offsetParam,
				},
			},
		},
	},
	{
		ID:        "pincode",
		Title:     "Pincodes",
		Tagline:   "Pincode to post office, district and state.",
		Intro:     "Every post office in the Department of Posts directory, with its district, state, postal division and coordinates.",
		SourceKey: "pincode",
		Endpoints: []endpoint{
			{
				Path: "/v1/pincode/{pincode}", Summary: "Post offices for a pincode",
				Doc:     "Returns the district and state of the pincode and every post office that uses it.",
				Example: "/v1/pincode/637001",
				Params:  []param{{Name: "pincode", In: "path", Type: "string", Required: true, Doc: "Six-digit pincode."}},
			},
			{
				Path: "/v1/pincode", Summary: "Search post offices",
				Doc:     "Find post offices by the start of their name, by the start of a pincode, or list a district.",
				Example: "/v1/pincode?q=namakkal&state=TN",
				Params: []param{
					{Name: "q", In: "query", Type: "string", Doc: "Start of a post office name (2+ characters) or start of a pincode."},
					stateFilter,
					{Name: "district", In: "query", Type: "string", Doc: "Exact district name."},
					limitParam, offsetParam,
				},
			},
			{
				Path: "/v1/pincode/near", Summary: "Post offices near a point",
				Doc:     "Post offices within a radius of a latitude and longitude, nearest first, with the distance. Positions come from the Department of Posts and some are approximate.",
				Example: "/v1/pincode/near?lat=11.2189&lng=78.1674&radius=5",
				Params: []param{
					{Name: "lat", In: "query", Type: "number", Required: true, Doc: "Latitude in decimal degrees."},
					{Name: "lng", In: "query", Type: "number", Required: true, Doc: "Longitude in decimal degrees."},
					{Name: "radius", In: "query", Type: "integer", Doc: "Kilometres, 1 to 50. Default 5."},
					{Name: "limit", In: "query", Type: "integer", Doc: "Results, 1 to 100. Default 10."},
				},
			},
		},
	},
	{
		ID:        "ifsc",
		Title:     "IFSC and banks",
		Tagline:   "IFSC to bank and branch.",
		Intro:     "Branch details for every IFSC in the Reserve Bank's NEFT and RTGS lists: bank, branch, address, MICR and which payment systems the branch supports.",
		SourceKey: "ifsc",
		Endpoints: []endpoint{
			{
				Path: "/v1/ifsc/{code}", Summary: "Branch for an IFSC",
				Example: "/v1/ifsc/SBIN0000001",
				Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "11-character IFSC."}},
			},
			{
				Path: "/v1/ifsc", Summary: "Search a bank's branches",
				Doc:     "Lists branches of one bank, filtered by state, city or text in the branch name or address.",
				Example: "/v1/ifsc?bank=SBIN&state=TN&q=namakkal",
				Params: []param{
					{Name: "bank", In: "query", Type: "string", Required: true, Doc: "Four-letter bank code, for example SBIN."},
					{Name: "q", In: "query", Type: "string", Doc: "Text to find in the branch name, address or city."},
					stateFilter,
					{Name: "city", In: "query", Type: "string", Doc: "Exact city name."},
					limitParam, offsetParam,
				},
			},
			{
				Path: "/v1/banks/{code}", Summary: "Bank for a four-letter code",
				Example: "/v1/banks/HDFC",
				Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "Four-letter bank code."}},
			},
			{
				Path: "/v1/banks", Summary: "Search banks",
				Doc:     "Finds banks by name or code, largest branch network first.",
				Example: "/v1/banks?q=gramin",
				Params: []param{
					{Name: "q", In: "query", Type: "string", Doc: "Text to find in the bank name or code."},
					limitParam, offsetParam,
				},
			},
		},
	},
	{
		ID:        "rto",
		Title:     "RTO codes",
		Tagline:   "Vehicle registration code to transport office.",
		Intro:     "Which Regional Transport Office a number plate belongs to: TN-28 is Namakkal (North). Accepts a code or a whole registration number.",
		SourceKey: "rto",
		Endpoints: []endpoint{
			{
				Path: "/v1/rto/{code}", Summary: "Office for an RTO code",
				Example: "/v1/rto/TN-28",
				Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "RTO code (TN-28, TN28) or a registration number (TN28AB1234)."}},
			},
			{
				Path: "/v1/rto", Summary: "List or search RTO codes",
				Example: "/v1/rto?state=TN&q=chennai",
				Params: []param{
					stateFilter,
					{Name: "q", In: "query", Type: "string", Doc: "Text to find in the office name or jurisdiction."},
					limitParam, offsetParam,
				},
			},
		},
	},
	{
		ID:        "holidays",
		Title:     "Holidays",
		Tagline:   "Bank, government and stock market holidays.",
		Intro:     "Holidays for a year, filterable by kind and state. Each entry says who observes it.",
		SourceKey: "holidays-bank", MoreSources: []string{"holidays-central", "holidays-market"},
		Endpoints: []endpoint{
			{
				Path: "/v1/holidays/{year}", Summary: "Holidays in a year",
				Example: "/v1/holidays/2026?type=market",
				Params: []param{
					{Name: "year", In: "path", Type: "integer", Required: true, Doc: "Four-digit year."},
					{Name: "type", In: "query", Type: "string", Enum: holidayTypeNames(), Doc: "Kind of holiday. clearing days are ones on which the market trades but nothing settles."},
					stateFilter,
					{Name: "month", In: "query", Type: "integer", Doc: "Month number, 1 to 12."},
				},
			},
		},
	},
	{
		ID:        "working-days",
		Title:     "Working days",
		Tagline:   "Bank, market, settlement and government working days.",
		Intro:     "Is a date a working day, what is the next one, and what date is N working days away. Bank days follow the Reserve Bank's holidays for a state plus Sundays and the second and fourth Saturdays; market and settlement days follow NSE's trading and clearing holidays. Only years with a published holiday list are answered.",
		SourceKey: "holidays-bank", MoreSources: []string{"holidays-market", "holidays-central"},
		Endpoints: []endpoint{
			{
				Path: "/v1/working-days/{date}", Summary: "Is a date a working day",
				Doc:     "Whether the date is a working day and why not, the previous and next working days, and with add=N the date N working days later (negative for earlier). A T+1 settlement date is calendar=settlement&add=1.",
				Example: "/v1/working-days/2026-01-16?calendar=bank&state=TN&add=3",
				Params: []param{
					{Name: "date", In: "path", Type: "string", Required: true, Doc: "Date, YYYY-MM-DD."},
					calendarParamDoc,
					{Name: "state", In: "query", Type: "string", Doc: "State code or name. Required for the bank calendar."},
					{Name: "add", In: "query", Type: "integer", Doc: "Working days to add, -366 to 366."},
				},
			},
			{
				Path: "/v1/working-days", Summary: "Count working days between two dates",
				Doc:     "Counts working days from one date to another, both included, and lists each day off with its reason. At most 366 days.",
				Example: "/v1/working-days?from=2026-01-01&to=2026-01-31&calendar=market",
				Params: []param{
					{Name: "from", In: "query", Type: "string", Required: true, Doc: "First date, YYYY-MM-DD."},
					{Name: "to", In: "query", Type: "string", Required: true, Doc: "Last date, YYYY-MM-DD."},
					calendarParamDoc,
					{Name: "state", In: "query", Type: "string", Doc: "State code or name. Required for the bank calendar."},
				},
			},
			{
				Path: "/v1/market/status", Summary: "Is the stock market open",
				Doc:     "NSE equities session at a moment: pre_open, open, closing or closed, the reason it is closed, and when it next opens. Without at, the current moment.",
				Example: "/v1/market/status?at=2026-01-26T10:00:00%2B05:30",
				Params: []param{
					{Name: "at", In: "query", Type: "string", Doc: "Moment with its offset, RFC 3339, for example 2026-10-13T10:30:00+05:30."},
				},
			},
		},
	},
	{
		ID:      "words",
		Title:   "Number to words",
		Tagline: "Numbers and rupee amounts in words, the Indian way.",
		Intro:   "Spells numbers with lakh and crore in English, Hindi or Tamil. 1,00,000 is \"one lakh\". Use style=currency for cheque-style amounts.",
		Endpoints: []endpoint{
			{
				Path: "/v1/words/{number}", Summary: "Spell out a number",
				Example: "/v1/words/1,25,000.50?style=currency",
				Params: []param{
					{Name: "number", In: "path", Type: "string", Required: true, Doc: "The number. Commas, spaces and underscores are ignored."},
					{Name: "lang", In: "query", Type: "string", Enum: []string{"en", "hi", "ta"}, Doc: "Language. Default en."},
					{Name: "style", In: "query", Type: "string", Enum: []string{"number", "currency"}, Doc: "currency gives rupees and paise. Default number."},
				},
			},
		},
	},
	{
		ID:      "validate",
		Title:   "Format checks",
		Tagline: "Check the format of a GSTIN, PAN or IFSC.",
		Intro:   "Structural checks only: character pattern, state code and, for GSTIN, the check character. Nothing is looked up with a government system, and these values are never written to the request log.",
		Endpoints: []endpoint{
			{
				Path: "/v1/validate/{kind}/{value}", Summary: "Check an identifier's format",
				Example: "/v1/validate/gstin/27AAPFU0939F1ZV",
				Params: []param{
					{Name: "kind", In: "path", Type: "string", Required: true, Enum: []string{"gstin", "pan", "ifsc"}, Doc: "Which identifier."},
					{Name: "value", In: "path", Type: "string", Required: true, Doc: "The identifier to check."},
				},
			},
		},
	},
	{
		ID:      "states",
		Title:   "States",
		Tagline: "States and union territories with their codes.",
		Intro:   "All 28 states and 8 union territories with the two-letter code, GST state code and vehicle registration prefixes. The other APIs accept these codes in their state filter.",
		Endpoints: []endpoint{
			{Path: "/v1/states", Summary: "All states and union territories", Example: "/v1/states"},
			{
				Path: "/v1/states/{code}", Summary: "One state",
				Example: "/v1/states/TN",
				Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "State code, vehicle prefix or name."}},
			},
		},
	},
}
