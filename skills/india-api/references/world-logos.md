# Brand, stock and asset logos API

> Logos for unlisted Indian brands, global stocks, crypto, indices, ETFs and commodities.

Three logo sets beside the NSE and BSE listing, with the same options: SVG or PNG, any size, and an initials tile instead of a broken image for a name that matches nothing (X-Logo-Known: false; fallback=none answers 404). Brands are unlisted Indian names people expect, such as PhonePe, Zepto, UPI, DigiLocker, the mutual fund houses and insurers; their artwork is a free-licence file from Wikimedia Commons or the brand site's own icon, and each answer says which. Global stocks are every share on NASDAQ, NYSE and NYSE American and the 500 largest companies on each of LSE, Tokyo, Hong Kong, Xetra and Frankfurt, Euronext, Toronto, ASX and SIX. Assets are the 500 largest coins, the main Indian and world indices, NSE ETFs and the largest US ETFs, and commodities drawn for this API. As with company logos, a 304 answer or an already drawn PNG does not count against the rate limit. The company logos address /v1/logos also looks in these sets, in that order, when a symbol is not an Indian listing, and says where the answer came from in X-Logo-Set.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Logo of an unlisted Indian brand

`GET /v1/brand-logos/{name}`

Returns the image itself. Looked up by name, slug, an earlier or common name, or the brand's domain (phonepe, Google Pay, gpay, phonepe.com). A brand known only by its site icon is a 256-pixel picture; format=svg wraps it in an SVG.

Parameters:

- `name` (path, required): Brand name, slug, alias or domain. A .svg or .png suffix selects the format.
- `format` (query): Image format. Default svg. One of: svg, png.
- `size` (query): PNG width and height in pixels, 16 to 512. Default 128.
- `fallback` (query): What an unknown name gets: monogram (the default, an initials tile) or none (404). One of: monogram, none.
- `category` (query): Only brands of this category. One of: payments, fintech, insurance, mutual-fund, commerce, health, mobility, travel, food, education, media, gaming, telecom, tech, government.

Example: https://api.dharun.dev/v1/brand-logos/phonepe?utm_medium=skill (returns the image itself)

## Search the brands

`GET /v1/brand-logos`

Brands whose name, slug, alias or domain starts with q, or every brand. Each has its logo addresses and, under artwork, where the picture came from and its licence.

Parameters:

- `q` (query): Start of a name, alias or domain.
- `category` (query): Only this category. One of: payments, fintech, insurance, mutual-fund, commerce, health, mobility, travel, food, education, media, gaming, telecom, tech, government.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/brand-logos?q=pay&limit=5&utm_medium=skill

## Logo of a global stock

`GET /v1/stock-logos/{symbol}`

Returns the image itself. A ticker (AAPL), exchange-qualified ticker (NASDAQ:AAPL), Yahoo Finance symbol (VOD.L, 0700.HK, SAP.DE), ISIN or company name (apple). When a ticker is used on several exchanges the largest company wins unless exchange is given.

Parameters:

- `symbol` (path, required): Ticker, exchange-qualified ticker, Yahoo symbol, ISIN or name. A .svg or .png suffix selects the format.
- `format` (query): Image format. Default svg. One of: svg, png.
- `size` (query): PNG width and height in pixels, 16 to 512. Default 128.
- `fallback` (query): What an unknown name gets: monogram (the default, an initials tile) or none (404). One of: monogram, none.
- `exchange` (query): Only this exchange. One of: NASDAQ, NYSE, AMEX, LSE, TSE, HKEX, XETR, FWB, EURONEXT, TSX, ASX, SIX.

Example: https://api.dharun.dev/v1/stock-logos/AAPL?utm_medium=skill (returns the image itself)

## Search the global stocks

`GET /v1/stock-logos`

Stocks whose ticker, name or ISIN starts with q, largest first, or every stock.

Parameters:

- `q` (query): Start of a ticker or name.
- `exchange` (query): Only this exchange. One of: NASDAQ, NYSE, AMEX, LSE, TSE, HKEX, XETR, FWB, EURONEXT, TSX, ASX, SIX.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/stock-logos?q=micro&limit=5&utm_medium=skill

## Logo of a coin, index, ETF or commodity

`GET /v1/asset-logos/{symbol}`

Returns the image itself. BTC (also BTCUSD, BTC-INR), NIFTY50, SENSEX, SPX, NIFTYBEES, GOLD, CRUDEOIL. kind settles a symbol used by two kinds.

Parameters:

- `symbol` (path, required): Symbol or name. A .svg or .png suffix selects the format.
- `format` (query): Image format. Default svg. One of: svg, png.
- `size` (query): PNG width and height in pixels, 16 to 512. Default 128.
- `fallback` (query): What an unknown name gets: monogram (the default, an initials tile) or none (404). One of: monogram, none.
- `kind` (query): Only this kind. One of: crypto, index, etf, commodity.

Example: https://api.dharun.dev/v1/asset-logos/BTC?utm_medium=skill (returns the image itself)

## Search coins, indices, ETFs and commodities

`GET /v1/asset-logos`

Assets whose symbol or name starts with q, largest first, or every asset of a kind.

Parameters:

- `q` (query): Start of a symbol or name.
- `kind` (query): Only this kind. One of: crypto, index, etf, commodity.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/asset-logos?kind=index&limit=5&utm_medium=skill
