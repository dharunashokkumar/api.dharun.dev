# Web pages API

> Link previews, a page as Markdown for agents, and HTML and Markdown to each other.

Pages are fetched from India through the same guard as the probe: public addresses and the usual ports only, at most 10 fetches of one site a minute from every caller together. The reader respects robots.txt. Addresses are never logged.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Link preview data

`GET /v1/unfurl`

Title, description, image, icon, site name, type and canonical address of a page, from its Open Graph and Twitter tags, for chat and notes apps. Counts as 20 against the rate limit.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `url` (query, required): An http or https address.

Example: https://api.dharun.dev/v1/unfurl?url=https://dharun.dev&utm_medium=skill

## A web page as Markdown

`GET /v1/read`

One page fetched and turned into clean Markdown, its main content only, for agents and retrieval. One page per call; robots.txt is respected (agent IndiaAPI-Reader). Needs a free API key once keys are required (see /keys).

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `url` (query, required): The page.
- `full` (query): true keeps navigation, headers and footers.
- `format` (query): markdown answers with the Markdown itself. One of: json, markdown.

Example: https://api.dharun.dev/v1/read?url=https://dharun.dev&utm_medium=skill

## HTML and Markdown to each other

`GET /v1/markdown`, also `POST`

Markdown to safe HTML (GitHub-flavoured, raw HTML left out), or HTML to Markdown.

POST body: The text, as text/plain or JSON {"text": "...", "to": "html"}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text.
- `to` (query): Default: markdown when the text has tags, else html. One of: html, markdown.
- `main` (query): HTML to Markdown: keep only the main content.

Example: https://api.dharun.dev/v1/markdown?to=html&text=%23%20Hello%0A%0A*India*%20API&utm_medium=skill
