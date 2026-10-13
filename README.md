# api.dharun.dev

Free, open APIs for Indian developers. No API key, no sign-up, CORS enabled.

```bash
curl https://api.dharun.dev/v1/pincode/637001
```

| API | Example | What you get |
|---|---|---|
| Company logos | `/v1/logos/TCS` | SVG or PNG logo of an NSE or BSE listed company |
| Companies | `/v1/companies/RELIANCE` | Sector, industry, ISIN, market cap, index memberships |
| Pincodes | `/v1/pincode/637001` | Post offices, district, state, coordinates |
| IFSC and banks | `/v1/ifsc/SBIN0000001` | Bank, branch, address, MICR, payment systems |
| RTO codes | `/v1/rto/TN-28` | Transport office for a vehicle registration |
| Holidays | `/v1/holidays/2026?type=market` | Bank, central government and stock market holidays |
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
- Data endpoints allow 600 requests per minute per client.

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
| RTO codes | Wikipedia, "List of Regional Transport Office districts in India" | CC BY-SA 4.0 (this dataset is shared under the same licence) | 1,361 codes |
| Bank holidays | Reserve Bank of India holiday matrix, 34 regional offices | Official publication | 2026 |
| Central government holidays | Department of Personnel and Training memoranda | Official publication | 2026, 2027 |
| Market holidays | National Stock Exchange of India | Official publication | 2026 |
| Logos and company details | Collected from TradingView | Logos are trademarks of the companies they identify | 3,156 logos |

Exact versions, retrieval dates and checksums are in [`data/sources.json`](data/sources.json),
and every response repeats the credit in `meta.source`.

Known gaps:

- **State government holidays** are not included. Bank holidays are per state, and they
  come from each state's own notification.
- **Andaman and Nicobar RTO codes** are missing: the source article has no section for them.
- **The pincode file** was fetched through a mirror because the government portal refuses
  scripted downloads. Its size matches the file the portal lists.
- **Hindi and Tamil number words** follow standard written forms, with Tamil compound
  numbers written as separate words. Corrections are welcome.

## Run it locally

Requires Go 1.26 or newer.

```bash
go run ./cmd/apidata build -out api.db    # compile data/ into the database
go run ./cmd/api                          # http://127.0.0.1:8080/
go test ./...
```

PNG logos are pre-rendered at build time by [resvg](https://github.com/linebender/resvg).
Without it on `PATH` the build still works and PNG requests answer `501`.

Configuration is by environment variable; see
[`deploy/dharun-api.env.example`](deploy/dharun-api.env.example).

## Repository layout

```text
cmd/api          the server
cmd/apidata      data tool: import datasets, build the database
internal/api     routing, endpoints, docs, MCP, metrics
internal/dataset database schema and build
internal/words   number to words
internal/validate GSTIN, PAN and IFSC checks
internal/india   states and their codes
data/            dataset snapshots and sources.json
logos/           SVG logo files
deploy/          systemd units, installer, self-updater, monitoring
tools/           release build, smoke test, logo metadata scraper
```

## Updating data

Updates are run by hand. Each command rewrites files under `data/`; review the diff,
then release.

```bash
go run ./cmd/apidata import ifsc                         # latest razorpay/ifsc release
go run ./cmd/apidata import rto                          # latest Wikipedia revision
go run ./cmd/apidata import pincode -file pincode.csv    # CSV saved from data.gov.in
go run ./cmd/apidata import bank-holidays -year 2027     # Reserve Bank matrix
go run ./cmd/apidata import market-holidays -year 2027   # NSE trading holidays
python tools/scrape_logo_metadata.py --refresh           # logo metadata
```

Central government holidays are typed into `data/holidays/<year>.json` from the yearly
memorandum, usually published in July for the following year.

## Release and deploy

Deployment is pull-based: the server fetches releases, nothing connects in to it.

1. Tag a version: `git tag v1.0.1 && git push origin v1.0.1`.
2. The **Release** workflow tests, builds the database and the Linux binary, smoke-tests
   the bundle against the full datasets, and publishes a GitHub release.
3. On the server, `dharun-api-update.timer` checks every 10 minutes. It downloads the new
   release, verifies the checksum, switches to it, and checks `/healthz`. If the new
   version does not come up, it switches back and marks that release as failed.

First-time setup on a Debian 12 or 13 machine or container, as root:

```bash
tar -C deploy -cz . | ssh <host> 'mkdir -p /root/api-deploy && tar -xz -C /root/api-deploy && sh /root/api-deploy/install.sh'
```

The server is one static binary plus one database file. It runs as the unprivileged user
`dharun-api` with a read-only filesystem, and uses about 20 MB of memory idle. The updater
runs as root and is only replaced by re-running `install.sh`. Restrict port `8080` to your
proxy and port `9464` to Prometheus with a firewall.

## Monitoring

- `:9464/metrics` serves Prometheus metrics: requests by API and status, response times,
  logo lookups, memory, and `api_lifetime_requests_total`.
- `:9464/stats` serves the all-time totals as JSON. They are saved to disk and survive
  restarts.
- [`deploy/monitoring/`](deploy/monitoring/) has the Prometheus scrape job and a Grafana
  dashboard.
- Every request is one JSON line on stdout: route, status, duration, what was looked up,
  country and user agent. Client IP addresses are not logged, and format checks log
  nothing about the value checked.

## Trademark notice

Company logos and trademarks belong to their respective owners. This project is an index
for discovery and convenience; review each company's brand rules before production use.
To request removal of a logo, open an issue.
