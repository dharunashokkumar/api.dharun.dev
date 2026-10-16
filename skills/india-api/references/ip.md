# IP address location API

> IP address to country, state, city and network.

Country, region (the state, for Indian addresses), city and network operator of a public IPv4 or IPv6 address, from DB-IP's free databases, refreshed monthly. Addresses are never written to the request log.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

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
