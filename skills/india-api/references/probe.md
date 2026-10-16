# HTTP probe from India API

> How fast a URL answers from India: DNS, connect, TLS, first byte and the CDN edge.

Fetches a URL from a server in India and times each step: DNS lookup, TCP connect, TLS handshake, time to first byte and download. It follows up to five redirects, lists them, and returns the status, HTTP version, response headers and which CDN answered and from which city (Cloudflare, CloudFront, Fastly, Vercel and others), so you can see whether Indian visitors are served from Mumbai or Chennai or sent abroad. The body is read, counted and hashed, never returned, and cookie values are hidden. Only http on port 80 and https on 443, only public addresses, GET only. Requests are sent with the user agent IndiaAPI-check/1.0 (+https://api.dharun.dev/docs/probe); a site that does not want them can refuse that user agent. Each site gets at most 10 checks a minute from all callers together. Counts as 20 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Time a URL from India

`GET /v1/probe`

With rounds above 1 the URL is fetched that many times and summary gives the fastest, median and slowest total; the other fields describe the first round. A redirect into private address space, to another port or to anything other than http or https is refused. The URL may carry a token, so the request is not logged and the answer is not cached.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `url` (query, required): An http or https address. A bare host name is taken as https.
- `rounds` (query): Times to fetch it, 1 to 5. Default 1.

Example: https://api.dharun.dev/v1/probe?url=https://dharun.dev&utm_medium=skill
