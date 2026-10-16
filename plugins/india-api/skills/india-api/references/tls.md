# TLS check API

> A website's certificate and TLS setup, checked from India.

Connects to a host on port 443 and reports what a browser would care about: whether the certificate is valid for the name and leads to a trusted root, how many days it has left, the chain as the server sends it with each certificate's issuer, key and signature, the TLS version, cipher and HTTP/2 negotiated, OCSP stapling and certificate transparency, and which of TLS 1.0, 1.1, 1.2 and 1.3 the server accepts and whether it takes a weak cipher. Problems come as a plain list, errors and warnings, rather than a grade. Only public addresses are checked, and only port 443. Counts as 20 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Check a host's TLS

`GET /v1/tls/{host}`

valid is false when any problem is an error: expired, not yet valid, name_mismatch, self_signed, incomplete_chain, untrusted, weak_key or sha1_signature. Warnings (expires_soon, old_versions, no_tls13, weak_cipher, chain_order) leave it true. Each version is tried with its own handshake offering every cipher Go supports; SSL 3 is not tested.

Parameters:

- `host` (path, required): Domain name or public IPv4 address.

Example: https://api.dharun.dev/v1/tls/dharun.dev?utm_medium=skill

## Guides

- [Get warned before a TLS certificate expires](https://api.dharun.dev/guides/certificate-expiry.md): Check your sites' certificates from a daily cron job and get an alert when one is close to expiry, invalid or missing an intermediate.
