# Document reading API

> PDFs and scans to Markdown and tables, GST invoices read and checked, e-invoice QR codes, Aadhaar masking.

These read documents without a language model, so they are fast and never make text up. A PDF's own text is read with the position of every word; a scan or photo is read with OCR (Tesseract). Headings come from the size of the text, tables from words that line up in columns, and an invoice's fields from their labels. Files are read and deleted; nothing is kept or logged. Needs a free API key once keys are required (see /keys).

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## PDF or scan to Markdown

`POST /v1/doc/markdown`

Markdown with headings (from text size), lists, paragraphs and tables, page by page with a rule between pages. Text set in columns side by side is read across. Good input for a language model or a search index; method says whether the PDF's text or OCR was used. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A PDF, or a picture of a page (JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF), up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/invoice.pdf

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `first` (query): PDF: first page. Default 1.
- `last` (query): PDF: last page. A PDF's own text is read up to 500 pages, a scan up to 10.
- `ocr` (query): PDF: auto (default) reads the PDF's own text and uses OCR only when there is none; true always uses OCR; false never. One of: auto, true, false.
- `lang` (query): OCR languages, as in /v1/ocr: up to three of eng, hin, tam, tel, kan, mal, ben, guj, pan, ori, mar joined by +. Default eng.

Example: `POST` https://api.dharun.dev/v1/doc/markdown?utm_medium=skill

## Tables in a PDF or scan

`POST /v1/doc/tables`

Every table found, as rows of cells with its page. A table is three or more rows whose words line up in two or more columns, with or without ruled lines. format=csv returns one table (table=N) as CSV. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A PDF, or a picture of a page (JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF), up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/invoice.pdf

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `format` (query): json (default); markdown adds each table as Markdown; csv returns one table as a CSV file. One of: json, markdown, csv.
- `table` (query): With format=csv: which table, from 1. Default 1.
- `first` (query): PDF: first page. Default 1.
- `last` (query): PDF: last page. A PDF's own text is read up to 500 pages, a scan up to 10.
- `ocr` (query): PDF: auto (default) reads the PDF's own text and uses OCR only when there is none; true always uses OCR; false never. One of: auto, true, false.
- `lang` (query): OCR languages, as in /v1/ocr: up to three of eng, hin, tam, tel, kan, mal, ben, guj, pan, ori, mar joined by +. Default eng.

Example: `POST` https://api.dharun.dev/v1/doc/tables?utm_medium=skill

## Read and check a GST invoice

`POST /v1/doc/invoice`

Reads the invoice number and date, seller and buyer with their GSTINs and states, place of supply, HSN codes, taxable value, CGST, SGST or UTGST, IGST, cess, round off, total, amount in words, IFSC and UPI handle. Then checks them: each GSTIN's check character, the taxes adding up to the total, CGST equal to SGST, the kind of tax against the place of supply, the amount in words against the total, and the invoice number's format (Rule 46). A field not found is null. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A PDF, or a picture of a page (JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF), up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/invoice.pdf

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `first` (query): PDF: first page. Default 1.
- `last` (query): PDF: last page. A PDF's own text is read up to 500 pages, a scan up to 10.
- `ocr` (query): PDF: auto (default) reads the PDF's own text and uses OCR only when there is none; true always uses OCR; false never. One of: auto, true, false.
- `lang` (query): OCR languages, as in /v1/ocr: up to three of eng, hin, tam, tel, kan, mal, ben, guj, pan, ori, mar joined by +. Default eng.

Example: `POST` https://api.dharun.dev/v1/doc/invoice?utm_medium=skill

## Read a GST e-invoice's signed QR code

`POST /v1/doc/einvoice-qr`

Finds the signed QR code that the Invoice Registration Portal puts on an e-invoice, in a picture or the first three pages of a PDF, and returns what it holds: IRN and its date, seller and buyer GSTINs, document number, type and date, total value, item count and main HSN code, with the GSTINs and the IRN's format checked. The signature is not verified here; the e-invoice portal's "Verify signed QR code" does that. The sample is shaped like one but not signed.

Send: A PDF, or a picture of a page (JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF), up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/einvoice-qr.png

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/doc/einvoice-qr?utm_medium=skill

## Mask an Aadhaar number in a picture

`POST /v1/doc/mask-aadhaar`

Returns the picture with the first eight digits of every Aadhaar number (and the first twelve of a 16-digit Virtual ID) covered in black, leaving the last four, as the UIDAI's masked Aadhaar does, and the QR code covered too, since it holds the full number. A number is four-digit groups that pass the Verhoeff check. Answers 422 when nothing is found, so an unmasked picture is never returned as if masked. X-Masked-Numbers and X-Masked-QR-Codes say what was covered. Check the result by eye before you share it. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/idcard.png

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `all` (query): true covers every digit. Default false (the last four stay). One of: true, false.
- `qr` (query): false leaves QR codes uncovered. Default true. One of: true, false.

Example: `POST` https://api.dharun.dev/v1/doc/mask-aadhaar?utm_medium=skill (returns the image itself)
