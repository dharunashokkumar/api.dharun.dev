# Mutual fund NAVs API

> Latest net asset value of every Indian mutual fund scheme.

Every scheme in AMFI's daily NAV file: fund house, category, plan (direct or regular), option (growth, IDCW or bonus), ISINs and the latest NAV with its date. Refreshed by the server several times a day; AMFI publishes each evening.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## A scheme and its NAV

`GET /v1/mutual-funds/{code}`

Parameters:

- `code` (path, required): AMFI scheme code, or the scheme's growth or reinvestment ISIN.

Example: https://api.dharun.dev/v1/mutual-funds/122639?utm_medium=skill

## Search schemes

`GET /v1/mutual-funds`

Every word of q must appear in the scheme name, in any order. Only active schemes unless all=true.

Parameters:

- `q` (query): Words of the scheme name.
- `fundHouse` (query): Part of the fund house name, such as hdfc.
- `category` (query): Part of the SEBI category, such as Small Cap.
- `plan` (query): Plan. One of: direct, regular.
- `option` (query): Option. One of: growth, idcw, bonus.
- `all` (query): Include wound-up and matured schemes. One of: true.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/mutual-funds?q=flexi%20cap&plan=direct&option=growth&utm_medium=skill

## Fund houses

`GET /v1/mutual-funds/fund-houses`

Example: https://api.dharun.dev/v1/mutual-funds/fund-houses?utm_medium=skill

## Returns and SIP returns

`GET /v1/mutual-funds/{code}/returns`

Point-to-point returns over 1, 3 and 6 months and 1, 3, 5 and 10 years (CAGR from a year), and what a monthly SIP of 1,000 would be worth now (XIRR), from AMFI's NAV history.

Parameters:

- `code` (path, required): AMFI scheme code or ISIN.

Example: https://api.dharun.dev/v1/mutual-funds/122639/returns?utm_medium=skill

## NAV history

`GET /v1/mutual-funds/{code}/history`

Month-end NAVs for up to ten years and every NAV of the last 45 days.

Parameters:

- `code` (path, required): AMFI scheme code or ISIN.
- `from` (query): From this date, YYYY-MM-DD.
- `to` (query): Up to this date.

Example: https://api.dharun.dev/v1/mutual-funds/122639/history?from=2026-01-01&utm_medium=skill
