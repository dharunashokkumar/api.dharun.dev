# Browser tools API

> Screenshots, PDFs and the text of web pages, from a real Chromium.

A page opened in headless Chromium, which runs its scripts: a screenshot (the window or the whole page, PNG, JPEG or WebP), the page as a PDF (or HTML you send, for invoices and reports), or its text as plain text or Markdown for an LLM. Each page gets a microVM of its own that reaches only public addresses, on ports 80 and 443. The browser names itself india-api-browser. The text tool reads one page you name, never follows links, and honours robots.txt for that name (or for every agent); a site's own refusal is passed on. No account is needed; each page counts as 30 requests against the rate limit, and a site can be opened 10 times a minute by all callers together. Nothing is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Screenshot of a page

`GET /v1/browser/screenshot`, also `POST`

Returns the image. GET with the options in the query, or POST JSON with url or html (up to 4 MB) and the same options.

POST body: JSON: url or html, and the options. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `url` (query): The page, http or https.
- `width` (query): Window width, 320 to 3840. Default 1280 (390 with mobile).
- `height` (query): Window height, 200 to 2160. Default 800.
- `fullPage` (query): The whole page, up to 16,384 pixels tall. One of: true.
- `format` (query): Default png. One of: png, jpeg, webp.
- `mobile` (query): A phone's window and user agent. One of: true.
- `waitMs` (query): Extra time after the page loads, up to 10,000 ms. Default 500.

Example: https://api.dharun.dev/v1/browser/screenshot?url=https://example.com&width=1280&height=800&utm_medium=skill (returns the image itself)

## A page or HTML as PDF

`GET /v1/browser/pdf`, also `POST`

Returns the PDF. The page's print styles apply. POST html to make a PDF from your own HTML and CSS (images inline or at public URLs).

POST body: JSON: url or html, and the options. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `url` (query): The page, http or https.
- `paper` (query): Default a4; a page's own @page size wins. One of: a4, a3, a5, letter, legal.
- `landscape` (query): Landscape pages. One of: true.
- `background` (query): Print background colours and images. One of: true.
- `waitMs` (query): Extra time after the page loads, up to 10,000 ms.

Example: https://api.dharun.dev/v1/browser/pdf?url=https://example.com&utm_medium=skill (returns the PDF itself)

## Text of a page

`GET /v1/browser/text`, also `POST`

url, title, language, the HTTP status and text: what a reader sees (innerText), or with format=markdown the page as Markdown with headings, lists, links, code and tables. 403 robots_disallowed when the site's robots.txt does not allow it.

POST body: JSON: url and the options. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `url` (query, required): The page, http or https.
- `format` (query): Default text. One of: text, markdown.
- `waitMs` (query): Extra time after the page loads, up to 10,000 ms.

Example: https://api.dharun.dev/v1/browser/text?url=https://example.com&format=markdown&utm_medium=skill
