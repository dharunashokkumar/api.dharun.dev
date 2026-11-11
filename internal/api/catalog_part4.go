package api

import (
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/calc"
)

// The catalog of growth plan Part 4: the calculators, batch lookups, today in India and
// the pictures (avatars, placeholders, charts), which join the generated images group.

const batchExample = `{
  "kind": "pincode",
  "values": ["637001", "600001", "110001"]
}`

var pictureFormatParam = param{Name: "format", In: "query", Type: "string", Enum: []string{"svg", "png"}, Doc: "svg (default) or png."}

func withPart4(groups []apiGroup) []apiGroup {
	for i := range groups {
		if groups[i].ID == "images" {
			groups[i].Tagline = "Share images, code screenshots, avatars, placeholders and charts from a URL."
			groups[i].Endpoints = append(groups[i].Endpoints,
				endpoint{
					Path: "/v1/avatar/{name}", Summary: "Avatar with initials", Image: true,
					Doc:     "A round (or square) tile with a person's initials, its colour fixed by the name, for user lists and comments. Initials in Indian scripts keep their vowel signs and conjuncts. SVG by default; a .png suffix or format=png gives PNG.",
					Example: "/v1/avatar/Priya%20Raman",
					Params: []param{
						{Name: "name", In: "path", Type: "string", Required: true, Doc: "The name, in any script. The first letters of its first and last words are drawn."},
						{Name: "size", In: "query", Type: "integer", Doc: "Width and height in pixels, 16 to 1024. Default 128."},
						{Name: "shape", In: "query", Type: "string", Enum: []string{"circle", "square", "rounded"}, Doc: "Default circle."},
						{Name: "bg", In: "query", Type: "string", Doc: "Background colour as hex, such as 2563eb. Default: picked from the name."},
						{Name: "fg", In: "query", Type: "string", Doc: "Letter colour as hex. Default white."},
						{Name: "initials", In: "query", Type: "string", Doc: "Draw these letters instead (up to 4)."},
						pictureFormatParam,
					},
				},
				endpoint{
					Path: "/v1/placeholder/{size}", Summary: "Placeholder image", Image: true,
					Doc:     "A plain box with its size (or your text) in the middle, for mock-ups and layouts.",
					Example: "/v1/placeholder/600x400",
					Params: []param{
						{Name: "size", In: "path", Type: "string", Required: true, Doc: "WIDTHxHEIGHT, or one number for a square; each 8 to 4000. A .png suffix gives PNG."},
						{Name: "text", In: "query", Type: "string", Doc: "Text to show instead of the size, up to 80 characters."},
						{Name: "bg", In: "query", Type: "string", Doc: "Background colour as hex. Default dde3ea."},
						{Name: "fg", In: "query", Type: "string", Doc: "Text colour as hex. Default 556070."},
						pictureFormatParam,
					},
				},
				endpoint{
					Path: "/v1/chart", Summary: "Chart image from numbers", Image: true, Post: true,
					Doc:         "A bar, line, pie or sparkline chart as SVG or PNG, for emails, chat bots, READMEs and anywhere JavaScript charts cannot run. Axis values are written the Indian way (K, L, Cr). In the query: values=3,5,2 (repeat values for more series, with names to name them) and labels=Jan,Feb,Mar. Counts as 10 against the rate limit.",
					Body:        `A chart as JSON: {"type": "bar", "title": "...", "labels": [...], "series": [{"name": "...", "values": [...], "color": "2563eb"}], "width": 800, "height": 450}.`,
					Example:     "/v1/chart?type=bar&title=Sales%20by%20quarter&labels=Q1,Q2,Q3,Q4&values=12,19,9,15",
					ExampleBody: `{"type": "line", "title": "Visitors", "labels": ["Mon", "Tue", "Wed"], "series": [{"name": "This week", "values": [120, 180, 150]}, {"name": "Last week", "values": [100, 140, 160]}]}`,
					Params: []param{
						{Name: "type", In: "query", Type: "string", Enum: []string{"bar", "line", "pie", "sparkline"}, Doc: "Default bar."},
						{Name: "values", In: "query", Type: "string", Doc: "Numbers separated by commas. Repeat the parameter for more series."},
						{Name: "labels", In: "query", Type: "string", Doc: "Labels for the values, separated by commas."},
						{Name: "names", In: "query", Type: "string", Doc: "A name for each series: repeated like values, or separated by commas."},
						{Name: "title", In: "query", Type: "string", Doc: "Title above the chart."},
						{Name: "width", In: "query", Type: "integer", Doc: "Pixels, 40 to 3000. Default 800 (160 for a sparkline)."},
						{Name: "height", In: "query", Type: "integer", Doc: "Pixels, 20 to 3000. Default 450 (40 for a sparkline)."},
						{Name: "theme", In: "query", Type: "string", Enum: []string{"light", "dark"}, Doc: "Default light."},
						pictureFormatParam,
					},
				},
			)
		}
	}
	return append(groups,
		apiGroup{
			ID: "calculators", Category: "money", Title: "Money calculators",
			Tagline: "SIP, EMI, GST, income tax, HRA, gratuity, PPF, capital gains and trade charges.",
			Intro:   "Indian money sums with the rules written in: each answer names the rule and the year it used. Income tax knows the financial years " + strings.Join(calc.TaxYears(), " and ") + ", both regimes. For planning: not tax or financial advice. Amounts are in rupees; commas are allowed.",
			Endpoints: []endpoint{
				{Path: "/v1/calc/sip", Summary: "SIP returns", Reference: true,
					Doc:     "What a monthly investment grows to, year by year, with an optional yearly step-up of the amount.",
					Example: "/v1/calc/sip?monthly=10000&rate=12&years=10",
					Params: []param{{Name: "monthly", In: "query", Type: "number", Required: true, Doc: "Monthly amount."}, {Name: "rate", In: "query", Type: "number", Required: true, Doc: "Expected return, percent a year."},
						{Name: "years", In: "query", Type: "integer", Required: true, Doc: "1 to 60."}, {Name: "stepup", In: "query", Type: "number", Doc: "Yearly increase of the monthly amount, percent."}}},
				{Path: "/v1/calc/lumpsum", Summary: "Lump sum growth", Reference: true,
					Example: "/v1/calc/lumpsum?amount=100000&rate=10&years=10",
					Params: []param{{Name: "amount", In: "query", Type: "number", Required: true, Doc: "Amount invested once."}, {Name: "rate", In: "query", Type: "number", Required: true, Doc: "Expected return, percent a year."},
						{Name: "years", In: "query", Type: "integer", Required: true, Doc: "1 to 60."}}},
				{Path: "/v1/calc/emi", Summary: "Loan EMI and schedule", Reference: true,
					Doc:     "The monthly instalment of a reducing-balance loan, the total interest and, with schedule=true, every month's split of interest and principal.",
					Example: "/v1/calc/emi?amount=1000000&rate=8.5&years=20",
					Params: []param{{Name: "amount", In: "query", Type: "number", Required: true, Doc: "Loan amount."}, {Name: "rate", In: "query", Type: "number", Required: true, Doc: "Interest, percent a year."},
						{Name: "years", In: "query", Type: "integer", Doc: "Term in years (or give months)."}, {Name: "months", In: "query", Type: "integer", Doc: "Term in months, 1 to 600."},
						{Name: "schedule", In: "query", Type: "boolean", Doc: "true adds the month-by-month schedule."}}},
				{Path: "/v1/calc/gst", Summary: "GST split", Reference: true,
					Doc:     "Adds GST to an amount, or takes it out of one that includes it, split into CGST and SGST within a state or IGST between states.",
					Example: "/v1/calc/gst?amount=1180&rate=18&inclusive=true",
					Params: []param{{Name: "amount", In: "query", Type: "number", Required: true, Doc: "The amount."}, {Name: "rate", In: "query", Type: "number", Required: true, Doc: "GST rate: 0, 0.1, 0.25, 1.5, 3, 5, 12, 18, 28 or 40."},
						{Name: "inclusive", In: "query", Type: "boolean", Doc: "true when the amount already includes GST."}, {Name: "interstate", In: "query", Type: "boolean", Doc: "true for a supply between states (IGST)."}}},
				{Path: "/v1/calc/income-tax", Summary: "Income tax, old and new regime", Reference: true,
					Doc:     "Tax on a year's income under the new regime (section 115BAC) and the old one, with the section 87A rebate, surcharge, marginal relief and 4% cess, slab by slab, and which regime costs less. Special-rate income such as capital gains is not included.",
					Example: "/v1/calc/income-tax?income=1500000&year=2025-26",
					Params: []param{{Name: "income", In: "query", Type: "number", Required: true, Doc: "Gross total income for the year, before the standard deduction."},
						{Name: "year", In: "query", Type: "string", Enum: calc.TaxYears(), Doc: "Financial year. Default the newest."},
						{Name: "regime", In: "query", Type: "string", Enum: []string{"both", "new", "old"}, Doc: "Default both."},
						{Name: "age", In: "query", Type: "string", Enum: []string{"below60", "60to80", "80plus"}, Doc: "Changes the old regime's exemption limit. Default below60."},
						{Name: "salaried", In: "query", Type: "boolean", Doc: "Salary or pension, so the standard deduction applies. Default true."},
						{Name: "deductions", In: "query", Type: "number", Doc: "Old regime deductions and exemptions in all (80C, 80D, HRA, home loan interest...)."}}},
				{Path: "/v1/calc/hra", Summary: "HRA exemption", Reference: true,
					Doc:     "The tax-free part of house rent allowance under section 10(13A). Give every amount for the same period (a month or a year).",
					Example: "/v1/calc/hra?basic=50000&hra=25000&rent=20000&metro=true",
					Params: []param{{Name: "basic", In: "query", Type: "number", Required: true, Doc: "Basic pay plus dearness allowance."}, {Name: "hra", In: "query", Type: "number", Required: true, Doc: "HRA received."},
						{Name: "rent", In: "query", Type: "number", Required: true, Doc: "Rent paid."}, {Name: "metro", In: "query", Type: "boolean", Doc: "true in Delhi, Mumbai, Kolkata or Chennai."}}},
				{Path: "/v1/calc/gratuity", Summary: "Gratuity", Reference: true,
					Example: "/v1/calc/gratuity?salary=52000&years=10.6",
					Params: []param{{Name: "salary", In: "query", Type: "number", Required: true, Doc: "Last monthly basic pay plus dearness allowance."}, {Name: "years", In: "query", Type: "number", Required: true, Doc: "Years of service; 10.6 is ten years and about seven months."},
						{Name: "covered", In: "query", Type: "boolean", Doc: "Covered by the Payment of Gratuity Act. Default true."}}},
				{Path: "/v1/calc/ppf", Summary: "PPF maturity", Reference: true,
					Example: "/v1/calc/ppf?yearly=150000&years=15",
					Params: []param{{Name: "yearly", In: "query", Type: "number", Required: true, Doc: "Yearly deposit, 500 to 1,50,000."}, {Name: "years", In: "query", Type: "integer", Doc: "15, or 15 plus blocks of 5. Default 15."},
						{Name: "rate", In: "query", Type: "number", Doc: "Interest, percent a year. Default the current rate."}}},
				{Path: "/v1/calc/capital-gains", Summary: "Capital gains tax", Reference: true,
					Doc:     "Short or long term, and the tax at the rates for sales from 23 July 2024: shares and equity funds, debt funds, property (with the indexation choice for land and buildings bought before that date), gold and other assets.",
					Example: "/v1/calc/capital-gains?asset=equity&bought=2023-01-10&sold=2025-01-10&cost=100000&price=400000",
					Params: []param{{Name: "asset", In: "query", Type: "string", Required: true, Enum: []string{"equity", "equity-fund", "debt-fund", "property", "gold", "other"}, Doc: "What was sold."},
						{Name: "bought", In: "query", Type: "string", Required: true, Doc: "Purchase date, YYYY-MM-DD."}, {Name: "sold", In: "query", Type: "string", Required: true, Doc: "Sale date, from 2024-07-23."},
						{Name: "cost", In: "query", Type: "number", Required: true, Doc: "Purchase cost with improvement costs."}, {Name: "price", In: "query", Type: "number", Required: true, Doc: "Sale price."},
						{Name: "expenses", In: "query", Type: "number", Doc: "Costs of the sale."}}},
				{Path: "/v1/calc/trade", Summary: "Share trade charges", Reference: true,
					Doc:     "Every charge on buying and selling an NSE share (STT, exchange charges, SEBI fee, stamp duty, GST), the net profit and the move needed to break even.",
					Example: "/v1/calc/trade?buy=1000&sell=1100&quantity=100&kind=delivery&brokerage=20",
					Params: []param{{Name: "buy", In: "query", Type: "number", Required: true, Doc: "Buy price per share."}, {Name: "sell", In: "query", Type: "number", Required: true, Doc: "Sell price per share."},
						{Name: "quantity", In: "query", Type: "integer", Required: true, Doc: "Shares."}, {Name: "kind", In: "query", Type: "string", Enum: []string{"delivery", "intraday"}, Doc: "Default delivery."},
						{Name: "brokerage", In: "query", Type: "number", Doc: "Your broker's charge per order, in rupees."}, {Name: "dp", In: "query", Type: "number", Doc: "Depository charge on a delivery sale."}}},
			},
		},
		apiGroup{
			ID: "batch", Category: "developer", Title: "Batch lookups",
			Tagline: "Up to 1,000 lookups in one request.",
			Intro:   "Pincodes, IFSC codes, banks, phone numbers, RTO codes, HSN codes, companies, states, airports, stations, GSTIN and PAN checks and number to words, many at once. Each answer is exactly what the lookup's own endpoint returns, in the order sent. Ten lookups cost one unit of the rate limit. What you send is never logged.",
			Endpoints: []endpoint{{
				Path: "/v1/batch", Summary: "Many lookups in one request", Private: true,
				Upload:      &upload{Accepts: `JSON: {"kind": "pincode", "values": [...]} for one kind, or {"lookups": [{"kind": "ifsc", "value": "..."}, ...]} for a mix. Kinds: ` + strings.Join(batchKindNames(), ", ") + ". Or a CSV file (text/csv) to enrich.", Types: []string{"application/json", "text/csv"}, JSON: true},
				Doc:         "Answers { data: [{kind, value, status, data or error}], meta }. With format=ndjson, one JSON line per answer instead. Send a CSV file as text/csv with kind (and column, the header of the column holding the values; by default the column named like the kind) to get the same file back with each answer's fields added as columns: a spreadsheet of pincodes comes back with district and state.",
				Example:     "/v1/batch",
				ExampleBody: batchExample,
				Params: []param{{Name: "format", In: "query", Type: "string", Enum: []string{"json", "ndjson", "csv"}, Doc: "Default json; csv with a CSV body."},
					{Name: "kind", In: "query", Type: "string", Doc: "With a CSV body: the lookup, such as pincode or ifsc."},
					{Name: "column", In: "query", Type: "string", Doc: "With a CSV body: the header of the column holding the values."}},
			}},
		},
		apiGroup{
			ID: "today", Category: "dates", Title: "Today in India",
			Tagline:   "India's \"now\" in one call: date, financial year, working days, holidays, market and panchang.",
			Intro:     "For agents and widgets: the date and time in IST, the financial year, quarter and assessment year, whether today is a bank (per state), market and central government working day, today's holidays, whether the market is open, and with a pincode the sunrise and the day's panchang in brief.",
			SourceKey: "holidays-bank", MoreSources: []string{"holidays-market", "holidays-central"},
			Endpoints: []endpoint{{
				Path: "/v1/today", Summary: "Today in India", Live: true,
				Doc:     "Without date it is about now, and cached for a minute. The panchang (with pincode) counts as 10 more units.",
				Example: "/v1/today?state=TN&pincode=600001",
				Params: []param{stateFilter, {Name: "pincode", In: "query", Type: "string", Doc: "Adds sunrise, sunset and the panchang for that place."},
					{Name: "date", In: "query", Type: "string", Doc: "Another day, YYYY-MM-DD, instead of today."}},
			}},
		},
	)
}
