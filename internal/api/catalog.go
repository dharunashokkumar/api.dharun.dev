package api

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/textkit"
)

// Example inputs for the text tools, shared by the GET and POST examples.
const (
	chunkExample         = "# Setup\n\nInstall Go. Clone the repository.\n\n# Deploy\n\nPush to main."
	redactExample        = "Call 98765 43210, PAN ABCPK1234F"
	tokensExample        = "வணக்கம், how many tokens is this?"
	languageExample      = "அகர முதல எழுத்தெல்லாம் ஆதி பகவன் முதற்றே உலகு"
	transliterateExample = "नमस्ते भारत"
	codeExample          = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"வணக்கம்\")\n}"
)

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
	// Reference marks an endpoint that answers from a fixed list kept in this code (such
	// as the railway zones) rather than from the group's dataset, so it credits none.
	Reference bool
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
		ID:      "codes",
		Title:   "QR codes and barcodes",
		Tagline: "QR codes, UPI payment QR codes and barcodes as SVG or PNG.",
		Intro:   "Generates codes; it does not read them. What you encode is never logged or kept, and the images are not cached by shared caches.",
		Endpoints: []endpoint{
			{
				Path: "/v1/qr", Summary: "QR code for any text", Image: true, Post: true, Private: true,
				Doc:         "Returns the image itself. Text up to about 2,900 bytes at error correction L.",
				Body:        "The text to encode.",
				Example:     "/v1/qr?text=https://api.dharun.dev&format=png&size=256",
				ExampleBody: "https://api.dharun.dev",
				Params: append([]param{
					{Name: "text", In: "query", Type: "string", Doc: "Text to encode (or POST it)."},
					{Name: "ecc", In: "query", Type: "string", Enum: []string{"L", "M", "Q", "H"}, Doc: "Error correction: how much damage the code survives. Default M."},
				}, codeImageParams(4)...),
			},
			{
				Path: "/v1/upi-qr", Summary: "UPI payment QR code", Image: true, Private: true,
				Doc:     "A QR code any UPI app can scan to pay, built to NPCI's upi://pay link format. format=json returns the link instead of the image. Nothing is checked with a bank.",
				Example: "/v1/upi-qr?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250&tn=Order%2042",
				Params: append([]param{
					{Name: "pa", In: "query", Type: "string", Required: true, Doc: "Payee UPI ID, such as shop@okaxis."},
					{Name: "pn", In: "query", Type: "string", Doc: "Payee name shown in the app."},
					{Name: "am", In: "query", Type: "string", Doc: "Amount in rupees, up to two decimals. Leave out to let the payer type it."},
					{Name: "tn", In: "query", Type: "string", Doc: "Note shown with the payment, up to 80 characters."},
					{Name: "tr", In: "query", Type: "string", Doc: "Your reference for the transaction."},
					{Name: "mc", In: "query", Type: "string", Doc: "Merchant category code, four digits."},
				}, codeImageParams(4)...),
			},
			{
				Path: "/v1/barcode/{type}", Summary: "Barcode", Image: true, Post: true, Private: true,
				Doc:         "Linear barcodes (code128, code39, code93, ean13, ean8, itf, codabar) and 2D codes (datamatrix, pdf417, aztec). Linear codes are sized with scale and height, 2D codes with size. No human-readable text is printed under the bars.",
				Body:        "The text to encode.",
				Example:     "/v1/barcode/ean13?text=890123456789",
				ExampleBody: "890123456789",
				Params: append([]param{
					{Name: "type", In: "path", Type: "string", Required: true, Enum: barcodeKindIDs(), Doc: "Kind of barcode."},
					{Name: "text", In: "query", Type: "string", Doc: "Text to encode (or POST it)."},
					{Name: "scale", In: "query", Type: "integer", Doc: "Linear codes: pixels per bar module, 1 to 10. Default 2."},
					{Name: "height", In: "query", Type: "integer", Doc: "Linear codes: bar height in pixels, 10 to 600. Default 80."},
				}, codeImageParams(0)...),
			},
		},
	},
	{
		ID:        "hsn",
		Title:     "HSN and SAC codes",
		Tagline:   "GST classification codes for goods and services.",
		Intro:     "Every HSN code for goods (from the Customs Tariff Act) and SAC code for services (from the GST services classification) with its description, the codes above it and the codes below it. GST rates are not included.",
		SourceKey: "hsn",
		Endpoints: []endpoint{
			{
				Path: "/v1/hsn/{code}", Summary: "A code, its parents and children",
				Example: "/v1/hsn/0902",
				Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "2 to 8 digits; spaces and dots are ignored. SAC codes start with 99."}},
			},
			{
				Path: "/v1/hsn", Summary: "Search codes",
				Doc:     "Every word of q must appear in the description; digits search by code prefix. Broader codes come first.",
				Example: "/v1/hsn?q=green%20tea",
				Params: []param{
					{Name: "q", In: "query", Type: "string", Required: true, Doc: "Words of the description, or the start of a code."},
					{Name: "kind", In: "query", Type: "string", Enum: []string{"hsn", "sac"}, Doc: "hsn for goods, sac for services."},
					limitParam, offsetParam,
				},
			},
		},
	},
	{
		ID:        "mutual-funds",
		Title:     "Mutual fund NAVs",
		Tagline:   "Latest net asset value of every Indian mutual fund scheme.",
		Intro:     "Every scheme in AMFI's daily NAV file: fund house, category, plan (direct or regular), option (growth, IDCW or bonus), ISINs and the latest NAV with its date. Refreshed by the server several times a day; AMFI publishes each evening.",
		SourceKey: "mutual-funds",
		Endpoints: []endpoint{
			{
				Path: "/v1/mutual-funds/{code}", Summary: "A scheme and its NAV",
				Example: "/v1/mutual-funds/122639",
				Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "AMFI scheme code, or the scheme's growth or reinvestment ISIN."}},
			},
			{
				Path: "/v1/mutual-funds", Summary: "Search schemes",
				Doc:     "Every word of q must appear in the scheme name, in any order. Only active schemes unless all=true.",
				Example: "/v1/mutual-funds?q=flexi%20cap&plan=direct&option=growth",
				Params: []param{
					{Name: "q", In: "query", Type: "string", Doc: "Words of the scheme name."},
					{Name: "fundHouse", In: "query", Type: "string", Doc: "Part of the fund house name, such as hdfc."},
					{Name: "category", In: "query", Type: "string", Doc: "Part of the SEBI category, such as Small Cap."},
					{Name: "plan", In: "query", Type: "string", Enum: []string{"direct", "regular"}, Doc: "Plan."},
					{Name: "option", In: "query", Type: "string", Enum: []string{"growth", "idcw", "bonus"}, Doc: "Option."},
					{Name: "all", In: "query", Type: "string", Enum: []string{"true"}, Doc: "Include wound-up and matured schemes."},
					limitParam, offsetParam,
				},
			},
			{Path: "/v1/mutual-funds/fund-houses", Summary: "Fund houses", Example: "/v1/mutual-funds/fund-houses"},
		},
	},
	{
		ID:        "ip",
		Title:     "IP address location",
		Tagline:   "IP address to country, state, city and network.",
		Intro:     "Country, region (the state, for Indian addresses), city and network operator of a public IPv4 or IPv6 address, from DB-IP's free databases, refreshed monthly. Addresses are never written to the request log.",
		SourceKey: "ip",
		Endpoints: []endpoint{
			{
				Path: "/v1/ip/{address}", Summary: "Location of an IP address", Private: true,
				Example: "/v1/ip/49.204.0.1",
				Params:  []param{{Name: "address", In: "path", Type: "string", Required: true, Doc: "IPv4 or IPv6 address."}},
			},
			{Path: "/v1/ip", Summary: "Location of your own address", Private: true, Example: "/v1/ip"},
		},
	},
	{
		ID:        "email",
		Title:     "Email checks",
		Tagline:   "Is an email address well formed, throwaway, and able to receive mail.",
		Intro:     "Checks the form of an address, whether its domain hands out throwaway mailboxes, whether it is a free provider or a role account such as support@, and whether the domain has mail servers. No mail server is contacted and no mailbox is tried. Addresses are never written to the request log. Counts as 10 requests against the rate limit.",
		SourceKey: "disposable-email",
		Endpoints: []endpoint{
			{
				Path: "/v1/email/{address}", Summary: "Check an email address", Private: true,
				Example: "/v1/email/someone@mailinator.com",
				Params:  []param{{Name: "address", In: "path", Type: "string", Required: true, Doc: "The email address."}},
			},
		},
	},
	{
		ID:      "dns",
		Title:   "DNS check",
		Tagline: "A domain's DNS records at four public resolvers at once.",
		Intro:   "Asks Cloudflare, Google, Quad9 and OpenDNS for the same record and says whether they agree: useful when a change is still spreading. Counts as 10 requests against the rate limit.",
		Endpoints: []endpoint{
			{
				Path: "/v1/dns/{name}", Summary: "Look up DNS records",
				Example: "/v1/dns/dharun.dev?type=MX",
				Params: []param{
					{Name: "name", In: "path", Type: "string", Required: true, Doc: "Domain name (Indian-script names are accepted), or an IP address for PTR."},
					{Name: "type", In: "query", Type: "string", Enum: dnsTypeNames(), Doc: "Record type. Default A."},
				},
			},
		},
	},
	{
		ID:        "stations",
		Title:     "Railway stations",
		Tagline:   "Indian Railways station codes to stations, and back.",
		Intro:     "About 4,000 stations with their code, state, railway zone and elevation, from Wikipedia's list of railway stations in India. No official open list exists; this one is kept by volunteers, so some codes are out of date and many entries lack a state or zone.",
		SourceKey: "stations",
		Endpoints: []endpoint{
			{
				Path: "/v1/stations/{code}", Summary: "Station for a code",
				Example: "/v1/stations/MAS",
				Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "Station code, such as MAS or NDLS."}},
			},
			{
				Path: "/v1/stations", Summary: "Search stations",
				Doc:     "By name (an exact code comes first, then names starting with the text), state or zone.",
				Example: "/v1/stations?q=chennai",
				Params: []param{
					{Name: "q", In: "query", Type: "string", Doc: "Part of a station name, or a code."},
					stateFilter,
					{Name: "zone", In: "query", Type: "string", Doc: "Zone code, such as SR. See /v1/stations/zones."},
					limitParam, offsetParam,
				},
			},
			{Path: "/v1/stations/zones", Summary: "Railway zones", Example: "/v1/stations/zones", Reference: true},
		},
	},
	{
		ID:        "phone",
		Title:     "Phone numbers",
		Tagline:   "Check and format an Indian phone number, and see whose series it is.",
		Intro:     "Validates an Indian number, tells mobile from toll-free and fixed lines, formats it (E.164 and national), and names the operator and telecom circle a mobile number's series was first given to. With number portability the current operator can differ. Numbers are never written to the request log.",
		SourceKey: "phones",
		Endpoints: []endpoint{
			{
				Path: "/v1/phone/{number}", Summary: "Check a phone number", Private: true,
				Example: "/v1/phone/+91%2095001%2012345",
				Params:  []param{{Name: "number", In: "path", Type: "string", Required: true, Doc: "The number, with or without +91, 91 or 0; spaces and dashes are ignored."}},
			},
		},
	},
	{
		ID:        "constituency",
		Title:     "Constituencies",
		Tagline:   "Which assembly and Lok Sabha constituency a place is in.",
		Intro:     "The assembly (MLA) and parliamentary (MP) constituency of a point or a pincode, with number, name, reserved category, district and state. From the DataMeet community's maps; boundaries are approximate near their edges, assembly seats in six states predate the latest delimitation, and sitting members are not given.",
		SourceKey: "constituencies",
		Endpoints: []endpoint{
			{
				Path: "/v1/constituency", Summary: "Constituencies of a place",
				Example: "/v1/constituency?pincode=637001",
				Params: []param{
					{Name: "pincode", In: "query", Type: "string", Doc: "Indian pincode (the average position of its post offices)."},
					{Name: "lat", In: "query", Type: "number", Doc: "Latitude, if no pincode."},
					{Name: "lng", In: "query", Type: "number", Doc: "Longitude, if no pincode."},
				},
			},
		},
	},
	{
		ID:        "airports",
		Title:     "Airports",
		Tagline:   "Airport codes to airports, and airports by city.",
		Intro:     "Every airport in the world with an IATA code, plus every Indian airfield and heliport, from OurAirports. Indian airports carry their state code.",
		SourceKey: "airports",
		Endpoints: []endpoint{
			{
				Path: "/v1/airports/{code}", Summary: "Airport for a code",
				Example: "/v1/airports/MAA",
				Params:  []param{{Name: "code", In: "path", Type: "string", Required: true, Doc: "IATA code (MAA) or ICAO code (VOMM)."}},
			},
			{
				Path: "/v1/airports", Summary: "Search airports",
				Doc:     "By name, city or code; airports with scheduled flights and larger airports first.",
				Example: "/v1/airports?q=chennai",
				Params: []param{
					{Name: "q", In: "query", Type: "string", Doc: "Name, city or code, 2 or more characters."},
					{Name: "country", In: "query", Type: "string", Doc: "Two-letter ISO country code, such as IN."},
					stateFilter,
					{Name: "type", In: "query", Type: "string", Enum: airportTypes, Doc: "Kind of airfield."},
					limitParam, offsetParam,
				},
			},
		},
	},
	{
		ID:        "thirukkural",
		Title:     "Thirukkural",
		Tagline:   "All 1,330 kurals in Tamil with English translations.",
		Intro:     "Thiruvalluvar's Thirukkural with a transliteration, G.U. Pope's verse translation (1886) and the prose translation by W.H. Drew and John Lazarus (1840, 1885), organised into 3 paals, 13 iyals and 133 chapters. Only text that is free to share is served; modern commentaries are added once their release into the public domain is confirmed.",
		SourceKey: "thirukkural",
		Endpoints: []endpoint{
			{
				Path: "/v1/thirukkural/{number}", Summary: "A kural by number",
				Example: "/v1/thirukkural/1",
				Params:  []param{{Name: "number", In: "path", Type: "integer", Required: true, Doc: "1 to 1330."}},
			},
			{
				Path: "/v1/thirukkural", Summary: "Search kurals",
				Doc:     "Finds kurals containing the text, in Tamil, the transliteration or either translation; or lists a chapter or a paal.",
				Example: "/v1/thirukkural?q=rain",
				Params: []param{
					{Name: "q", In: "query", Type: "string", Doc: "Text to find, 2 or more characters, Tamil or English."},
					{Name: "chapter", In: "query", Type: "integer", Doc: "Chapter (அதிகாரம்), 1 to 133."},
					{Name: "paal", In: "query", Type: "integer", Doc: "1 virtue (அறத்துப்பால்), 2 wealth (பொருட்பால்), 3 love (காமத்துப்பால்)."},
					limitParam, offsetParam,
				},
			},
			{
				Path: "/v1/thirukkural/today", Summary: "Kural of the day",
				Doc:     "The same kural for everyone on a day in India, a different one the next day.",
				Example: "/v1/thirukkural/today?date=2026-10-13",
				Params:  []param{{Name: "date", In: "query", Type: "string", Doc: "A date, YYYY-MM-DD. Default today in India."}},
			},
			{Path: "/v1/thirukkural/random", Summary: "A random kural", Example: "/v1/thirukkural/random"},
			{Path: "/v1/thirukkural/chapters", Summary: "All 133 chapters", Example: "/v1/thirukkural/chapters"},
			{
				Path: "/v1/thirukkural/chapters/{number}", Summary: "One chapter",
				Example: "/v1/thirukkural/chapters/2",
				Params:  []param{{Name: "number", In: "path", Type: "integer", Required: true, Doc: "1 to 133."}},
			},
		},
	},
	{
		ID:      "text",
		Title:   "Text tools",
		Tagline: "Chunk documents, redact Indian identifiers, count tokens, detect language.",
		Intro:   "Tools for working with text, especially in AI and retrieval pipelines. Send short text as ?text=, or POST it as text/plain, or as JSON {\"text\": ...} with the options beside it. Nothing you send is logged or kept. Each request counts as 10 against the rate limit.",
		Endpoints: []endpoint{
			{
				Path: "/v1/text/chunk", Summary: "Split a document into chunks", Post: true, Private: true,
				Doc:         "Splits at headings, paragraphs, sentences and words, in that order of preference, so chunks stay readable. Markdown chunks never cross a heading and carry their heading path. Each chunk is an exact slice of the input with its character offsets.",
				Body:        "The document, plain text or Markdown.",
				Example:     "/v1/text/chunk?size=200&overlap=20&text=" + url.QueryEscape(chunkExample),
				ExampleBody: chunkExample,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "The text (or POST it)."},
					{Name: "size", In: "query", Type: "integer", Doc: "Largest chunk in characters, overlap included, 100 to 20000. Default 1000."},
					{Name: "overlap", In: "query", Type: "integer", Doc: "Characters repeated from the previous chunk, up to half the size. Default a tenth of the size."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"auto", "markdown", "text"}, Doc: "Default auto: Markdown if the text has headings or code fences."},
				},
			},
			{
				Path: "/v1/text/redact", Summary: "Redact Indian personal identifiers", Post: true, Private: true,
				Doc:         "Masks Aadhaar numbers (Verhoeff-checked), PAN, GSTIN (checksum-checked), card numbers (Luhn-checked), Indian mobile numbers, email addresses, UPI IDs, vehicle numbers, voter IDs, passport numbers and IP addresses. Returns where each was found, never the values. Names and addresses are not detected.",
				Body:        "The text to redact.",
				Example:     "/v1/text/redact?text=" + url.QueryEscape(redactExample),
				ExampleBody: redactExample,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "The text (or POST it)."},
					{Name: "types", In: "query", Type: "string", Doc: "Comma-separated kinds to redact; default all. Kinds: " + strings.Join(textkit.KindIDs(), ", ") + "."},
					{Name: "style", In: "query", Type: "string", Enum: []string{"label", "partial"}, Doc: "label replaces with [PAN]; partial keeps the last digits, as on a bank statement. Default label."},
				},
			},
			{
				Path: "/v1/text/tokens", Summary: "Count tokens", Post: true, Private: true,
				Doc:         "Exact token counts with OpenAI tokenizers, by encoding or model name. Other vendors do not publish their tokenizers, so their counts are not offered.",
				Body:        "The text to count.",
				Example:     "/v1/text/tokens?model=gpt-4o&text=" + url.QueryEscape(tokensExample),
				ExampleBody: tokensExample,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "The text (or POST it)."},
					{Name: "model", In: "query", Type: "string", Doc: "Model name, such as gpt-4o or gpt-4, or an encoding: " + strings.Join(textkit.EncodingNames(), ", ") + ". Default o200k_base."},
				},
			},
			{
				Path: "/v1/text/language", Summary: "Detect the language", Post: true, Private: true,
				Doc:         "Language and script, with a confidence. Covers about 80 languages including Hindi, Bengali, Marathi, Gujarati, Punjabi, Tamil, Telugu, Kannada, Malayalam, Odia and Urdu. Romanised Indian text (Hinglish, Tanglish) is flagged as such.",
				Body:        "The text.",
				Example:     "/v1/text/language?text=" + url.QueryEscape(languageExample),
				ExampleBody: languageExample,
				Params:      []param{{Name: "text", In: "query", Type: "string", Doc: "The text (or POST it)."}},
			},
		},
	},
	{
		ID:      "transliterate",
		Title:   "Transliteration",
		Tagline: "Rewrite text between Indian scripts and Roman letters.",
		Intro:   "Converts by rule between Devanagari, Bengali, Gurmukhi, Gujarati, Odia, Tamil, Telugu, Kannada and Malayalam, and to ISO 15919, IAST or plain ASCII. ISO 15919 and IAST can also be turned back into any of the scripts. The spelling changes, not the language: this is not translation. Casual Roman typing (vanakkam) cannot be converted by rule and is not accepted as input.",
		Endpoints: []endpoint{
			{
				Path: "/v1/transliterate", Summary: "Transliterate text", Post: true, Private: true,
				Body:        "The text.",
				Example:     "/v1/transliterate?to=tamil&text=" + url.QueryEscape(transliterateExample),
				ExampleBody: transliterateExample,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "The text (or POST it). Letters of other scripts pass through unchanged."},
					{Name: "to", In: "query", Type: "string", Required: true, Doc: "A script (" + strings.Join(transliterationTargets()[3:], ", ") + "), a language code such as ta or hi, or iso15919, iast, ascii."},
					{Name: "from", In: "query", Type: "string", Doc: "auto (default: the script of the text), a script, or iso15919 or iast for romanised input."},
				},
			},
		},
	},
	{
		ID:      "panchang",
		Title:   "Panchang",
		Tagline: "Hindu almanac for any date and place: tithi, nakshatra, Rahu kalam and more.",
		Intro:   "Computed, not looked up, from the positions of the Sun and the Moon (within seconds of NASA JPL Horizons), with the Lahiri ayanamsa. Gives the five elements with the moments they change, sunrise, sunset, moonrise and moonset, Rahu kalam, Yamagandam, Gulika kalam and Abhijit muhurta, the Tamil month and day, the lunar month both ways it is reckoned, and the Shaka and Vikram years. Names come in English transliteration and, where the Tamil calendar has its own, in Tamil. Festival dates are not given. Counts as 10 requests against the rate limit.",
		Endpoints: []endpoint{
			{
				Path: "/v1/panchang/{date}", Summary: "Panchang for a date",
				Doc:     "For a pincode, a latitude and longitude, or (by default) New Delhi. Times are in the time zone asked for.",
				Example: "/v1/panchang/2026-01-14?pincode=600001",
				Params: []param{
					{Name: "date", In: "path", Type: "string", Required: true, Doc: "YYYY-MM-DD, 1900 to 2100, or today."},
					{Name: "pincode", In: "query", Type: "string", Doc: "Indian pincode of the place."},
					{Name: "lat", In: "query", Type: "number", Doc: "Latitude, if no pincode."},
					{Name: "lng", In: "query", Type: "number", Doc: "Longitude, if no pincode."},
					{Name: "tz", In: "query", Type: "string", Doc: "IANA time zone for the times. Default Asia/Kolkata."},
				},
			},
		},
	},
	{
		ID:        "fake",
		Title:     "Fake test data",
		Tagline:   "Made-up Indian people, companies and addresses for testing apps.",
		Intro:     "Realistic test records that pass format checks: names from nine language regions, addresses at real pincodes with the right district and state, phone numbers, PANs and GSTINs with a correct check character. Every record is marked fake and none describes a real person or business. Give a seed to get the same records every time.",
		SourceKey: "pincode",
		Endpoints: []endpoint{
			{
				Path: "/v1/fake/{kind}", Summary: "Generate fake records",
				Doc:     "people have a name, gender, birth date, email (on example domains), phone, PAN and address; companies a name, PAN, GSTIN, email, phone and address.",
				Example: "/v1/fake/people?count=3&state=TN&seed=42",
				Params: []param{
					{Name: "kind", In: "path", Type: "string", Required: true, Enum: fakeKinds, Doc: "What to generate."},
					{Name: "count", In: "query", Type: "integer", Doc: "How many, 1 to 100. Default 10."},
					stateFilter,
					{Name: "seed", In: "query", Type: "integer", Doc: "The same seed gives the same records. Without one, each request differs."},
				},
			},
		},
	},
	{
		ID:      "images",
		Title:   "Generated images",
		Tagline: "Social preview images and code screenshots from a URL.",
		Intro:   "Pictures made from text, with no browser involved: link-preview (Open Graph) cards and syntax-coloured code images. Latin, Devanagari, Bengali, Gurmukhi, Gujarati, Odia, Tamil, Telugu, Kannada and Malayalam are shaped properly (conjuncts, vowel signs) and can be mixed in one line; other scripts come out as empty boxes. PNG by default; format=svg gives the same picture as an SVG whose text is drawn as outlines, so it needs no fonts where it is shown. Text and colours only: no images are fetched from anywhere. Each request counts as 10 against the rate limit.",
		Endpoints: []endpoint{
			{
				Path: "/v1/og-image", Summary: "Social preview image", Image: true,
				Doc:     "A 1200 by 630 card for og:image and twitter:image: a title (set as large as fits in three lines, cut with an ellipsis beyond that), an optional subtitle of up to two lines, and a site name and tag along the bottom. The same URL always gives the same picture, so it can be used directly in a meta tag.",
				Example: "/v1/og-image?title=Free%20APIs%20for%20Indian%20developers&subtitle=Pincodes%2C%20IFSC%2C%20holidays%20and%20more&site=api.dharun.dev",
				Params: []param{
					{Name: "title", In: "query", Type: "string", Required: true, Doc: "Up to 200 characters, in any script."},
					{Name: "subtitle", In: "query", Type: "string", Doc: "Up to 300 characters; two lines are shown."},
					{Name: "site", In: "query", Type: "string", Doc: "Site name, bottom left."},
					{Name: "tag", In: "query", Type: "string", Doc: "Short label, bottom right."},
					{Name: "theme", In: "query", Type: "string", Enum: []string{"dark", "light"}, Doc: "Default dark."},
					{Name: "bg", In: "query", Type: "string", Doc: "Background colour, hex."},
					{Name: "fg", In: "query", Type: "string", Doc: "Title colour, hex."},
					{Name: "accent", In: "query", Type: "string", Doc: "Accent colour, hex."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"png", "svg"}, Doc: "Default png."},
				},
			},
			{
				Path: "/v1/code-image", Summary: "Code as an image", Image: true, Post: true, Private: true,
				Doc:         "A code snippet as a window with syntax colouring. Up to 150 lines of 140 characters. PNGs are drawn at twice the pixel density for sharp text (very large ones at single). The language is guessed from the title's file extension or the code when not given, and reported in the X-Code-Language header.",
				Body:        "The code.",
				Example:     "/v1/code-image?language=go&title=main.go&code=" + url.QueryEscape(codeExample),
				ExampleBody: codeExample,
				Params: []param{
					{Name: "code", In: "query", Type: "string", Doc: "The code (or POST it)."},
					{Name: "language", In: "query", Type: "string", Doc: "Language name or file extension, such as go, python, js, sql."},
					{Name: "title", In: "query", Type: "string", Doc: "File name shown in the title bar."},
					{Name: "theme", In: "query", Type: "string", Enum: []string{"dark", "light"}, Doc: "Default dark."},
					{Name: "lineNumbers", In: "query", Type: "string", Enum: []string{"true"}, Doc: "Show line numbers."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"png", "svg"}, Doc: "Default png."},
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

// codeImageParams are the drawing options every code endpoint takes.
func codeImageParams(margin int) []param {
	marginDoc := "Light border, in modules, 0 to 16. The default depends on the code."
	if margin > 0 {
		marginDoc = "Light border (quiet zone), in modules, 0 to 16. Default " + strconv.Itoa(margin) + "."
	}
	return []param{
		{Name: "format", In: "query", Type: "string", Enum: []string{"svg", "png"}, Doc: "Image format. Default svg."},
		{Name: "size", In: "query", Type: "integer", Doc: "2D codes: width in pixels to stay within, 32 to 2048. Default 256."},
		{Name: "margin", In: "query", Type: "integer", Doc: marginDoc},
		{Name: "fg", In: "query", Type: "string", Doc: "Dark colour, hex such as 000000. Default black."},
		{Name: "bg", In: "query", Type: "string", Doc: "Light colour, hex, or transparent. Default white."},
	}
}
