# Part 2: free accounts and API keys

Dharun, 2026-11-10:

- Big, compute-heavy endpoints are offered **only with an API key**. The key is free.
- People come to api.dharun.dev, log in with **email and password, Google or GitHub**,
  get their key and go. Not GitHub-only.
- A key also gives **higher limits on every other endpoint**, logos included.
- Do not spend effort building an account system. **Attach an open-source one:
  PocketBase**, run beside the API.
- Heavy endpoints that are public today **move behind the key**.

This reopens "no API keys, rate limit per IP" (2026-10-13). What stays true: light
endpoints work without a key, and nothing costs money.

## 1. Why PocketBase

Looked at on 2026-11-10:

| Option | Verdict |
|---|---|
| **PocketBase** (Go, MIT, SQLite, one program; v0.39.x in mid-2026, still before 1.0) | Chosen. Signup, login, email verification, password reset, OAuth logins and an admin screen exist already. About 30 MB of memory. It can be used as a Go library, so our small additions live in this repository and are tested in CI |
| Unkey (key management) | Needs MySQL and Redis, has no signup pages, and directories disagree on its licence |
| Keycloak, Zitadel, Authentik | Full identity servers: their own database, 500 MB to 1 GB of memory, much more to run than the API itself |
| Go libraries inside the API (authboss, goth) | One binary, but the security-sensitive code becomes ours to write and keep right |

Risk accepted: PocketBase is before 1.0 and changes between minor versions. Pin one
version, read its changelog before raising it, and keep our additions small.

## 2. Shape

```text
            browser                         programs, agents
               |                                   |
   /signup /login /account                 Authorization: Bearer <key>
               |                                   |
               v                                   v
        +------------------------- api (the Go service) ------------------------+
        |  serves the account pages (our design, no framework)                  |
        |  passes /account/api/* through to the accounts program                |
        |  checks a key against an in-memory table, charges the key's allowance |
        +------------+----------------------------------------------+-----------+
                     | localhost only                               | reads, read-only
                     v                                              v
        accounts program (PocketBase as a Go library,     accounts database (SQLite)
        cmd/accounts): users, keys, mail, OAuth
```

- **`cmd/accounts`**: a small Go program that imports PocketBase at a pinned version and
  adds the `api_keys` collection, the rule that a key is shown once and stored only as a
  SHA-256 hash, a cap on keys per user, and the migrations. It ships in the release
  bundle and runs as its own systemd unit in the same container, listening on localhost
  only. "One container" stays true.
- **The admin screen is never public.** The API passes through only the few account
  addresses the pages need, never PocketBase's admin path.
- **The pages** (`/signup`, `/login`, `/account`) are ours, in the site's design, with
  plain JavaScript served from the binary, calling the passed-through addresses.
- **Checking a key costs no network call.** The API keeps a table of key hash to
  account, tier and state in memory, reloaded from the accounts database every 15
  seconds the way datasets are reloaded, with one direct lookup when an unknown key
  arrives. A revoked key stops working within 15 seconds.
- **This is the first state on the server that cannot be rebuilt.** Datasets can be
  fetched again; accounts cannot. A backup of the accounts database is part of the
  infrastructure request, and so is where it lives (`STATE_DIR/accounts/`), which a
  deploy must never touch.

## 3. Login methods

| Method | Needs |
|---|---|
| Email and password | A mail service for the verification and reset mails; a bot check on signup; PocketBase stores the password hash |
| Google | An OAuth client in Google Cloud (client id and secret in the server's env file) |
| GitHub | An OAuth app on GitHub (same) |

A key works only once the email is verified (Google and GitHub logins arrive verified).

**Open:** which mail service. Free tiers that cover this volume: Resend, Brevo, Amazon
SES, or the mailbox behind `me@dharun.dev` if it offers SMTP. The sender should be an
address at dharun.dev with SPF and DKIM set, or the mails land in spam.

## 4. Signup abuse

Free compute attracts scripted signups. Layers, cheapest first:

1. Verified email before a key works.
2. A bot check on the signup form. **Open:** Cloudflare Turnstile (strong, but loads a
   script from Cloudflare, the first thing on the site loaded from another host), or a
   self-hosted proof-of-work check such as ALTCHA (MIT; nothing loaded from elsewhere;
   weaker against a determined script).
3. Disposable email domains refused, using the dataset the API already has.
4. A cap on signups per address per day, and on keys per account (proposed: 3).
5. Daily quotas per key on model endpoints (section 5), so one account cannot take the
   whole machine.
6. An admin switch to disable an account, and the heavy services' own kill switches.

## 5. Limits

Numbers are proposals for Dharun to set. Units are the costs the routes already charge
(light 1, heavy 10, upload 15, probe 20, model 30).

| | Without a key | With a free key |
|---|---|---|
| Counted per | IP address, as today | Key (so apps on a shared address stop sharing one allowance) |
| Per minute | 600 units, as today | 3,000 units |
| Model endpoints | Not available after the notice period | 500 requests a day in all, one at a time per key |
| New heavy services (LLM, sandbox, document AI) | Not available | Their own daily quota each (see 06) |
| Logos | Free when cached (02, section 5) | The same, plus the larger allowance |

Answers carry what a caller needs to pace itself: the limit, what is left, and when it
resets, in the response headers, the same for keyless and keyed calls.

## 6. How a key is sent

- `Authorization: Bearer <key>`, or `X-API-Key: <key>` for tools that make that easier.
- Key format: a fixed prefix, random characters and a short checksum (for example
  `india_live_...`), so a leaked key can be recognised by secret scanners and a typing
  mistake is refused without a lookup.
- **Never in the query string** for secret keys: addresses end up in logs and browser
  history.
- **MCP:** clients send the same header from their configuration. A tool for a
  key-required endpoint that is called without one answers with a clear tool error
  saying where to get a key. The MCP `instructions`, the skill and the plugin README
  explain it.

**Open: keys for web pages.** An `<img>` tag cannot send a header, and a key inside a
web page is public. Two ways:

1. No key for browsers: logo-heavy sites rely on caching and the free allowance for
   cached images. Simplest; recommended to start.
2. A second kind of key that is safe to publish: limited to logos and lookups, and
   accepted only from the sites the owner lists. More to build; add it if the 429s
   continue after caching.

## 7. Moving today's heavy endpoints behind the key

Decided: OCR, speech to text, text to speech, translation, embeddings, reranking and
background removal. The same reasoning covers the other model-based routes, to be
confirmed: audio denoise, moderation, Roman typing suggestions, name detection in
redaction (`names=true`; plain redaction stays keyless).

"Never break `/v1`" was the rule, so this is done with notice:

1. **Day 0:** keys ship. The endpoints work as before with or without a key.
   Every keyless answer from them carries a header naming the date a key becomes
   required and the signup address; the docs pages, `llms.txt`, the OpenAPI
   description, the skill and the MCP tool descriptions say the same.
2. **Notice period** (proposed: 30 days). The dashboard shows keyless against keyed
   use of these routes, so the remaining callers are visible.
3. **After the date:** a keyless call answers 401 with the code `key_required`, a
   message with the signup address, and the same body shape as every other error.

Things this touches:

- The docs samples are produced in memory at build time and are not affected.
- `TestEveryDocumentedExampleWorks`, the smoke checks and the production check need a
  key for these routes: a test key in CI, and a real low-quota key in a repository
  secret for the live check.
- The try-it panel and the tool pages (04, section 3).
- The catalog gains a mark for "needs a key"; pages, OpenAPI (`security`), `/v1`, the
  skill and MCP are generated from it.

## 8. The account page

- Create, name, rotate and revoke keys (the key itself is shown once).
- Use today and this month by API group, and what is left of each quota.
- Change password, log out everywhere, **delete the account** (removes the user, keys
  and usage counts at once).
- Later: the allowed-sites list for publishable keys, if section 6 option 2 is built.

Usage counts per key and day are kept by the API in `STATE_DIR` like the visitor
counts, not in the request log.

## 9. Privacy and trust pages

The privacy page changes in the same release. It must say: an account holds an email
address, a password hash (when a password is used), the Google or GitHub account id,
key hashes and creation dates, and daily usage counts per key; what is sent to the mail
service (the address and the mail); that Google or GitHub learn of the login; how to
delete everything.

**Open: what the request log holds.** Proposed: the key's id (never the key) on
key-required routes only, so abuse can be traced to an account and stopped; nothing on
other routes. Private routes stay as they are: the input is never logged.

## 10. Infrastructure request (for the homelab side)

- A second unit `dharun-api-accounts.service` and its directory; `install.sh` changes.
- Env settings: OAuth client ids and secrets, mail service credentials, the bot check's
  secret, the public origin for redirects.
- A backup of the accounts database, and a restore that has been tried once.
- Outbound: the mail service and the OAuth providers (the internet is already open).
- Memory: about 30 MB more.

## 11. Tests

- Signup, verification, login, reset and OAuth against PocketBase in a temporary
  directory, with a fake mail server and fake OAuth providers. No network.
- Key lifecycle: created, shown once, hash stored, used, revoked within the reload time.
- Limits: keyless per address, keyed per key, quotas, the headers.
- The notice header before the date and the 401 after it (the date injected in tests).
- The pass-through exposes only the listed addresses; the admin path answers 404.
- `internal/deploytest` and the install test cover the second unit.

## Open questions

1. Mail service.
2. Bot check: Turnstile or self-hosted.
3. The numbers in section 5 and the notice period.
4. Keys for web pages: start without (recommended) or build publishable keys now.
5. The full list of routes that need a key (section 7's "to be confirmed").
6. What the log holds about a key.
7. Sandbox design: it assumed GitHub login and Turnstile; update `docs/sandbox-design.md`
   to use these accounts.
