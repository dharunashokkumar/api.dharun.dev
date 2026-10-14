# HSN and SAC codes API

> GST classification codes for goods and services.

Every HSN code for goods (from the Customs Tariff Act) and SAC code for services (from the GST services classification) with its description, the codes above it and the codes below it. GST rates are not included.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## A code, its parents and children

`GET /v1/hsn/{code}`

Parameters:

- `code` (path, required): 2 to 8 digits; spaces and dots are ignored. SAC codes start with 99.

Example: https://api.dharun.dev/v1/hsn/0902

## Search codes

`GET /v1/hsn`

Every word of q must appear in the description; digits search by code prefix. Broader codes come first.

Parameters:

- `q` (query, required): Words of the description, or the start of a code.
- `kind` (query): hsn for goods, sac for services. One of: hsn, sac.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/hsn?q=green%20tea
