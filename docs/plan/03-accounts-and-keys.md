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

## Change of 2026-11-11 (later the same day): codes by mail, no passwords

Dharun: login and signup by a one-time code only, mailed through **Amazon SES** from an
address at api.dharun.dev; Google and GitHub logins move to a later part. This replaces
sections 3 and 4's password and OAuth logins:

- `/signup` and `/login` are one form: email address, then the 6-digit code.
- `POST /account/api/code` (the API's own address): browser proof of work, disposable
  domains refused, at most 10 codes per email address and 5 new accounts per caller
  address a day. It makes the account (with a random password nobody knows) if there is
  none, then asks PocketBase to mail the code (`request-otp`). Same answer whether the
  account existed or not.
- `auth-with-otp` logs in and marks the address verified (PocketBase does this). Password
  login, password reset, verification links and OAuth are switched off and not passed
  through; the password-change address is gone. The new-device alert mail is off.
- Mail: SES in ap-south-1 (production access was already granted on the account, used
  by labs.dharun.dev). Identity `api.dharun.dev` with Easy DKIM, custom MAIL FROM
  `mail.api.dharun.dev`, DMARC `p=quarantine`, sender `no-reply@api.dharun.dev`. Nothing
  can sit on `api.dharun.dev` itself (it is a proxied CNAME), so no inbound mail there.
  IAM user `pve-api-ses`, allowed only `ses:SendRawEmail`/`SendEmail` from
  `*@api.dharun.dev`; its SMTP credentials and a random site secret go to the homelab secret store.
  The setup script (idempotent, prints no secret) lives in the private homelab
  repository, beside the infrastructure request.
- [ ] Run the SES setup script (it was written here, but the session was not allowed to
      run it); check DKIM and MAIL FROM show SUCCESS.
- [ ] Google and GitHub login: a later part.

## Build status (2026-11-11)

Built, tested and committed; **not pushed** (Dharun: commit only). Dharun's answers of
2026-11-11 to the open questions: any SMTP service set in the environment; the
self-hosted ALTCHA-style proof of work (not Turnstile); the numbers in section 5 as
proposed, 30 days' notice; no publishable browser keys yet; every model route needs a
key (denoise, moderation, Roman typing and `names=true` included); the key's id in the
log on key routes only.

- [x] `cmd/accounts` and `internal/accounts`: PocketBase **v0.39.11** (v0.40.x needs Go
      1.27, so not yet), users with `disabled` and `tier`, `api_keys` (hash, hint,
      revoked, cascade delete), at most 3 keys, verified email first, our mail templates,
      settings from the environment, PocketBase's request log and rate limiter off.
      Tested against a real PocketBase in a temporary directory (`accounts_test.go`):
      signup, verification mail, login, keys, revoke, disable, delete, reset mail.
- [x] `internal/apikey`: `india_live_` + 32 base62 + 6-character checksum; the table
      query is shared and tested against the real database.
- [x] API side (`keys.go`, `account.go`): key table reloaded every 15 s, direct lookup
      with a one-minute memory of misses; `Authorization: Bearer` or `X-API-Key`;
      keyed allowance; `RateLimit-*` headers everywhere; key gate with quota, one at a
      time per key, notice header, 401 after `KEY_REQUIRED_FROM`; site tokens; usage in
      `STATE_DIR/key-usage.json`; pass-through allow-list; signup with proof of work,
      disposable check and daily cap; usage address for the account page.
- [x] Catalog `Key` mark, generated into the pages, OpenAPI (`security`,
      `x-key-required`), `/v1`, skill, plugin and MCP; MCP tool calls carry the key.
- [x] Pages: `/signup`, `/login`, `/account`, `/account/verify`, `/account/reset`,
      `/account/oauth`, `/keys`; privacy and security pages; FAQ.
- [x] The accounts binary is in the release bundle and in NOTICES.
- [ ] **Infrastructure request (section 12)**: nothing works on the server until it is done.
- [ ] Set `KEY_REQUIRED_FROM` (at least 30 days after keys are live) and announce it
      on `/changelog`.
- [ ] `internal/deploytest` and the install test for the second unit (comes with the
      `deploy/` change on the homelab side).
- [ ] A low-quota key in a repository secret for the production check once the date is set.
- [ ] Sandbox design (`docs/sandbox-design.md`) to be re-read with these accounts (Part 9).

## 12. Infrastructure request (written 2026-11-11, for the homelab side)

To be copied into the private homelab repository; no addresses here.

1. A second unit `dharun-api-accounts.service` running `/opt/dharun-api/current/accounts`
   as the service user, `ACCOUNTS_DIR=/var/lib/dharun-api/state/accounts`,
   `ACCOUNTS_ADDR=127.0.0.1:8091`, restarted with the API after an update. A deploy must
   never touch `ACCOUNTS_DIR`.
2. Its env file: `PUBLIC_ORIGIN=https://api.dharun.dev`,
   `SMTP_HOST=email-smtp.ap-south-1.amazonaws.com`, `SMTP_PORT=465`, `SMTP_TLS=true`,
   `SMTP_USERNAME` and `SMTP_PASSWORD` from the homelab secret store,
   `MAIL_FROM=no-reply@api.dharun.dev`, `MAIL_FROM_NAME=India API`. Outbound TCP 465 to
   SES (the internet is open). No OAuth settings.
3. In the API's env file: `ACCOUNTS_DB=/var/lib/dharun-api/state/accounts/data.db`,
   `ACCOUNTS_URL=http://127.0.0.1:8091`, `SITE_SECRET` from the homelab secret store, later
   `KEY_REQUIRED_FROM`. See `deploy/dharun-api.env.example`.
4. A daily backup of `ACCOUNTS_DIR` (PocketBase's `accounts superuser` and backup
   commands, or a file copy with the unit stopped), and one restore tried.
5. A superuser for the admin screen, reached only through an SSH tunnel to port 8091.
6. Memory: about 30 MB more.
