# Part 0 and Part 1: fix what is used, then make logos never fail

Why first: logos are 56% of all requests and the only API in daily use by real
applications, and 16% of logo requests fail (see [01-usage-analysis.md](01-usage-analysis.md)).

Dharun, 2026-11-10: grow the logo set in three directions (unlisted Indian brands; US and
global stocks; crypto, indices, ETFs and commodities); when a name is still unknown,
answer with the monogram instead of failing; write the missed names down.

## 1. Part 0: before anything else

- [x] **Panchang 500s.** Find the `error` and the request path in the logs for 10 Nov
      (1,007 failures) and 30 Oct (19). Write a failing test from the real request, fix
      it, and add the case to the fuzz or table tests. Not reproduced from outside with
      25 edge cases, so do not guess at the cause.
      **Done 2026-11-11, without the logs** (Loki does not answer from the laptop).
      Prometheus shows the 500s as one steady caller, about 120 an hour, from 15:00 UTC
      on 9 Nov to 00:30 UTC on 10 Nov (and 19 at 17:00 UTC on 29 Oct). On the code
      before da07127, `panchang.Compute` fails with "no crossing found" at 643 of 1,891
      grid places on 2026-11-09 (16 on the 10th, 540 on the 11th), all west of India
      (the Americas, the Atlantic, London), never at an Indian place; New Delhi itself
      fails on five other days in 2026-27. The fixed code fails at none. So the cause is
      the wrapped step in `crossing`, fixed in da07127 for sun-and-moon (the panchang
      uses the same search) and released in v1.1.38. `TestCrossingWrapRegressions` in
      `internal/panchang/panchang_test.go` holds the cases that failed. If the logs show
      a panchang 500 after v1.1.38, this conclusion is wrong: look again.
- [ ] **The missed-ticker list** from the logs (section 6 of the usage analysis). The
      rest of this part depends on what it shows.
      **Still open:** no log access from here. The API now keeps the list itself
      (section 4); run `apidata missed-logos` on the server a few days after the release.
- [x] **MCP 405s:** one look at which clients send GET to `/mcp`, and whether a helpful
      answer (or the server card) would serve them better.
      **Done 2026-11-11: no change.** The MCP Streamable HTTP spec has a client send GET
      to open an optional SSE stream, and a server without one must answer 405. The 405s
      are clients following the spec; another answer could break them.

## 2. Never fail  (built 2026-11-11)

Built as below. Also: a symbol over 64 characters or with control characters answers 400
`bad_symbol`; the tile for a known company without a logo keeps its 5-minute cache. Files:
`internal/api/logos.go`, tests in `api_test.go` (`TestLogos`) and `store_test.go`.

Today `/v1/logos/{symbol}` answers 404 for an unknown symbol unless the caller adds
`?fallback=monogram`. New behaviour:

| Case | Answer |
|---|---|
| Known symbol with a logo | As today |
| Known company without a logo | Monogram, as today, `X-Logo-Fallback: monogram` |
| Unknown symbol | **Monogram, status 200**, `X-Logo-Fallback: monogram`, `X-Logo-Known: false` |
| Unknown symbol with `?fallback=none` | 404, as before, for callers that want to detect a miss |
| A symbol that cannot be a name at all (too long, control characters) | 400, as today |

Details to keep:

- A monogram for an unknown symbol gets a **short** `Cache-Control` (proposed: one day),
  so that a logo added later replaces it soon. Real logos keep their long lifetime.
- `/v1/companies/{symbol}` keeps answering 404 for an unknown company. Data must not be
  invented; only the picture has a stand-in.
- This changes an existing answer under `/v1`. It is safe for the common use (an `<img>`
  tag) and `?fallback=none` keeps the old contract available. Say so in the docs page and
  the changelog.
- The metric `api_logo_requests_total` gains the result `unknown` (monogram for an
  unknown symbol), so the dashboard still shows the miss rate.

## 3. Three new logo sets  (built 2026-11-11)

Built as agreed (see "Answers"). Datasets `brand-logos`, `stock-logos`, `asset-logos`, one
per set so each answer credits its own source (not a `set` column in `logos`). Handlers
in `internal/api/worldlogos.go`; collectors `tools/scrape_world_logos.py` and
`tools/fetch_brand_logos.py`. Size: the three seed files are 147 MB with PNG masters
(stocks 115 MB). Numbers and the limits chosen (500 per non-US market, 500 coins, 300 US
ETFs) are in `ROADMAP.md`. Brand artwork was checked by eye; seven Commons matches and two
site icons were other organisations and are switched off in `brands.json`.

"Three more endpoints" (Dharun). The settled URL rule is flat `/v1/<resource>`.
Proposed addresses:

| Set | Address | Looked up by |
|---|---|---|
| Unlisted Indian brands | `/v1/brand-logos/{name}` | Slug or alias (`phonepe`, `zepto`, `google-pay`), or domain (`phonepe.com`) |
| US and global stocks | `/v1/stock-logos/{symbol}` | Ticker, with `?exchange=` (`NASDAQ`, `NYSE`, `LSE`, ...) when a ticker is used on several |
| Crypto, indices, ETFs, commodities | `/v1/asset-logos/{symbol}` | `BTC`, `NIFTY50`, `SENSEX`, `GOLD`, `NIFTYBEES`, with `?kind=` when names clash |

Each has the same options as `/v1/logos` (format, size, monogram), a list or search
address beside it, an MCP tool only if an agent would use it (probably one tool,
`find_logo`, covering all sets), and the same never-fail rule.

**Open question for Dharun:** should `/v1/logos/{symbol}` fall through to the new sets
when the Indian listing has no match? Recommended: **yes**, in a fixed order (NSE and
BSE, old tickers, brands, global stocks, assets), with `X-Logo-Set` naming where the
answer came from. The 23,000 failed requests arrive at `/v1/logos`, and those callers
will not change their URLs. The missed list from Part 0 confirms or changes this.

### Sources (each needs Dharun's yes before it is built)

| Set | Proposed source | Terms | Refresh |
|---|---|---|---|
| Global stocks | The same TradingView collection as today (`tools/scrape_logos.py`, more markets). Start with the US (S&P 500, Nasdaq 100, popular ADRs), then by demand | As today: logos are company trademarks, collected by hand-run script | By hand, with the Indian run |
| Crypto | `spothq/cryptocurrency-icons` (stated CC0; **check the licence file before use**), topped up from the TradingView collection | CC0 for the files; marks belong to the projects | By hand |
| Indices, ETFs, commodities | TradingView collection for index and ETF marks; commodity icons (gold, silver, crude) drawn here | Trademarks for indices (NSE Indices, BSE, S&P); our own drawings for commodities | By hand |
| Unlisted Indian brands | A curated list in `data/brands/` (name, slug, aliases, domain, category: payments, banks' apps, insurers, mutual fund houses, startups, government services). Artwork per brand from its press kit or Wikimedia Commons where the file's licence allows; otherwise the site's own icon, re-drawn as PNG through the existing `/v1/icon` code | Trademarks; each file's origin recorded in `data/sources.json` | By hand; the missed list says what to add |

Start the brand list from the missed list plus about 300 names people expect: UPI apps,
payment gateways, neobanks, insurers, the 45 mutual fund houses, large unlisted
companies, popular consumer apps, government services (UPI, RuPay, DigiLocker, IRCTC).

Dataset work: either new tables in the `logos` dataset (a `set` column) or one dataset
per set. One dataset with a `set` column keeps the fall-through a single query; bump
`Schema`.

## 4. Writing down the misses  (built 2026-11-11)

Built as below in `internal/api/missedlogos.go` (saved with the totals every 30 seconds
and on shutdown, mode 600) and `cmd/apidata/missed.go` (`apidata missed-logos [-state dir]
[-n 100] [-json]`). A miss is not recorded while the server has no logo dataset.

Dharun: "note it down somewhere". Read as: keep a list of names that fell back to the
monogram, so the logos can be added. (If he meant only "record this decision", this
section is still worth building; ask.)

- The API counts unknown symbols in memory: normalised (upper case, letters, digits and
  `& - _ .` only, at most 24 characters), at most 5,000 distinct names, least-seen
  dropped first.
- Written to `STATE_DIR/missed-logos.json` every few minutes and on shutdown, like the
  visitor counts, so a deploy does not lose it. Name, count, first seen, last seen. No
  address, no user agent.
- Read with `apidata missed-logos` on the server. Not a public endpoint (it would show
  what other callers ask for) and not a metric label (caller text must not become label
  values).
- Privacy page: one sentence, since it is something new that is kept.

## 5. Limits and caching for logos  (2026-11-11: cached answers free, done; cache rule not now)

Dharun, 2026-11-11: a 304 or a PNG cache hit on a logo route gives its unit back
(`freeWhenCached` in `server.go`, `TestCachedLogosAreFree`). The Cloudflare cache rule
is not requested for now.

Logo users are the ones being refused (7,000 times in 30 days). Decision 8 keeps light
endpoints keyless and lets a key raise the limit; two cheaper steps come first:

- **Cloudflare cache rule for `/v1/logos/*`** and the three new addresses (they already
  send long `Cache-Control` and an ETag). Repeat requests then never reach the server.
  This is an infrastructure request, and it needs the pending roadmap item "Count what
  Cloudflare serves from its cache", or the dashboard will show logo traffic falling.
- **A larger allowance for cached images.** Proposed: a 304 or a logo served from the
  server's PNG cache costs nothing; only a fresh render costs 1.

## 6. Logo pages on the website  (moved to Part 3, Dharun 2026-11-11)

`/logos/{SYMBOL}`: one page per company with the logo, SVG and PNG download, the embed
code, the company's details and its API address. This is the search form of the one
proven product ("reliance logo svg"). Details and cautions are in
[04-website.md](04-website.md), section 5.

## 7. Checklist for the session that builds this

- Sources agreed and recorded in `ROADMAP.md`; `data/sources.json` entries.
- Catalog entries with real examples, docs pages, smoke checks, README table, skill
  description (logos are a headline API), MCP tool and `mcpTestCalls`.
- Tests: never-fail in both directions (`fallback=none`), the fall-through order, the
  missed list (bounded, normalised, survives restart), cache headers for the three cases.
- Run the rendering tests on Linux with `RESVG` set.
- `privacy.md` for the missed list. The trademarks note on the docs page covers the new
  sets.

## Answers (Dharun, 2026-11-11)

1. Fall-through from `/v1/logos`: **yes**, in the fixed order above, with `X-Logo-Set`.
2. Addresses: **as proposed** (`/v1/brand-logos`, `/v1/stock-logos`, `/v1/asset-logos`).
3. Brand artwork: **press kit, Wikimedia Commons where the file's licence allows, else the
   brand site's own icon**; each file's origin recorded. Crypto and global stocks as in
   the sources table. Crypto licence checked 2026-11-11: `LICENSE.md` in
   `spothq/cryptocurrency-icons` is CC0 1.0 Universal (the coins' marks still belong to
   their projects). The set is old, so newer coins come from the TradingView collection.
4. Global set: **US plus the major markets** (LSE, Tokyo, Hong Kong, Frankfurt and
   similar) from the start, not US only.
5. Unknown monogram cache: one day, as proposed (built; not asked separately).

## Open questions

1. Fall-through from `/v1/logos` (recommended yes), and the order.
2. The three addresses and their names.
3. Sources per set, above all the brand artwork rule.
4. How far the global set goes at first (US only, or more markets).
5. Short cache lifetime for an unknown symbol's monogram: one day?
