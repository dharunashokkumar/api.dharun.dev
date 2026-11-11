# RBI rates and exchange rates API

> Repo rate, CRR and SLR, FBIL reference exchange rates, lending and deposit rates, and bond yields.

What the Reserve Bank of India shows as its current rates, checked every three hours: the policy rates, reserve ratios, FBIL reference rates for the dollar, pound, euro, yen, dirham, rupiah and rouble, the range of base rates and MCLR, deposit rates, treasury bill cut-offs and government bond yields. Exchange rate history builds up from the day this server began recording.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Current RBI rates

`GET /v1/rates`

Example: https://api.dharun.dev/v1/rates?utm_medium=skill

## Reference exchange rates

`GET /v1/fx`

Parameters:

- `currency` (query): One currency. Default all. One of: USD, GBP, EUR, JPY, AED, IDR, RUB.
- `days` (query): History for this many days, 1 to 3650, instead of the latest.

Example: https://api.dharun.dev/v1/fx?currency=USD&utm_medium=skill
