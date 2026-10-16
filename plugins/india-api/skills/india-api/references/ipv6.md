# IPv6 check API

> Is a domain ready for IPv6: website, nameservers and mail.

Looks up IPv6 (AAAA) addresses for a domain and for www, for each of its nameservers and for each mail server, and says plainly what is missing. Addresses that are not ordinary public IPv6 (6to4, Teredo, NAT64, private) are pointed out. This is a DNS check only: the server has no IPv6 connection, so it does not try to connect over IPv6. Counts as 10 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Check a domain's IPv6

`GET /v1/ipv6/{domain}`

ready is true when the site (the name, or www when the name has no address) has a public IPv6 address, at least one nameserver does, and every mail server does (a domain without mail servers passes that part). website.network names the network of the first IPv6 address, from DB-IP.

Parameters:

- `domain` (path, required): Domain name.

Example: https://api.dharun.dev/v1/ipv6/dharun.dev?utm_medium=skill
