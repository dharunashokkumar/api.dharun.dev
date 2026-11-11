# IP address location API

> IP address to country, state, city and network.

Country, region (the state, for Indian addresses), city and network operator of a public IPv4 or IPv6 address, from DB-IP's free databases, refreshed monthly. Addresses are never written to the request log.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Location of an IP address

`GET /v1/ip/{address}`

What you send is never written to the log, and the answer is not cached.

Parameters:

- `address` (path, required): IPv4 or IPv6 address.

Example: https://api.dharun.dev/v1/ip/49.204.0.1?utm_medium=skill

## Location of your own address

`GET /v1/ip`

What you send is never written to the log, and the answer is not cached.

Example: https://api.dharun.dev/v1/ip?utm_medium=skill
