# Roadmap and status

The one place that tracks what `api.dharun.dev` serves, what is being built, what comes
later and what was turned down. Every API lists where its data comes from and how that
data is refreshed. Update this file in the same change that adds or alters an API.

Last updated: 2026-10-14

**Status words:** `Live` released · `Building` in the current round · `Planned` agreed,
later round · `Open` waiting for a decision · `Not building` turned down, with the reason.

## 1. Where things stand

| | |
|---|---|
| Released version | `v1.0.0` (2026-10-13), 9 API groups |
| Production | Not deployed yet. The server is offline; first deploy follows when it is back |
| Current round | Round 1 is built: the platform changes (section 2, two items left) and group A (section 4). Left open: three Tamil encodings (4.3) and INR reference rates (section 5) |
| Next rounds | Round 2 uploads, round 3 AI models, round 4 network tools. One round per session |

## 2. Platform work in round 1

These change how the service is built and deployed. They come before the new APIs because
the new APIs depend on them.

- [x] **Split data from the server.** A release carries the program and a *seed* copy of
      each dataset. On the server, datasets live in their own directory that a deploy
      never overwrites. See section 8.
- [x] **The server refreshes its own data.** `dharun-api-refresh.timer` runs
      `apidata refresh` hourly; each dataset decides whether it is due, is fetched,
      checked and built into a new file. The API picks it up without a restart. No GitHub
      job fetches data. Tried against the real IFSC, NSE and Wikipedia sources.
- [x] **One database file per dataset** instead of one `api.db`.
- [x] **Refresh metrics**: per dataset, which copy is in use, when its source was fetched,
      whether the last refresh worked, when it last worked, record counts.
- [ ] Grafana panels and a stale-data alert for those metrics.
- [x] **Deploy on every push to `main`.** CI passes, a release `v1.1.<run>` is published,
      the server installs it within 10 minutes and rolls back if the health check fails.
      Automatic releases beyond the newest 15 are deleted.
- [x] **Text input by POST**, with a size limit (`MAX_BODY_KB`, default 256).
- [x] **Per-route cost and a queue for CPU-heavy routes**, so a burst waits instead of
      using more memory.
- [ ] **Install test in CI.** Run `deploy/install.sh` inside a Debian 13 container with
      systemd, since the real server cannot be tested until it is back.
- [x] Memory measured after the split: API 22 MB idle, 40 MB peak under a burst; refresh
      job 200 MB peak during the IFSC refresh.
- [x] Memory measured again after group A (Linux, release bundle, all datasets): the API
      idles at 34 MB and peaked at 144 MB under a mixed burst over every kind of endpoint,
      24 requests at a time. On top of that, `resvg` uses 11 MB for a preview image and up
      to 75 MB for the largest code image, while it runs.
- [x] Dharun, 2026-10-14: memory up to 512 MB or 1 GB is fine; do not give up speed to
      save it. So parsed fonts are kept (a preview card is shaped in 1 ms, not 13), a
      tokenizer vocabulary stays for half a day after its last use, each database
      connection has a 4 MB cache, and four images render at once. The service limit is
      `MemoryMax=768M` with `GOMEMLIMIT=384MiB`.

Already live: single Go binary, read-only SQLite, `{ data, meta }` envelope, per-IP rate
limit, docs site, OpenAPI, `llms.txt`, MCP server, Prometheus metrics, lifetime request
totals, pull-based updater with rollback.

## 3. Live APIs

| API | Endpoint | Data from | Refresh |
|---|---|---|---|
| Company logos | `/v1/logos/{symbol}` | Collected from TradingView; logos are company trademarks | Manual (scraper) |
| Companies | `/v1/companies/{symbol}` | Same collection | Manual (scraper) |
| Pincodes | `/v1/pincode/{pincode}` | All India Pincode Directory, Department of Posts (data.gov.in), GODL-India | Manual: the portal refuses scripted downloads |
| IFSC and banks | `/v1/ifsc/{code}`, `/v1/banks` | `razorpay/ifsc` releases, compiled from RBI and NPCI lists | Moves to automatic, weekly |
| RTO codes | `/v1/rto/{code}` | Wikipedia, CC BY-SA 4.0 | Moves to automatic, monthly, held back if many rows change at once |
| Holidays | `/v1/holidays/{year}` | RBI holiday matrix; NSE trading holidays; DoPT memoranda | Bank and market move to automatic, monthly. Central government stays typed by hand |
| Number to words | `/v1/words/{number}` | Computed | None |
| Format checks | `/v1/validate/{kind}/{value}` | Computed (GSTIN, PAN, IFSC) | None |
| States | `/v1/states` | Fixed list in the code | With code changes |

Known gaps in live data: state government holidays; Andaman and Nicobar RTO codes; 2027
has only central government holidays so far; Hindi and Tamil number words need a native
review.

## 4. Round 1: group A (lookup and compute)

All of these run inside the existing server. "Memory" is what the API adds while idle.

### 4.1 Built on data already held

| API | Endpoint | How | Status |
|---|---|---|---|
| Working days | `/v1/working-days/...` | Holidays already held, plus Sundays and second/fourth Saturdays for banks. "Is this a working day", "next working day", "add N working days", settlement date T+1. Needs NSE *clearing* holidays too (same NSE source, different list). Only answers for years with holiday data | Live |
| Market status | `/v1/market/status` | Trading holidays plus exchange hours. Special sessions (Muhurat, budget-day Saturdays) come from the notes in the NSE list | Live |
| Pincodes nearby | `/v1/pincode/near?lat=&lng=` | Coordinates in the pincode file, spatial index in SQLite | Live |

### 4.2 New datasets

| API | Endpoint | Data from, licence | Refresh | Memory | Status |
|---|---|---|---|---|---|
| HSN and SAC codes | `/v1/hsn/{code}`, `/v1/hsn?q=` | `HSN_SAC.xlsx` from the GST portal, about 22,000 codes with descriptions. The nomenclature is statute (Customs Tariff Act, 1975 and GST notification 11/2017), and that is what is credited; the GST Council site asks for permission to reuse its own material, so nothing else is taken from it | Automatic, monthly | Disk only | Live. Codes and descriptions only, no rates |
| Thirukkural | `/v1/thirukkural/{number}`, search, random | 1,330 couplets, public domain, from `tk120404/thirukkural` (Apache-2.0): text, transliteration, and the translations of G.U. Pope (1886) and Drew and Lazarus. Eight couplets with broken characters in that compilation are repaired on import | None; the text does not change | Disk only | Live. The modern commentaries (Mu. Varadarajan, Kalaignar, Solomon Pappaiah) and the Shuddhananda Bharati translation are left out until a release into the public domain is confirmed; Parimelazhagar needs another source |
| Phone numbers | `/v1/phone/{number}` | Format rules from the national numbering plan; mobile series to operator and circle from Wikipedia "Mobile telephone numbering in India" (CC BY-SA) | Manual, yearly | Disk only | Live. Circle and operator are the *original* allocation; number portability means the current operator can differ, and the response says so |
| Constituency | `/v1/constituency?lat=&lng=`, `?pincode=` | DataMeet community maps (CC BY 4.0): assembly constituencies and 2019 parliamentary constituencies | Manual; changes only on delimitation | Disk only | Live. Assembly boundaries pre-date the redrawing in six states, and the response says so. No member names |
| Boundaries | `/v1/geojson/states`, `/districts` | DataMeet (CC BY 4.0) | Manual | Disk only | Not building (section 7) |
| IP lookup | `/v1/ip/{address}`, `/v1/ip` | DB-IP Lite (CC BY 4.0, requires a credit link): country, city and network | Automatic, weekly check; a new file each month. Never in a release: only ever a server's own copy | Files are memory-mapped; about 140 MB on disk | Live |
| Email check | `/v1/email/{address}` | Syntax rules; disposable-domain list from `disposable-email-domains` (CC0); mail-server lookup through public DNS. No mailbox probing | List: automatic, weekly | Small | Live. Treated as private: not logged, not cached |
| DNS check | `/v1/dns/{name}` | Live queries to a fixed set of public resolvers; compares their answers | None | None | Live |
| Airports | `/v1/airports/{code}` | OurAirports (public domain) | Automatic, monthly | Disk only | Live |
| Railway stations | `/v1/stations/{code}`, `/v1/stations?q=` | Wikipedia "List of railway stations in India" (CC BY-SA 4.0), credited. No official list with a clear licence exists | Automatic, monthly, held back if many rows change at once | Disk only | Live. 3,975 stations; the list has gaps and some retired codes, and the documentation says so |

### 4.3 Pure logic, no data source

| API | Endpoint | How | Status |
|---|---|---|---|
| QR and UPI QR | `/v1/qr`, `/v1/upi-qr` | QR encoder in Go; `upi://pay` link built to the NPCI linking format. Generate only, SVG or PNG. UPI IDs are treated as private | Live |
| Barcodes | `/v1/barcode` | `boombuler/barcode` (MIT): Code 128, EAN, Code 39, ITF, Data Matrix, PDF417. Generate only; reading is round 2 because it needs uploads | Live |
| Tamil legacy encodings | `/v1/tamil-font/convert`, `/v1/tamil-font/encodings` | Tables written here from the published standards, not taken from `open-tamil` (its tables are GPL-3.0). TSCII 1.7 follows the IANA-registered chart and Unicode Technical Note 15, and was checked byte for byte against the GNU C library's table | Live for TSCII, both directions. **Open**: TAB, TAM and Bamini. The TamilNet 99 standard pages for TAB and TAM survive in the Wayback Machine, but they show each code only in the TAB-Anna font, and Bamini is defined only by its font; building those tables means reading the glyphs out of the fonts and checking them against a second source. Not shipped until that is done |
| Panchang | `/v1/panchang/{date}?lat=&lng=`, `?pincode=` | Computed from Sun and Moon positions (`go-sampa`, MIT: NREL SPA and Meeus) with true Lahiri ayanamsa as the Swiss Ephemeris defines it: tithi, nakshatra, yoga, karana, weekday, sunrise, sunset, moonrise, Rahu kalam, Tamil and lunar month. Positions agree with NASA JPL Horizons to 2 arc seconds | Live. Element end times differ from some printed almanacs by a minute or so (and by about seven minutes at a sankranti, where almanacs use a slightly different ayanamsa). Festival dates in a later round |
| Transliteration | `/v1/transliterate` | Rule-based: between nine Indian scripts, to ISO 15919, IAST and plain ASCII, and from ISO 15919 and IAST back to a script. This is not "type English, get Tamil" guessing; that needs a model and is round 3 | Live |
| Fake Indian data | `/v1/fake/{kind}` | Generated: names, addresses with real pincode and city pairs, phone-shaped numbers, PAN and GSTIN that pass format checks. Seeded so results repeat. Every response marked as fake | Live |
| Text chunking | `/v1/text/chunk` | Splits text or Markdown into overlap-aware chunks for retrieval | Live |
| PII redaction | `/v1/text/redact` | Patterns with checksums: Aadhaar (Verhoeff), PAN, GSTIN, cards (Luhn), phone, email, UPI ID, IFSC, vehicle number, passport, voter ID. No name detection; that needs a model and is round 3. Private | Live |
| Token count | `/v1/text/tokens` | OpenAI encodings (`o200k_base`, `cl100k_base`, `p50k_base`), counted by our own byte-pair merger over the published vocabularies (MIT), which holds 8 MB while in use against 22 MB for `tiktoken-go`; tests check it gives the same counts. Loaded on first use, dropped after half a day unused | Live. Claude and Llama tokenizers are not openly licensed for this, so not offered |
| Language detection | `/v1/text/language` | Script detection plus `whatlanggo` (MIT) trigram model; about 80 languages, 12 Indian. Weak on short or Romanised text, and the response carries a confidence value | Live |
| Social preview image | `/v1/og-image` | A 1200x630 card. Text is shaped here with `go-text/typesetting` (BSD) and Noto fonts (OFL 1.1, compiled into the server) for Latin and nine Indian scripts, and drawn as glyph outlines; `resvg` then only fills paths. Text and colours only, no remote images | Live. PNG needs `resvg` (in every release); SVG works anywhere |
| Code image | `/v1/code-image` | `chroma` (MIT) highlighting, drawn the same way. Up to 150 lines of 140 characters | Live. PNGs are drawn at twice the size; only the very largest at single size |

## 5. Data that changes daily

Decided 2026-10-13: the server fetches this itself (section 8).

| API | Data from | Refresh | Status |
|---|---|---|---|
| Mutual fund NAV | `NAVAll.txt` published by AMFI, about 1.5 MB. AMFI's site terms do not allow republishing; built anyway by decision (section 11), credited to AMFI | Automatic, every four hours | Live |
| INR reference rates | FBIL publishes the official rates; RBI republishes them | Automatic, daily on Mumbai working days | **Open**: FBIL's reuse terms are not confirmed. Not built until they are read |

## 6. Later rounds

### Round 2: uploads (group B)

OCR (Tesseract), image resize and convert (libvips), PDF tools (`pdfcpu`), Typst to PDF
with a ready GST invoice template, barcode reading, perceptual hash, text to speech
(Piper), audio denoise, background removal. A tool is started per request and exits, so
memory is used only while a request runs. Needs designing first: upload size limits, a
queue, and what happens to uploaded files (processed in memory, never stored).

### Round 3: AI models (group C)

Embeddings, reranking, model-based transliteration, moderation, translation, speech to
text. One model process beside the Go server in the same container, with its own memory
limit, started on demand and stopped when idle.

### Round 4: network tools (group D)

Probe from India, TLS check, IPv6 test, logo for any domain, RDAP. Decided 2026-10-13:
build with guards (no private addresses, fixed ports, strict limits) and accept that
outgoing requests leave from the server's own connection.

Screenshot and HTML to PDF need a headless browser (200 to 500 MB each) and are the last
candidates; Typst covers most of the PDF need.

## 7. Not building

| Idea | Reason |
|---|---|
| PAN, Aadhaar or vehicle owner lookup | Personal data |
| IRCTC PNR status | The source prohibits scraping |
| GSTIN verification against the GST portal | Captcha-protected government site |
| Live or delayed share prices | Licensed by the exchanges |
| Open web search or scraping | Uses the server's connection to scrape third parties; search engines block it |
| State and district boundary files | Publishing a map of India whose boundary differs from the Survey of India's is an offence; DataMeet already offers the files free |

## 8. How data reaches the server

Decided 2026-10-13: GitHub carries the program, the server keeps its own data fresh.

```text
repository data/            seed copies, as fresh as the last manual import
        |  built into every release
        v
server:  release/seed/<dataset>.db      replaced on every deploy
         data/<dataset>.db              written only by the refresh job, never by a deploy
```

- The API checks both directories every 15 seconds and serves, per dataset, the copy
  whose source was fetched more recently. A file built for different tables is ignored.
- The refresh job is a separate short-lived process, so the memory it needs to parse and
  build is released when it exits. It never touches the copy in use: it builds a new file,
  checks it, then renames it into place.
- A refresh is rejected, and the previous copy kept, when the source is unreachable, the
  format changed, or the record count moved more than the dataset's allowed margin.
- When a deploy changes a dataset's table layout, the server's copy no longer matches. The
  API serves the seed, and the next refresh run (within the hour) rebuilds the server's
  copy whether or not it is due.

| Dataset | Fetched from | How often | Fetched by |
|---|---|---|---|
| IFSC | GitHub release of `razorpay/ifsc` | Weekly | Server |
| RTO codes | Wikipedia API | Monthly | Server |
| Bank holidays | rbi.org.in holiday matrix | Monthly, and next year's list from December | Server |
| Market and clearing holidays | nseindia.com | Monthly | Server |
| HSN and SAC | GST portal file | Monthly | Server |
| IP database | download.db-ip.com | Checked weekly, new each month | Server (never in a release) |
| Mutual fund NAV | AMFI `NAVAll.txt` | Every four hours | Server |
| Airports | OurAirports | Monthly | Server |
| Railway stations | Wikipedia API | Monthly | Server |
| Disposable email domains | GitHub | Weekly | Server |
| Pincodes | data.gov.in file saved by hand | When Dharun runs it | Repository seed |
| Central government holidays | DoPT memorandum, typed in | Yearly, around July | Repository seed |
| Logos and company details | TradingView scraper | When Dharun runs it | Repository seed |
| Thirukkural, phone series, constituencies | See section 4 | Rarely | Repository seed |

## 9. Questions settled on 2026-10-13

1. Seed against server copy: **the newer one wins**, compared by when its source was fetched.
2. Thirukkural: **public domain plus confirmed releases.** Couplets, transliteration and
   the 19th-century English translations now. The Mu. Varadarajan and Kalaignar
   commentaries only once a Tamil Nadu government order releasing them is found;
   Parimelazhagar once a clean source is found. Solomon Pappaiah's commentary and the
   Shuddhananda Bharati translation are not served.
3. HSN: **codes and descriptions only**, no GST rates.
4. Constituency: **constituency only** (name, number, state, reserved category), no
   member names.
5. Boundaries (GeoJSON): **not built.** The constituency lookup uses boundaries
   internally without publishing a map.
6. Panchang: **elements and timings first**; festival dates in a later round.
7. Extras in round 1: **mutual fund NAV, airport codes, railway station codes.**
8. Memory cap: **measured per round and reported**; Dharun sets the limit.

## 10. First production deploy

Waiting for the server. Steps, done in a session on the private homelab repository:

- [ ] Create the container; run `deploy/install.sh`
- [ ] Tunnel route for `api.dharun.dev` to the service port
- [ ] Prometheus scrape job and Grafana dashboard from `deploy/monitoring/`
- [ ] Ship logs; add an uptime monitor on `/healthz`
- [ ] Firewall: service port from the tunnel only, metrics port from Prometheus only
- [ ] Check the first automatic refresh run and the first push-triggered deploy

## 11. Decisions

| Date | Decision |
|---|---|
| 2026-10-13 | Name `api.dharun.dev`; flat `/v1/<resource>` URLs; Go; `{ data, meta }` envelope |
| 2026-10-13 | No API keys, rate limit per IP; documentation-only website; private monitoring |
| 2026-10-13 | RTO codes from Wikipedia, attributed, shared under CC BY-SA |
| 2026-10-13 | Everything runs in one container |
| 2026-10-13 | Memory is kept low; CPU may be used freely; features are not dropped to save memory |
| 2026-10-14 | Memory up to 512 MB or 1 GB is acceptable; speed is not given up to save memory. Replaces "memory is kept low" where the two conflict |
| 2026-10-13 | The server fetches and refreshes its own data. Replaces the earlier "manual updates only" |
| 2026-10-13 | A push deploys the program; it does not overwrite data the server has refreshed |
| 2026-10-13 | Group A is built first, in one round; later groups each get their own session |
| 2026-10-13 | Network tools (group D) will be built with guards; outgoing-address exposure accepted |
| 2026-10-13 | Seed against server copy: the newer one wins |
| 2026-10-13 | Every push to `main` that passes CI is released and deployed |
| 2026-10-13 | Thirukkural public-domain plus confirmed; HSN without rates; constituency without members; no boundary files; panchang elements first |
| 2026-10-13 | Round 1 extras: mutual fund NAV, airports, railway stations (once a source is found) |
| 2026-10-13 | Tamil legacy encodings: tables are written here from the published standards; `open-tamil`'s GPL-3.0 tables are not used |
| 2026-10-13 | Mutual fund NAV is built although AMFI's site terms do not allow republishing |
| 2026-10-13 | HSN is credited to the statute (Customs Tariff Act, GST notification 11/2017), codes and descriptions only |
| 2026-10-13 | Railway stations come from Wikipedia, attributed, since no official list has a clear licence |
| 2026-10-13 | No Aadhaar format check endpoint (redaction still recognises Aadhaar numbers) |
| 2026-10-14 | Fonts for generated images (Noto, OFL 1.1) are committed and compiled into the server |
