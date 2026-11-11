# India API: growth plan (discussion of 2026-11-10)

Dharun and Claude went through a month of production data and agreed where India API
goes next. Nothing here is built. Each file is written so that a later session can pick
up one part cold and build it.

| File | What it holds |
|---|---|
| [01-usage-analysis.md](01-usage-analysis.md) | What the first 27 days of production data show, and what follows from it |
| [02-logos.md](02-logos.md) | Part 1: fix what is used. Logos that never fail, three new logo sets, a record of missed tickers |
| [03-accounts-and-keys.md](03-accounts-and-keys.md) | Part 2: free accounts and API keys on PocketBase; heavy endpoints move behind a key |
| [04-website.md](04-website.md) | Part 3: try-it panels, the playground, free tool pages, demos, and the search work |
| [05-new-endpoints.md](05-new-endpoints.md) | The idea catalogue: about 90 endpoint ideas in four directions, with sources and risk |
| [06-heavy-ai.md](06-heavy-ai.md) | Document AI, the small hosted LLM and the code sandbox |

## Progress (update this first, every session)

A session that builds part of the plan adds a line here and ticks the boxes in the part
file, so the next one starts from what is true now. Newest first.

| Date | Part | Done | Next |
|---|---|---|---|
| 2026-11-11 | 2 | Logins changed to **one-time codes mailed by Amazon SES** (`no-reply@api.dharun.dev`), no passwords; Google and GitHub logins moved to a later part. Code, tests and docs done; SES setup script written in the homelab repository (SES identity, DKIM and MAIL FROM records in Cloudflare, IAM user, SMTP credentials into the secret store), with the infra request | Dharun runs the SES script (the session was not allowed to), then the homelab request |
| 2026-11-11 | 2, 3, 4 | **Built, tested, committed; not pushed** (Dharun: commit only). Part 2: accounts on PocketBase v0.39.11 (`cmd/accounts`), keys (`india_live_...`), keyed limits and `RateLimit-*` headers, the key gate with quota, notice header and 401 after `KEY_REQUIRED_FROM`, site tokens, account pages, `/keys`, privacy. Part 3: try-it panel on every endpoint, `/play` with chains and simulate, ten tool pages, logo pages, four demos, `/changelog`. Part 4: `/v1/batch`, `/v1/holidays/{year}.ics`, `/v1/today`, ten calculators, avatars, placeholders, charts. Details and what is left: the "Build status" section at the end of 03, 04 and 05 | Push when Dharun says. Then the infrastructure request in 03 section 12 (accounts do nothing on the server until then), and set `KEY_REQUIRED_FROM`. Check the 2026-27 income tax rules. Then Part 5 or 6 (sources to agree first), or the rest of Part 3 |
| 2026-11-11 | 1 | **Part 1 done**, except what Dharun moved: never fail, missed names, the three sets (`/v1/brand-logos` 294 brands, `/v1/stock-logos` 8,799 stocks, `/v1/asset-logos` 1,586 coins, indices, ETFs, commodities) with search addresses, the `find_logo` MCP tool, fall-through from `/v1/logos` with `X-Logo-Set`, cached logo answers free of the rate limit. Collectors: `tools/scrape_world_logos.py`, `tools/fetch_brand_logos.py`. All descriptions, README, roadmap, `CLAUDE.md` updated; `go test ./...` passes, also with `RESVG` | Not done by decision: the Cloudflare cache rule (Dharun: not now) and logo pages (moved to Part 3). After the release: run `apidata missed-logos` on the server after a few days and add the names that matter. Next part: 2 (accounts and keys) |
| 2026-11-11 | 0 | Panchang 500s explained without the logs: the crossing-search bug fixed in da07127 (released in v1.1.38 on 10 Nov). A regression test (`TestCrossingWrapRegressions`) holds the failing cases. MCP 405s: correct per the MCP spec, no change | The missed-ticker list from the logs is still open: Loki does not answer from the laptop. The API now keeps its own list (Part 1 section 4), so after a few days `apidata missed-logos` on the server gives the same answer |

## Decisions Dharun made on 2026-11-10

These replace older decisions where they conflict. The first session that builds a part
records the matching rows in `ROADMAP.md` section 12 and updates `CLAUDE.md` ("Rules that
the code does not show") in the same change.

| # | Decision | Replaces |
|---|---|---|
| 1 | The audience stays wide: Indian app developers, AI agent builders, students. No single focus | |
| 2 | The website gets interactive parts: a try-it panel on every API page, a full playground, free tool pages for normal people at `api.dharun.dev/tools` (English), and visual demos | "Documentation-only website" (2026-10-13) |
| 3 | New endpoints in all four directions: markets and company data, India civic and daily life, developer utilities, AI and agent tools | |
| 4 | The public launch stays on hold until these parts are built | Confirms 2026-10-14 |
| 5 | Data rule for new datasets: anything published openly may be used, decided case by case, credited. Each one still states its source and terms before it is built | Widens "only data that is legal to redistribute"; follows the AMFI precedent |
| 6 | Order of work: fix and deepen what is used first (logos, limits, the panchang fault), then the website, then new endpoints | |
| 7 | Logos grow in three sets: unlisted Indian brands; US and global stocks; crypto, indices, ETFs and commodities. An unknown name answers with the monogram, never a broken image, and the miss is written down so the logo can be added | The 404 for an unknown ticker |
| 8 | Free accounts and API keys. Login at api.dharun.dev by email and password, Google or GitHub. A key gives higher limits on everything (logos included) | "No API keys" (2026-10-13). Keyless use of light endpoints stays |
| 9 | The account system is PocketBase, run beside the API in the same container. As little of our own auth code as possible | The sandbox design's GitHub-only login |
| 10 | Heavy, model-based endpoints that are public today (OCR, speech to text, text to speech, translation, embeddings, reranking, background removal) move behind the key after a notice period. New heavy services (document AI, LLM, sandbox) need a key from the start | Keyless access to those endpoints |
| 11 | All three heavy AI items are wanted: document AI without an LLM, a small hosted LLM, the code sandbox | The LLM and sandbox were Open |
| 12 | Dharun plans here and builds later with AI sessions, part by part. Plans are written as handoff packages; time estimates are left out | |

## Order of the parts

```text
Part 0  Find and fix the panchang fault; read the logs for missed tickers      (02, section 1)
Part 1  Logos: never fail, three new sets, missed list, caching, logo pages    (02)
Part 2  Accounts and keys; notice period for heavy endpoints starts            (03)
Part 3  Website: try-it panel, playground, first ten tool pages, demos         (04)
Part 4  Cheap endpoints with proven or obvious demand: batch lookups,
        calculators, calendar feeds, avatars, placeholders, charts             (05, set A)
Part 5  Markets and company data                                               (05, set B)
Part 6  Civic and daily life data                                              (05, set C)
Part 7  Document AI                                                            (06)
Part 8  Small hosted LLM                                                       (06)
Part 9  Code sandbox                                                           (06, docs/sandbox-design.md)
Then    The launch in ROADMAP.md section 7.2, with refreshed counts
```

Parts 4 to 6 can swap or interleave. Parts 7 to 9 depend on Part 2 (keys).

## How a building session should use these files

1. Read `CLAUDE.md`, `ROADMAP.md`, then the one plan file for the part.
2. Every part lists **Open questions**. Ask Dharun those first; do not pick for him.
3. For any new dataset: state source, terms and refresh method and get a yes before
   building (decision 5 does not remove that step).
4. Follow "Adding an API" and "Keeping every description in step" in `CLAUDE.md`.
5. Anything under `deploy/`, a new program in the bundle, a new env setting, more disk or
   a Cloudflare rule is an infrastructure request (see `INFRA.local.md`).
6. Every push to `main` deploys. Build a part on a branch if it lands in pieces.

## Open questions across the whole plan

Collected from the part files, most urgent first.

1. ~~**Logos:** fall-through~~ Answered 2026-11-11: yes, in the fixed order. (02)
2. ~~Keys in the browser~~ Answered 2026-11-11: start without publishable keys. (03)
3. ~~Tool pages against decision 10~~ Answered 2026-11-11: yes, a short-lived site token, 10 a day. (04)
4. ~~Mail service and bot check~~ Answered 2026-11-11: any SMTP in the environment; self-hosted proof of work. (03)
5. ~~The numbers~~ Answered 2026-11-11: as proposed; 30 days' notice. The date itself is still to be set. (03)
6. ~~What the log may hold about a key~~ Answered 2026-11-11: the key's id on key routes only. (03)
7. ~~Programmatic pages~~ Answered 2026-11-11: company logo pages now. (04)
8. **Which ideas from the catalogue** go into Parts 4, 5 and 6. Each carries a
   recommendation; Dharun marks the ones he wants. (05)
