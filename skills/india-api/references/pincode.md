# Pincodes API

> Pincode to post office, district and state.

Every post office in the Department of Posts directory, with its district, state, postal division and coordinates.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Post offices for a pincode

`GET /v1/pincode/{pincode}`

Returns the district and state of the pincode and every post office that uses it.

Parameters:

- `pincode` (path, required): Six-digit pincode.

Example: https://api.dharun.dev/v1/pincode/637001?utm_medium=skill

## Search post offices

`GET /v1/pincode`

Find post offices by the start of their name, by the start of a pincode, or list a district.

Parameters:

- `q` (query): Start of a post office name (2+ characters) or start of a pincode.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `district` (query): Exact district name.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/pincode?q=namakkal&state=TN&utm_medium=skill

## Post offices near a point

`GET /v1/pincode/near`

Post offices within a radius of a latitude and longitude, nearest first, with the distance. Positions come from the Department of Posts and some are approximate.

Parameters:

- `lat` (query, required): Latitude in decimal degrees.
- `lng` (query, required): Longitude in decimal degrees.
- `radius` (query): Kilometres, 1 to 50. Default 5.
- `limit` (query): Results, 1 to 100. Default 10.

Example: https://api.dharun.dev/v1/pincode/near?lat=11.2189&lng=78.1674&radius=5&utm_medium=skill

## Guides

- [Handle India API errors, rate limits and retries](https://api.dharun.dev/guides/errors-and-limits.md): Every error has the same JSON shape; rate limits are weighted per endpoint and told in RateLimit headers. A reusable client in Python and JavaScript with timeouts, retries and backoff.
- [Pincode autofill and IFSC lookup in a Flutter app](https://api.dharun.dev/guides/flutter-pincode-ifsc.md): Fill in district and state from a pincode, check an IFSC code and show a company logo in Flutter, with the http package and no API key.
- [Pincode autofill and IFSC lookup in an Android app (Kotlin)](https://api.dharun.dev/guides/android-kotlin-pincode-ifsc.md): Fill in district and state from a pincode and check an IFSC code in an Android app with Kotlin coroutines and HttpURLConnection, no library and no key.
- [Fill city and state from a pincode in a web form](https://api.dharun.dev/guides/pincode-autofill.md): Look up the district and state as soon as someone types a six-digit pincode, with plain JavaScript and no API key.
