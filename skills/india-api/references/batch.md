# Batch lookups API

> Up to 1,000 lookups in one request.

Pincodes, IFSC codes, banks, phone numbers, RTO codes, HSN codes, companies, states, airports, stations, GSTIN and PAN checks and number to words, many at once. Each answer is exactly what the lookup's own endpoint returns, in the order sent. Ten lookups cost one unit of the rate limit. What you send is never logged.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Many lookups in one request

`POST /v1/batch`

Answers { data: [{kind, value, status, data or error}], meta }. With format=ndjson, one JSON line per answer instead.

Send: JSON: {"kind": "pincode", "values": [...]} for one kind, or {"lookups": [{"kind": "ifsc", "value": "..."}, ...]} for a mix. Kinds: airport, bank, company, gstin, hsn, ifsc, ifsc-format, pan, phone, pincode, rto, state, station, words.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `format` (query): Default json. One of: json, ndjson.

Example: `POST` https://api.dharun.dev/v1/batch?utm_medium=skill

Example body (invoice.json):

```json
{
  "kind": "pincode",
  "values": ["637001", "600001", "110001"]
}
```
