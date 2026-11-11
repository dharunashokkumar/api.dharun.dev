package api

// Document reading (growth plan Part 7).

var docTypes = append([]string{"application/pdf"}, pictureTypes...)

const docNote = "A PDF, or a picture of a page (JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF), up to 10 MB."

var docParams = []param{
	{Name: "first", In: "query", Type: "integer", Doc: "PDF: first page. Default 1."},
	{Name: "last", In: "query", Type: "integer", Doc: "PDF: last page. A PDF's own text is read up to 500 pages, a scan up to 10."},
	{Name: "ocr", In: "query", Type: "string", Enum: []string{"auto", "true", "false"}, Doc: "PDF: auto (default) reads the PDF's own text and uses OCR only when there is none; true always uses OCR; false never."},
	{Name: "lang", In: "query", Type: "string", Doc: "OCR languages, as in /v1/ocr: up to three of eng, hin, tam, tel, kan, mal, ben, guj, pan, ori, mar joined by +. Default eng."},
}

func withDocAI(groups []apiGroup) []apiGroup {
	return append(groups, apiGroup{
		ID: "documents", Category: "files", Title: "Document reading",
		Tagline: "PDFs and scans to Markdown and tables, GST invoices read and checked, e-invoice QR codes, Aadhaar masking.",
		Intro:   "These read documents without a language model, so they are fast and never make text up. A PDF's own text is read with the position of every word; a scan or photo is read with OCR (Tesseract). Headings come from the size of the text, tables from words that line up in columns, and an invoice's fields from their labels. Files are read and deleted; nothing is kept or logged." + keyNote,
		Endpoints: []endpoint{
			{
				Path: "/v1/doc/markdown", Key: true, Summary: "PDF or scan to Markdown", Private: true,
				Doc:     "Markdown with headings (from text size), lists, paragraphs and tables, page by page with a rule between pages. Text set in columns side by side is read across. Good input for a language model or a search index; method says whether the PDF's text or OCR was used." + formNote,
				Example: "/v1/doc/markdown",
				Upload:  &upload{Accepts: docNote, Types: docTypes, Sample: "invoice.pdf"},
				Params:  docParams,
			},
			{
				Path: "/v1/doc/tables", Key: true, Summary: "Tables in a PDF or scan", Private: true,
				Doc:     "Every table found, as rows of cells with its page. A table is three or more rows whose words line up in two or more columns, with or without ruled lines. format=csv returns one table (table=N) as CSV." + formNote,
				Example: "/v1/doc/tables",
				Upload:  &upload{Accepts: docNote, Types: docTypes, Sample: "invoice.pdf"},
				Params: append([]param{
					{Name: "format", In: "query", Type: "string", Enum: []string{"json", "markdown", "csv"}, Doc: "json (default); markdown adds each table as Markdown; csv returns one table as a CSV file."},
					{Name: "table", In: "query", Type: "integer", Doc: "With format=csv: which table, from 1. Default 1."},
				}, docParams...),
			},
			{
				Path: "/v1/doc/invoice", Key: true, Summary: "Read and check a GST invoice", Private: true,
				Doc:     "Reads the invoice number and date, seller and buyer with their GSTINs and states, place of supply, HSN codes, taxable value, CGST, SGST or UTGST, IGST, cess, round off, total, amount in words, IFSC and UPI handle. Then checks them: each GSTIN's check character, the taxes adding up to the total, CGST equal to SGST, the kind of tax against the place of supply, the amount in words against the total, and the invoice number's format (Rule 46). A field not found is null." + formNote,
				Example: "/v1/doc/invoice",
				Upload:  &upload{Accepts: docNote, Types: docTypes, Sample: "invoice.pdf"},
				Params:  docParams,
			},
			{
				Path: "/v1/doc/einvoice-qr", Key: true, Summary: "Read a GST e-invoice's signed QR code", Private: true,
				Doc:     "Finds the signed QR code that the Invoice Registration Portal puts on an e-invoice, in a picture or the first three pages of a PDF, and returns what it holds: IRN and its date, seller and buyer GSTINs, document number, type and date, total value, item count and main HSN code, with the GSTINs and the IRN's format checked. The signature is not verified here; the e-invoice portal's \"Verify signed QR code\" does that. The sample is shaped like one but not signed.",
				Example: "/v1/doc/einvoice-qr",
				Upload:  &upload{Accepts: docNote, Types: docTypes, Sample: "einvoice-qr.png"},
			},
			{
				Path: "/v1/doc/mask-aadhaar", Key: true, Summary: "Mask an Aadhaar number in a picture", Private: true, Image: true,
				Doc:     "Returns the picture with the first eight digits of every Aadhaar number (and the first twelve of a 16-digit Virtual ID) covered in black, leaving the last four, as the UIDAI's masked Aadhaar does, and the QR code covered too, since it holds the full number. A number is four-digit groups that pass the Verhoeff check. Answers 422 when nothing is found, so an unmasked picture is never returned as if masked. X-Masked-Numbers and X-Masked-QR-Codes say what was covered. Check the result by eye before you share it." + formNote,
				Example: "/v1/doc/mask-aadhaar",
				Upload:  &upload{Accepts: pictureNote, Types: pictureTypes, Sample: "idcard.png"},
				Params: []param{
					{Name: "all", In: "query", Type: "string", Enum: []string{"true", "false"}, Doc: "true covers every digit. Default false (the last four stay)."},
					{Name: "qr", In: "query", Type: "string", Enum: []string{"true", "false"}, Doc: "false leaves QR codes uncovered. Default true."},
				},
			},
		},
	})
}
