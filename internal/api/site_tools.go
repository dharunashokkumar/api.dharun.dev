package api

// Free tool pages for people who do not write code (growth plan Part 3 section 3):
// /tools and the first ten. Each is a real page that explains itself without
// JavaScript, answers the questions people ask about the task, and ends with the API
// it runs on. The model tools use a site token (keys.go) once keys are required.

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/traffic"
)

type tool struct {
	Slug, Title, Heading, Lead, Description string
	About                                   []string // paragraphs
	FAQ                                     []faq
	Uses                                    []string // API group IDs
	Calls                                   []string // the API addresses the page sends
	UI                                      string   // the form; tools.js fills it in
}

var tools = []tool{
	{
		Slug: "gst-invoice", Title: "Free GST invoice maker: PDF with CGST, SGST and IGST", Heading: "GST invoice maker",
		Lead:        "Fill in seller, buyer and items; get a GST invoice as a PDF, with the tax split worked out and the amount in words.",
		Description: "Make a GST tax invoice PDF for free: CGST and SGST or IGST worked out from the GSTINs, HSN codes, totals in words, bank details and a UPI QR code.",
		About: []string{"Whether the invoice shows CGST and SGST or IGST depends on the place of supply: when the seller's and buyer's GSTINs start with the same state code the tax is split in two, otherwise it is charged as IGST. The tool works that out from the GSTINs you type.",
			"Nothing you type is stored or logged. The PDF is made on the server and sent straight back."},
		FAQ: []faq{{"Is this a valid tax invoice?", "It has the fields rule 46 of the CGST Rules asks for when you fill them in: number, date, GSTINs, HSN, taxable value, rate and amount of tax, and the place of supply. Whether you must also issue an e-invoice depends on your turnover; this tool does not register invoices on the IRP."},
			{"Can I add my logo?", "Not yet. The invoice is plain and prints well in black and white."}},
		Uses: []string{"invoice"}, Calls: []string{"POST /v1/invoice"},
		UI: `<form class="card form" data-tool="gst-invoice">
<div class="fields"><label class="field"><span>Invoice number</span><input name="number" value="INV-0001"></label><label class="field"><span>Date</span><input name="date" type="date"></label></div>
<h2>Seller</h2><div class="fields"><label class="field"><span>Name</span><input name="seller.name" value="Sri Murugan Stores"></label><label class="field"><span>GSTIN</span><input name="seller.gstin" value="33AABCS1429B1Z1"></label><label class="field"><span>Address</span><input name="seller.address" value="12 Car Street, Namakkal 637001"></label></div>
<h2>Buyer</h2><div class="fields"><label class="field"><span>Name</span><input name="buyer.name" value="Kaveri Traders"></label><label class="field"><span>GSTIN (optional)</span><input name="buyer.gstin" value="29AAACK6812C1ZS"></label><label class="field"><span>Address</span><input name="buyer.address" value="4 MG Road, Bengaluru 560001"></label></div>
<h2>Items</h2><div id="invoice-items"></div><div class="row"><button class="key key--small" type="button" id="add-item">Add an item</button></div>
<div class="fields"><label class="field"><span>UPI ID for a payment QR (optional)</span><input name="upi" value="srimurugan@okaxis"></label></div>
<div class="row"><button class="key key--primary" type="submit">Make the PDF</button></div>
</form>`,
	},
	{
		Slug: "upi-qr", Title: "UPI QR code generator: free, with amount and note", Heading: "UPI QR code maker",
		Lead:        "Type your UPI ID and name; get a QR code any UPI app can scan, with an amount and note if you want.",
		Description: "Make a UPI payment QR code for free: your UPI ID, payee name, an optional fixed amount and note. Works with Google Pay, PhonePe, Paytm and every UPI app.",
		About: []string{"A UPI QR code holds a upi://pay link: the payee's UPI ID (pa), their name (pn), and optionally an amount (am) and a note (tn). Any UPI app reads it. The code does not pass through any payment company; it is only a picture of the link.",
			"Leave the amount empty for a code customers can pay any amount to, such as one printed at a shop counter."},
		FAQ: []faq{{"Does the money go through this site?", "No. The QR code only holds your UPI ID. Payments go from the customer's bank to yours through UPI, as with any UPI QR."},
			{"Which UPI ID should I use?", "The one your UPI app shows for receiving money, such as name@okaxis or 98xxxxxxxx@ybl."}},
		Uses: []string{"codes"}, Calls: []string{"GET /v1/upi-qr"},
		UI: `<form class="card form" data-tool="upi-qr"><label>UPI ID <input name="pa" value="shop@okaxis" required></label><label>Payee name <input name="pn" value="Sri Murugan Stores" required></label><div class="fields"><label class="field"><span>Amount in rupees (optional)</span><input name="am" inputmode="decimal"></label><label class="field"><span>Note (optional)</span><input name="tn"></label></div><div class="row"><button class="key key--primary" type="submit">Make the QR code</button></div></form>`,
	},
	{
		Slug: "ifsc", Title: "IFSC code finder: search any bank branch in India", Heading: "IFSC code finder",
		Lead:        "Find a branch's IFSC code by bank and place, or look up an IFSC code to see its branch, address and MICR.",
		Description: "Find the IFSC code of any bank branch in India by bank, state and city, or check an IFSC code: branch, address, MICR, and whether it takes NEFT, RTGS, IMPS and UPI.",
		About: []string{"An IFSC code is 11 characters: four letters for the bank, a zero, and six characters for the branch. Banks print it on cheque leaves and passbooks.",
			"The branch list comes from an open dataset compiled from Reserve Bank of India and NPCI publications, refreshed weekly."},
		FAQ: []faq{{"Where do I find my branch's IFSC code?", "On your cheque book, passbook or bank statement. Or search here by your bank and town."},
			{"What is MICR?", "A nine-digit code printed at the bottom of a cheque that identifies the bank branch for cheque clearing."}},
		Uses: []string{"ifsc"}, Calls: []string{"GET /v1/ifsc/{code}", "GET /v1/ifsc", "GET /v1/banks"},
		UI: `<form class="card form" data-tool="ifsc"><label>IFSC code <input name="code" placeholder="SBIN0000001" autocapitalize="characters"></label><p class="note">Or search:</p><div class="fields"><label class="field"><span>Bank</span><input name="bank" placeholder="State Bank of India" list="bank-list"></label><label class="field"><span>State</span><input name="state" placeholder="TN or Tamil Nadu"></label><label class="field"><span>City or branch</span><input name="q" placeholder="Namakkal"></label></div><datalist id="bank-list"></datalist><div class="row"><button class="key key--primary" type="submit">Find</button></div></form>`,
	},
	{
		Slug: "pincode", Title: "Pincode finder: post offices, district and state", Heading: "Pincode finder",
		Lead:        "Type a pincode to see its post offices, district and state, or search a place name to find its pincode.",
		Description: "Find any Indian pincode: the post offices it covers, district, state and location. Or search a town or post office name to get its pincode.",
		About: []string{"An Indian pincode has six digits. The first is the region, the first two the postal circle (roughly the state), the first three the sorting district.",
			"The list is India Post's All India Pincode Directory from the government open data platform."},
		FAQ:  []faq{{"Why does one pincode have many post offices?", "A head or sub post office and the branch offices it serves share one pincode."}},
		Uses: []string{"pincode"}, Calls: []string{"GET /v1/pincode/{pincode}", "GET /v1/pincode"},
		UI: `<form class="card form" data-tool="pincode"><label>Pincode or place <input name="q" placeholder="637001 or Namakkal" required></label><div class="row"><button class="key key--primary" type="submit">Find</button></div></form>`,
	},
	{
		Slug: "amount-in-words", Title: "Amount in words: rupees for cheques, in lakh and crore", Heading: "Amount in words",
		Lead:        "Type an amount; get it in words the Indian way, in lakh and crore, in English, Hindi or Tamil, ready for a cheque.",
		Description: "Write any amount in words for a cheque or invoice: Indian numbering (lakh, crore), rupees and paise, in English, Hindi or Tamil.",
		About:       []string{"Cheques and invoices in India write amounts with lakh and crore: 1,25,000 is one lakh twenty-five thousand. The words end with \"only\" so nothing can be added after them."},
		FAQ:         []faq{{"How do I write 1,50,000 on a cheque?", "One lakh fifty thousand rupees only."}, {"What about paise?", "Type them after a decimal point: 2500.50 becomes two thousand five hundred rupees and fifty paise only."}},
		Uses:        []string{"words"}, Calls: []string{"GET /v1/words/{number}"},
		UI: `<form class="card form" data-tool="amount-in-words"><label>Amount <input name="number" value="125000.50" inputmode="decimal" required></label><label>Language <select name="lang"><option value="en">English</option><option value="hi">Hindi</option><option value="ta">Tamil</option></select></label><div class="row"><button class="key key--primary" type="submit">In words</button></div></form>`,
	},
	{
		Slug: "bank-holidays", Title: "Bank holidays by state, with a calendar to subscribe to", Heading: "Bank holidays calendar",
		Lead:        "Bank holidays for your state this year, from the Reserve Bank of India, and a calendar link that keeps your phone up to date.",
		Description: "Bank holidays in every Indian state from the RBI holiday list, and a free calendar feed to add to Google Calendar or Outlook. Also NSE market holidays.",
		About: []string{"Banks close on Sundays, on the second and fourth Saturdays, and on the holidays the Reserve Bank lists for each regional office, which differ by state.",
			"The calendar link is a feed: add it once to Google Calendar, Outlook or your phone, and holidays added later appear by themselves."},
		FAQ: []faq{{"Are banks open on the second Saturday?", "No. Banks are closed on the second and fourth Saturdays of every month."},
			{"How do I add the calendar to Google Calendar?", "Copy the calendar address, then in Google Calendar choose Other calendars, From URL, and paste it."}},
		Uses: []string{"holidays"}, Calls: []string{"GET /v1/holidays/{year}", "GET /v1/holidays/{year}.ics"},
		UI: `<form class="card form" data-tool="bank-holidays"><div class="fields"><label class="field"><span>State</span><select name="state">` + stateOptions + `</select></label><label class="field"><span>Year</span><input name="year" inputmode="numeric"></label><label class="field"><span>Calendar</span><select name="type"><option value="bank">Bank holidays</option><option value="market">NSE market holidays</option></select></label></div><div class="row"><button class="key key--primary" type="submit">Show</button></div></form>`,
	},
	{
		Slug: "panchang", Title: "Today's panchang: tithi, nakshatra, yoga and rahu kalam", Heading: "Today's panchang",
		Lead:        "Tithi, nakshatra, yoga, karana, sunrise and rahu kalam for any day and place in India, worked out for that place.",
		Description: "Free panchang for any date and Indian pincode: tithi, nakshatra, yoga, karana, sunrise, sunset, rahu kalam, yamagandam and gulika, with Tamil names.",
		About: []string{"A panchang depends on the place: the day starts at local sunrise, and tithi and nakshatra are read at that moment. Type a pincode for your town.",
			"It is computed with astronomical algorithms for the sun and moon (Lahiri ayanamsa), not copied from a printed almanac, so it can differ from one by a minute or two."},
		FAQ:  []faq{{"What is rahu kalam?", "A period of about an hour and a half each day, fixed by the weekday and the day's length, which many avoid for starting new things."}},
		Uses: []string{"panchang"}, Calls: []string{"GET /v1/panchang/{date}"},
		UI: `<form class="card form" data-tool="panchang"><div class="fields"><label class="field"><span>Date</span><input name="date" type="date"></label><label class="field"><span>Pincode</span><input name="pincode" value="600001" inputmode="numeric"></label></div><div class="row"><button class="key key--primary" type="submit">Show the panchang</button></div></form>`,
	},
	{
		Slug: "image-to-text", Title: "Image to text in Indian languages: free OCR", Heading: "Image to text",
		Lead:        "Upload a photo or scan; get the text in it, in English and Indian languages: Hindi, Tamil, Telugu, Kannada, Malayalam, Bengali and more.",
		Description: "Free OCR for Indian languages: turn a photo, scan or PDF into text in Hindi, Tamil, Telugu, Kannada, Malayalam, Bengali, Gujarati, Marathi, Punjabi, Odia and English.",
		About: []string{"Text recognition runs Tesseract with its Indian language models on our own server. Your file is not kept, not logged and not sent anywhere else.",
			"Choose the languages that are in the picture: fewer languages give better results."},
		FAQ:  []faq{{"Is there a limit?", "Files up to 10 MB. The free tool allows a few pages a day; a free API key allows many more."}},
		Uses: []string{"ocr"}, Calls: []string{"POST /v1/ocr"},
		UI: `<form class="card form" data-tool="image-to-text"><label>Picture or PDF <input type="file" name="file" accept="image/*,application/pdf" required></label><label>Languages in it <select name="lang"><option value="eng+hin">English and Hindi</option><option value="eng+tam">English and Tamil</option><option value="eng+tel">English and Telugu</option><option value="eng+kan">English and Kannada</option><option value="eng+mal">English and Malayalam</option><option value="eng+ben">English and Bengali</option><option value="eng+guj">English and Gujarati</option><option value="eng+mar">English and Marathi</option><option value="eng">English only</option></select></label><div class="row"><button class="key key--primary" type="submit">Read the text</button></div></form>`,
	},
	{
		Slug: "indian-language-typing", Title: "Type in Tamil, Hindi and other Indian languages with English letters", Heading: "Indian language typing",
		Lead:        "Type the way you text, in English letters (vanakkam, namaste); get it in the script, with suggestions to pick from.",
		Description: "Type Tamil, Hindi, Telugu, Kannada, Malayalam, Bengali and more using English letters. Free, in the browser, with suggestions for each word.",
		About: []string{"This is phonetic typing: you spell the sound in English letters and a model trained on how people actually type (IndicXlit, from AI4Bharat) suggests the word in the script.",
			"Click a suggestion to swap a word. Copy the result when you are done."},
		FAQ:  []faq{{"Is my text stored?", "No. It is sent to our server to make the suggestions and is not kept or logged."}},
		Uses: []string{"transliterate"}, Calls: []string{"GET /v1/transliterate/suggest"},
		UI: `<form class="card form" data-tool="indian-language-typing"><label>Language <select name="lang"><option value="ta">Tamil</option><option value="hi">Hindi</option><option value="te">Telugu</option><option value="kn">Kannada</option><option value="ml">Malayalam</option><option value="bn">Bengali</option><option value="gu">Gujarati</option><option value="mr">Marathi</option><option value="pa">Punjabi</option></select></label><label>Type in English letters <textarea name="text" rows="3">vanakkam nanbargale</textarea></label><div class="row"><button class="key key--primary" type="submit">Convert</button></div></form>`,
	},
	{
		Slug: "pdf", Title: "Free PDF tools: merge, split, rotate, compress, to text", Heading: "PDF tools",
		Lead:        "Merge PDFs, pick or reorder pages, rotate, make a PDF smaller, or take the text out. Nothing is kept.",
		Description: "Free online PDF tools: merge several PDFs, take or reorder pages, rotate pages, compress a PDF, and extract its text. Files are not stored.",
		About:       []string{"The PDF tools run on our own server with pdfcpu and Poppler. A file is worked on in memory and deleted when the answer is sent."},
		FAQ:         []faq{{"Is there a size limit?", "10 MB per file, 25 MB for a merge."}},
		Uses:        []string{"pdf"}, Calls: []string{"POST /v1/pdf/merge", "POST /v1/pdf/pages", "POST /v1/pdf/rotate", "POST /v1/pdf/compress", "POST /v1/pdf/text"},
		UI: `<form class="card form" data-tool="pdf"><label>What to do <select name="action"><option value="merge">Merge several PDFs into one</option><option value="pages">Take or reorder pages</option><option value="rotate">Rotate pages</option><option value="compress">Make it smaller</option><option value="text">Take the text out</option></select></label><label>PDF file(s) <input type="file" name="file" accept="application/pdf" multiple required></label><div class="fields"><label class="field"><span>Pages (for take, rotate)</span><input name="pages" placeholder="1-3,5"></label><label class="field"><span>Angle (for rotate)</span><select name="angle"><option>90</option><option>180</option><option>270</option></select></label></div><div class="row"><button class="key key--primary" type="submit">Go</button></div></form>`,
	},
}

const stateOptions = `<option value="AN">Andaman and Nicobar</option><option value="AP">Andhra Pradesh</option><option value="AR">Arunachal Pradesh</option><option value="AS">Assam</option><option value="BR">Bihar</option><option value="CH">Chandigarh</option><option value="CT">Chhattisgarh</option><option value="DL">Delhi</option><option value="GA">Goa</option><option value="GJ">Gujarat</option><option value="HR">Haryana</option><option value="HP">Himachal Pradesh</option><option value="JK">Jammu and Kashmir</option><option value="JH">Jharkhand</option><option value="KA">Karnataka</option><option value="KL">Kerala</option><option value="MP">Madhya Pradesh</option><option value="MH">Maharashtra</option><option value="MN">Manipur</option><option value="ML">Meghalaya</option><option value="MZ">Mizoram</option><option value="NL">Nagaland</option><option value="OR">Odisha</option><option value="PY">Puducherry</option><option value="PB">Punjab</option><option value="RJ">Rajasthan</option><option value="SK">Sikkim</option><option value="TN" selected>Tamil Nadu</option><option value="TG">Telangana</option><option value="TR">Tripura</option><option value="UP">Uttar Pradesh</option><option value="UT">Uttarakhand</option><option value="WB">West Bengal</option>`

func toolBySlug(slug string) (tool, bool) {
	for _, t := range tools {
		if t.Slug == slug {
			return t, true
		}
	}
	return tool{}, false
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	s.sendPage(w, r, "tools", func(origin string) []byte {
		var html strings.Builder
		html.WriteString(`<ul class="tiles">`)
		for _, t := range tools {
			html.WriteString(`<li><a href="/tools/` + t.Slug + `"><b>` + template.HTMLEscapeString(t.Heading) + `</b><span>` + template.HTMLEscapeString(t.Lead) + `</span></a></li>`)
		}
		html.WriteString(`</ul><p class="aside">Developers: every tool here is a page on top of a free API. See <a href="/docs">the API reference</a> or try any endpoint in <a href="/play">the playground</a>.</p>`)
		p := page{Origin: origin, Path: "/tools", Nav: "tools", Markdown: "/tools.md", Title: "Free tools for India: GST invoice, UPI QR, IFSC, pincode and more | " + productName, Heading: "Free tools",
			Description: "Free online tools for everyday Indian tasks: GST invoice PDF, UPI QR code, IFSC and pincode finder, amount in words, bank holidays, panchang, OCR in Indian languages, PDF tools.",
			Body:        appPage{Heading: "Free tools", Lead: "Everyday tasks, done in your browser. No sign-up, nothing kept.", HTML: template.HTML(html.String())}}
		p.JSONLD = jsonLD(breadcrumbs(origin, [2]string{"Tools", "/tools"}))
		return s.renderHTML("app", p)
	}, func(origin string) []byte {
		var out strings.Builder
		out.WriteString("# Free tools\n\nEveryday tasks done in the browser, each on top of a free API.\n\n")
		for _, t := range tools {
			out.WriteString("- [" + t.Heading + "](" + origin + "/tools/" + t.Slug + "): " + t.Lead + "\n")
		}
		return []byte(out.String())
	})
}

func (s *Server) handleTool(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSuffix(r.PathValue("slug"), ".md")
	t, ok := toolBySlug(slug)
	if !ok {
		s.notFound(w, r, "No such tool. The list is at /tools.")
		return
	}
	s.sendPage(w, r, "tool-"+slug, func(origin string) []byte {
		var html strings.Builder
		html.WriteString(t.UI)
		html.WriteString(`<div class="card answer" id="tool-answer" aria-live="polite" hidden></div>`)
		html.WriteString(`<noscript><p class="aside">This tool needs JavaScript to send your request. Without it, you can call the API it runs on directly: see below.</p></noscript>`)
		html.WriteString(`<div class="prose" style="margin-top:var(--space-2xl)">`)
		html.WriteString(`<h2>About this tool</h2>`)
		for _, paragraph := range t.About {
			html.WriteString(`<p>` + template.HTMLEscapeString(paragraph) + `</p>`)
		}
		if len(t.FAQ) > 0 {
			html.WriteString(`<h2>Questions</h2>`)
			for _, f := range t.FAQ {
				html.WriteString(`<h3>` + template.HTMLEscapeString(f.Question) + `</h3><p>` + template.HTMLEscapeString(f.Answer) + `</p>`)
			}
		}
		html.WriteString(`<p class="aside">Developers: this page runs on `)
		for i, id := range t.Uses {
			if group, ok := groupByID(id); ok {
				if i > 0 {
					html.WriteString(", ")
				}
				html.WriteString(`<a href="` + traffic.Tag("/docs/"+group.ID, "tool") + `">the ` + template.HTMLEscapeString(group.Title) + ` API</a>`)
			}
		}
		html.WriteString(` (` + template.HTMLEscapeString(strings.Join(t.Calls, ", ")) + `). Free, open source, no key for most of it.</p></div>`)
		var faqItems []any
		for _, f := range t.FAQ {
			faqItems = append(faqItems, map[string]any{"@type": "Question", "name": f.Question, "acceptedAnswer": map[string]any{"@type": "Answer", "text": f.Answer}})
		}
		p := page{Origin: origin, Path: "/tools/" + t.Slug, Nav: "tools", Markdown: "/tools/" + t.Slug + ".md", Title: t.Title + " | " + productName, Heading: t.Heading, Description: t.Description,
			Body: appPage{Crumbs: [][2]string{{"Tools", "/tools"}}, Heading: t.Heading, Lead: t.Lead, HTML: template.HTML(html.String()), Scripts: []string{"pow.js", "tools.js"}}}
		graph := []map[string]any{breadcrumbs(origin, [2]string{"Tools", "/tools"}, [2]string{t.Heading, "/tools/" + t.Slug}),
			{"@type": "WebApplication", "name": t.Heading, "url": origin + "/tools/" + t.Slug, "applicationCategory": "UtilitiesApplication", "operatingSystem": "Any",
				"isAccessibleForFree": true, "offers": map[string]any{"@type": "Offer", "price": "0", "priceCurrency": "INR"}, "description": t.Description}}
		if len(faqItems) > 0 {
			graph = append(graph, map[string]any{"@type": "FAQPage", "mainEntity": faqItems})
		}
		p.JSONLD = jsonLD(graph...)
		return s.renderHTML("app", p)
	}, func(origin string) []byte {
		var out strings.Builder
		out.WriteString("# " + t.Heading + "\n\n> " + t.Lead + "\n\n")
		for _, paragraph := range t.About {
			out.WriteString(paragraph + "\n\n")
		}
		for _, f := range t.FAQ {
			out.WriteString("## " + f.Question + "\n\n" + f.Answer + "\n\n")
		}
		out.WriteString("Developers: this page runs on " + strings.Join(t.Calls, ", ") + ". Documentation: " + origin + "/docs/" + t.Uses[0] + ".md\n")
		return []byte(out.String())
	})
}
