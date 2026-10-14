# Typst to PDF API

> Turn a Typst document into a PDF, with Indian scripts set correctly.

Typst is a modern markup language for documents, quicker to write than LaTeX. Send a document and get a PDF, or one page as PNG or SVG. Noto fonts for Latin and nine Indian scripts are available; packages, other files and the network are not, so a document must be self-contained. Each document has 20 seconds.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Compile Typst to PDF

`GET /v1/typst`, also `POST`

Returns the PDF itself, or one page as an image. A mistake in the document returns 422 with Typst's message, line and column.

POST body: The Typst document. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The document (or POST it).
- `format` (query): Default pdf. One of: pdf, png, svg.
- `page` (query): png and svg: which page. Default 1.
- `ppi` (query): png: pixels per inch, 36 to 300. Default 144.

Example: https://api.dharun.dev/v1/typst?text=%23set%20page%28width%3A%2012cm%2C%20height%3A%20auto%2C%20margin%3A%201cm%29%0A=%20வணக்கம்%2C%20India%20API%0ATypst%20sets%20%2ATamil%2A%2C%20हिन्दी%20and%20English%20on%20one%20page. (returns the PDF itself)
