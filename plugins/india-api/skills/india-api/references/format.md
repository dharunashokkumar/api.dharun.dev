# Indian number formatting API

> Lakh and crore grouping, 1.2 Cr short forms, rupees and Indian-script digits.

One number written every way it is written in India: 12,34,567 grouping, the international form, rupees with paise, short forms (12.3 L, 1.25 Cr), in words, and with the digits of nine Indian scripts.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Format a number the Indian way

`GET /v1/format/number/{number}`

Parameters:

- `number` (path, required): The number: digits, a point, optional commas or a minus sign. Indian-script digits are read too.
- `decimals` (query): Round to this many decimals, 0 to 10.

Example: https://api.dharun.dev/v1/format/number/123456789.5?utm_medium=skill
