# Index constituents API

> Which companies are in Nifty 50, Bank Nifty, Nifty 500 and 28 more NSE indices, and which indices a stock is in.

The member lists of 31 NSE indices (broad, sector and thematic) as NSE Indices Ltd publishes them, refreshed weekly, each member with its industry, ISIN and logo address. Weights are not published here.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## List indices, or the indices of a stock

`GET /v1/indices`

Parameters:

- `symbol` (query): An NSE symbol or ISIN: only the indices it is in.

Example: https://api.dharun.dev/v1/indices?symbol=HDFCBANK&utm_medium=skill

## Constituents of an index

`GET /v1/indices/{id}`

Parameters:

- `id` (path, required): Such as nifty-50, nifty-bank, nifty-it, nifty-midcap-150.
- `industry` (query): Only members in this industry, as the list names it.

Example: https://api.dharun.dev/v1/indices/nifty-bank?utm_medium=skill
