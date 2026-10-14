# Fake test data API

> Made-up Indian people, companies and addresses for testing apps.

Realistic test records that pass format checks: names from nine language regions, addresses at real pincodes with the right district and state, phone numbers, PANs and GSTINs with a correct check character. Every record is marked fake and none describes a real person or business. Give a seed to get the same records every time.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Generate fake records

`GET /v1/fake/{kind}`

people have a name, gender, birth date, email (on example domains), phone, PAN and address; companies a name, PAN, GSTIN, email, phone and address.

Parameters:

- `kind` (path, required): What to generate. One of: people, companies, addresses.
- `count` (query): How many, 1 to 100. Default 10.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `seed` (query): The same seed gives the same records. Without one, each request differs.

Example: https://api.dharun.dev/v1/fake/people?count=3&state=TN&seed=42
