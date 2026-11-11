# README badges API

> Small SVG badges: market open or closed, the next holiday, a dataset's freshness, or your own.

Badges in the familiar two-part style, for READMEs, dashboards and status pages. The text is drawn as outlines, so any script shows the same everywhere.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## A badge of your own

`GET /v1/badge`

Parameters:

- `label` (query): Left part, up to 60 characters. Optional.
- `message` (query, required): Right part, up to 80 characters.
- `color` (query): Message colour: green, red, orange, yellow, blue, grey, purple, teal, or hex. Default blue.
- `labelColor` (query): Label colour, hex. Default 555.
- `format` (query): svg (default) or png. One of: svg, png.

Example: https://api.dharun.dev/v1/badge?label=made%20in&message=India&color=orange&utm_medium=skill (returns the image itself)

## Market or holiday badge

`GET /v1/badge/{kind}`

market: NSE open, closed or in its pre-open or closing session. holiday: the next holiday (bank by default; type and state as in /v1/holidays).

Parameters:

- `kind` (path, required): Which badge. One of: market, holiday.
- `label` (query): Your own label.
- `type` (query): holiday: bank (default), market, central.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `format` (query): svg (default) or png. One of: svg, png.

Example: https://api.dharun.dev/v1/badge/market?utm_medium=skill (returns the image itself)

## Dataset freshness badge

`GET /v1/badge/dataset/{name}`

When one of this API's datasets was last updated; orange when older than 45 days.

Parameters:

- `name` (path, required): A dataset name, as /healthz lists them.
- `format` (query): svg (default) or png. One of: svg, png.

Example: https://api.dharun.dev/v1/badge/dataset/ifsc?utm_medium=skill (returns the image itself)
