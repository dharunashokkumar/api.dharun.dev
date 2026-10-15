# Security policy

## Reporting a vulnerability

Please report security problems privately, not as a public issue:

- through GitHub: [Report a vulnerability](https://github.com/dharunashokkumar/api.dharun.dev/security/advisories/new)
- or by email to me@dharun.dev or dharun.dak@gmail.com

Say what you found, how to reproduce it and what an attacker could do with it. You will
get a reply, and a fix is released as soon as one is ready. Every push to `main` that
passes CI is released and deployed, so a fix reaches the live service within minutes of
being merged. Please give time for that before you publish details.

When testing, please do not:

- overload the service or try to take it down,
- access, change or delete data that is not yours,
- use the network tools (DNS check, email check, traceroute) against hosts you do not own
  beyond what a normal request does.

The same policy is published at
[api.dharun.dev/.well-known/security.txt](https://api.dharun.dev/.well-known/security.txt).

## Supported versions

Only the latest release is supported: the live service always runs it, and older releases
are not patched.

## Scope

In scope: the code in this repository and the service at `api.dharun.dev`, including its
MCP server at `/mcp`. Reports about the data sources themselves (for example a wrong IFSC
in the Reserve Bank's lists) are welcome as ordinary issues.

## What the service does to limit harm

- No accounts, no API keys and no stored user data.
- Datasets are read-only SQLite files; no request can change them.
- A per-client rate limit, a cap on request bodies, and a fixed number of slots for heavy
  work.
- The network tools refuse private and reserved addresses.
- Image endpoints draw from text alone and never fetch from an address a caller gives.
- Client IP addresses are never logged; endpoints that take personal values keep their
  input out of logs and shared caches.
- CI runs the tests, the race detector, `govulncheck` and `shellcheck` on every change,
  and CodeQL scans the code; the parsers that take caller text have Go fuzz tests.
  Workflow actions are pinned to commits. OpenSSF Scorecard and Dependabot watch the
  repository, which holds the OpenSSF Best Practices passing badge
  ([project 15520](https://www.bestpractices.dev/projects/15520)).

## Verifying a release

Each release bundle is built by GitHub Actions from a public commit and carries a signed
build provenance attestation, also attached to the release as `.intoto.jsonl`:

```bash
gh attestation verify dharun-api-linux-amd64.tar.gz --repo dharunashokkumar/api.dharun.dev
# or with the attestation file downloaded from the release
gh attestation verify dharun-api-linux-amd64.tar.gz --repo dharunashokkumar/api.dharun.dev   --bundle dharun-api-linux-amd64.tar.gz.intoto.jsonl
sha256sum -c dharun-api-linux-amd64.tar.gz.sha256
```
