{{define "privacy"}}# Privacy

India API is built to know as little about you as possible.

## What is logged

One line per request: the time, the method, the path with its query, the status, how long it took, the browser or program name (user agent), the referring page, and roughly where the request came from: country, state or region, city with that city's coordinates, and the network (internet provider) the address belongs to. The place is looked up from your IP address in DB-IP's free database on this server, before the address is dropped; it is the city of the address range, often not where you are. This is used to find faults and abuse and to see where the API is used. Logs are deleted on a rolling basis.

Each line also gets three labels that say how the request arrived, worked out from that same line and nothing else: the kind of client (a person's browser, a program, an AI assistant, a crawler, another bot), who sent it or led to it (for example claude, chatgpt, googlebot or curl), and which of this site's links it followed. They come from the user agent, the referring site, the name an MCP client gives itself, and two optional query parameters, `utm_source` and `utm_medium`, which the example addresses on this site carry and which AI agents are asked to fill in with their own name. No endpoint reads them. Counts of requests by these labels are kept, with only names from a fixed list; anything else is counted as "other". For the endpoints listed below, only names from that list are logged.

## What is never logged

- **Your IP address.** It is held in memory for up to a minute to count requests against the rate limit, and never written anywhere. To count how many different visitors come each day and each month, the address is also turned into a one-way hash with a random key that is replaced every day (and, for the monthly count, every month), and the hash is folded into a statistical counter (a HyperLogLog sketch). The counter estimates how many distinct addresses it has seen; no address or hash can be read back out of it. The counters and the current keys are kept on the server so a restart does not reset the count, and each is thrown away when its day or month ends.
- **Personal values.** For these endpoints the path and query are replaced by `(redacted)` in the log, and the answer is marked `private, no-store` so no shared cache keeps it: {{range $i, $e := .Private}}{{if $i}}, {{end}}`{{$e}}`{{end}}. The same holds for the matching MCP tools.
- **Request bodies.** Text you POST is processed in memory and dropped once the answer is sent. Translation, moderation, name detection, embeddings and the other AI models run on this server; your text is not sent to any other service or used to train anything.
- **Files you upload.** Pictures, PDFs, recordings and invoices are held in memory, or in a temporary folder only this service can read, while the answer is made, and deleted as soon as it is sent. They are never kept, never used to train anything, and never sent to another service: OCR, image, PDF, speech, speech recognition and noise removal programs all run on this server. When a program fails on a file, the log records only which program failed, not what it was given.

## Sandbox accounts

The code sandbox, the terminal and throwaway Postgres run machines for you, so they need an account: you sign in with GitHub. The API asks GitHub for no permissions, reads only your public profile once, and does not keep GitHub's token. It keeps:

- your GitHub account's numeric ID, login and creation date, and when you first signed in;
- the API's own tokens for your account, stored only as hashes, each for 30 days unless you sign out;
- how many runs and how much processor time your account used each day, for the daily allowance;
- an audit log, kept for one year to answer abuse reports and then deleted: for each run, session, command, terminal and database, the time, your account, which language or machine, the session's ID, a SHA-256 hash of the code or command (not the code itself), the exit code, the processor time and any limit it hit.

What you run, its output, the files you put in a session and what you type in the terminal are never logged or kept: each sandbox is a machine of its own that is destroyed with everything in it when the run or session ends. A throwaway database and its contents are deleted after an hour. To have your account and its records deleted, write to the address below.

## Request bins

A request bin keeps the requests sent to it for up to 24 hours, the last 100: method, path, query, headers and the first 64 KB of the body. The headers that carry the sender's address are dropped before anything is stored. Only the holder of the bin's token can read it, always as JSON, and the bin and everything in it is deleted after 24 hours, or at once when its holder deletes it.

## Cookies and tracking

None. The site sets no cookies and loads no analytics, advertising or third-party scripts, with one exception: the terminal page loads Cloudflare Turnstile, the browser check it needs before starting a machine, which runs in your browser under Cloudflare's privacy policy. The documentation remembers which code language you picked in your own browser's storage, and the terminal page keeps your sandbox token there; neither is sent anywhere else.

## Others who see requests

Requests pass through Cloudflare, which protects the site and sees them on the way, under its own privacy policy. The DNS check, the email check and the IPv6 check send the domain you give to public DNS resolvers, and the traceroute sends probes along the network path to the host you give. The TLS check, the HTTP probe and website icons connect to the site you name and fetch from it, with a user agent that names this API; the site sees a request from this server, not from you. Whois sends the domain, address or AS number you ask about to the registry that holds it (for example NIXI for .in, APNIC for Indian IP addresses). A whois lookup of an IP address is not logged. The browser tools open the page you name in a browser on this server that names itself india-api-browser; the site sees this server, not you, and the page text tool reads only pages the site's robots.txt allows. Signing in to the sandbox goes through GitHub, which sees that you approved the sign-in.

## The data in answers

Answers are public reference data: post offices, bank branches, holidays, codes. The fake data endpoint makes up people and companies; none of them is real.

## Questions

Write to [me@dharun.dev](mailto:me@dharun.dev) or open an issue on [GitHub]({{.Repo}}/issues).
{{end}}
