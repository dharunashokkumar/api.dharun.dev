# PDF tools API

> Merge, split, rotate and compress PDFs, read their text and draw their pages.

Upload a PDF and get a new one back: merged with others, cut down to some pages, rotated, or made smaller. Or read its details and text, or draw a page as a picture. Password-protected PDFs are refused. Files are processed and deleted; nothing is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Pages, size and details of a PDF

`POST /v1/pdf/info`

Page count, PDF version, title, author, dates, page sizes (named when A4, Letter and so on), and whether it has forms, signatures or attachments.

Send: A PDF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/document.pdf

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/pdf/info

## Text of a PDF, page by page

`POST /v1/pdf/text`

The text of each page, in reading order, or laid out as on the page with layout=true. needs_ocr is true when the pages hold almost no text, as in a scan: send those to /v1/ocr. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A PDF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/document.pdf

What you send is never written to the log, and the answer is not cached.

Parameters:

- `first` (query): First page. Default 1.
- `last` (query): Last page; at most 500 pages per request.
- `layout` (query): Keep the page's layout with spaces, for tables. One of: true, false.

Example: `POST` https://api.dharun.dev/v1/pdf/text

## Merge PDFs into one

`POST /v1/pdf/merge`

Send two to twenty PDFs as form fields, in order; up to 25 MB together. Returns one PDF.

Send: Two to twenty PDFs as multipart/form-data, up to 25 MB together. Post the file as the request body, or as `multipart/form-data` with one field per file. Sample file: https://api.dharun.dev/samples/document.pdf

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/pdf/merge (returns the PDF itself)

## Pick, reorder or split pages

`POST /v1/pdf/pages`

Returns a PDF of the pages named, in the order named; a page may be named twice. To split a PDF, ask for each part in turn. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A PDF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/document.pdf

What you send is never written to the log, and the answer is not cached.

Parameters:

- `pages` (query, required): Pages: numbers and ranges such as 1-3,5,8- (8 to the end), odd, even or l (the last page).

Example: `POST` https://api.dharun.dev/v1/pdf/pages?pages=2,1 (returns the PDF itself)

## Rotate pages

`POST /v1/pdf/rotate`

Turns pages clockwise by a multiple of 90 degrees. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A PDF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/document.pdf

What you send is never written to the log, and the answer is not cached.

Parameters:

- `angle` (query): Degrees, clockwise. Default 90. One of: 90, 180, 270, -90, -180, -270.
- `pages` (query): Pages: numbers and ranges such as 1-3,5,8- (8 to the end), odd, even or l (the last page).

Example: `POST` https://api.dharun.dev/v1/pdf/rotate?angle=90&pages=1 (returns the PDF itself)

## Make a PDF smaller

`POST /v1/pdf/compress`

Removes duplicate fonts and images and packs the file's structure. Pictures are not re-compressed, so a scanned PDF shrinks little. The original size is in the X-Original-Bytes header; when nothing was gained, the original comes back.

Send: A PDF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/document.pdf

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/pdf/compress (returns the PDF itself)

## Draw a page as a picture

`POST /v1/pdf/render`

Returns one page as PNG or JPEG, for previews and thumbnails. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A PDF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/document.pdf

What you send is never written to the log, and the answer is not cached.

Parameters:

- `page` (query): Page number. Default 1.
- `width` (query): Width in pixels, 50 to 3000. Default 1000.
- `format` (query): Default png. One of: png, jpeg.

Example: `POST` https://api.dharun.dev/v1/pdf/render?page=1&width=600 (returns the image itself)
