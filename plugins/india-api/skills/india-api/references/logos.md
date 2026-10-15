# Company logos API

> Logos and details for companies listed on NSE and BSE.

Put a ticker in the URL and get the logo back as SVG or PNG. Every company listed on NSE, those listed only on BSE, and REITs and InvITs. A symbol can be a ticker (TCS), an exchange-qualified ticker (NSE:TCS, NSE_TCS), a Yahoo Finance symbol (TCS.NS, AAIL.BO), an ISIN, or the old ticker of a renamed company. A bare ticker prefers NSE. Delisted companies are kept and marked.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Logo image for a listed company

`GET /v1/logos/{symbol}`

Returns the image itself, ready for an <img> tag. A listed company without a published logo gets an initials tile, marked with the X-Logo-Fallback: monogram header. Unknown symbols return 404 unless fallback=monogram is set.

Parameters:

- `symbol` (path, required): Ticker, exchange-qualified ticker, Yahoo symbol, ISIN or earlier ticker. A .svg or .png suffix selects the format.
- `format` (query): Image format. Default svg. One of: svg, png.
- `size` (query): PNG width and height in pixels, 16 to 512. Default 128.
- `fallback` (query): Return an initials tile instead of 404 for unknown symbols. One of: monogram.
- `exchange` (query): Force an exchange. One of: nse, bse.

Example: https://api.dharun.dev/v1/logos/TCS (returns the image itself)

## Details of a listed company

`GET /v1/companies/{symbol}`

Name, sector, industry, ISIN, market capitalisation, index memberships and logo URLs. listed is false for a delisted company (with delistedOn, the date it was first missed); aliases holds earlier tickers; hasLogo is false when the logo URL serves an initials tile.

Parameters:

- `symbol` (path, required): Ticker, exchange-qualified ticker, Yahoo symbol, ISIN or earlier ticker.
- `exchange` (query): Force an exchange. One of: nse, bse.

Example: https://api.dharun.dev/v1/companies/RELIANCE

## Search listed companies

`GET /v1/companies`

Searches ticker, earlier tickers, company name, sector, industry, ISIN and index names. Best match first. Delisted companies are left out unless delisted=true.

Parameters:

- `q` (query): Search text.
- `exchange` (query): Limit to one exchange. One of: nse, bse.
- `sector` (query): Exact sector name.
- `index` (query): Index name, for example Nifty 50.
- `delisted` (query): Include delisted companies. Default false. One of: true, false.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/companies?q=tata&limit=5

## Guides

- [Show NSE and BSE company logos in your app](https://api.dharun.dev/guides/company-logos.md): Put a listed company's logo in an img tag by ticker or ISIN, as SVG or sized PNG, with an initials tile when no logo exists.
