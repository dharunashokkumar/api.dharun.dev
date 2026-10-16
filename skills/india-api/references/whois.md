# Whois API

> Who registered a domain, an IP network or an AS number, through RDAP.

RDAP is the registries' JSON replacement for the old whois protocol. Give a domain, an IP address or an AS number; IANA's bootstrap registry names the registry that answers for it (NIXI for .in and .co.in, Verisign for .com, APNIC for Indian IP addresses), and the registration is fetched from it on the spot. For a domain: the registrar and its abuse contact, when it was registered and when it expires, its status codes, nameservers and DNSSEC. For an IP network or AS number: the range, its name, the organisation that holds it, country and abuse contact. The contacts of the people behind a domain are never returned, and a network held by a person is not named. Domains under the Indian-script top-level domains (.भारत) cannot be looked up: their registry offers no RDAP. Counts as 10 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Look up a registration

`GET /v1/whois/{query}`

A name below a registered domain (www.example.co.in) is looked up as that domain (example.co.in). daysToExpiry counts from now. An IP lookup is treated as private: not logged, not cached.

Parameters:

- `query` (path, required): A domain name (Indian-script names are accepted), an IPv4 or IPv6 address, or an AS number such as AS55836.

Example: https://api.dharun.dev/v1/whois/google.in

## Guides

- [Get warned before a TLS certificate expires](https://api.dharun.dev/guides/certificate-expiry.md): Check your sites' certificates from a daily cron job and get an alert when one is close to expiry, invalid or missing an intermediate.
