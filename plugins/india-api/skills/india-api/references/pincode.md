# Pincodes API

> Pincode to post office, district and state.

Every post office in the Department of Posts directory, with its district, state, postal division and coordinates.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

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

- [Fill city and state from a pincode in a web form](https://api.dharun.dev/guides/pincode-autofill.md): Look up the district and state as soon as someone types a six-digit pincode, with plain JavaScript and no API key.
