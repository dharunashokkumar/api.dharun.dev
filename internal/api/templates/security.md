{{define "security"}}# Security

How to report a problem, what the service does to limit harm, and how to check what it runs.

## Report a vulnerability

Report it privately through [GitHub's vulnerability form]({{.Repo}}/security/advisories/new), or by email to [me@dharun.dev](mailto:me@dharun.dev) or [dharun.dak@gmail.com](mailto:dharun.dak@gmail.com). Please do not open a public issue, do not test by overloading the service, and give time for a fix before you publish. The same policy is in [security.txt]({{.Origin}}/.well-known/security.txt) and [SECURITY.md]({{.Repo}}/blob/main/SECURITY.md).

## How the service limits harm

- No accounts, no keys and no stored user data, so there is nothing of yours to leak.
- Datasets are opened read-only; no request can change them.
- Every client has a rate limit ({{.Limit}} requests a minute), request bodies are capped at {{.BodyLimit}}, and heavy work runs in a fixed number of slots, so a burst waits in line instead of exhausting the server.
- The network tools refuse private and reserved addresses, so they cannot be used to reach inside a network.
- The image endpoints draw from text alone and never fetch anything from an address you give.
- Personal values are kept out of logs and shared caches (see [privacy]({{.Origin}}/privacy)).

## Check the code

All of it is open source on [GitHub]({{.Repo}}). Every change runs the tests, Go's race detector, `govulncheck` against the Go vulnerability database and `shellcheck` before it can be released, and CodeQL scans the code. The parsers that take your text have fuzz tests. [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/{{.RepoSlug}}) grades the repository's security practices every week, the project holds the [OpenSSF Best Practices](https://www.bestpractices.dev/projects/15520) passing badge, and Dependabot proposes dependency updates.

## Verify a release

Each release is built by GitHub Actions from a public commit and carries a signed build provenance attestation, also attached to the release as `.intoto.jsonl`. To check a downloaded bundle:

```
gh attestation verify dharun-api-linux-amd64.tar.gz --repo {{.RepoSlug}}
sha256sum -c dharun-api-linux-amd64.tar.gz.sha256
```

Add `--bundle dharun-api-linux-amd64.tar.gz.intoto.jsonl` to check against the attestation file from the release instead of fetching it from GitHub.

## Using it safely

- Treat it like any outside service: set a timeout, handle errors, and cache answers you reuse.
- Format checks (GSTIN, PAN, IFSC) confirm the structure, not that the identifier was ever issued.
- Send only the personal data a task needs.
{{end}}
