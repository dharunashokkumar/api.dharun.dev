# Mutual fund NAVs API

> Latest net asset value of every Indian mutual fund scheme.

Every scheme in AMFI's daily NAV file: fund house, category, plan (direct or regular), option (growth, IDCW or bonus), ISINs and the latest NAV with its date. Refreshed by the server several times a day; AMFI publishes each evening.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## A scheme and its NAV

`GET /v1/mutual-funds/{code}`

Parameters:

- `code` (path, required): AMFI scheme code, or the scheme's growth or reinvestment ISIN.

Example: https://api.dharun.dev/v1/mutual-funds/122639

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

Example: https://api.dharun.dev/v1/mutual-funds?q=flexi%20cap&plan=direct&option=growth

## Fund houses

`GET /v1/mutual-funds/fund-houses`

Example: https://api.dharun.dev/v1/mutual-funds/fund-houses
