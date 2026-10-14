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
bash tools/fetch-resvg.sh ~/.cache/tools   # Linux only: the pinned SVG renderer
RESVG=~/.cache/tools/resvg go test ./...   # with RESVG set, PNG output is tested too
bash tools/build-release.sh 0.0.0-ci       # Linux only: renders PNG logos with resvg
bash tools/smoke-test.sh dist/dharun-api-linux-amd64.tar.gz
bash tools/install-test.sh dist            # needs Docker: installs on a fresh Debian 13 with systemd
```

resvg has no Windows build. Without `RESVG` the PNG tests skip themselves and the image
endpoints are tested as SVG only, so run the suite on Linux before trusting a change to
rendering.

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

- **Seed**: built from the repository's `data/` (one folder per dataset, logos included)
  by `apidata build` and shipped inside
  every release under `seed/`. A deploy replaces it.
- **Server copy**: fetched and built on the server by `apidata refresh` into `DATA_DIR`
  (`/var/lib/dharun-api/data`), named `<dataset>.<UTC stamp>.db`. A deploy never touches
  it.

`dataset.Current` decides which one is served: the newer one by `Source.Retrieved`
(a UTC timestamp; older `sources.json` entries are plain dates), ignoring any file built for a
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

Two spec flags mark the odd ones. `Fixed` (Thirukkural, constituencies) is a small text
or map that never refreshes; tests build it from the real `data/` rather than a fixture.
`NoSeed` (the IP database) is too large and too short-lived for the repository: it only
ever exists as a server's own copy, and its endpoints answer 503 until the first refresh.

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
`private` for anything that could be personal (validators, email checks, redaction),
and mark the endpoint `Private` in the catalog: `handle` reads that mark, so a request
refused before the handler runs (rate limit, busy, wrong method, preflight) stays out of
the log too. A test fails when a handler marks itself private and the catalog does not.
Client IP addresses are never logged.

`/healthz` starts with `{"status":"ok","version":"X",`; the updater greps for that, so
keep the version first. It also lists every dataset with the copy in use.

### The catalog is the single source for documentation

`catalog.go` describes every endpoint once. The docs site (`/`, `/docs/{group}`),
`/openapi.json`, `/llms.txt`, `/sitemap.xml` and the `/v1` index are all generated from
it. `TestEveryDocumentedExampleWorks` requests every `Example` in the catalog, and every
`GET /v1/...` route must be in the catalog.

### Generated images (`render.go`, `ogimage.go`, `codeimage.go`, `internal/typeset`)

Text is never handed to the renderer. `internal/typeset` cuts a text into runs by script,
shapes each run with `go-text/typesetting` using Noto fonts compiled into the binary
(Latin, nine Indian scripts, and a monospaced face), and returns glyph outlines with exact
positions. A `canvas` (`render.go`) draws those as `<use>` references to outlines defined
once, so the SVG needs no fonts wherever it is shown and line breaking uses real widths.
resvg's own text layout was tried first and drops conjuncts when scripts are mixed; do not
go back to `<text>` elements.

PNG is that SVG piped through the `resvg` program, one short-lived process per image
(`RENDER_CONCURRENCY`, default 4). A large picture (resvg needs 40 to 75 MB for one)
counts as half the slots. Parsed fonts are kept once used (16 MB for all twenty-two);
drawing and bitmap resizing go through `renderer.work`, one per CPU.
The fonts are committed in `internal/typeset/fonts` with their licence; a test checks
each file against `SHA256SUMS` there, which also records where they came from.

### Logic packages

Endpoints that compute rather than look up keep their logic out of `internal/api`:
`internal/panchang` (almanac, on `go-sampa`), `internal/translit` (scripts and Roman
schemes), `internal/tamilfont` (8-bit Tamil encodings; tables are written from the
published standards, never copied from GPL converters), `internal/textkit` (chunking,
redaction, token counts with a vocabulary that is loaded on first use and kept while it
is in use, language detection), `internal/fake`, `internal/words`, `internal/validate`.

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
   A test file adds its MCP tool's arguments to `mcpTestCalls` in `init`. Tests never
   touch the network: DNS goes through `fakeDNS`, and the IP database is a fixture built
   in `TestMain`.
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

Measured on Linux: the API idles at 34 MB and peaked at 144 MB under a mixed burst over
every kind of endpoint; resvg adds 11 to 75 MB per image while it runs (`MemoryMax=768M`,
`GOMEMLIMIT=384MiB`). The refresh job peaks at 200 MB during the weekly
IFSC refresh (`MemoryMax=512M`, separate unit).

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
- **Be sensible with memory, but never give up speed for it.** Dharun, 2026-10-14: up to
  512 MB or 1 GB is fine. Keep things that are costly to rebuild (parsed fonts, tokenizer
  vocabularies, database caches); still do bulk work at build time or in the short-lived
  refresh job rather than holding whole datasets in the API's memory. Measure after a
  round and report the numbers.
- **Commits carry no AI attribution trailer.**
- **Line endings are LF everywhere** (`.gitattributes`); CRLF breaks the deploy scripts.
