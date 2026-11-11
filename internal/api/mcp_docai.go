package api

// MCP tools for document reading (growth plan Part 7).

func init() {
	for _, name := range []string{"read_document_as_markdown", "read_document_tables", "read_gst_invoice", "read_einvoice_qr", "mask_aadhaar_number", "read_receipt", "read_cheque", "read_bank_statement", "redact_document"} {
		privateTools[name] = true
	}
	number := func(description string) map[string]any {
		return map[string]any{"type": "integer", "description": description}
	}
	pages := map[string]any{
		"first": number("PDF: first page. Default 1."), "last": number("PDF: last page."),
		"ocr":  oneOf("PDF: auto (default) uses OCR only when the PDF has no text.", "auto", "true", "false"),
		"lang": text("OCR languages joined by +, such as eng+hin. Default eng."),
	}
	mapping := map[string]string{"first": "first", "last": "last", "ocr": "ocr", "lang": "lang"}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range pages {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	mcpTools = append(mcpTools,
		uploadTool("read_receipt", "Read a shop receipt from a photo or PDF: merchant, GSTIN, date, items, tax, total and how it was paid.", "/v1/doc/receipt", pages, mapping),
		uploadTool("read_cheque", "Read a cheque photo: IFSC, MICR code, cheque number, account number and date, with the bank branch looked up.", "/v1/doc/cheque", pages, mapping),
		uploadTool("read_bank_statement", "Turn a bank statement (PDF or scan) into transactions: date, description, reference, debit, credit and balance, each balance checked.", "/v1/doc/statement", pages, mapping),
		uploadTool("redact_document", "Cover Aadhaar, PAN, phone numbers, email addresses, account numbers and other identifiers in a PDF or picture, returning the redacted file.", "/v1/doc/redact",
			with(map[string]any{"types": text("Kinds to cover, comma-separated, as in /v1/text/redact (aadhaar, pan, phone, email ...). Default all.")}),
			map[string]string{"first": "first", "last": "last", "ocr": "ocr", "lang": "lang", "types": "types"}),
		uploadTool("read_document_as_markdown", "Turn a PDF, scan or photo of a page into Markdown with headings, lists and tables, without a language model. Uses the PDF's own text, or OCR for a scan.", "/v1/doc/markdown", pages, mapping),
		uploadTool("read_document_tables", "Find the tables in a PDF, scan or photo, as rows of cells.", "/v1/doc/tables",
			with(map[string]any{"format": oneOf("json (default) or markdown, which adds each table as Markdown.", "json", "markdown")}),
			map[string]string{"first": "first", "last": "last", "ocr": "ocr", "lang": "lang", "format": "format"}),
		uploadTool("read_gst_invoice", "Read a GST invoice (PDF, scan or photo): number, date, seller and buyer GSTINs, place of supply, HSN codes, taxable value, CGST, SGST, IGST, total and amount in words, and check them against each other and the GST rules.", "/v1/doc/invoice", pages, mapping),
		uploadTool("read_einvoice_qr", "Read the signed QR code of a GST e-invoice from a picture or PDF: IRN, GSTINs, document number and date, total value. The signature is not verified.", "/v1/doc/einvoice-qr", nil, nil),
		uploadTool("mask_aadhaar_number", "Cover the first eight digits of every Aadhaar number (and the QR code) in a picture of a card or letter, leaving the last four. Fails when no number is found.", "/v1/doc/mask-aadhaar",
			map[string]any{"all": oneOf("true covers every digit.", "true", "false"), "qr": oneOf("false leaves QR codes uncovered.", "true", "false")},
			map[string]string{"all": "all", "qr": "qr"}),
	)
}
