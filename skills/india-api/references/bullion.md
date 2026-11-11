# Gold and silver rates API

> Today's gold (24K, 22K, 18K...), silver and platinum rates from IBJA, and their history.

The India Bullion and Jewellers Association's benchmark rates, published each working day in two sessions, checked every two hours. Before GST and making charges. Gold and platinum per 10 grams, silver per kilogram, with the price per gram.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Today's rates

`GET /v1/bullion`

Example: https://api.dharun.dev/v1/bullion?utm_medium=skill

## Rates over time

`GET /v1/bullion/history`

Parameters:

- `metal` (query): Default gold. One of: gold, silver, platinum.
- `purity` (query): Default 999. One of: 999, 995, 916, 750, 585.
- `days` (query): 1 to 3650. Default 30.

Example: https://api.dharun.dev/v1/bullion/history?metal=gold&purity=916&days=30&utm_medium=skill
