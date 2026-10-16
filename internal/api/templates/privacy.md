{{define "privacy"}}# Privacy

India API is built to know as little about you as possible.

## What is logged

One line per request: the time, the method, the path with its query, the status, how long it took, the browser or program name (user agent), the referring page, and roughly where the request came from: country, state or region, city with that city's coordinates, and the network (internet provider) the address belongs to. The place is looked up from your IP address in DB-IP's free database on this server, before the address is dropped; it is the city of the address range, often not where you are. This is used to find faults and abuse and to see where the API is used. Logs are deleted on a rolling basis.

## What is never logged

- **Your IP address.** It is held in memory for up to a minute to count requests against the rate limit, and never written anywhere. To count how many different visitors come each day and each month, the address is also turned into a one-way hash with a random key that is replaced every day (and, for the monthly count, every month), and the hash is folded into a statistical counter (a HyperLogLog sketch). The counter estimates how many distinct addresses it has seen; no address or hash can be read back out of it. The counters and the current keys are kept on the server so a restart does not reset the count, and each is thrown away when its day or month ends.
- **Personal values.** For these endpoints the path and query are replaced by `(redacted)` in the log, and the answer is marked `private, no-store` so no shared cache keeps it: {{range $i, $e := .Private}}{{if $i}}, {{end}}`{{$e}}`{{end}}. The same holds for the matching MCP tools.
- **Request bodies.** Text you POST is processed in memory and dropped once the answer is sent. Translation, moderation, name detection, embeddings and the other AI models run on this server; your text is not sent to any other service or used to train anything.
- **Files you upload.** Pictures, PDFs, recordings and invoices are held in memory, or in a temporary folder only this service can read, while the answer is made, and deleted as soon as it is sent. They are never kept, never used to train anything, and never sent to another service: OCR, image, PDF, speech, speech recognition and noise removal programs all run on this server. When a program fails on a file, the log records only which program failed, not what it was given.

## Cookies and tracking

None. The site sets no cookies and loads no analytics, advertising or third-party scripts. The documentation remembers which code language you picked in your own browser's storage; it is never sent anywhere.

## Others who see requests

Requests pass through Cloudflare, which protects the site and sees them on the way, under its own privacy policy. The DNS check, the email check and the IPv6 check send the domain you give to public DNS resolvers, and the traceroute sends probes along the network path to the host you give. The TLS check, the HTTP probe and website icons connect to the site you name and fetch from it, with a user agent that names this API; the site sees a request from this server, not from you. Whois sends the domain, address or AS number you ask about to the registry that holds it (for example NIXI for .in, APNIC for Indian IP addresses). A whois lookup of an IP address is not logged.

## The data in answers

Answers are public reference data: post offices, bank branches, holidays, codes. The fake data endpoint makes up people and companies; none of them is real.

## Questions

Write to [me@dharun.dev](mailto:me@dharun.dev) or open an issue on [GitHub]({{.Repo}}/issues).
{{end}}
