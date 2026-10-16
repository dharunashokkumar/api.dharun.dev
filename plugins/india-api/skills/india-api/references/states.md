# States API

> States and union territories with their codes.

All 28 states and 8 union territories with the two-letter code, GST state code and vehicle registration prefixes. The other APIs accept these codes in their state filter.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## All states and union territories

`GET /v1/states`

Example: https://api.dharun.dev/v1/states?utm_medium=skill

## One state

`GET /v1/states/{code}`

Parameters:

- `code` (path, required): State code, vehicle prefix or name.

Example: https://api.dharun.dev/v1/states/TN?utm_medium=skill

## Guides

- [Check a GSTIN or PAN before you save it](https://api.dharun.dev/guides/gstin-pan-check.md): Catch typing mistakes in GSTINs and PANs with the check character, and read the state and PAN out of a GSTIN, in JavaScript.
- [Fill city and state from a pincode in a web form](https://api.dharun.dev/guides/pincode-autofill.md): Look up the district and state as soon as someone types a six-digit pincode, with plain JavaScript and no API key.
