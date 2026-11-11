# Company logos API

> Logos and details for companies listed on NSE and BSE.

Put a ticker in the URL and get the logo back as SVG or PNG. Every company listed on NSE, those listed only on BSE, and REITs and InvITs; a symbol that is not an Indian listing falls through to the brand, global stock and asset logo sets. A symbol can be a ticker (TCS), an exchange-qualified ticker (NSE:TCS, NSE_TCS), a Yahoo Finance symbol (TCS.NS, AAIL.BO), an ISIN, or the old ticker of a renamed company. A bare ticker prefers NSE. Delisted companies are kept and marked.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Logo image for a listed company

`GET /v1/logos/{symbol}`

Returns the image itself, ready for an <img> tag. A listed company without a published logo gets an initials tile, marked with the X-Logo-Fallback: monogram header. An unknown symbol also gets the tile, with status 200, the headers X-Logo-Fallback: monogram and X-Logo-Known: false, and a one-day cache lifetime, so a logo added later shows up soon; fallback=none answers 404 instead. A symbol that is not an Indian listing is looked up in the brand, global stock and asset logo sets, in that order, and X-Logo-Set says which answered (india, brand, stock or asset). Unknown names are counted (the name only) so that their logos can be added. A 304 answer, or a PNG this server has already drawn, does not count against the rate limit.

Parameters:

- `symbol` (path, required): Ticker, exchange-qualified ticker, Yahoo symbol, ISIN or earlier ticker. A .svg or .png suffix selects the format.
- `format` (query): Image format. Default svg. One of: svg, png.
- `size` (query): PNG width and height in pixels, 16 to 512. Default 128.
- `fallback` (query): What an unknown symbol gets: monogram (the default, an initials tile) or none (404). One of: monogram, none.
- `exchange` (query): Force an exchange: nse or bse for the Indian listing, or a global exchange for the global stocks. One of: nse, bse, nasdaq, nyse, amex, lse, tse, hkex, xetr, fwb, euronext, tsx, asx, six.

Example: https://api.dharun.dev/v1/logos/TCS?utm_medium=skill (returns the image itself)

## Details of a listed company

`GET /v1/companies/{symbol}`

Name, sector, industry, ISIN, market capitalisation, index memberships and logo URLs. listed is false for a delisted company (with delistedOn, the date it was first missed); aliases holds earlier tickers; hasLogo is false when the logo URL serves an initials tile.

Parameters:

- `symbol` (path, required): Ticker, exchange-qualified ticker, Yahoo symbol, ISIN or earlier ticker.
- `exchange` (query): Force an exchange. One of: nse, bse.

Example: https://api.dharun.dev/v1/companies/RELIANCE?utm_medium=skill

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

Example: https://api.dharun.dev/v1/companies?q=tata&limit=5&utm_medium=skill

## Sectors and industries of listed companies

`GET /v1/sectors`

Every sector and industry of the NSE and BSE listed companies with how many companies each has; give sector to list its industries and largest companies.

Parameters:

- `sector` (query): One sector, as listed.

Example: https://api.dharun.dev/v1/sectors?utm_medium=skill

## Guides

- [Show NSE and BSE company logos in your app](https://api.dharun.dev/guides/company-logos.md): Put a listed company's logo in an img tag by ticker or ISIN, as SVG or sized PNG, with an initials tile when no logo exists.
