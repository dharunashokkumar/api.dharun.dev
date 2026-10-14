# OCR API

> Text from photos, scans and scanned PDFs, in English and ten Indian languages.

Send a picture or a PDF and get its text back, with Tesseract 5. Name the languages the text uses: English, Hindi, Tamil, Telugu, Kannada, Malayalam, Bengali, Gujarati, Punjabi, Odia and Marathi, up to three at once. Files are read and deleted; nothing is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Read the text in a picture or PDF

`POST /v1/ocr`

Returns the text, Tesseract's confidence and, with words=true, every word with its box. A PDF is read up to 10 pages at a time at 300 dpi; a PDF that already holds text is read faster by /v1/pdf/text. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB. Or a PDF. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/receipt.png

What you send is never written to the log, and the answer is not cached.

Parameters:

- `lang` (query): Languages, joined by + or a comma: eng, hin, tam, tel, kan, mal, ben, guj, pan, ori, mar (or en, ta, hi...). Default eng.
- `words` (query): Also return each word with its confidence and box. One of: true, false.
- `first` (query): PDF: first page to read. Default 1.
- `last` (query): PDF: last page to read; at most 10 pages per request.

Example: `POST` https://api.dharun.dev/v1/ocr?lang=eng+tam
