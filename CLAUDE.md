# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`api.dharun.dev`: free, open APIs for Indian developers (company logos, pincodes, IFSC,
RTO codes, holidays, number to words, format checks, and more being added). One Go
service owns the whole hostname; every API is a path under `/v1`. No API keys, rate
limit per IP.

`ROADMAP.md` tracks what is implemented, what is planned and what was rejected, with the
data source and refresh method for each API. Read it before adding an API, and update it
in the same change.

## Commands

Go 1.26 or newer. The repo is developed on Windows and built and deployed on Linux.

```bash
go run ./cmd/apidata build -resvg none     # build every dataset from data/ into seed/
go run ./cmd/apidata build rto             # or just one
go run ./cmd/api                           # http://127.0.0.1:8080/ (metrics on :9464)
go test ./...                              # everything
go test ./internal/api -run TestPincode    # one test
gofmt -l cmd internal                      # CI fails if this prints anything
go vet ./...
```

Trying the server-side refresh locally (fetches from the real sources):

```bash
go run ./cmd/apidata refresh -live live ifsc   # -force to ignore "not due" and the sanity checks
go run ./cmd/apidata status -live live         # which copy of each dataset is in use
DATA_DIR=live go run ./cmd/api                 # serves the newer of seed/ and live/
```

`seed/` and `live/` are local build output and ignored by git.

CI (`.github/workflows/ci.yml`) also runs `shellcheck deploy/*.sh tools/*.sh`, then
builds the real release bundle and smoke-tests it:

```bash
bash tools/build-release.sh 0.0.0-ci       # Linux only: downloads resvg, renders PNGs
bash tools/smoke-test.sh dist/dharun-api-linux-amd64.tar.gz
```

Linux-only steps (release build, shellcheck, `internal/deploytest`, which has a
`//go:build linux` tag) are run locally through WSL:
`MSYS_NO_PATHCONV=1 wsl.exe -d Ubuntu -- bash <script file>`. WSL has Go and shellcheck
in `~/.cache/tools` (put it on `PATH` in the script). WSL's `/tmp` is wiped between
calls; work under `~/.cache`. Write script files with the Write tool: heredocs containing
apostrophes break in the Bash tool.

## Architecture

### Datasets: seed copies and server copies

Each dataset is its own read-only SQLite file, described by a `Spec` registered in
`internal/dataset` (one file per dataset: `ifsc.go`, `rto.go`, `holidays.go`, ...). A
spec says how to `Build` the tables from source files, how to `Import` those files from
the source, how often a server refreshes it (`Every`, zero means only by hand), and the
sanity limits a refresh must pass (`Main`/`MaxShrink`, optional `Check`).

There are two copies of a dataset, built by the same code:

- **Seed**: built from the repository's `data/` by `apidata build` and shipped inside
  every release under `seed/`. A deploy replaces it.
- **Server copy**: fetched and built on the server by `apidata refresh` into `DATA_DIR`
  (`/var/lib/dharun-api/data`), named `<dataset>.<UTC stamp>.db`. A deploy never touches
  it.

`dataset.Current` decides which one is served: the newer one by `Source.Retrieved`
(a date for seeds, a timestamp for server copies), ignoring any file built for a
different `Schema` number. So a correction committed to `data/` reaches the server, and
an old seed never replaces fresher server data. **Bump a spec's `Schema` whenever its
tables or their meaning change**; the server then serves the seed and rebuilds its own
copy on the next refresh run.

A refresh writes into a temporary directory, builds a new file, checks it against the
copy in use (record count may not fall more than `MaxShrink`; `Check` may compare
contents), and only then leaves it in `DATA_DIR`. A failed source is retried after six
hours. Outcomes go to `DATA_DIR/refresh.json`, which the API turns into metrics. Holiday
sources only publish the current year, so their importers carry earlier years forward
from the copy in use (`carryForward`).

Some datasets cannot be fetched by a program and stay seed-only: pincodes (the portal
refuses scripted downloads), central government holidays (typed in by hand), logos.
For those, `apidata import <dataset>` updates `data/` and the change ships with a push.

### The server (`cmd/api`, `internal/api`)

`store.go` holds the copy in use of every dataset behind an atomic pointer and checks
both directories every `RELOAD_SECONDS`; a new file is opened and swapped in without a
restart, and the old one is closed a minute later. Handlers get a dataset per request
with `s.need(w, r, "name")` (answers 503 when the server has no copy) or `s.data(name)`,
and query it through `handle.row` / `handle.rows`, which prepare statements per file.
Never keep a `*sql.DB` or derived data across requests without hooking
`Server.datasetChanged`, which is how the company index is rebuilt when logos change.

`Server.ServeHTTP` in `server.go` wraps every request: common headers, CORS, method
check (GET, HEAD, POST), panic recovery, metrics, one JSON log line. Routes are
registered in `registerRoutes` (`docs.go`) and `registerMore` (`extras.go`) through
`s.handle(route, cost, pattern, handler)`: `route` is the metric label, `cost` is what a
request charges against the per-IP allowance (`costFree`, `costLight`, `costHeavy`). A
`costHeavy` route also takes one of `HEAVY_CONCURRENCY` slots, so bursts queue instead of
using more memory.

Handlers answer through `s.ok` / `s.fail` (`respond.go`), which produce the
`{ data, meta }` envelope, ETag, 304 and gzip. Do not write JSON to the response
directly. Text endpoints read input with `s.readText` (`input.go`): `?text=` on GET, or a
POST body as text/plain or JSON, capped at `MAX_BODY_KB`.

Each handler fills `info(r)` (a `requestInfo`): `subject` is what gets logged as the
thing looked up, and `private = true` keeps the value out of logs and caches. Use
`private` for anything that could be personal (validators, email checks, redaction).
Client IP addresses are never logged.

`/healthz` starts with `{"status":"ok","version":"X",`; the updater greps for that, so
keep the version first. It also lists every dataset with the copy in use.

### The catalog is the single source for documentation

`catalog.go` describes every endpoint once. The docs site (`/`, `/docs/{group}`),
`/openapi.json`, `/llms.txt`, `/sitemap.xml` and the `/v1` index are all generated from
it. `TestEveryDocumentedExampleWorks` requests every `Example` in the catalog, and every
`GET /v1/...` route must be in the catalog.

### MCP

`mcp.go` is a stateless MCP server (Streamable HTTP) at `POST /mcp`. A tool does not
have its own logic: `callTool` builds a REST path and runs it through the same mux in
memory, so a tool returns exactly what the REST endpoint returns.

### Adding an API

1. Agree the data source, licence and refresh method first (see the rules below), and
   record them in `ROADMAP.md`.
2. Data, if any: a `Spec` in its own file in `internal/dataset` with `Build`, and
   `Import` if a program can fetch it; seed files under `data/`; a record in
   `data/sources.json`.
3. A handler in `internal/api` in its own file, registered in `registerMore`.
4. A catalog entry with a real `Example`; an MCP tool if an agent would call it.
5. Fixture rows in `internal/api/testdata/` (real rows cut from the full data) and tests.
6. A line in `tools/smoke-test.sh` and the README table.

### Deploy

Pull-based: nothing connects in to the server, and GitHub never fetches data.

1. Every push to `main` runs CI. When it passes, the `publish` job releases it as
   `v1.1.<run number>` with `dharun-api-linux-amd64.tar.gz` (api, apidata, resvg, seed/,
   VERSION), its `.sha256` and `VERSION`, and prunes automatic releases beyond 15.
2. On the server, `dharun-api-update.timer` (every 10 min) runs
   `deploy/dharun-api-update.sh`: download, verify checksum, switch the
   `/opt/dharun-api/current` symlink, restart, check `/healthz` reports the new version,
   roll back and mark the release failed if not. `internal/deploytest` tests it end to
   end against a fake release server.
3. `dharun-api-refresh.timer` (hourly) runs `apidata refresh` as the service user; each
   dataset decides whether it is due.

Measured on Linux: the API idles at 22 MB and peaked at 40 MB under a burst
(`MemoryMax=192M`); the refresh job peaks at 200 MB during the weekly IFSC refresh
(`MemoryMax=512M`, separate unit).

## Rules that the code does not show

- **Settled product decisions** (do not reverse as "improvements"): flat `/v1/<resource>`
  URLs, Go, the `{ data, meta }` envelope with `meta.source` on every response, no API
  keys, documentation-only website, private monitoring, one container, servers refresh
  their own data, every push to `main` deploys. Ask before changing any of them.
- **Ask Dharun before deciding product questions.** He wants clarifying questions and to
  make the calls himself.
- **Say where data comes from before building.** For a new dataset, state the source,
  its licence and how it will be refreshed, and get agreement first. Only use data that
  is legal to redistribute.
- **This repository is public; the server is a private homelab.** Container IDs, LAN IP
  addresses, tunnel names and scrape targets belong in the separate private homelab
  repository, never here.
- **Memory is the scarce resource, CPU is not.** Prefer work at build time, in the
  short-lived refresh job, or per request over data held in the API's memory. Do not drop
  features to save memory; measure and report instead.
- **Commits carry no AI attribution trailer.**
- **Line endings are LF everywhere** (`.gitattributes`); CRLF breaks the deploy scripts.
