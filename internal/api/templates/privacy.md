{{define "privacy"}}# Privacy

India API is built to know as little about you as possible.

## What is logged

One line per request: the time, the method, the path with its query, the status, how long it took, the country of the request, the browser or program name (user agent) and the referring page. This is used to find faults and abuse. Logs are deleted on a rolling basis.

## What is never logged

- **Your IP address.** It is held in memory for up to a minute to count requests against the rate limit, and never written anywhere.
- **Personal values.** For these endpoints the path and query are replaced by `(redacted)` in the log, and the answer is marked `private, no-store` so no shared cache keeps it: {{range $i, $e := .Private}}{{if $i}}, {{end}}`{{$e}}`{{end}}. The same holds for the matching MCP tools.
- **Request bodies.** Text you POST is processed in memory and dropped once the answer is sent.
- **Files you upload.** Pictures, PDFs, recordings and invoices are held in memory, or in a temporary folder only this service can read, while the answer is made, and deleted as soon as it is sent. They are never kept, never used to train anything, and never sent to another service: OCR, image, PDF, speech and noise removal programs all run on this server. When a program fails on a file, the log records only which program failed, not what it was given.

## Cookies and tracking

None. The site sets no cookies and loads no analytics, advertising or third-party scripts. The documentation remembers which code language you picked in your own browser's storage; it is never sent anywhere.

## Others who see requests

Requests pass through Cloudflare, which protects the site and sees them on the way, under its own privacy policy. The DNS check and the email check send the domain you give to public DNS resolvers, and the traceroute sends probes along the network path to the host you give.

## The data in answers

Answers are public reference data: post offices, bank branches, holidays, codes. The fake data endpoint makes up people and companies; none of them is real.

## Questions

Write to [me@dharun.dev](mailto:me@dharun.dev) or open an issue on [GitHub]({{.Repo}}/issues).
{{end}}
