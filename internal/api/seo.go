package api

// Search titles and descriptions written for the task people search for (growth plan
// Part 3 section 5), for the API pages most searched; the rest keep the title made from
// the catalog.

type seoText struct{ Title, Description string }

var seoTexts = map[string]seoText{
	"pincode":      {"Pincode API: post offices, district and state for any Indian PIN", "Free pincode API for India: post offices, district, state and coordinates for every PIN code, search by name, nearest pincode. JSON, no key."},
	"ifsc":         {"IFSC code API: bank branch, MICR and NEFT/RTGS/UPI support", "Free IFSC API: look up any Indian bank branch by IFSC code, with address, MICR, SWIFT and whether it takes NEFT, RTGS, IMPS and UPI. JSON, no key."},
	"holidays":     {"Bank holidays API: RBI, NSE and central holidays by state", "Free Indian holidays API: bank holidays per state from RBI, NSE and BSE market holidays and central government holidays, as JSON or a calendar feed."},
	"invoice":      {"GST invoice API: make a tax invoice PDF from JSON", "Free GST invoice API: send items and GSTINs as JSON, get a tax invoice PDF with CGST, SGST or IGST worked out, HSN summary, amount in words and UPI QR."},
	"logos":        {"Indian company logo API: NSE and BSE logos as SVG and PNG", "Free logo API for every company listed on NSE and BSE: SVG or PNG by ticker, with company details, sectors and a placeholder that never breaks."},
	"rto":          {"RTO code API: vehicle registration office for TN-28, MH-12", "Free RTO code API: the regional transport office, district and state for any Indian vehicle registration code such as TN-28 or KA-01."},
	"hsn":          {"HSN and SAC code API with descriptions for GST", "Free HSN and SAC code API: search GST classification codes for goods and services by name or number, with descriptions. JSON, no key."},
	"mutual-funds": {"Mutual fund NAV API: latest NAV, history and returns", "Free mutual fund API for India: the latest NAV of every scheme from AMFI, NAV history, point-to-point returns, CAGR and SIP returns as XIRR."},
	"words":        {"Number to words API: rupees in lakh and crore", "Free amount in words API: spell numbers and rupee amounts the Indian way, with lakh and crore, in English, Hindi and Tamil, for invoices and cheques."},
	"calculators":  {"Indian finance calculator API: EMI, SIP, GST, income tax", "Free calculator API: EMI, SIP, lumpsum, GST, income tax in both regimes, HRA, gratuity, PPF, capital gains and trade charges, as JSON."},
	"panchang":     {"Panchang API: tithi, nakshatra, rahu kalam for any place", "Free panchang API: tithi, nakshatra, yoga, karana, rahu kalam, sunrise and moonrise for any date and Indian place, in English and Tamil."},
	"ocr":          {"OCR API for Indian languages: Hindi, Tamil and 9 more", "Free OCR API: text from photos, scans and PDFs in English and ten Indian languages including Hindi, Tamil, Telugu and Bengali, with word boxes."},
	"translate":    {"Translation API for 22 Indian languages", "Free translation API between English and 22 Indian languages, running IndicTrans2 on the server. Hindi, Tamil, Telugu, Bengali and more, as JSON."},
	"law":          {"IPC to BNS API: section converter for the new criminal laws", "Free IPC to BNS converter API: the Bharatiya Nyaya Sanhita section for any IPC section, CrPC to BNSS and Evidence Act to BSA, both ways."},
	"mandi":        {"Mandi price API: daily Agmarknet commodity prices", "Free mandi price API: today's wholesale prices of vegetables, fruit, grains and pulses in Indian markets from Agmarknet, by state and district."},
	"air-quality":  {"AQI API: air quality index for Indian cities, hourly", "Free air quality API for India: AQI, category and PM2.5, PM10, NO2 and ozone at CPCB monitoring stations, by city or nearest to a point, hourly."},
	"lgd":          {"Village and LGD code API: every village in India", "Free LGD API: every state, district, sub-district and village in India with LGD and Census 2011 codes, and the villages under a PIN code."},
	"hospitals":    {"Hospital directory API: hospitals near a place in India", "Free hospital API: about 30,000 Indian hospitals with phone, specialties and position, by PIN code, district, name or nearest to a point."},
	"festivals":    {"Indian festival dates API: Diwali, Pongal, Eid and more", "Free festival calendar API: dates of Hindu, Tamil, Sikh, Jain, Buddhist, Islamic and Christian festivals in India for any year, as JSON or iCal."},
	"documents":    {"Document AI API: PDF to Markdown, tables and GST invoices", "Free document reading API: PDFs and scans to Markdown and tables, GST invoices, receipts, cheques and bank statements read and checked."},
}

func seoTitle(group apiGroup) string {
	if t, ok := seoTexts[group.ID]; ok {
		return t.Title + " | " + productName
	}
	return group.Title + " API for India, free and open | " + productName
}

func seoDescription(group apiGroup) string {
	if t, ok := seoTexts[group.ID]; ok {
		return t.Description
	}
	return shorten(group.Tagline+" "+group.Intro, 158)
}
