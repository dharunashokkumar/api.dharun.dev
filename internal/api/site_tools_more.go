package api

// The next twenty tool pages (growth plan Part 3 section 3). Each is a form on one GET
// endpoint, handled by the generic handler in tools.js: fields named like the endpoint's
// parameters, {name} in the address filled from the field of that name.

import (
	"html/template"
	"strings"
)

type toolField struct {
	Name, Label, Value, Type string // Type: text (default), number, date, select, check
	Options                  []string
}

// genericForm writes a tool's form: get is the address, with {name} for path parts.
func genericForm(get string, fields []toolField, button string, extra string) string {
	var b strings.Builder
	b.WriteString(`<form class="card form" data-tool="generic" data-get="` + template.HTMLEscapeString(get) + `"` + extra + `><div class="fields">`)
	for _, f := range fields {
		name, label, value := template.HTMLEscapeString(f.Name), template.HTMLEscapeString(f.Label), template.HTMLEscapeString(f.Value)
		switch f.Type {
		case "select":
			b.WriteString(`<label class="field"><span>` + label + `</span><select name="` + name + `">`)
			for _, o := range f.Options {
				value, text, _ := strings.Cut(o, "=")
				if text == "" {
					text = value
				}
				selected := ""
				if value == f.Value {
					selected = " selected"
				}
				b.WriteString(`<option value="` + template.HTMLEscapeString(value) + `"` + selected + `>` + template.HTMLEscapeString(text) + `</option>`)
			}
			b.WriteString(`</select></label>`)
		case "check":
			checked := ""
			if f.Value == "true" {
				checked = " checked"
			}
			b.WriteString(`<label class="field"><span>` + label + `</span><input type="checkbox" name="` + name + `"` + checked + `></label>`)
		default:
			kind := `type="text"`
			switch f.Type {
			case "number":
				kind = `type="text" inputmode="decimal"`
			case "date":
				kind = `type="date"`
			}
			b.WriteString(`<label class="field"><span>` + label + `</span><input ` + kind + ` name="` + name + `" value="` + value + `"></label>`)
		}
	}
	b.WriteString(`</div><div class="row"><button class="key key--primary" type="submit">` + template.HTMLEscapeString(button) + `</button></div></form>`)
	return b.String()
}

func init() {
	years := []string{"2026-27", "2025-26"}
	tools = append(tools,
		tool{Slug: "gst-calculator", Title: "GST calculator: add or remove GST, with CGST and SGST split", Heading: "GST calculator",
			Lead:        "Add GST to a price or take it out of a price that includes it, with the CGST and SGST (or IGST) split.",
			Description: "Free GST calculator for India: add GST to an amount or remove GST from an inclusive price, at 5%, 12%, 18% or 28%, with the CGST, SGST and IGST split.",
			About: []string{"GST inclusive means the price already has the tax in it: the tax is the price times rate over (100 plus rate). Exclusive means the tax is added on top.",
				"Within a state the tax is split equally into CGST and SGST; between states it is charged whole as IGST."},
			FAQ:  []faq{{"Which GST rate applies to my product?", "It depends on the HSN or SAC code. Look the code up with the HSN code finder, which shows the rate for most goods and services."}},
			Uses: []string{"calculators"}, Calls: []string{"GET /v1/calc/gst"},
			UI: genericForm("/v1/calc/gst", []toolField{{Name: "amount", Label: "Amount in rupees", Value: "1180", Type: "number"},
				{Name: "rate", Label: "GST rate", Value: "18", Type: "select", Options: []string{"0.25=0.25%", "3=3%", "5=5%", "12=12%", "18=18%", "28=28%", "40=40%"}},
				{Name: "inclusive", Label: "The amount already includes GST", Value: "true", Type: "check"},
				{Name: "interstate", Label: "Between states (IGST)", Type: "check"}}, "Work it out", "")},
		tool{Slug: "emi-calculator", Title: "EMI calculator: home, car and personal loan EMI with interest", Heading: "EMI calculator",
			Lead:        "Your monthly instalment, the total interest and the total you repay, for any loan.",
			Description: "Free EMI calculator for home loans, car loans and personal loans in India: monthly EMI, total interest and total payment from amount, rate and tenure.",
			About: []string{"An EMI (equated monthly instalment) stays the same each month; early instalments are mostly interest and later ones mostly principal.",
				"The calculation uses the standard reducing-balance formula banks use. Processing fees and insurance are not included."},
			FAQ:  []faq{{"Does a longer tenure lower my cost?", "It lowers the EMI but raises the total interest. Compare both numbers before choosing."}},
			Uses: []string{"calculators"}, Calls: []string{"GET /v1/calc/emi"},
			UI: genericForm("/v1/calc/emi", []toolField{{Name: "amount", Label: "Loan amount in rupees", Value: "1000000", Type: "number"},
				{Name: "rate", Label: "Interest rate, % a year", Value: "8.5", Type: "number"}, {Name: "years", Label: "Tenure in years", Value: "20", Type: "number"}}, "Work out the EMI", "")},
		tool{Slug: "sip-calculator", Title: "SIP calculator: future value of a monthly mutual fund SIP", Heading: "SIP calculator",
			Lead:        "What a monthly SIP could grow to, with an optional yearly step-up.",
			Description: "Free SIP calculator: the future value of a monthly systematic investment plan at an expected return, with total invested, gains, and an optional annual step-up.",
			About:       []string{"The result assumes the same return every year, which no fund delivers; treat it as a guide, not a promise. Mutual fund returns are not guaranteed."},
			FAQ:         []faq{{"What return should I assume?", "Many planners use 10 to 12% for equity funds over long periods and 6 to 7% for debt funds. Check a fund's actual history with the mutual fund NAV tool."}},
			Uses:        []string{"calculators"}, Calls: []string{"GET /v1/calc/sip"},
			UI: genericForm("/v1/calc/sip", []toolField{{Name: "monthly", Label: "Monthly amount in rupees", Value: "10000", Type: "number"},
				{Name: "rate", Label: "Expected return, % a year", Value: "12", Type: "number"}, {Name: "years", Label: "Years", Value: "10", Type: "number"},
				{Name: "stepup", Label: "Yearly step-up, % (optional)", Type: "number"}}, "Work it out", "")},
		tool{Slug: "income-tax-calculator", Title: "Income tax calculator: new vs old regime, with rebate and cess", Heading: "Income tax calculator",
			Lead:        "Your income tax under the new and old regimes side by side, with the 87A rebate, surcharge and cess.",
			Description: "Free income tax calculator for India: compare the new and old tax regimes for FY 2026-27 and 2025-26, with standard deduction, 87A rebate, surcharge and 4% cess.",
			About: []string{"The new regime has lower rates and few deductions; the old one allows 80C, 80D, HRA and others. The cheaper one depends on how much you can deduct.",
				"Figures follow the Finance Act for the year chosen. For a return, check with the income tax portal or a tax professional."},
			FAQ:  []faq{{"What goes in deductions?", "For the old regime: the total of 80C, 80D, home loan interest and the like, beyond the standard deduction, which is applied for salaried people automatically."}},
			Uses: []string{"calculators"}, Calls: []string{"GET /v1/calc/income-tax"},
			UI: genericForm("/v1/calc/income-tax", []toolField{{Name: "income", Label: "Yearly income in rupees", Value: "1500000", Type: "number"},
				{Name: "year", Label: "Financial year", Value: "2026-27", Type: "select", Options: years},
				{Name: "deductions", Label: "Deductions for the old regime (optional)", Type: "number"},
				{Name: "age", Label: "Age", Value: "below60", Type: "select", Options: []string{"below60=Below 60", "60to80=60 to 80", "80plus=80 and above"}},
				{Name: "salaried", Label: "Salaried", Value: "true", Type: "check"}}, "Work out the tax", "")},
		tool{Slug: "hra-calculator", Title: "HRA exemption calculator for salaried employees", Heading: "HRA exemption calculator",
			Lead:        "How much of your house rent allowance is free of tax under section 10(13A).",
			Description: "Free HRA exemption calculator: the tax-free part of house rent allowance from basic salary, HRA received and rent paid, for metro and non-metro cities.",
			About:       []string{"The exemption is the least of three: the HRA received, the rent paid minus 10% of basic, and 50% of basic in Delhi, Mumbai, Kolkata or Chennai (40% elsewhere). It applies only in the old regime."},
			Uses:        []string{"calculators"}, Calls: []string{"GET /v1/calc/hra"},
			UI: genericForm("/v1/calc/hra", []toolField{{Name: "basic", Label: "Basic salary a month", Value: "50000", Type: "number"},
				{Name: "hra", Label: "HRA received a month", Value: "25000", Type: "number"}, {Name: "rent", Label: "Rent paid a month", Value: "20000", Type: "number"},
				{Name: "metro", Label: "Delhi, Mumbai, Kolkata or Chennai", Value: "true", Type: "check"}}, "Work it out", "")},
		tool{Slug: "gratuity-calculator", Title: "Gratuity calculator: amount under the Payment of Gratuity Act", Heading: "Gratuity calculator",
			Lead:        "The gratuity due when you leave, from your last salary and years of service.",
			Description: "Free gratuity calculator for India: gratuity under the Payment of Gratuity Act from last drawn basic plus DA and years of service, and the tax-free limit.",
			About:       []string{"For an employer covered by the Act, gratuity is 15 days' salary for each year of service, a month counted as 26 days. Five years of service are needed, except on death or disability."},
			Uses:        []string{"calculators"}, Calls: []string{"GET /v1/calc/gratuity"},
			UI: genericForm("/v1/calc/gratuity", []toolField{{Name: "salary", Label: "Last basic plus DA a month", Value: "52000", Type: "number"},
				{Name: "years", Label: "Years of service", Value: "10.6", Type: "number"}, {Name: "covered", Label: "Employer covered by the Act", Value: "true", Type: "check"}}, "Work it out", "")},
		tool{Slug: "ppf-calculator", Title: "PPF calculator: maturity value of a Public Provident Fund account", Heading: "PPF calculator",
			Lead:        "What your PPF account grows to over 15 years or more at the current rate.",
			Description: "Free PPF calculator: maturity amount, total deposits and interest of a Public Provident Fund account from yearly deposits, at the current government rate.",
			About:       []string{"PPF interest is set by the government each quarter and compounded yearly. Deposits are up to ₹1.5 lakh a year; the account runs 15 years and can be extended in blocks of 5."},
			Uses:        []string{"calculators"}, Calls: []string{"GET /v1/calc/ppf"},
			UI: genericForm("/v1/calc/ppf", []toolField{{Name: "yearly", Label: "Yearly deposit in rupees", Value: "150000", Type: "number"},
				{Name: "years", Label: "Years", Value: "15", Type: "number"}, {Name: "rate", Label: "Rate, % (blank for the current one)", Type: "number"}}, "Work it out", "")},
		tool{Slug: "gstin-checker", Title: "GSTIN checker: check a GST number's format and state", Heading: "GSTIN checker",
			Lead:        "Check that a GSTIN is well formed: its check character, state code and the PAN inside it.",
			Description: "Free GSTIN checker: verify the format and check character of a GST identification number and see its state and the PAN it holds.",
			About: []string{"A GSTIN is 15 characters: a two-digit state code, the holder's PAN, an entity number, the letter Z and a check character worked out from the rest. A typing mistake almost always breaks the check character.",
				"This checks the number's structure only. Whether it is registered and active is shown on the GST portal's taxpayer search."},
			Uses: []string{"validate"}, Calls: []string{"GET /v1/validate/gstin/{value}"},
			UI: genericForm("/v1/validate/{kind}/{value}", []toolField{{Name: "kind", Label: "Kind", Value: "gstin", Type: "select", Options: []string{"gstin=GSTIN"}}, {Name: "value", Label: "GSTIN", Value: "33AABCS1429B1Z1"}}, "Check", "")},
		tool{Slug: "pan-checker", Title: "PAN format checker: check a PAN card number", Heading: "PAN checker",
			Lead:        "Check that a PAN is well formed and see what kind of holder it is for.",
			Description: "Free PAN checker: verify the format of a Permanent Account Number and see the holder type (person, company, firm, trust and so on).",
			About: []string{"A PAN is ten characters: five letters, four digits, a letter. The fourth letter tells the kind of holder: P for a person, C for a company, F for a firm, and so on.",
				"This checks the structure only, not that the PAN was issued."},
			Uses: []string{"validate"}, Calls: []string{"GET /v1/validate/pan/{value}"},
			UI: genericForm("/v1/validate/{kind}/{value}", []toolField{{Name: "kind", Label: "Kind", Value: "pan", Type: "select", Options: []string{"pan=PAN"}}, {Name: "value", Label: "PAN", Value: "ABCPS1234D"}}, "Check", "")},
		tool{Slug: "hsn-code-finder", Title: "HSN and SAC code finder with GST rates", Heading: "HSN code finder",
			Lead:        "Find the HSN code of goods or the SAC code of a service by its name.",
			Description: "Free HSN and SAC code finder: search goods and services by name and get the HSN or SAC code and description for GST invoices and returns.",
			About:       []string{"Invoices above a turnover limit must show HSN codes (goods) or SAC codes (services). Search by a plain word such as tea, steel or repair."},
			Uses:        []string{"hsn"}, Calls: []string{"GET /v1/hsn"},
			UI: genericForm("/v1/hsn", []toolField{{Name: "q", Label: "Goods or service", Value: "green tea"},
				{Name: "kind", Label: "Kind", Type: "select", Options: []string{"=Both", "hsn=Goods (HSN)", "sac=Services (SAC)"}}}, "Search", "")},
		tool{Slug: "rto-code-finder", Title: "RTO code finder: which office a number plate belongs to", Heading: "RTO code finder",
			Lead:        "Type the first part of a number plate, such as TN 28 or MH 12, and see the RTO and district.",
			Description: "Free RTO code finder: find the regional transport office, district and state for a vehicle registration code such as TN-28 or KA-01.",
			About:       []string{"The first two letters of an Indian number plate are the state; the digits after them are the regional transport office that registered the vehicle."},
			Uses:        []string{"rto"}, Calls: []string{"GET /v1/rto/{code}"},
			UI: genericForm("/v1/rto/{code}", []toolField{{Name: "code", Label: "Code", Value: "TN-28"}}, "Find", "")},
		tool{Slug: "mutual-fund-nav", Title: "Mutual fund NAV today: search any Indian mutual fund scheme", Heading: "Mutual fund NAV",
			Lead:        "Today's NAV of any mutual fund scheme, by name, with direct or regular and growth or IDCW.",
			Description: "Free mutual fund NAV search: the latest NAV of every Indian mutual fund scheme from AMFI, by scheme name, plan and option.",
			About:       []string{"NAVs are published by AMFI each working evening. A direct plan has a lower expense ratio than the regular plan of the same scheme, so its NAV grows faster."},
			Uses:        []string{"mutual-funds"}, Calls: []string{"GET /v1/mutual-funds"},
			UI: genericForm("/v1/mutual-funds", []toolField{{Name: "q", Label: "Scheme name", Value: "flexi cap"},
				{Name: "plan", Label: "Plan", Value: "direct", Type: "select", Options: []string{"=Both", "direct=Direct", "regular=Regular"}},
				{Name: "option", Label: "Option", Value: "growth", Type: "select", Options: []string{"=Any", "growth=Growth", "idcw=IDCW"}}}, "Search", "")},
		tool{Slug: "gold-rate", Title: "Gold rate today: 24K, 22K and silver rates from IBJA", Heading: "Gold and silver rate today",
			Lead:        "Today's benchmark gold (24K, 22K, 18K), silver and platinum rates published by IBJA.",
			Description: "Gold rate today in India: 24 carat, 22 carat and 18 carat gold, silver and platinum benchmark rates from IBJA, per 10 grams and per gram.",
			About:       []string{"These are the India Bullion and Jewellers Association's benchmark rates before GST and making charges. Jewellers' prices are higher by those."},
			Uses:        []string{"bullion"}, Calls: []string{"GET /v1/bullion"},
			UI: genericForm("/v1/bullion", nil, "Show today's rates", "")},
		tool{Slug: "ipc-to-bns", Title: "IPC to BNS converter: find the new section for an IPC section", Heading: "IPC to BNS section finder",
			Lead:        "Type an IPC, CrPC or Evidence Act section and see the section of the new law that replaced it, or the other way round.",
			Description: "Free IPC to BNS converter: find the Bharatiya Nyaya Sanhita section for an IPC section (302 to 103), CrPC to BNSS and Evidence Act to BSA, and back.",
			About: []string{"From 1 July 2024 the Bharatiya Nyaya Sanhita replaced the IPC, the Bharatiya Nagarik Suraksha Sanhita the CrPC and the Bharatiya Sakshya Adhiniyam the Indian Evidence Act. Offences before that date stay under the old laws.",
				"The correspondence comes from the Bureau of Police Research and Development's tables. Check the Act as published before relying on a section."},
			Uses: []string{"law"}, Calls: []string{"GET /v1/law/{law}/{section}"},
			UI: genericForm("/v1/law/{law}/{section}", []toolField{{Name: "law", Label: "The section is in", Value: "ipc", Type: "select",
				Options: []string{"ipc=IPC (to BNS)", "crpc=CrPC (to BNSS)", "iea=Evidence Act (to BSA)", "bns=BNS (to IPC)", "bnss=BNSS (to CrPC)", "bsa=BSA (to Evidence Act)"}},
				{Name: "section", Label: "Section", Value: "302"}}, "Find", "")},
		tool{Slug: "mandi-prices", Title: "Mandi prices today: vegetable and grain prices in your market", Heading: "Mandi prices today",
			Lead:        "Today's wholesale prices of tomato, onion, rice and more in the agricultural markets of your state.",
			Description: "Mandi prices today from Agmarknet: minimum, maximum and modal price per quintal of vegetables, fruit, grains and pulses in markets across India.",
			About:       []string{"Prices are what markets reported to Agmarknet, in rupees per quintal (100 kg). The modal price is the one most trades were made at. Not every market reports every day."},
			Uses:        []string{"mandi"}, Calls: []string{"GET /v1/mandi"},
			UI: genericForm("/v1/mandi", []toolField{{Name: "commodity", Label: "Commodity", Value: "Tomato"}, {Name: "state", Label: "State", Value: "Tamil Nadu"},
				{Name: "district", Label: "District (optional)"}}, "Show prices", "")},
		tool{Slug: "air-quality", Title: "Air quality index (AQI) now in your city", Heading: "Air quality now",
			Lead:        "The AQI and pollutants at the monitoring stations in your city, updated every hour.",
			Description: "Air quality index now for Indian cities: AQI, category and PM2.5, PM10, NO2 and ozone levels at CPCB monitoring stations, updated hourly.",
			About:       []string{"The index follows the Central Pollution Control Board's National Air Quality Index: 0 to 50 Good, to 100 Satisfactory, to 200 Moderate, to 300 Poor, to 400 Very poor, above that Severe."},
			Uses:        []string{"air-quality"}, Calls: []string{"GET /v1/air-quality"},
			UI: genericForm("/v1/air-quality", []toolField{{Name: "city", Label: "City", Value: "Chennai"}}, "Show the air", "")},
		tool{Slug: "festivals", Title: "Indian festival dates: Diwali, Pongal, Eid and more for any year", Heading: "Festival dates",
			Lead:        "The dates of Indian festivals for a year: Hindu, Tamil, Sikh, Jain, Buddhist, Islamic and Christian.",
			Description: "Indian festival calendar for any year: Diwali, Holi, Pongal, Onam, Eid, Christmas and more, computed from the Indian calendar, with a calendar file to subscribe to.",
			About:       []string{"Hindu festivals follow the lunar calendar, so their dates move each year. They are worked out here from the positions of the sun and moon for India; Islamic dates are expected dates, as they depend on the moon being seen."},
			Uses:        []string{"festivals"}, Calls: []string{"GET /v1/festivals/{year}"},
			UI: genericForm("/v1/festivals/{year}", []toolField{{Name: "year", Label: "Year", Value: "2026", Type: "number"},
				{Name: "month", Label: "Month (optional)", Type: "select", Options: []string{"=All", "1=January", "2=February", "3=March", "4=April", "5=May", "6=June", "7=July", "8=August", "9=September", "10=October", "11=November", "12=December"}}}, "Show festivals", "")},
		tool{Slug: "thirukkural", Title: "Thirukkural with meaning in English: any kural by number", Heading: "Thirukkural",
			Lead:        "Any of the 1,330 kurals of Thiruvalluvar, in Tamil with its English translation.",
			Description: "Read Thirukkural online: any of the 1,330 couplets by number, in Tamil with the English translations of G. U. Pope and others.",
			About:       []string{"The Thirukkural has 133 chapters of 10 couplets in three books: virtue, wealth and love."},
			Uses:        []string{"thirukkural"}, Calls: []string{"GET /v1/thirukkural/{number}"},
			UI: genericForm("/v1/thirukkural/{number}", []toolField{{Name: "number", Label: "Kural number, 1 to 1330", Value: "1", Type: "number"}}, "Show the kural", "")},
		tool{Slug: "village-finder", Title: "Village finder: LGD code, taluk and district of any village", Heading: "Village finder",
			Lead:        "Find the villages under a PIN code, with their LGD codes, sub-district (taluk) and district.",
			Description: "Find Indian villages by PIN code: LGD village code, Census 2011 code, sub-district (taluk, tehsil, mandal), district and state, from the Local Government Directory.",
			About:       []string{"The Local Government Directory gives every village a code that government systems use. Towns and city wards are not villages and are not listed."},
			Uses:        []string{"lgd"}, Calls: []string{"GET /v1/lgd/villages"},
			UI: genericForm("/v1/lgd/villages", []toolField{{Name: "pincode", Label: "PIN code", Value: "637001"}}, "Find villages", "")},
		tool{Slug: "hospital-finder", Title: "Hospitals near me: find hospitals by PIN code or district", Heading: "Hospital finder",
			Lead:        "Hospitals in a PIN code or district, with their phone numbers and specialties.",
			Description: "Find hospitals in India by PIN code or district: government and private hospitals with phone, emergency number and specialties, from the National Hospital Directory.",
			About:       []string{"From the National Hospital Directory as of May 2016: call before you go. In an emergency, dial 112 or 108 for an ambulance."},
			Uses:        []string{"hospitals"}, Calls: []string{"GET /v1/hospitals"},
			UI: genericForm("/v1/hospitals", []toolField{{Name: "pincode", Label: "PIN code", Value: "637001"}}, "Find hospitals", "")},
		tool{Slug: "qr-code-generator", Title: "QR code generator: free, no sign-up, PNG or SVG", Heading: "QR code generator",
			Lead:        "Turn any text or link into a QR code you can download as PNG or SVG.",
			Description: "Free QR code generator: make a QR code for a link, text or Wi-Fi details, download as PNG or SVG, no sign-up and no tracking link.",
			About:       []string{"The QR code holds your text itself, not a redirect through another site, so it keeps working for ever and nobody counts the scans."},
			Uses:        []string{"codes"}, Calls: []string{"GET /v1/qr"},
			UI: genericForm("/v1/qr", []toolField{{Name: "text", Label: "Text or link", Value: "https://api.dharun.dev"},
				{Name: "format", Label: "Format", Value: "png", Type: "select", Options: []string{"png=PNG", "svg=SVG"}},
				{Name: "size", Label: "Size in pixels", Value: "512", Type: "number"}}, "Make the QR code", ` data-file="qr-code"`)},
	)
}
