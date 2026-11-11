{{define "privacy"}}# Privacy

India API is built to know as little about you as possible.

## What is logged

One line per request: the time, the method, the path with its query, the status, how long it took, the browser or program name (user agent), the referring page, and roughly where the request came from: country, state or region, city with that city's coordinates, and the network (internet provider) the address belongs to. The place is looked up from your IP address in DB-IP's free database on this server, before the address is dropped; it is the city of the address range, often not where you are. This is used to find faults and abuse and to see where the API is used. Logs are deleted on a rolling basis.

Each line also gets three labels that say how the request arrived, worked out from that same line and nothing else: the kind of client (a person's browser, a program, an AI assistant, a crawler, another bot), who sent it or led to it (for example claude, chatgpt, googlebot or curl), and which of this site's links it followed. They come from the user agent, the referring site, the name an MCP client gives itself, and two optional query parameters, `utm_source` and `utm_medium`, which the example addresses on this site carry and which AI agents are asked to fill in with their own name. No endpoint reads them. Counts of requests by these labels are kept, with only names from a fixed list; anything else is counted as "other". For the endpoints listed below, only names from that list are logged.

A failed request is logged with its error code (for example `not_found` or `rate_limited`). A request that looks like an attack is labelled with the kind of attack and where in the request it was found: a probe for files or pages this API does not have (such as `.env` or `wp-login.php`), an injection (SQL, script, command, Log4Shell), or a known attack tool. The line then also keeps up to 200 characters of what was sent around the match, from the path, the query, a header, or the body of a request to an address this API does not have. For the endpoints listed below only the kind of attack is logged, never what was sent. Headers that carry credentials or addresses (`Authorization`, `Cookie` and the forwarding headers) are never read for this. Counts of attacks by kind and of errors by code are kept.

When a company logo is asked for by a name that matches no company, that name is counted, so that its logo can be added: the name in capitals with anything other than letters, digits and `& - _ .` removed, at most 24 characters, with how often and when it was first and last asked for. Nothing about who asked is kept with it. The list holds at most 5,000 names and stays on the server.

## What is never logged

- **Your IP address.** It is held in memory for up to a minute to count requests against the rate limit, and never written anywhere. To count how many different visitors come each day and each month, the address is also turned into a one-way hash with a random key that is replaced every day (and, for the monthly count, every month), and the hash is folded into a statistical counter (a HyperLogLog sketch). The counter estimates how many distinct addresses it has seen; no address or hash can be read back out of it. The counters and the current keys are kept on the server so a restart does not reset the count, and each is thrown away when its day or month ends.
- **Personal values.** For these endpoints the path and query are replaced by `(redacted)` in the log, and the answer is marked `private, no-store` so no shared cache keeps it: {{range $i, $e := .Private}}{{if $i}}, {{end}}`{{$e}}`{{end}}. The same holds for the matching MCP tools.
- **Request bodies.** Text you POST to an endpoint is processed in memory and dropped once the answer is sent. Only a body sent to an address this API does not have is read, to see whether it is an attack, as described above. Translation, moderation, name detection, embeddings and the other AI models run on this server; your text is not sent to any other service or used to train anything.
- **Files you upload.** Pictures, PDFs, recordings and invoices are held in memory, or in a temporary folder only this service can read, while the answer is made, and deleted as soon as it is sent. They are never kept, never used to train anything, and never sent to another service: OCR, image, PDF, speech, speech recognition and noise removal programs all run on this server. When a program fails on a file, the log records only which program failed, not what it was given.

## Accounts and API keys

An account is optional: most endpoints need none. If you make one, it holds your email address, the SHA-256 hashes of your keys (never the keys themselves), their names and creation dates, and how many requests each key made per day and API group. The usage counts are kept by the API on the server, not in the request log. On the endpoints that need a key, the log line holds the key's id (never the key), so that abuse can be traced to an account and stopped; on every other endpoint nothing about keys is logged. The account system (PocketBase) keeps no request log of its own and never sees your IP address.

There are no passwords: you log in with a six-digit code mailed to you, which works once for 10 minutes. Asking for a code runs a short proof-of-work check in your browser (it works out a small sum, nothing is loaded from another site), and the number of new accounts per IP address and of codes per email address each day are counted in memory, by keyed hashes. Throwaway email domains are refused.

Delete your account on the account page: the account, its keys and their usage counts go at once.

## Cookies and tracking

None. The site sets no cookies and loads no analytics, advertising or third-party scripts. Your own browser's storage keeps what you choose there, and none of it is sent anywhere unless you send a request with it: which code language you picked in the documentation, the login token and the key you made (on the account page, so the playground and try-it panels can use it), your last playground requests (never those to private endpoints), and for an hour the token the free tool pages use.

## Others who see requests

Requests pass through Cloudflare, which protects the site and sees them on the way, under its own privacy policy. If you ask for a login code, Amazon Web Services (Amazon SES), which delivers the mail, receives your email address and the mail with the code. The DNS check, the email check and the IPv6 check send the domain you give to public DNS resolvers, and the traceroute sends probes along the network path to the host you give. The TLS check, the HTTP probe and website icons connect to the site you name and fetch from it, with a user agent that names this API; the site sees a request from this server, not from you. Whois sends the domain, address or AS number you ask about to the registry that holds it (for example NIXI for .in, APNIC for Indian IP addresses). A whois lookup of an IP address is not logged.

Link previews and the page reader fetch the address you give from the site that serves it (the reader reads its robots.txt first). These sites see a request from this server, with a user agent that names this API.

## What request bins, databases, document collections and notes keep

A request bin keeps the requests sent to it (method, path, query, headers and up to 64 KB of body each, never the sender's address), and a throwaway database keeps the rows put in it, for a day at most, in the server's memory or a temporary file, and only for someone who has its secret. A restart or the delete address removes them sooner. Document collections (for search over your own documents) are kept the same way, in memory for a day. Agent notes are kept in a file on the server for 30 days after their last write, so they outlive a restart; deleting a note removes it. None of these is ever logged or backed up. The bin, database, collection and note store named demo are public, for trying the API: do not send anything private to them.

## The data in answers

Answers are public reference data: post offices, bank branches, holidays, codes. The fake data endpoint makes up people and companies; none of them is real.

## Questions

Write to [me@dharun.dev](mailto:me@dharun.dev) or open an issue on [GitHub]({{.Repo}}/issues).
{{end}}
