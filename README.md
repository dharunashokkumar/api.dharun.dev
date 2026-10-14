# api.dharun.dev

Free, open APIs for Indian developers. No API key, no sign-up, CORS enabled.

```bash
curl https://api.dharun.dev/v1/pincode/637001
```

| API | Example | What you get |
|---|---|---|
| Company logos | `/v1/logos/TCS` | SVG or PNG logo of an NSE or BSE listed company |
| Companies | `/v1/companies/RELIANCE` | Sector, industry, ISIN, market cap, index memberships |
| Pincodes | `/v1/pincode/637001`, `/v1/pincode/near?lat=&lng=` | Post offices, district, state, coordinates; pincodes near a point |
| IFSC and banks | `/v1/ifsc/SBIN0000001` | Bank, branch, address, MICR, payment systems |
| RTO codes | `/v1/rto/TN-28` | Transport office for a vehicle registration |
| Holidays | `/v1/holidays/2026?type=market` | Bank, central government and stock market holidays |
| Working days | `/v1/working-days/2026-01-16?calendar=bank&state=TN` | Is it a working day, the next one, add N working days, settlement dates; `office=` for one RBI office in states with several |
| Market status | `/v1/market/status` | Whether NSE is open, and when it next opens or closes |
| HSN and SAC | `/v1/hsn/0902`, `/v1/hsn?q=tea` | Tariff codes with descriptions and their chapter |
| Mutual funds | `/v1/mutual-funds/122639` | Latest NAV of a scheme, search by name or fund house |
| Railway stations | `/v1/stations/MAS` | Station name, zone, state |
| Airports | `/v1/airports/MAA` | IATA and ICAO codes, city, coordinates |
| Phone numbers | `/v1/phone/9500112345` | Valid or not, mobile or landline, original operator and circle |
| Constituency | `/v1/constituency?pincode=637001` | Assembly and parliamentary constituency for a place |
| Panchang | `/v1/panchang/2026-01-14?pincode=600001` | Tithi, nakshatra, yoga, karana, sunrise, Rahu kalam, Tamil month |
| Sun and moon | `/v1/sun-moon/today?pincode=600001` | Sunrise, sunset, twilight, moonrise, moon phase and illumination |
| Satellite passes | `/v1/sat-pass?sat=ISS&pincode=600001` | When the ISS or an amateur radio satellite passes over, and whether it can be seen; where it is now |
| HF propagation | `/v1/propagation` | Solar flux, K and A index, X-ray class, NOAA scales, and a day and night rating per band |
| Thirukkural | `/v1/thirukkural/1` | Couplet, transliteration, two English translations; search and random |
| Transliteration | `/v1/transliterate?to=tamil&text=...` | Between nine Indian scripts, ISO 15919, IAST and ASCII |
| Tamil legacy encodings | `/v1/tamil-font/convert?from=tscii&text=...` | TSCII text to Unicode and back |
| IP lookup | `/v1/ip/8.8.8.8`, `/v1/ip` | Country, city and network of an address, or of the caller |
| Email check | `/v1/email/someone@example.com` | Syntax, disposable domain, whether the domain takes mail |
| DNS check | `/v1/dns/example.com?type=MX` | Records as several public resolvers see them |
| Traceroute from India | `/v1/mtr/example.com` | MTR-style path from a server in India: network, loss and latency per hop |
| QR and UPI QR | `/v1/qr?text=hello`, `/v1/upi-qr?pa=shop@okaxis&am=250` | QR codes, and UPI payment QR codes and links |
| Barcodes | `/v1/barcode/code128?text=AWB1234` | Code 128, EAN, Code 39, ITF, Data Matrix, PDF417 |
| Social preview image | `/v1/og-image?title=Hello` | A 1200x630 card for link previews, in ten scripts |
| Code image | `/v1/code-image?language=go&code=...` | A code snippet as a syntax-coloured picture |
| Text tools | `/v1/text/chunk`, `/redact`, `/tokens`, `/language` | Chunking for retrieval, PII redaction, token counts, language detection |
| Regex tester | `/v1/regex?pattern=...&text=...` | Every match with its groups, replacement, and the pattern explained; JavaScript or RE2 |
| Fake Indian data | `/v1/fake/companies?count=5` | Test names, addresses, phone numbers, PAN and GSTIN that pass format checks |
| Number to words | `/v1/words/1,25,000?lang=ta` | Lakh and crore spelling in English, Hindi, Tamil |
| Format checks | `/v1/validate/gstin/27AAPFU0939F1ZV` | GSTIN, PAN and IFSC structure checks |
| States | `/v1/states/TN` | State codes, GST codes, vehicle prefixes |

Full documentation with every parameter is served by the API itself at `/`, and in
machine-readable form at `/openapi.json`, `/llms.txt` and `/v1`.

## One shape everywhere

Every JSON response is an envelope:

```json
{
  "data": { "code": "TN-28", "office": "Namakkal (North)", "state": "Tamil Nadu" },
  "meta": { "source": { "name": "...", "licence": "CC BY-SA 4.0", "attribution": "..." } }
}
```

- `data` is an object, or an array for list endpoints.
- `meta.source` names the dataset, its version and the credit its licence asks for.
- List endpoints add `meta.total`, `meta.limit` and `meta.offset`.
- Errors are `{ "error": { "code", "message" } }` with a matching HTTP status.
- Each client has an allowance of 600 per minute. A lookup costs 1; a request that takes
  real CPU work (image rendering, astronomy, live DNS) costs 10.
- Text endpoints take `?text=`, or a POST body as `text/plain` or JSON `{"text": "..."}`.

Logo endpoints return the image itself, ready for an `<img>` tag. `?format=png&size=128`
gives PNG (16 to 512 pixels); `?fallback=monogram` returns an initials tile for unknown
symbols.

## For AI agents

`POST /mcp` is a Model Context Protocol server (Streamable HTTP, stateless, no
authentication). It exposes one tool per lookup; each returns exactly what the matching
REST endpoint returns.

## Data sources

| Dataset | Source | Licence | Records |
|---|---|---|---|
| Pincodes | All India Pincode Directory, Department of Posts (data.gov.in) | Government Open Data License - India | 157,125 post offices |
| IFSC | [razorpay/ifsc](https://github.com/razorpay/ifsc), compiled from Reserve Bank of India and NPCI lists | Public domain, as stated by the publisher | 183,214 branches |
| RTO codes | Wikipedia, "List of Regional Transport Office districts in India" | CC BY-SA 4.0 (shared under the same licence) | 1,361 codes |
| Bank holidays | Reserve Bank of India holiday matrix, 34 regional offices | Official publication | 2026 |
| Central government holidays | Department of Personnel and Training memoranda | Official publication | 2026, 2027 |
| Market and clearing holidays | National Stock Exchange of India | Official publication | 2026 |
| HSN and SAC codes | GST portal code list; the nomenclature is from the Customs Tariff Act, 1975 and GST notification 11/2017 | Statute | 22,614 codes |
| Mutual fund NAV | Association of Mutual Funds in India, `NAVAll.txt` | Published daily by AMFI; credited | 14,375 schemes |
| Railway stations | Wikipedia, "List of railway stations in India" | CC BY-SA 4.0 (shared under the same licence) | 3,975 stations |
| Airports | [OurAirports](https://ourairports.com/data/) | Public domain | 9,509 airports, 622 in India |
| Phone series | Wikipedia, "Mobile telephone numbering in India" | CC BY-SA 4.0 (shared under the same licence) | 2,247 series |
| Constituencies | [DataMeet](https://github.com/datameet/maps) community maps | CC BY 4.0 | 4,182 assembly, 543 parliamentary |
| Thirukkural | [tk120404/thirukkural](https://github.com/tk120404/thirukkural); translations by G.U. Pope and by Drew and Lazarus | Public domain text; compilation Apache-2.0 | 1,330 couplets |
| IP addresses | [DB-IP](https://db-ip.com) Lite | CC BY 4.0 | fetched by each server |
| Orbit elements | [CelesTrak](https://celestrak.org/NORAD/elements/), from US Space Force data via Space-Track.org; only computed passes and positions are published | None stated; redistribution with credit allowed by USSPACECOM | fetched by each server, 111 satellites |
| Space weather | [NOAA Space Weather Prediction Center](https://www.swpc.noaa.gov/) | Public domain (US government work) | fetched by each server, hourly |
| Disposable email domains | [disposable-email-domains](https://github.com/disposable-email-domains/disposable-email-domains) | CC0 | 9,881 domains |
| Logos and company details | Collected from TradingView | Logos are trademarks of the companies they identify | 3,156 logos |

Exact versions, retrieval dates and checksums are in [`data/sources.json`](data/sources.json),
and every response repeats the credit in `meta.source`.

Known gaps:

- **State government holidays** are not included. Bank holidays are per state, and they
  come from each state's own notification.
- **Andaman and Nicobar RTO codes** are missing: the source article has no section for them.
- **The pincode file** was fetched through a mirror because the government portal refuses
  scripted downloads. Its size matches the file the portal lists.
- **Railway stations** come from a community list: some stations are missing and a few
  codes are out of date.
- **Assembly constituency boundaries** pre-date the latest redrawing in six states; the
  response says so where it applies.
- **Phone operator and circle** are the original allocation of a number series. A ported
  number keeps its series, so the current operator can differ.
- **Panchang timings** are computed from Sun and Moon positions and can differ from a
  printed almanac by a minute or so.
- **Tamil legacy encodings**: TSCII only so far; TAB, TAM and Bamini are planned.
- **Hindi and Tamil number words** follow standard written forms, with Tamil compound
  numbers written as separate words. Corrections are welcome.

## Run it locally

Requires Go 1.26 or newer.

```bash
go run ./cmd/apidata build -resvg none    # build every dataset from data/ into seed/
go run ./cmd/api                          # http://127.0.0.1:8080/
go test ./...
```

PNG output (logos and generated images) is rendered by
[resvg](https://github.com/linebender/resvg); `bash tools/fetch-resvg.sh <dir>` downloads
the pinned Linux build. Without it everything else works and PNG requests answer `501`.

Configuration is by environment variable; see
[`deploy/dharun-api.env.example`](deploy/dharun-api.env.example).
[`ROADMAP.md`](ROADMAP.md) tracks what is built, what is planned and what was turned down.

## Repository layout

```text
cmd/api            the server
cmd/apidata        data tool: build, import, refresh, status
internal/api       routing, endpoints, docs, MCP, metrics
internal/dataset   one file per dataset: how it is built, fetched and checked
internal/typeset   text shaping and the fonts for generated images
internal/panchang  almanac calculations
internal/translit  transliteration between scripts
internal/tamilfont Tamil 8-bit encodings
internal/textkit   chunking, redaction, token counts, language detection
internal/fake      test data
internal/words     number to words
internal/validate  GSTIN, PAN, IFSC and checksum rules
internal/india     states and their codes
data/              one folder per dataset (logos included), and sources.json
deploy/            systemd units, installer, self-updater, monitoring
tools/             release build, smoke test, install test, resvg download, logo scraper
```

## How data stays fresh

Each dataset is its own read-only SQLite file. A release carries a *seed* copy of every
dataset, built from `data/`. On a server, `apidata refresh` runs hourly as a separate
short-lived job: each dataset that a program can fetch (IFSC, RTO codes, bank and market
holidays, HSN, mutual fund NAV, airports, stations, IP addresses, disposable domains) is
downloaded when it is due, checked, and built into a new file in the server's own data
directory. The API serves whichever copy is newer and picks up a new file without a
restart. A deploy replaces the seeds and never touches the server's own copies.

Datasets that cannot be fetched by a program are updated by hand and ship with a push:

```bash
go run ./cmd/apidata import pincode -file pincode.csv    # CSV saved from data.gov.in
python tools/scrape_logo_metadata.py --refresh           # logo metadata
go run ./cmd/apidata status                              # which copy of each dataset is in use
```

Central government holidays are typed into `data/holidays/central/<year>.json` from the
yearly memorandum, usually published in July for the following year.

## Release and deploy

Deployment is pull-based: the server fetches releases, nothing connects in to it.

1. Every push to `main` runs CI: tests, shell lint, a full release build, a smoke test of
   the bundle against the full datasets, and an install of it on a fresh Debian 13.
2. When it passes, the bundle is published as release `v1.1.<run number>`.
3. On the server, `dharun-api-update.timer` checks every 10 minutes. It downloads the new
   release, verifies the checksum, switches to it, and checks `/healthz`. If the new
   version does not come up, it switches back and marks that release as failed.

First-time setup on a Debian 12 or 13 machine or container, as root:

```bash
tar -C deploy -cz . | ssh <host> 'mkdir -p /root/api-deploy && tar -xz -C /root/api-deploy && sh /root/api-deploy/install.sh'
```

The server is one static binary plus a database file per dataset. It runs as the
unprivileged user `dharun-api` with a read-only filesystem, and uses about 35 MB of
memory idle and about 150 MB under a burst. The updater
runs as root and is only replaced by re-running `install.sh`. Restrict port `8080` to your
proxy and port `9464` to Prometheus with a firewall.

## Monitoring

- `:9464/metrics` serves Prometheus metrics: requests by API and status, response times,
  logo lookups, memory, `api_lifetime_requests_total`, and for each dataset which copy is in
  use, how old it is and whether its last refresh worked.
- `:9464/stats` serves the all-time totals as JSON. They are saved to disk and survive
  restarts.
- [`deploy/monitoring/`](deploy/monitoring/) has the Prometheus scrape job, alert rules
  (API down, a refresh that keeps failing, stale data) and a Grafana dashboard.
- Every request is one JSON line on stdout: route, status, duration, what was looked up,
  country and user agent. Client IP addresses are not logged, and format checks log
  nothing about the value checked.

## Trademark notice

Company logos and trademarks belong to their respective owners. This project is an index
for discovery and convenience; review each company's brand rules before production use.
To request removal of a logo, open an issue.
