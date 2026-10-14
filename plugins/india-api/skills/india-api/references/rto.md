# RTO codes API

> Vehicle registration code to transport office.

Which Regional Transport Office a number plate belongs to: TN-28 is Namakkal (North). Accepts a code or a whole registration number.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Office for an RTO code

`GET /v1/rto/{code}`

Parameters:

- `code` (path, required): RTO code (TN-28, TN28) or a registration number (TN28AB1234).

Example: https://api.dharun.dev/v1/rto/TN-28

## List or search RTO codes

`GET /v1/rto`

Parameters:

- `state` (query): State code or name, for example TN or Tamil Nadu.
- `q` (query): Text to find in the office name or jurisdiction.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/rto?state=TN&q=chennai
