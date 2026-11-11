# New endpoints: the idea catalogue

Dharun, 2026-11-10: new endpoints in all four directions, and creative ideas above all.
Data rule: anything published openly may be used, case by case, credited.

**Nothing here is agreed for building.** Every idea with a data source still needs its
source, terms and refresh method confirmed with Dharun first; sources and licences
below are from memory and **must be checked** before use. Dharun marks the ideas he
wants; the session that builds them moves them into `ROADMAP.md`.

How to read the tables:

- **Risk**: `none` pure logic, no data. `low` open licence or statute. `medium`
  official body, terms unclear (the AMFI kind of case). `high` private site, or data
  someone sells; a takedown request is possible.
- **★** marks the ideas recommended first: cheap to build, clearly useful, or unlike
  anything other free APIs offer.

Still not built, whatever the data rule says (`ROADMAP.md` section 8): personal data
lookups (PAN, Aadhaar, vehicle owner), PNR status, captcha-protected sites, live share
prices, open web scraping, maps of India's boundary, and anything that publishes
strangers' content under the domain.

## Set A: cheap, with proven or obvious demand (Part 4)

Mostly pure logic on what the server already has.

| ★ | Idea | Address (proposed) | What it does | Risk |
|---|---|---|---|---|
| ★ | **Batch lookups** | `POST /v1/batch` | Up to 1,000 lookups in one call (pincodes, IFSC, phone numbers, GSTIN checks, email checks), one answer per line. The data shows people already do this one request at a time: 29,000 phone numbers in a day, 11,000 emails | none |
| ★ | **Calendar feeds** | `/v1/holidays/2027.ics?state=TN`, also market holidays and festivals | A calendar file people subscribe to in Google Calendar or Outlook. Subscribed calendars are fetched again and again, so this brings steady use, and it suits a tool page | none |
| ★ | **Today in India** | `/v1/today` | One call: date and time in IST, financial year and quarter, is it a bank, market and government working day (per state), today's festivals, panchang in brief, sunrise in a named city. Made for agents and widgets that need India's "now" | none |
| ★ | **Money calculators** | `/v1/calc/{kind}` | SIP, lump sum, EMI with schedule, GST split (inclusive and exclusive), income tax under both regimes for a named year, HRA, gratuity, PPF, capital gains with indexation, brokerage and statutory charges on a trade. Rules typed from the Finance Act and circulars, each answer naming the rule and year | low (rules change yearly: a yearly review) |
| ★ | **Avatars** | `/v1/avatar/{name}` | Initials or a generated pattern from a name, in Indian scripts too (the monogram and typesetting code exist). Developers use such services in every app with user lists | none |
| ★ | **Placeholder images** | `/v1/placeholder/{size}` | A grey box with its size or a text, for mock-ups | none |
| ★ | **Chart images** | `/v1/chart` | JSON in, a bar, line, pie or sparkline chart out as SVG or PNG. For emails, chat bots and READMEs, where JavaScript charts cannot run. Uses the drawing code | none |
| | Number formatting | `/v1/format/number/{n}` | Lakh and crore grouping, short forms (1.2 Cr), Indian-script digits, rupee formatting | none |
| | Indian date tools | `/v1/dates/...` | Financial year, assessment year and quarter for a date; Saka (national calendar) date; Tamil calendar date; tabular Hijri date; age and date differences | none |
| | UPI handle lookup | `/v1/upi-handle/{handle}` | `@okhdfcbank` → the app and the bank behind it. From NPCI's published list of UPI apps and handles | medium |
| | Bharat QR and UPI link reader | `/v1/qr/parse` | Reads the fields of a UPI link or an EMVCo merchant QR string (payee, amount, merchant category) | none |
| | Readme badges | `/v1/badge/...` | Small SVG badges: market open or closed, next holiday, a dataset's freshness | none |
| | Address reader | `/v1/address/parse` | Free-text Indian address → house, street, locality, city, district, state, pincode, checked against the pincode data; flags a pincode and state that do not match | none (Private) |
| | Company cross-search | `/v1/companies?q=` | One search over ticker, ISIN, BSE code and name (in the roadmap's idea list) | none |

## Set B: markets and company data (Part 5)

Sits next to the one proven product. Most sources are exchanges and official bodies
whose terms are unclear or restrictive; each needs its own decision.

| ★ | Idea | Address | Source | Refresh | Risk |
|---|---|---|---|---|---|
| ★ | **Index constituents** | `/v1/indices`, `/v1/indices/{name}` | NSE Indices and BSE constituent lists (CSV files) | Monthly | medium |
| ★ | **IPO calendar** | `/v1/ipos` | NSE and BSE public issue pages; SEBI filings | Daily | medium |
| ★ | **Mutual fund history and returns** | `/v1/mutual-funds/{code}/history`, `/returns` | AMFI's historical NAV download. Returns over 1, 3, 5 years, rolling returns, SIP result computed here | Daily | medium (the AMFI decision exists) |
| | Corporate actions | `/v1/corporate-actions` | Exchange corporate action lists: dividends, splits, bonus, record dates | Daily | medium |
| | Results and board meeting calendar | `/v1/results-calendar` | Exchange announcements | Daily | medium |
| | Derivatives calendar | `/v1/expiries` | Computed from the exchange rules and the holiday data: weekly and monthly expiry dates, moved for holidays | With rule changes | low |
| | Institutional flows | `/v1/fii-dii` | The daily FII and DII figures the exchanges publish | Daily | medium |
| | Sectors and industries | `/v1/sectors` | The industry classification the exchanges publish, joined to companies | Monthly | medium |
| ★ | **Gold and silver rates** | `/v1/bullion` | IBJA's published daily rates | Twice daily | high |
| | Fuel prices | `/v1/fuel?city=` | The oil companies' published city prices; PPAC | Daily | high |
| | INR reference rates | `/v1/fx` | FBIL (open since October: terms unread) | Daily | medium |
| | Policy and savings rates | `/v1/rates` | RBI repo and reverse repo, small savings rates, cost inflation index, typed from the notifications | On change | low |
| | Inflation | `/v1/cpi` | MOSPI's consumer price index releases | Monthly | low |
| | Bank codes | `/v1/micr/{code}`, SWIFT | MICR is in the IFSC data already; SWIFT lists are compiled privately | With IFSC | low / high |
| | Card number prefix | `/v1/bin/{prefix}` | Open BIN lists: network, type, issuing bank. Quality varies; Private | Monthly | medium |

## Set C: India civic and daily life (Part 6)

| ★ | Idea | Address | Source | Refresh | Risk |
|---|---|---|---|---|---|
| ★ | **Places of India** | `/v1/places/...` | The Local Government Directory: states, districts, sub-districts, blocks, villages and towns with official codes. Gives every app proper State → District → Taluk dropdowns | Monthly | low |
| ★ | **Festivals** | `/v1/festivals/{year}` | Computed from the panchang (Hindu and Tamil festivals by tithi and nakshatra rules); Islamic dates marked as expected, subject to sighting; fixed-date ones typed | Computed | none |
| ★ | **Old law to new law** | `/v1/law/ipc/{section}`, `/bns/{section}` | The official correspondence tables between the IPC, CrPC and Evidence Act and the three 2023 codes, with section headings. Statute text is public. Law students and legal-tech developers look this up constantly | With amendments | low |
| ★ | **Mandi prices** | `/v1/mandi?commodity=&state=` | Agmarknet daily market prices through the open government data platform | Daily | low |
| | State government holidays | `/v1/holidays/{year}?state=` | Each state's notification, typed by hand (a known gap in the live data) | Yearly | low |
| | Minimum support prices | `/v1/msp` | The government's announced prices per crop and season | Per season | low |
| | Colleges and universities | `/v1/colleges` | AISHE directory | Yearly | medium |
| | Schools | `/v1/schools` | UDISE+ directory | Yearly | medium |
| | Hospitals and blood banks | `/v1/hospitals`, `/v1/blood-banks` | Directories on the open government data platform | Monthly | low |
| | Census figures | `/v1/census/{place}` | Census 2011 tables: population, literacy, households by district and town | None | low |
| | Toll plazas | `/v1/tolls` | NHAI's plaza list and fee tables | Monthly | medium |
| | Air quality | `/v1/air-quality?city=` | CPCB's station readings on the open government data platform | Hourly | low to medium |
| | Weather | `/v1/weather` | IMD publishes openly but states no reuse terms; a global open model is the other route | Hourly | medium |
| | Helplines | `/v1/helplines` | National and state emergency and helpline numbers, typed | Yearly | low |
| | STD and ISD codes | `/v1/std/{code}` | The numbering plan | Rarely | low |
| | Trains and timetables | `/v1/trains` | The only open timetable is an old government data release; fresh data is the railway's own | | high, and stale |
| | The Constitution | `/v1/constitution/{article}` | The text is public; articles, parts, schedules, amendments | With amendments | low |
| | Indian classics | `/v1/classics/...` | Public domain texts beside the Thirukkural: Aathichoodi, Bharathiyar's poems, Kabir's dohas, the Gita in Sanskrit with a pre-1900 translation | None | low (check each translation) |
| | Braille for Indian scripts | `/v1/braille` | Bharati braille rules, pure logic (from the October review) | None | none |
| | Keyboard layouts | `/v1/keyboard/convert` | Text typed on InScript or Tamil99 as if on a QWERTY board, converted both ways | None | none |
| | Indic text tools | `/v1/indic/...` | Split into syllables, sort in dictionary order, normalise look-alike sequences, count letters as readers do | None | none |

## Set D: developer utilities

Pure logic unless noted. Small each; best built several at a time.

| ★ | Idea | Address | Notes |
|---|---|---|---|
| ★ | **Link preview data** | `/v1/unfurl?url=` | Title, description, image and icon of a page, through the guarded client that the probe uses. Every chat and notes app needs it. Charged like the probe; Private |
| ★ | **Request bin** | `/v1/hook` | A private address that records what is sent to it, read back by its owner. In the roadmap as Open; accounts make it safe: bins belong to a key, live for a day, and are never shown as a page |
| ★ | **Web page to Markdown** | `/v1/read?url=` | One named page fetched through the guarded client and returned as clean Markdown, for agents. Close to the "no scraping" rule: one page per call, robots.txt respected, key only, strict per-domain limits. **Needs Dharun's decision** |
| | Convert data formats | `/v1/convert` | JSON, YAML, TOML, CSV, XML to each other |
| | Compare | `/v1/diff` | Text or JSON differences, as a patch or as a list |
| | Cron | `/v1/cron` | Explains an expression in words and lists the next run times in IST |
| | Tokens and ids | `/v1/jwt/decode` (Private), `/v1/uuid`, `/v1/hash` | Decode without verifying; UUID and ULID; hashes and HMAC |
| | User agent reader | `/v1/user-agent` | Browser, system, device, bot or not. The rules exist in `internal/traffic` |
| | Time | `/v1/time` | Convert between zones and formats; "3 pm IST in Berlin" |
| | HTML to Markdown and back | `/v1/markdown` | For content pipelines |
| | Colours | `/v1/color` | Palette from an image, contrast check, conversions |
| | Image extras | `/v1/image/...` | Icon pack from one picture, blur placeholder (ThumbHash), SVG to PNG (cleaned first), watermark |
| | PDF extras | `/v1/pdf/...` | Watermark, page numbers, images to PDF, password removal when the caller gives it |
| | Audio and video | `/v1/media/...` | Convert, trim, waveform image, video thumbnail with ffmpeg. Key only |
| | Screenshot and HTML to PDF | `/v1/screenshot` | Needs a headless browser (200 to 500 MB a run). Key only, guarded client, last in line |
| | Spreadsheets | `/v1/sheet` | XLSX or CSV to JSON and back |
| | Throwaway database | `/v1/db` | In the roadmap as Open; still blocked by how a database port would reach the public |

## Set E: AI and agent tools

The heavy three (document AI, LLM, sandbox) are in [06-heavy-ai.md](06-heavy-ai.md).
Lighter ones:

| ★ | Idea | Address | Notes |
|---|---|---|---|
| ★ | **Search over your own documents** | `/v1/rag` | Upload a few documents under a key; they are chunked and embedded with the models already on the server; ask questions and get the best passages back, reranked. Kept for a day, private to the key. Chunk, embed and rerank exist; this joins them |
| ★ | **MCP tool sets** | `/mcp?tools=` | 75 tools fill an assistant's context. Offer named sets (lookups, files, text, network) and one "find a tool" tool that returns the right few |
| | Agent notes | `/v1/kv` | A small private key-value store per key, for agents that need to remember between runs. Size and lifetime capped |
| | More speech | `/v1/tts` | Voices for Tamil, Kannada, Bengali, Gujarati and Marathi (none in Piper's 2023 set). Candidates carry non-commercial licences; check each |
| | Better Indian speech to text | `/v1/stt` | An Indian-language model beside Whisper small, where a licence allows |
| | Typed MCP answers | | `outputSchema` from the same definitions as OpenAPI (in the search audit) |

## Platform work that makes the rest stronger

Not endpoints, but asked for ("more powerful and fastest"):

- **Cache at Cloudflare** for logos and other long-lived answers, with edge counts on
  the dashboard (roadmap item, not started).
- **Keep the small embedding model loaded**, so the first call is not 2.4 s.
- **Typed OpenAPI**, then generated clients for npm, PyPI and pub.dev.
- **Dataset changes**: which records a refresh added, removed or changed (from the
  October review), as `/v1/changes/{dataset}`. Lets apps keep a local copy in step.
- **The audit tracker's open items** that matter more with accounts: one refresh at a
  time (A12), stop work for callers who left (A08).

## Recommended picks, if Dharun wants a starting set

Part 4: batch lookups, calendar feeds, Today in India, money calculators, avatars,
placeholders, chart images.
Part 5: index constituents, IPO calendar, mutual fund history and returns, gold and
silver rates.
Part 6: Places of India, festivals, old law to new law, mandi prices.
Alongside: link preview data, request bin, search over your own documents, MCP tool sets.

## Build status (2026-11-11)

Part 4 (set A, the recommended picks) built, tested and committed, not pushed. Dharun
chose all the calculators, income tax included.

- [x] Batch lookups `POST /v1/batch` (14 kinds, up to 1,000, ten per unit, `format=ndjson`).
- [x] Calendar feeds `/v1/holidays/{year}.ics` (bank, market, central; state filter).
      Festivals are not in it: there is no festival list yet (set C).
- [x] Today in India `/v1/today`.
- [x] Money calculators `/v1/calc/...` (`internal/calc`, with tests against known
      figures): SIP, lump sum, EMI, GST, income tax 2025-26 and 2026-27, HRA, gratuity,
      PPF, capital gains, trade charges. **Check before release:** the 2026-27 income tax
      rules are taken as unchanged from 2025-26, and the F&O STT changes are not modelled
      (equity delivery and intraday only).
- [x] Avatars, placeholders, chart images.
- [x] MCP tools: `calculate_indian_money`, `today_in_india`, `batch_lookup`, `make_chart_image`.
- [x] Number formatting, Indian date tools, UPI handle lookup, QR parse, badges, address
      reader; company cross-search was already `/v1/companies` (2026-11-11, branch `growth-catalogue`).
- [x] Set D, every item (2026-11-11, branch `growth-catalogue`); screenshot and HTML to PDF wait for Chromium on the server.
- [ ] Sets B, C, D, E: each data source to be agreed with Dharun first (Parts 5 and 6).
