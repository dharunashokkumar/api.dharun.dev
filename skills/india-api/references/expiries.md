# Derivative expiries API

> NSE and BSE futures and options expiry dates, moved for holidays.

Every weekly and monthly expiry of a year from the exchange rules in force since 1 September 2025: NSE on Tuesdays (Nifty 50 weekly), BSE on Thursdays (Sensex weekly), monthly contracts on the month's last one; an expiry on a holiday moves to the trading day before.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Expiry dates

`GET /v1/expiries`

Parameters:

- `exchange` (query): Default nse. One of: nse, bse.
- `year` (query): From 2025. Default this year.
- `month` (query): 1 to 12.
- `weekly` (query): false lists only the monthly expiries.

Example: https://api.dharun.dev/v1/expiries?exchange=nse&year=2026&month=3&utm_medium=skill
