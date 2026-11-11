# Mandi prices API

> Today's wholesale prices of vegetables, fruit, grain and pulses in India's markets, from Agmarknet.

The prices that agricultural markets (mandis) report to Agmarknet each day: minimum, maximum and modal price in rupees a quintal, by market, commodity, variety and grade. From data.gov.in, checked every three hours, the last 30 days kept.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Prices by commodity, state, district or market

`GET /v1/mandi`

The latest day's prices unless date is given, 100 rows unless limit says otherwise.

Parameters:

- `commodity` (query): As Agmarknet names it: Tomato, Onion, Potato, Paddy(Dhan)(Common) ... (see /v1/mandi/commodities).
- `state` (query): Such as Tamil Nadu.
- `district` (query): Such as Dharmapuri.
- `market` (query): Part of a market's name.
- `date` (query): YYYY-MM-DD, within the last 30 days. Default the latest.
- `limit` (query): 1 to 1000. Default 100.

Example: https://api.dharun.dev/v1/mandi?commodity=Tomato&state=Tamil%20Nadu&utm_medium=skill

## What was reported on the latest day

`GET /v1/mandi/commodities`

Each commodity with how many markets reported it and the range of their modal prices.

Parameters:

- `state` (query): Only this state's markets.

Example: https://api.dharun.dev/v1/mandi/commodities?state=Karnataka&utm_medium=skill
