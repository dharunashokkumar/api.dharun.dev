# DNS check API

> A domain's DNS records at four public resolvers at once.

Asks Cloudflare, Google, Quad9 and OpenDNS for the same record and says whether they agree: useful when a change is still spreading. Counts as 10 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Look up DNS records

`GET /v1/dns/{name}`

Parameters:

- `name` (path, required): Domain name (Indian-script names are accepted), or an IP address for PTR.
- `type` (query): Record type. Default A. One of: A, AAAA, CNAME, MX, NS, TXT, CAA, SOA, SRV, PTR.

Example: https://api.dharun.dev/v1/dns/dharun.dev?type=MX
