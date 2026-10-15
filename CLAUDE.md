# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`api.dharun.dev`, named **India API**: free, open source APIs for Indian developers (company
logos, pincodes, IFSC, RTO codes, holidays, number to words, format checks, GST invoices,
OCR, PDF, image and speech tools, and more being added). One Go service owns the whole hostname; every API is a path under `/v1`, and the
same service serves the website, the guides and the files for AI agents. No API keys,
rate limit per IP. Code AGPL-3.0-only; examples, guides' code, skill and plugin MIT-0.

`ROADMAP.md` tracks what is implemented, what is planned and what was rejected, with the
data source and refresh method for each API. Read it before adding an API, and update it
in the same change.

Production (where it runs, how deploys reach it, how to ask for infrastructure changes):
read the git-ignored `INFRA.local.md` first. See "Deploy" below.

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
bash tools/smoke-test.sh dist/dharun-api-linux-amd64.tar.gz   # route checks: tools/smoke-checks.sh
bash tools/install-test.sh dist            # needs Docker: installs on a fresh Debian 13 with systemd
```

resvg has no Windows build. Without `RESVG` the PNG tests skip themselves and the image
endpoints are tested as SVG only, so run the suite on Linux before trusting a change to
rendering.

The file tools run outside programs. On Windows most are missing and their tests skip;
on Linux install them as CI does (apt: tesseract and its language packs, libvips-tools,
libheif-plugin-aomenc, ffmpeg, poppler-utils; then `bash tools/fetch-tools.sh <dir>` for
typst, piper, whisper.cpp (needs cmake and g++), deep-filter and onnx-run, and put `<dir>`,
`<dir>/piper` and `<dir>/whisper` on `PATH`).
`REQUIRE_TOOLS=1` turns a missing program into a failure. The speech, background
removal and AI model tests also need the server-only model datasets: fetch them once
with `go run ./cmd/apidata refresh -live <dir> voices matting embeddings reranker whisper
names moderation translation xlit` (about 3.7 GB) and set `MODELS_DIR=<dir>`;
`REQUIRE_MODELS=1` makes their absence a failure. The sample files the
docs and tests post are made by `tools/samples/make-samples.sh`.

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
request charges against the per-IP allowance (`costFree`, `costLight`, `costHeavy`,
`costProbe`). A `costHeavy` route also takes one of `HEAVY_CONCURRENCY` slots, so bursts
queue instead of using more memory. `costProbe` (the traceroute) waits on the network for
seconds, so it takes no CPU slot and has its own limit of two at a time. Probes from the
server never target private or reserved addresses (`reserved()` in `ip.go`), and never
publish the private addresses of the network the server sits in.

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

`catalog.go` describes every endpoint once, and each group names its `Category` (one of
`siteCategories` in `site.go`). Everything people and agents read is generated from it:

- The website (`site.go`, `site_layout.go`, `site_pages.go`): a short landing page, `/docs`,
  one page per group with a real sample response (`samples.go` runs the `Example` through
  the router in memory; endpoints marked `Live` are never run), `/ai`, `/guides`, and
  `/privacy`, `/security`, `/about`, which are written once as Markdown in `templates/`
  and rendered. Plain white, system fonts, nothing loaded from other hosts.
- A Markdown form of every page (`.md` address or `Accept: text/markdown`), `/llms.txt`,
  `/llms-full.txt`, `/openapi.json`, `/sitemap.xml`, `/robots.txt` (content signals) and
  the discovery files: `/.well-known/security.txt`, `api-catalog`, `ai-catalog.json`,
  the MCP server card (`/mcp/server-card`) and the Agent Skills index (`site_meta.go`).
- **One** Agent Skill, **one** Claude Code plugin and **one** MCP Registry entry for the
  whole API, never one per API (`skill.go`, `templates/skill.md`). They are committed as
  `skills/india-api/`, `plugins/india-api/`, `.claude-plugin/marketplace.json` and
  `server.json`; `TestGeneratedFilesAreCurrent` fails when they differ from the catalog.
  Regenerate with `go test ./internal/api -run TestGeneratedFilesAreCurrent -update`.
- Guides are Markdown files in `internal/api/guides/` with a small header; every API
  address in them is requested by `TestGuides`. Their code is MIT-0.

`TestEveryDocumentedExampleWorks` requests every `Example` in the catalog, and every
`GET /v1/...` route must be in the catalog. CI publishes `server.json` to the MCP Registry
and pings IndexNow from `production.yml` after a release is live.

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
`internal/panchang` (almanac and the plain sun and moon day, on `go-sampa`),
`internal/translit` (scripts and Roman schemes), `internal/tamilfont` (8-bit Tamil
encodings; tables are written from the published standards, never copied from GPL
converters), `internal/textkit` (chunking, redaction, token counts with a vocabulary that
is loaded on first use and kept while it is in use, language detection), `internal/fake`,
`internal/words`, `internal/validate`, `internal/regextest` (RE2 and JavaScript via
regexp2, with fixes for regexp2's group numbering and `s` flag), `internal/spaceweather`
(the HF band rule of thumb), `internal/orbit` (SGP4 copied from go-satellite with its
licence and fixed, pass search; checked against Vallado's sgp4 and Skyfield) and
`internal/trace` (traceroute over unprivileged ICMP sockets and the error queue, Linux
only).

Two datasets exist only as server copies besides the IP database: `satellites`
(CelesTrak, four-hourly) and `space-weather` (NOAA, every 50 minutes). Tests build them
from `internal/api/testdata/data/` into the live directory in `TestMain`; the satellite
fixture's epochs are moved to the time of the test run. Never return a satellite's orbit
elements, only what is computed from them.

### Files in, files out (`upload.go` and the round 2 files)

Endpoints that take files register through `s.handleUpload` with an `uploadRoute`: the
whole upload (raw body or multipart, 10 MB, 25 MB for a merge) is read before the request
takes a slot, so a slow upload holds none, and the connection deadlines are moved out
for it. CPU work waits for `s.heavy`; anything that runs a neural network (speech,
denoise, background removal) waits for `s.models` (`MODEL_CONCURRENCY`, default one).
Costs: `costUpload` 15, `costModel` 30. Every upload endpoint is `Private`. Files live in
memory or a per-request `workspace()` deleted when the answer is sent, and answers are
`private, no-store` (`sendFile`).

Outside programs run through `s.run(ctx, job{...})`: found once by `programs` (an env
override such as `TYPST=`, next to the binary, then `PATH`), started with a minimal
environment (never the server's own, which holds the Cloudflare token), a timeout, and
stderr kept to 8 KB. On failure only the program and exit status are logged, never what
it printed, which can quote the upload. A missing program answers 503 `not_available`.
From the release: typst (run with an empty root, no packages, a proxy to nowhere, one
thread, `prlimit` on memory), piper (in `piper/`, with espeak-ng), deep-filter, and
`onnx-run` (`cmd/onnx-run`, cgo, loads `libonnxruntime.so` beside it; one model, one
float32 tensor in, the first output out). From Debian: tesseract, vips, vipsheader,
ffmpeg, pdftotext, pdftoppm. pdfcpu (PDF info, merge, pages, rotate, compress), gozxing
(barcode reading) and the perceptual hashes run in the server itself.

The model files are server-only datasets like the IP database: `voices` (10 Piper voices,
each with its own licence in `dataset.Voices`) and `matting` (IS-Net), pinned by revision
and SHA-256, checked monthly and downloaded only when the pinned set changes.

In the catalog an endpoint that takes files has `Upload` (what it accepts, a sample file
under `internal/api/samples/` that the docs and `TestEveryDocumentedExampleWorks` post)
and is POST only; `Output` marks one that returns a PDF or audio.

### AI models (round 3: `textmodels.go`, `onnx-run serve`, `internal/subword`)

Embeddings, reranking, moderation, name detection (`/v1/text/redact?names=true`),
translation and Roman-typing transliteration run in `onnx-run serve` helpers, one per
kind (dataset), started on first use and stopped after `TEXT_MODEL_IDLE_MINUTES` (10).
Together they stay under `TEXT_MODEL_MEMORY_MB` (2048): starting one that does not fit
stops the idle one used longest ago. They speak `internal/onnxwire` over stdin and stdout;
a generation (IndicTrans2 greedy with its cache, IndicXlit beam search) runs inside the
helper. Tokenizers are Go (`internal/subword`, read from tokenizer.json, held once per
file content); `TOKENIZER_REFERENCE` runs its comparison with the Hugging Face library.
IndicTrans2's pre- and post-processing is `internal/indictrans`. Speech to text is
whisper.cpp (`whisper/` in the release) as a model job.

The model datasets (`internal/dataset/aimodels.go`) are server-only like the voices.
Files that needed converting were made by `tools/models/convert.py` and attached to the
GitHub release `models-2026-10` (never pruned: only `v1.1.N` tags are). Changing a model
means a new release name and new checksums. `DOWNLOAD_MIRROR=<dir>` (host/path layout)
makes `apidata refresh` copy files instead of downloading them, for tests on a slow line.

### MCP

`mcp.go` is a stateless MCP server (Streamable HTTP) at `POST /mcp`. A tool does not
have its own logic: `callTool` builds a REST path and runs it through the same mux in
memory, so a tool returns exactly what the REST endpoint returns. A tool with `send`
POSTs a body instead (a document, or a file passed as base64, up to 5 MB), and a file
that comes back is returned as an image, audio or embedded resource block
(`mcp_files.go`).

### Adding an API

1. Agree the data source, licence and refresh method first (see the rules below), and
   record them in `ROADMAP.md`.
2. Data, if any: a `Spec` in its own file in `internal/dataset` with `Build`, and
   `Import` if a program can fetch it; seed files under `data/`; a record in
   `data/sources.json`.
3. A handler in `internal/api` in its own file, registered in `registerMore`.
4. A catalog entry with a real `Example` and a `Category`; mark it `Live` if its answer
   depends on the network or the caller, `Private` if its input may be personal. An MCP
   tool if an agent would call it; update the MCP `instructions` in `mcp.go` if the API
   is a new kind of data.
5. Fixture rows in `internal/api/testdata/` (real rows cut from the full data) and tests.
   A test file adds its MCP tool's arguments to `mcpTestCalls` in `init`. Tests never
   touch the network: DNS goes through `fakeDNS`, and the IP database is a fixture built
   in `TestMain`.
6. A line in `tools/smoke-checks.sh` and the README table (with its docs link).
7. Regenerate the skill, plugin and registry entry (`-update`, above) and commit them.
   Mention the API in the skill's `description` (`skill.go`) and the home page if it is a
   headline API. A guide for a common task helps both search and AI answers.
8. Go through the checklist below.

### Keeping every description in step (always, in the same change)

Dharun's rule (2026-10-14): **every time an API is added, removed, renamed or changed**
(new parameter, new field, different data source or limit), everything that describes it
is updated in the same change, never "later". Most of it is generated from the catalog,
but not all. Check each line:

| What | Where | How |
|---|---|---|
| API pages, `/docs`, sitemap, `llms.txt`, `llms-full.txt`, OpenAPI, `/v1` index, Markdown pages | `catalog.go` | Generated; just keep the catalog right |
| Agent Skill, Claude Code plugin (README, version), MCP Registry `server.json` | `skill.go`, `templates/` | `go test ./internal/api -run TestGeneratedFilesAreCurrent -update`, commit the files |
| MCP tool, its test arguments, the server `instructions`, open-world or changing hints | `mcp.go`, the API's file | By hand. Removing an API removes its tool and `mcpTestCalls` entry |
| Skill `description` and `skillDescription`, MCP `registryDescription` | `skill.go`, `site_meta.go` | By hand when a headline API comes or goes (limits: 1024 and 100 characters) |
| Home page lead, FAQ, `siteSummary`, `llms` intro, AI page "Try asking" | `site.go`, `site_pages.go` | By hand when a headline API comes or goes, or a fact in them changes (rate limit, counts) |
| Guides that call the API | `internal/api/guides/` | `TestGuides` fails on a removed or broken address; rewrite or delete the guide |
| Privacy page | `templates/privacy.md` | The `Private` list is generated; by hand if the API sends input to a third party or logs something new |
| About page and README data sources | `data/sources.json`, `README.md` | The about table is generated; the README table and data list by hand |
| README API table and the counts in it | `README.md` | By hand, with the docs link |
| Smoke checks | `tools/smoke-checks.sh` | Add a line for a new API; remove the line for a removed one |
| Roadmap | `ROADMAP.md` | Status, source, refresh, decisions |
| GitHub description and topics | `gh repo edit` | Only when a headline API comes or goes; ask Dharun first, it is public text |

Before committing: `go test ./...` passes (it covers the generated files, the guides, every
documented example and every page), and `claude plugin validate ./plugins/india-api --strict`
passes.

### Deploy

Pull-based: nothing connects in to the server, and GitHub never fetches data.

**Read `INFRA.local.md` before any deploy, server or infrastructure question.** It is
git-ignored and exists only on Dharun's machine: where production runs, what it depends on
from this repo, how to check it, and how to ask for an infrastructure change (ask Dharun,
then write the request in the private homelab repo; infrastructure is never changed from
here). Releases replace only the bundle, so a change under `deploy/` needs the server-side
install to be re-run: that is an infrastructure request too.

1. Every push to `main` runs CI. When it passes, the `publish` job releases it as
   `v1.1.<run number>` with `dharun-api-linux-amd64.tar.gz` (api, apidata, resvg, seed/,
   NOTICES, VERSION), its `.sha256` and `VERSION`, and prunes automatic releases beyond
   15. `NOTICES` is written by `tools/notices.sh`, which stops the build when a linked
   module has no licence file.
2. On the server, `dharun-api-update.timer` (every 2 min) runs
   `deploy/dharun-api-update.sh`: download, verify checksum, switch the
   `/opt/dharun-api/current` symlink, restart, check `/healthz` reports the new version,
   roll back and mark the release failed if not. `internal/deploytest` tests it end to
   end against a fake release server.
3. `dharun-api-refresh.timer` (hourly) runs `apidata refresh` as the service user; each
   dataset decides whether it is due.
4. The workflow `production.yml` (`tools/prod-check.sh`) starts when CI on main has
   finished, so it never holds CI's queue. It waits for the public `/healthz`
   to report the release, then runs `tools/smoke-checks.sh` (the route list shared with
   the smoke test) against the live site. A rollback shows as a failed run. Add a route to
   `smoke-checks.sh` only when the live server answers it the same way as a fresh bundle.

Measured on Linux: the API idles at 34 MB and peaked at 144 MB under a mixed burst over
every kind of endpoint; resvg adds 11 to 75 MB per image while it runs. The file tools
(2026-10-15, API plus the program, pinned to 10 CPUs as on the server): OCR 80 to 170 MB,
image conversion of a 12 MP photo 150 to 210 MB, PDF tools 50 to 100 MB, speech up to
320 MB (3,000 characters of Hindi in 14 s), denoise 145 MB, background removal 1 GB
(5 s). After a large picture the API hands memory back (`debug.FreeOSMemory`) and idles
at about 40 MB again. Limits: `MemoryMax=3G`, `GOMEMLIMIT=384MiB`, container 10 CPUs
and 5 GB (Dharun, 2026-10-15). The refresh job peaks at 200 MB during the weekly IFSC
refresh (`MemoryMax=512M`, separate unit).

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
- **Licence and contributions** (2026-10-14): code AGPL-3.0-only; code examples, guides,
  skill and plugin MIT-0; data keeps its source's licence. Issues only: outside pull
  requests are not accepted (keeps relicensing possible).
- **The trust pages say nothing about hosting** (no server, home or uptime statements).
  Facts in `/privacy` must stay true: check `logRequest` before changing it.
- **Line endings are LF everywhere** (`.gitattributes`); CRLF breaks the deploy scripts.
