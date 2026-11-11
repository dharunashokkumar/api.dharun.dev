# Developer utilities API

> Cron in words, UUIDs and ULIDs, hashes and HMACs, JWT decoding, user agents and time zones.

Small tools a program or an assistant needs often. Times are in India time unless you name a zone.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Explain a cron expression

`GET /v1/cron`

The expression in words and its next run times.

Parameters:

- `expr` (query, required): Five fields, or @daily, @hourly, @weekly, @monthly, @yearly.
- `tz` (query): Time zone the schedule runs in. Default Asia/Kolkata.
- `count` (query): Next runs, 1 to 50. Default 5.
- `from` (query): Start from this time instead of now.

Example: https://api.dharun.dev/v1/cron?expr=30%209%20*%20*%201-5&utm_medium=skill

## New UUIDs, ULIDs and nano ids

`GET /v1/uuid`

Parameters:

- `kind` (query): Default uuid4. One of: uuid4, uuid7, ulid, nanoid.
- `count` (query): 1 to 100. Default 1.

Example: https://api.dharun.dev/v1/uuid?kind=uuid7&count=3&utm_medium=skill

## Read a UUID or ULID

`GET /v1/uuid/{id}`

Its version and variant, and the time inside a version 1 or 7 UUID or a ULID.

Parameters:

- `id` (path, required): The UUID or ULID.

Example: https://api.dharun.dev/v1/uuid/01936f4e-5a6a-7cc2-9a4b-3e5b2a1f0c11?utm_medium=skill

## Hashes and HMACs

`GET /v1/hash`, also `POST`

MD5, SHA-1, SHA-2, SHA-3, BLAKE2b and CRC-32 of a text, in hex and base64; with key, HMACs.

POST body: The text, as text/plain, or JSON {"text": "...", "algo": "sha256", "key": "..."}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text.
- `algo` (query): Comma-separated: md5, sha1, sha256, sha384, sha512, sha3-256, sha3-512, blake2b-256, crc32. Default md5,sha1,sha256,sha512.
- `key` (query): For an HMAC. Send it in a POST body.

Example: https://api.dharun.dev/v1/hash?text=hello&algo=sha256,md5&utm_medium=skill

## Decode a JWT

`GET /v1/jwt/decode`, also `POST`

The header, payload and times of a JSON Web Token. The signature is not checked.

POST body: The token, as text/plain or JSON {"text": "..."}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The token.

Example: https://api.dharun.dev/v1/jwt/decode?text=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJkZW1vIiwiaWF0IjoxNzYyODQ4MDAwfQ.c2ln&utm_medium=skill

## Read a user agent

`GET /v1/user-agent`

Browser, engine, system, device and vendor, and whether it is a person, an AI assistant, a crawler or a program. Without ua, your own request's.

Parameters:

- `ua` (query): A User-Agent header. Default: yours.

Example: https://api.dharun.dev/v1/user-agent?ua=Mozilla%2F5.0%20(Linux%3B%20Android%2014%3B%20SM-S918B)%20AppleWebKit%2F537.36%20(KHTML%2C%20like%20Gecko)%20Chrome%2F131.0.0.0%20Mobile%20Safari%2F537.36&utm_medium=skill

## Convert a time between zones

`GET /v1/time`

"3 pm IST in Berlin": a time read many ways (2026-11-11 15:00, 3pm, a Unix time) in one zone, shown in others with their offsets and the day difference.

Parameters:

- `at` (query): The time. Default now.
- `from` (query): Its zone: IANA name, short name, city or offset. Default India.
- `to` (query): Zones to show it in, separated by commas. Default UTC.

Example: https://api.dharun.dev/v1/time?at=2026-11-11%2015%3A00&from=IST&to=Europe%2FBerlin,PST,SGT&utm_medium=skill
