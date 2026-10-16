# Traceroute from India API

> The network path from a server in India to any host, with loss and latency per hop.

An MTR-style trace from the server's own connection in India: each hop is probed several times, and every hop comes back with its address, name, network (AS) and country, packet loss and round-trip times. Useful to see how Jio, Airtel, BSNL and the rest reach a site, and where along the way delay or loss starts. Only public addresses can be traced. Hops inside private or shared address space are shown without their address. A trace takes several seconds and counts as 20 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Trace the route to a host

`GET /v1/mtr/{host}`

Lists hops up to the one where the host answered, or one past the last router that answered. A hop with 100% loss is common: many routers forward traffic without answering probes. otherAddresses lists further routers seen at the same hop, where traffic is balanced over several paths.

Parameters:

- `host` (path, required): Domain name or public IP address.
- `rounds` (query): Probes per hop, 1 to 10. Default 5; a trace takes about a second per round.
- `v` (query): IP version for a domain name. Default 4. One of: 4, 6.

Example: https://api.dharun.dev/v1/mtr/1.1.1.1?utm_medium=skill
