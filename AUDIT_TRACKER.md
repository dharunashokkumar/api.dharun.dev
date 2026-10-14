# Audit tracker

What is left from the October 2026 code review. The fixes for everything else went in
with the commit "Fix the audit findings". Tick an item when it is done or decided
against; when nothing is left, delete this file.

## Left open on purpose

Each of these is real but needs an unusual situation to matter, or changes a contract.

- [ ] **A07 Readiness in the health check.** `/healthz` says `ok` even when datasets are
  missing. Not done because the updater reads `/healthz`, so changing what it means is a
  deploy-contract decision, and CI already smoke-tests the exact release bundle. Done
  when: a release missing a seed dataset fails the health check and is rolled back.
- [ ] **A11 Rollback keeps compatible data.** A refresh after a schema bump deletes the
  server copies the previous release could read. Only matters for a manual rollback days
  later (an automatic rollback happens before any refresh). Needs a decision: keep one
  copy per schema, or accept that a rollback serves the seed. Done when the choice is
  made and tested.
- [ ] **A12 One refresh at a time.** Running `apidata refresh` by hand while the timer
  runs can mix up their temporary files. Done when: a lock on `DATA_DIR` makes the second
  run wait or stop with a clear message.
- [ ] **A08 Stop work for callers who left.** Database queries and render queues do not
  stop when a client disconnects. Bounded today by the heavy-request slots and the
  45-second write timeout; fixing it means passing a context through every handler.
  Worth doing only if measurements show abandoned work.
- [ ] **A09 One dataset version per response.** During a reload a response can mix rows
  from one version with source details from the next. Happens only in the instant a
  dataset is swapped, about once per refresh.
- [ ] **A05 Exact coordinates and plates in the request log.** `/v1/pincode/near` and
  `/v1/constituency` log full coordinates; `/v1/rto` logs a full vehicle number. The log
  stays on the server. Needs a decision on what the log may hold. Done when the logged
  path rounds coordinates and keeps only the RTO code.
- [ ] **A24 Importers that trust malformed files.** The XLSX and shapefile readers can
  panic on a broken file. They only read sources we choose, and a panic fails that one
  refresh. Done when malformed inputs give an error, with a fuzz test.
- [ ] **A29 POST bodies in openapi.json.** Options such as `to` for transliteration can
  go in the JSON body, but the generated schema only lists them as query parameters.
  Documentation only; no wrong answers.
- [ ] **A27 MCP header checks.** The server does not check `Origin` or
  `MCP-Protocol-Version`. The Origin check protects servers on localhost from DNS
  rebinding, which does not apply to a public keyless server; a strict version check
  could turn away clients that work today. Revisit if the MCP spec makes it mandatory.
- [ ] **A28 Oversized MCP bodies.** The rest of a body over 64 KB is read and thrown away
  so the client gets its 413. On purpose; capped by the 15-second read timeout.

## Smaller ideas worth a look

From the review's suggestion list; none is a known bug.

- [ ] Render a PNG logo once when several requests for it arrive together (`logos.go`).
- [ ] Cap the number of words in a search query (`/v1/mutual-funds`, `/v1/ifsc`, ...).
- [ ] A minimum record count for a dataset's first refresh, when there is no copy to
  compare with (matters for the IP database).
- [ ] Split `apidata refresh -force` into "fetch now" and "accept a suspicious result".
- [ ] `/v1/dns`: `consistent` is true when every resolver failed; say "unknown" instead.
- [ ] Count MCP tool calls that failed inside a 200 response in the metrics.

## Decisions for Dharun

- [ ] **Licence.** The repository has no LICENSE file, so nobody may legally reuse the
  code. Skipped for now (2026-10-14).
- [ ] **New API directions** suggested by the review, none agreed: changes between
  dataset versions (which records a refresh added, removed or changed), a whole-address
  check built on pincodes, and braille output for Indian scripts. Anything chosen goes
  to `ROADMAP.md` with its source and refresh method first.
