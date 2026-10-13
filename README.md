# Indian Listed Company Logos

A free logo API and searchable directory for companies listed on India's NSE and BSE.

- `3156` logos (2468 NSE, 688 BSE) as SVG or PNG
- Look up by ticker, exchange-qualified ticker, Yahoo Finance symbol or ISIN
- Company metadata: name, sector, industry, ISIN, market cap, index memberships
- No API key, CORS enabled, OpenAPI description, MCP server for AI agents
- One small Node service with a single runtime dependency

Base URL: `https://api.dharun.dev/logos`

## Quick start

```html
<img src="https://api.dharun.dev/logos/v1/logo/TCS" alt="TCS logo" width="48" height="48" />
```

```bash
curl https://api.dharun.dev/logos/v1/companies/RELIANCE
curl "https://api.dharun.dev/logos/v1/search?q=tata&limit=5"
curl -o infy.png "https://api.dharun.dev/logos/v1/logo/INFY?format=png&size=256"
```

## API

| Endpoint | Returns |
|---|---|
| `GET /v1/logo/{symbol}` | The logo image |
| `GET /v1/companies/{symbol}` | Company metadata and logo URLs as JSON |
| `GET /v1/search?q=` | Matching companies, best match first |
| `GET /v1` | Dataset summary: counts, sectors, index names |
| `GET /openapi.json` | OpenAPI 3.1 description |
| `GET /llms.txt` | Short guide for language models |
| `POST /mcp` | Model Context Protocol server (Streamable HTTP) |
| `GET /company/{symbol}` | Human-readable page for one company |
| `GET /` | Searchable directory with bulk ZIP downloads |

### Symbols

Any of these forms work wherever a symbol is accepted:

| Form | Example |
|---|---|
| Ticker | `TCS`, `tcs`, `M&M` (URL-encode `&` as `%26`, or write `MM`) |
| Exchange-qualified | `NSE:TCS`, `BSE:AAIL`, `NSE_TCS` |
| Yahoo Finance | `TCS.NS`, `AAIL.BO` |
| ISIN | `INE467B01029` |

A bare ticker prefers NSE. Add `?exchange=nse` or `?exchange=bse` to force one.

### Logo options

| Parameter | Values | Notes |
|---|---|---|
| `format` | `svg` (default), `png` | A `.svg` or `.png` suffix on the symbol works too |
| `size` | `16` to `1024`, default `128` | PNG width and height in pixels |
| `fallback` | `monogram` | Return an initials tile instead of `404` for unknown symbols |
| `exchange` | `nse`, `bse` | Force an exchange |

Unknown symbols return `404` with a JSON body: `{"error":{"code":"not_found","message":"..."}}`.
Responses carry `ETag` and long `Cache-Control` headers. API routes are limited to 600
requests per minute per client.

### Search options

`q` matches ticker, company name, sector, industry, ISIN and index names. Filter with
`exchange`, `sector` and `index` (for example `index=Nifty 50`). Page with `limit` (max 100)
and `offset`.

### MCP

Point an MCP client at `https://api.dharun.dev/logos/mcp`. It is stateless and needs no
authentication. Tools: `search_logos` and `get_logo`.

## Run it locally

Requires Node 22 or newer.

```bash
npm ci
npm start          # http://127.0.0.1:8080/logos/
npm test
```

Configuration is by environment variable. See `deploy/logos-api.env.example` for the full
list. Prometheus metrics are served on a separate port (`METRICS_PORT`, default `9464`)
at `/metrics`, never on the public port.

## Repository layout

```text
logos/nse, logos/bse   the SVG files, named NSE_<TICKER>.svg and BSE_<TICKER>.svg
data/                  logos.json (served index) and logo_metadata.full.json (its source)
server/                the API
web/                   the directory website
test/                  node:test suites for the dataset, the API and the updater
tools/                 data build, validation and release scripts
deploy/                systemd units, installer, self-updater, monitoring
.github/workflows/     CI and release
```

## Updating logos and data

```bash
python tools/scrape_logo_metadata.py --refresh   # rewrites data/logo_metadata.full.json
npm run normalize:svgs                           # adds a viewBox to any new SVG
npm run build:data                               # rebuilds data/logos.json
npm run validate                                 # every logo present, safe and indexed
```

The scraper reads company metadata from TradingView's India scanner endpoint. It is run
by hand; nothing in CI calls it.

## Release and deploy

Deployment is pull-based: the server fetches releases, nothing connects in to it.

1. `npm version patch` (or `minor` / `major`), then `git push --follow-tags`.
2. The **Release** workflow tests, builds `logos-api.tar.gz` with its checksum, starts the
   bundle to smoke-test it, and publishes a GitHub release.
3. On the server, `logos-api-update.timer` checks every 10 minutes. It downloads the new
   release, verifies the checksum, switches to it, and checks `/healthz`. If the new
   version does not come up, it switches back and marks that release as failed.

First-time setup on a Debian 12 or 13 machine or container, as root:

```bash
tar -C deploy -cz . | ssh <host> 'mkdir -p /root/logos-deploy && tar -xz -C /root/logos-deploy && sh /root/logos-deploy/install.sh'
```

Then set `PUBLIC_ORIGIN` in `/etc/logos-api/logos-api.env` and route your proxy or tunnel
to port `8080`. To roll back by hand, point `/opt/logos-api/current` at an older directory
in `/opt/logos-api/releases` and restart `logos-api`.

The updater runs as root and is only replaced by re-running `install.sh`. The service
itself runs as the unprivileged user `logos-api` with a read-only filesystem. Restrict
port `8080` to your proxy and port `9464` to Prometheus with a firewall.

## Monitoring

- `deploy/monitoring/prometheus-scrape.yml`: scrape job for Prometheus.
- `deploy/monitoring/grafana-dashboard.json`: requests, status codes, latency, logo
  lookups, memory.
- Every request is logged as one JSON line on stdout (route, status, duration, requested
  and resolved symbol, country, user agent). Client IP addresses are not logged.

Metric labels never contain symbols. Use the request log to see which logos are requested
or missing.

## Trademark notice

Company logos and trademarks belong to their respective owners. This project is an index
for discovery and convenience; review each company's brand usage rules before production
use. To request removal of a logo, open an issue.
