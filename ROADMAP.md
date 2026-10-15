# Roadmap and status

The one place that tracks what `api.dharun.dev` serves, what is being built, what comes
later and what was turned down. Every API lists where its data comes from and how that
data is refreshed. Update this file in the same change that adds or alters an API.

Last updated: 2026-10-15

**Status words:** `Live` released · `Building` in the current round · `Planned` agreed,
later round · `Open` waiting for a decision · `Not building` turned down, with the reason.

## 1. Where things stand

| | |
|---|---|
| Released version | Every push to `main` that passes CI is released as `v1.1.<run>` and installed by the server within two minutes; `/healthz` shows the one running |
| Production | **Live since 2026-10-14** on the homelab (section 11): `v1.1.8` installed by hand, `v1.1.9` the first deploy from a push. the `Production` workflow checks every release on the live site |
| Search and AI round | Live in `v1.1.14` (section 7.1). Its launch (7.2) is on hold until the next rounds are done |
| Current round | Round 1 is built: the platform changes (section 2) and group A (section 4). Left open: three Tamil encodings (4.3) and INR reference rates (section 5) |
| Released 2026-10-14 | Satellite passes, HF propagation, sun and moon, regex tester, traceroute, Cloudflare cache purge after a deploy (section 6), licence notices in the bundle. Tested on Linux, install test included; in `v1.1.6`, with CI's race detector and vulnerability check passing |
| Waiting for Dharun | The code sandbox design (`docs/sandbox-design.md`) and the questions in section 6 |
| Round 2 | Uploads (section 6): **live 2026-10-15 in `v1.1.18`**, after the server got 10 CPUs, 5 GB, 16 GB disk and the packages. Every tool checked on the live site; speech and background removal answer 503 until the first refresh fetches the voices and the model |
| Round 3 | AI models (section 6): built 2026-10-15, models published in the `models-2026-10` release |
| Next rounds | Round 4 network tools. One round per session |
| Open, not scheduled | Disposable services and sandboxes (design only so far), the rest of the Cloudflare automation, TLS fingerprint (section 6) |

## 2. Platform work in round 1

These change how the service is built and deployed. They come before the new APIs because
the new APIs depend on them.

- [x] **Split data from the server.** A release carries the program and a *seed* copy of
      each dataset. On the server, datasets live in their own directory that a deploy
      never overwrites. See section 9.
- [x] **The server refreshes its own data.** `dharun-api-refresh.timer` runs
      `apidata refresh` hourly; each dataset decides whether it is due, is fetched,
      checked and built into a new file. The API picks it up without a restart. No GitHub
      job fetches data. Tried against the real IFSC, NSE and Wikipedia sources.
- [x] **One database file per dataset** instead of one `api.db`.
- [x] **Refresh metrics**: per dataset, which copy is in use, when its source was fetched,
      whether the last refresh worked, when it last worked, record counts.
- [x] **Grafana panels and alerts for those metrics.** The dashboard has a Data row
      (failing refreshes, missing datasets, time since each refresh against its interval,
      age of the data, copy in use); `deploy/monitoring/prometheus-alerts.yml` alerts when
      the API is down, a refresh keeps failing, or a dataset misses three intervals.
- [x] **Deploy on every push to `main`.** CI passes, a release `v1.1.<run>` is published,
      the server installs it within 2 minutes and rolls back if the health check fails.
      Automatic releases beyond the newest 15 are deleted.
- [x] **Text input by POST**, with a size limit (`MAX_BODY_KB`, default 256).
- [x] **Per-route cost and a queue for CPU-heavy routes**, so a burst waits instead of
      using more memory.
- [x] **Install test in CI.** `tools/install-test.sh` runs `deploy/install.sh` inside a
      Debian 13 container with systemd on every push: installer, updater, sandboxed
      service, PNG rendering, the refresh job and the API picking up the new copy.
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
- [x] Memory measured after the 2026-10-14 round (Linux, release bundle plus fetched
      satellite and space weather copies): 37 MB idle, 140 MB after the same burst with
      the new endpoints added, 140 MB after a traceroute. A ten-day ISS prediction takes
      66 ms of CPU, a three-round traceroute 4 s of waiting.
- [x] **Licence notices in the release bundle.** Found missing 2026-10-14. The bundle now
      carries `NOTICES`, written at build time by `tools/notices.sh`: the Go standard
      library, every module linked into `api` and `apidata` (the build stops if one has
      no licence file), go-satellite's SGP4 and the Noto fonts (OFL). resvg is shipped
      unchanged as its own program; `NOTICES` names its licence (Apache-2.0 or MIT) and
      points to its releases for the Rust crates inside it. The smoke test checks the file

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
| Working days | `/v1/working-days/...` | Holidays already held, plus Sundays and second/fourth Saturdays for banks. "Is this a working day", "next working day", "add N working days", settlement date T+1. Needs NSE *clearing* holidays too (same NSE source, different list). Only answers for years with holiday data. Maharashtra, Uttar Pradesh, Jammu and Kashmir and Kerala have several RBI offices with their own lists: by default a day off at any of them counts and names those offices; `office=` gives one office's list | Live |
| Market status | `/v1/market/status` | Trading holidays plus exchange hours. Special sessions (Muhurat, budget-day Saturdays) come from the notes in the NSE list | Live |
| Pincodes nearby | `/v1/pincode/near?lat=&lng=` | Coordinates in the pincode file, spatial index in SQLite | Live |

### 4.2 New datasets

| API | Endpoint | Data from, licence | Refresh | Memory | Status |
|---|---|---|---|---|---|
| HSN and SAC codes | `/v1/hsn/{code}`, `/v1/hsn?q=` | `HSN_SAC.xlsx` from the GST portal, about 22,000 codes with descriptions. The nomenclature is statute (Customs Tariff Act, 1975 and GST notification 11/2017), and that is what is credited; the GST Council site asks for permission to reuse its own material, so nothing else is taken from it | Automatic, monthly | Disk only | Live. Codes and descriptions only, no rates |
| Thirukkural | `/v1/thirukkural/{number}`, search, random | 1,330 couplets, public domain, from `tk120404/thirukkural` (Apache-2.0): text, transliteration, and the translations of G.U. Pope (1886) and Drew and Lazarus. Eight couplets with broken characters in that compilation are repaired on import | None; the text does not change | Disk only | Live. The modern commentaries (Mu. Varadarajan, Kalaignar, Solomon Pappaiah) and the Shuddhananda Bharati translation are left out until a release into the public domain is confirmed; Parimelazhagar needs another source |
| Phone numbers | `/v1/phone/{number}` | Format rules from the national numbering plan; mobile series to operator and circle from Wikipedia "Mobile telephone numbering in India" (CC BY-SA) | Manual, yearly | Disk only | Live. Circle and operator are the *original* allocation; number portability means the current operator can differ, and the response says so |
| Constituency | `/v1/constituency?lat=&lng=`, `?pincode=` | DataMeet community maps (CC BY 4.0): assembly constituencies and 2019 parliamentary constituencies | Manual; changes only on delimitation | Disk only | Live. Assembly boundaries pre-date the redrawing in six states, and the response says so. No member names |
| Boundaries | `/v1/geojson/states`, `/districts` | DataMeet (CC BY 4.0) | Manual | Disk only | Not building (section 8) |
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
| PII redaction | `/v1/text/redact` | Patterns with checksums: Aadhaar (Verhoeff), PAN, GSTIN, cards (Luhn), phone, email, UPI ID, vehicle number, passport, voter ID, IP address. IFSC is not redacted: it names a bank branch, not a person. No name detection; that needs a model and is round 3. Private | Live |
| Token count | `/v1/text/tokens` | OpenAI encodings (`o200k_base`, `cl100k_base`, `p50k_base`), counted by our own byte-pair merger over the published vocabularies (MIT), which holds 8 MB while in use against 22 MB for `tiktoken-go`; tests check it gives the same counts. Loaded on first use, dropped after half a day unused | Live. Claude and Llama tokenizers are not openly licensed for this, so not offered |
| Language detection | `/v1/text/language` | Script detection plus `whatlanggo` (MIT) trigram model; about 80 languages, 12 Indian. Weak on short or Romanised text, and the response carries a confidence value | Live |
| Social preview image | `/v1/og-image` | A 1200x630 card. Text is shaped here with `go-text/typesetting` (BSD) and Noto fonts (OFL 1.1, compiled into the server) for Latin and nine Indian scripts, and drawn as glyph outlines; `resvg` then only fills paths. Text and colours only, no remote images | Live. PNG needs `resvg` (in every release); SVG works anywhere |
| Code image | `/v1/code-image` | `chroma` (MIT) highlighting, drawn the same way. Up to 150 lines of 140 characters | Live. PNGs are drawn at twice the size; only the very largest at single size |

## 5. Data that changes daily

Decided 2026-10-13: the server fetches this itself (section 9).

| API | Data from | Refresh | Status |
|---|---|---|---|
| Mutual fund NAV | `NAVAll.txt` published by AMFI, about 1.5 MB. AMFI's site terms do not allow republishing; built anyway by decision (section 12), credited to AMFI | Automatic, every four hours | Live |
| INR reference rates | FBIL publishes the official rates; RBI republishes them | Automatic, daily on Mumbai working days | **Open**: FBIL's reuse terms are not confirmed. Not built until they are read |

## 6. Later rounds

### Round 2: uploads (group B), live 2026-10-15 in `v1.1.18`

Started 2026-10-14. Dharun's decisions, 2026-10-14:

- **Scope: everything in the group**, the model-based tools (speech, denoise, background
  removal) included rather than moved to round 3.
- **Uploads:** up to 10 MB per file (a PDF merge up to 25 MB in all), sent as the raw
  body or as `multipart/form-data`. No `?url=` fetching: the server does not download
  files strangers name. Files are processed in a temporary directory and deleted when
  the request ends; never stored, never logged (every upload endpoint is Private).
- **Images through libvips** (JPEG, PNG, WebP, AVIF, HEIC, TIFF, GIF), installed with apt.
- **Packaging, mixed:** apt on the server for Tesseract with the Indian language packs,
  libvips, ffmpeg and poppler; the release carries typst, piper, deep-filter and a small
  ONNX helper; model and voice files are fetched by the server once, pinned by checksum,
  like the IP database, and never ship in a release. Their endpoints answer 503 until then.
- **Memory and CPU:** first `MemoryMax=1.5G` and a 3 GB container; then, once speech
  proved too slow on two CPUs (1,000 characters of Hindi took 33 s), Dharun chose
  (2026-10-15) **10 CPUs and 5 GB for the container** and a 3,000-character speech limit.
  The service gets `MemoryMax=3G`: one model job at a time (background removal peaks at
  1 GB) beside one CPU job per CPU (each under 320 MB). Infra request.
- **Voices: all Piper voices for Indian languages**, each with its licence in the response.
  Clear: English LJSpeech and Cori (public domain), Telugu Padmavathi and Venkatesh
  (CC BY 4.0). IIT Madras IndicTTS: Hindi Rohan, Telugu Maya, Malayalam Arjun and Meera;
  the licence was read in full (2026-10-15, archived copy): royalty-free, sub-licensable,
  derived works freely distributable, its copyright notice kept, which each of these
  voices carries in its attribution. Non-commercial (CC BY-NC-SA 4.0), marked as such:
  Hindi Pratham and Priyamvada. Piper has no Tamil, Kannada, Bengali or Gujarati voice.
  **The Marathi voice was dropped**: it was trained with a newer phoneme set than the 2023
  Piper release reads, and Piper stops on it. The newer Piper is Python only (GPL-3.0);
  revisit with round 3's model process.
- **Background removal: IS-Net general use** (Apache-2.0, from `xuebinqin/DIS`).

| API | Endpoint | How | Status |
|---|---|---|---|
| OCR | `POST /v1/ocr` | Tesseract 5.5 (Apache-2.0), English and ten Indian languages; images, and PDFs up to 10 pages a request through poppler at 300 dpi | Live |
| Image info and convert | `POST /v1/image/info`, `/v1/image/convert` | libvips 8.16 (`vips`, LGPL, its own process): resize, fit inside, cover or fill, JPEG, PNG, WebP, AVIF, GIF, TIFF out, HEIC and AVIF in, metadata always stripped; info warns of a GPS position | Live |
| Background removal | `POST /v1/image/remove-background` | IS-Net at 1024 px through ONNX Runtime 1.29 (MIT) in `onnx-run`, a small cgo helper started per job; PNG, WebP or the mask | Live |
| Perceptual hash | `POST /v1/image/hash`, `/v1/image/compare` | Computed here: average, difference and DCT hashes; similar at 10 bits or fewer of 64 | Live |
| Barcode reading | `POST /v1/barcode/read` | `gozxing` (Apache-2.0): every QR code, and Data Matrix, Aztec, EAN, UPC, Code 128/39/93, ITF, Codabar. Not PDF417: gozxing has no reader for it | Live |
| PDF tools | `POST /v1/pdf/info`, `merge`, `pages`, `rotate`, `compress`, `text`, `render` | `pdfcpu` (Apache-2.0) in the server; text and page images by poppler (`pdftotext`, `pdftoppm`). Password-protected PDFs are refused | Live |
| Typst to PDF | `/v1/typst` | Typst 0.15.1 (Apache-2.0), shipped in the release; empty project folder, no packages (a proxy to nowhere), no system fonts, one thread, 20 s, 2 GB address space; the server's Noto fonts | Live |
| GST invoice | `POST /v1/invoice` | JSON in, PDF (or PNG, or the worked-out JSON) out: a Typst template here; CGST and SGST, UTGST in union territories without a legislature, or IGST from the GSTINs or place of supply; HSN summary, round-off, amount in words, UPI QR for the total | Live |
| Text to speech | `/v1/tts`, `/v1/tts/voices` | Piper (MIT, the 2023 release with espeak-ng, GPL-3.0, as its own program); 10 voices; MP3, Ogg Opus or WAV through ffmpeg | Live |
| Audio denoise | `POST /v1/audio/denoise` | DeepFilterNet 0.5.6 (MIT or Apache-2.0, one static program); ffmpeg reads any audio or video in and writes the result; first five minutes | Live |

Also in this round: sample files for every upload endpoint (`/samples/`, made by
`tools/samples/make-samples.sh`), MCP tools for invoices, Typst, speech, OCR and barcode
reading (files as base64), a downloader that gives up on a stalled transfer instead of
after ten minutes (a 179 MB model on a slow line took longer), and the docs, OpenAPI,
skill and plugin describing uploads.

### Round 3: AI models (group C), built 2026-10-15

Dharun's decisions, 2026-10-15:

- **Scope: all four parts**: embeddings and reranking, speech to text, names and
  moderation, translation and IndicXlit. The small general model (0.5B, 4-bit) stays Open:
  no model or licence chosen.
- **Runtime: Go and ONNX, no Python on the server.** `onnx-run serve` keeps a kind's
  models loaded while they are used and is stopped after `TEXT_MODEL_IDLE_MINUTES` (10);
  all such helpers together stay under `TEXT_MODEL_MEMORY_MB` (2048), stopping the idle one
  used longest ago to make room. Tokenizers (SentencePiece Unigram, BPE, WordPiece) are
  written in Go (`internal/subword`) and match the Hugging Face library token for token on
  the test texts. Speech to text is whisper.cpp, built in the release like deep-filter.
- **Models: multilingual.** multilingual-e5-small and bge-reranker-v2-m3; Whisper small,
  5-bit, answering in the request, the first five minutes heard.
- **Moderation: the Apache and MIT trio**: Detoxify multilingual (Unitary), the Hinglish
  classifier of Darel Philip (MIT, built on L3Cube HingRoBERTa, CC BY 4.0, credited) and
  Sean Benhur's Tanglish model. The Tanglish ONNX file on Hugging Face lacks its
  classification layer, so it is exported from the PyTorch weights.
- **Names:** IndicNER for Indian scripts. Dharun chose the XLM-R WikiANN model for English;
  its repository states no licence, so it was dropped and `dslim/bert-base-NER` (MIT) used
  for English instead. Name detection is opt-in (`names=true` on `/v1/text/redact`).
- **Converted files are attached to the GitHub release `models-2026-10`** of this
  repository: IndicXlit (fairseq weights loaded into a PyTorch copy of the network and
  exported), the classifiers quantised to int8, IndicTrans2's int8 ONNX files with their
  weights merged in. `tools/models/convert.py` repeats it from the original weights.
- Hosting the IndicTrans2 and IndicNER weights: the AI4Bharat repositories are gated on
  Hugging Face, so the files come from ungated copies of the same MIT weights
  (hari31416's ONNX export, abbasrazalaghari's IndicNER export), pinned by checksum.

| API | Endpoint | How | Status |
|---|---|---|---|
| Embeddings | `/v1/embed` | multilingual-e5-small, int8 (MIT): 384 numbers per text, query and passage prefixes, mean pooling, unit length; up to 64 texts | Built |
| Reranking | `/v1/rerank` | bge-reranker-v2-m3, uint8 (Apache-2.0), up to 100 documents. The int8 file gave wrong scores on the server's processor (no VNNI: int8 weights with uint8 activations overflow in ONNX Runtime); the uint8 file matches the reference. Embeddings and moderation were checked against the reference on the server and match | Live |
| Speech to text | `POST /v1/stt` | Whisper small q5_1 through whisper.cpp 1.9.4 (MIT), every CPU variant of ggml; JSON with segments, SRT, WebVTT or text; `task=translate` for English | Built |
| Moderation | `/v1/text/moderate` | Three classifiers, scores 0 to 1, `flagged` at a threshold (0.5) | Built |
| Names in redaction | `/v1/text/redact?names=true` | IndicNER (MIT) for Indian scripts, bert-base-NER (MIT) for the rest; people only | Built |
| Translation | `/v1/translate`, `/v1/translate/languages` | IndicTrans2 distilled 200M, int8 (MIT), English to and from 22 languages; Indian to Indian through English; IndicProcessor's steps rewritten in Go (`internal/indictrans`); greedy decoding | Built |
| Roman typing to a script | `/v1/transliterate/suggest` | IndicXlit en-indic v1.0 (MIT), beam search, up to eight guesses a word, 21 languages | Built |

Left for later: beam search for translation (greedy now), the indic-indic IndicTrans2 model
instead of going through English, IndicXlit's word-frequency rescoring, a Tamil Whisper
fine-tune if a licensed one appears.

### Round 4: network tools (group D)

Probe from India, TLS check, IPv6 test, logo for any domain, RDAP. Decided 2026-10-13:
build with guards (no private addresses, fixed ports, strict limits) and accept that
outgoing requests leave from the server's own connection.

Screenshot and HTML to PDF need a headless browser (200 to 500 MB each) and are the last
candidates; Typst covers most of the PDF need.

Added 2026-10-14. Traceroute was built ahead of the rest of round 4 (Dharun,
2026-10-14); the TLS fingerprint stays Open:

| API | Endpoint | How | Open points |
|---|---|---|---|
| Traceroute from India | `/v1/mtr/{host}` | MTR-style path from the server, showing how Jio, Airtel and others peer: per hop the address, PTR name, network (DB-IP), loss and best, average, worst and spread of round-trip times | **Live in `v1.1.6`.** No raw socket and no `CAP_NET_RAW`: ICMP datagram ("ping") sockets, which systemd and Docker allow every group, with routers' answers read from the error queue (`IP_RECVERR`, as `tracepath` does); Linux only, other systems answer 501. Tried in WSL against 1.1.1.1. Guards: public targets only (a name is resolved through 1.1.1.1, then the address checked), 1 to 10 rounds, 30 hops, two traces at once, 20 against the rate limit, no CPU slot taken. Hops in private or shared space (the homelab's LAN, the ISP's carrier-grade NAT) are returned without their address, so the network the server sits in is not published. The install test checks that the sandboxed service can trace |
| TLS fingerprint | `/v1/tls-fp` | The caller's JA4 TLS fingerprint and HTTP/2 settings, for scraper and bot-detection developers | **Blocked as things stand**: Cloudflare ends the TLS connection at its edge, so the server never sees the caller's handshake. Would need a hostname that bypasses Cloudflare, which exposes the home address |

### More lookup and compute (built 2026-10-14, not released)

From the brainstorm of 2026-10-14. These run inside the existing server like group A.
Dharun, 2026-10-14: build all four. Satellite passes return computed passes only, never
the orbit elements themselves; the regex tester offers RE2 and JavaScript; HF propagation
returns the indices plus a per-band estimate labelled as a rule of thumb.

| API | Endpoint | Data from, licence | Refresh | Notes | Status |
|---|---|---|---|---|---|
| Satellite passes | `/v1/sat-pass?sat=&pincode=` or `?lat=&lng=`, `/v1/satellites`, `/v1/satellites/{id}` (where it is now) | Orbit elements (TLE / GP data) from CelesTrak's `stations` and `amateur` groups, 111 satellites; dataset `satellites`, never in a release | Automatic, every four hours (CelesTrak updates every two and asks for no more than that) | CelesTrak has no redistribution clause; its usage policy only limits how often it is asked (once per two-hour update, stop on any error). The elements come from Space-Track, whose documentation allows redistribution with credit while its user agreement forbids passing on data or analysis; hence passes and positions only, never the elements (a test checks), credited to CelesTrak and USSPACECOM. SGP4 is `joshuaferrara/go-satellite` (BSD-2), copied into `internal/orbit` with its licence, because the original calls `log.Fatal` on a bad element set and truncates the epoch to whole seconds. Checked: positions match Vallado's sgp4 library to under 0.1 m (near-earth, deep space, geostationary); rise, peak and set times match Skyfield within 2 s and peak elevations within 0.05°. 5 ms per day searched; predictions refused when a satellite's elements are over 30 days old | Live in `v1.1.6` |
| HF propagation | `/v1/propagation` | NOAA Space Weather Prediction Center, US government work, public domain: 10.7 cm flux, planetary K-index (A is the mean of a day's eight a values), daily sunspot number, GOES X-ray class and latest flare, NOAA R/S/G scales, solar wind speed. Dataset `space-weather`, never in a release | Automatic, every 50 minutes so the hourly timer never skips it. A failed fetch is retried after one interval, not six hours (applies to any dataset refreshed more often than six-hourly) | Band ratings (80m-40m, 30m-20m, 17m-15m, 12m-10m, day and night) follow written rules from flux, K and X-ray, returned with the answer as `rules` and labelled a rule of thumb (`internal/spaceweather`) | Live in `v1.1.6` |
| Sun and moon | `/v1/sun-moon/{date}?pincode=` or `?lat=&lng=` | Computed with the panchang's code (`go-sampa`) | None | Sunrise, sunset, solar noon, day length, three twilights, moonrise and moonset, phase, illumination, age, the next four principal phases. The date is in the path, as for the panchang. Checked against Skyfield with JPL DE421 for Chennai: sun and twilight within 1 s, phases within 8 s, moonrise and moonset within 70 s | Live in `v1.1.6` |
| Regex tester | `/v1/regex` | Computed: `regexp` (RE2) and `regexp2` (MIT, already a dependency) in ECMAScript mode for JavaScript, with a one-second limit | None | Matches with groups and character offsets, replacement, a token by token explanation. Private. regexp2 numbers groups the .NET way and ignores the `s` flag in ECMAScript mode; both are corrected here, and a test checks it. Left: a backreference like `\1` in a pattern mixing named and unnamed groups counts the .NET way, and the response says so | Live in `v1.1.6` |

### Disposable services and sandboxes (Open)

From the brainstorm of 2026-10-14. Disposable code sandboxes are the most asked-for thing
here (AI agent developers need a safe place to run code; E2B, built on Firecracker, is
paid) and also the riskiest thing to give away. **Every one of these is Open**: they would
reopen settled decisions, listed after the table, and need a decision before any work.

| Service | Endpoint | How | Memory | Risk |
|---|---|---|---|---|
| Code sandbox, phase 1 | `/v1/sandbox/run` | Code in, output out. A Firecracker microVM restored from a snapshot (already booted with Python and Node) for each request, then destroyed. Hard limits: 30 s, 1 vCPU, 256 MB, no network | 256 MB per running sandbox, less with shared snapshot pages | High |
| Code sandbox, phase 2 | `/v1/sandbox` | Kept alive up to 5 minutes, file upload, `pip` and `npm` only through a proxy we control | Same | High |
| Code sandbox, phase 3 | | Disposable shell over SSH for learners, only with login and strict quotas | Same | High. SSH cannot pass through the Cloudflare tunnel without `cloudflared` on the user's side or Cloudflare's browser-based SSH |
| Throwaway database | `/v1/db` | One shared Postgres (later Redis); each request gets a new database and user, dropped after an hour. For CI and tests | Almost none per database | Low. Postgres has no per-database size limit, so a watcher must enforce one. **Open**: Postgres is not HTTP, so the tunnel cannot carry it to the public without `cloudflared` on the client; opening a port exposes the home address |
| Request bin | `/v1/hook` | A URL that records every request sent to it, read back through the API (webhook.site charges for API access) | Tiny | Low. What it records is strangers' data: private, short-lived, returned only as JSON, never shown as a page under the domain |
| Throwaway Kubernetes | `/v1/k8s` | A k3s cluster in a microVM for 30 minutes | 512 MB or more each | Medium |
| Try a distribution | `/v1/distro` | Arch, Alpine, Debian or Fedora for 10 minutes, in the browser or over SSH; reuses the sandbox machinery | About 128 MB each | Medium |

**How a sandbox would be hosted.** Firecracker needs KVM. Running it on the Proxmox host
is ruled out (an escape reaches the whole homelab), and inside an LXC the isolation is
weak because the host kernel is shared. The safe layout is a dedicated VM with nested
virtualization on its own isolated network with no access to the LAN or the Proxmox
interface; microVMs run inside it, so an escape from a microVM still lands in a throwaway
VM. Each microVM runs under Firecracker's jailer with cgroup limits. With a 3 GB VM that
is about 8 to 10 sandboxes at once; more requests wait in a queue, as the heavy routes do
now. Network and VM details belong in the homelab repository, not here.

**Rules a sandbox cannot run without** (free public compute is found by miners and
spammers within days):

- No internet by default; at most `pip` and `npm` through an allowlisting proxy we run
- Short lifetime (30 seconds to 5 minutes) and per-VM CPU and memory caps
- Login and a daily quota per user, plus Turnstile (see Cloudflare automation below)
- A log of which user ran which sandbox, for abuse reports
- A kill switch: one command stops every sandbox

**Settled decisions this would reopen:**

| Decision | Why the sandbox conflicts |
|---|---|
| Everything runs in one container | Needs a separate VM with nested virtualization |
| No API keys, rate limit per IP | Needs a login (GitHub) and a quota per user |
| Go | The notes suggested a FastAPI controller; a Go controller (`firecracker-go-sdk`, Apache-2.0) would keep one language |
| Memory up to 512 MB or 1 GB | Needs about 3 GB for the sandbox VM |

E2B's own infrastructure is open source and worth reading before designing this.

**Design written 2026-10-14:** `docs/sandbox-design.md` lays out phase 1 (layout, one run
step by step, limits, login and quota, monitoring, alternatives such as gVisor) and ends
with the seven decisions it needs. Nothing is built until those are made.

### Monitoring for these services (Open)

Dharun, 2026-10-14: services like these need more advanced monitoring, to watch them and
to analyse their use. On top of what the API already reports:

- **Per-sandbox resources**: CPU, memory, run time and exit reason of every microVM; pool
  size, queue length and wait time
- **Abuse signals with alerts**: CPU pinned at the cap for a whole run (mining), blocked
  outbound attempts, the same user or address hitting limits again and again
- **Audit log**: user to sandbox, a hash of the code, limits hit; kept in Loki, never
  public. Retention period to be decided
- **Kill switch and its status**: the dashboard shows whether the service is paused and why

### Cloudflare automation (Open)

Applies to the existing API as well as the services above. Dharun's choice, 2026-10-14:
extend the existing analytics token (section 11) with write permissions rather than
create a second one, knowing it sits in the API's environment file on the server.

- **Turnstile** before a sandbox starts. A Turnstile check needs a browser, so callers
  without one (agents, CLI tools) need another path, such as a GitHub login token.
  Verifying a Turnstile answer uses the widget's own secret, not the API token
- **Cache purge after a deploy**, once the new release passes its health check.
  **Built 2026-10-14:** `dharun-api-update.sh` purges by hostname (PUBLIC_ORIGIN's host
  only, never the whole zone; available on every plan since April 2025) when
  `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ZONE_ID` are in `/etc/dharun-api/env`. The token
  is passed to curl on stdin, not on the command line. A failed purge is logged and
  does not undo the deploy; a failed deploy purges nothing. Tested in
  `internal/deploytest` against a fake Cloudflare. To do on the server: add the
  Zone > Cache Purge permission to the token and put both values in the env file
- **Edge rate limits** for the heavy routes, so bursts stop at Cloudflare
- **Blocking abusers at the edge**, without over-blocking:
  - never block hosting and cloud platforms (Vercel, Netlify, Cloudflare Workers, AWS,
    Google Cloud, Azure, Render, Railway, Fly.io and the like): many apps call the API
    from them and share their outgoing addresses
  - block only after repeated, clear abuse, and let every block expire
  - for sandbox-type services, limit the user and their quota, not the address
- To check first: how many rate-limiting rules and IP lists the zone's plan allows

Related open point: the API's own per-IP allowance has the same problem. Apps on a
hosting platform that share an outgoing address share one allowance.

### Ideas not scheduled yet

From the first brainstorm, kept so they are not lost. None has a source or a decision yet.

| Idea | What it would need |
|---|---|
| Index constituents with weights (Nifty 50, Bank Nifty, Sensex) | A source that may be republished; companies already list their index memberships |
| Corporate actions (splits, bonus issues, dividends, record dates) | Same: exchange filings are public, redistribution terms are not clear |
| Festival dates (Tamil, Hindu, Islamic calendars) | Hindu and Tamil festivals follow from the panchang, planned as its next step; Islamic dates depend on moon sighting |
| State government holidays | Each state's own notification, typed in by hand like the central list |
| Colleges and universities (AISHE), school boards | Check the AISHE portal's reuse terms |
| Placeholder images | Pure logic; the drawing code for preview images would do it |
| Symbol, ISIN, BSE code and company name cross-reference | Mostly served by `/v1/companies` already; a search across all four would finish it |

## 7. Getting it used

Building it is half the work.

### 7.1 Search and AI discoverability (live 2026-10-14 in `v1.1.14`)

Goal: builders and AI assistants find the API, understand it, and can show it is safe to
use. Decisions are in section 12 (2026-10-14). Built in this repository:

| Item | Status |
|---|---|
| Licence: code AGPL-3.0; examples, guides' code, skill and plugin MIT-0; data keeps its sources' licences | Live |
| SECURITY.md, `/.well-known/security.txt` (GitHub private reporting, me@dharun.dev, dharun.dak@gmail.com), CONTRIBUTING.md (issues only, no pull requests), issue forms | Live |
| Site split into pages, plain white minimal design: short landing page, `/docs` directory, one page per API with real sample responses and curl / JavaScript / Python, `/ai`, `/guides`, `/privacy`, `/security`, `/about` | Live |
| Name "India API by dharun.dev" in titles, OpenAPI, MCP (`india-api`, was `india-data`) | Live |
| JSON-LD, share images from our own `/v1/og-image`, sitemap with dates, a Markdown copy of every page, short `llms.txt` plus `llms-full.txt`, robots.txt content signals | Live |
| Guides, first set of eight (pincode autofill, IFSC, GSTIN and PAN, working days, logos, amount in words, UPI QR, redaction before an LLM); every request in them tested | Live |
| Agent Skill `skills/india-api` and Claude Code plugin `plugins/india-api` (marketplace in this repo), generated from the catalog and checked by a test | Live |
| MCP Registry `server.json` (`io.github.dharunashokkumar/india-api`), published from CI; MCP server card at `/mcp/server-card`; `/.well-known/ai-catalog.json` | Live |
| CI: build provenance attestation on each release, OpenSSF Scorecard, Dependabot (monthly, grouped), IndexNow ping when pages change | Live |
| README rework; GitHub description, website link and topics | Live |

Done by Dharun, outside the repository:

- [x] Google Search Console property for `api.dharun.dev` (verified through dharun.dev);
      Bing Webmaster Tools imported from Google. Submit `/sitemap.xml` in both once the
      new site is live
- [x] Turn on GitHub private vulnerability reporting and Dependabot security updates
      (repository settings, Security), 2026-10-14
- [x] Submit `/sitemap.xml` in Search Console and Bing, 2026-10-14
### 7.2 Launch: listings and posts (On hold)

**On hold by Dharun's decision, 2026-10-14:** the launch waits until the remaining
roadmap rounds are built, so people arrive at the finished set of APIs. Everything is
written and ready; nothing has been posted. When the rounds are done, refresh the counts
in the texts (APIs, endpoints, tools) and go in this order, one community a day:

- [ ] Day 1, MCP directories: PulseMCP and Glama (both copy the official registry: search
      first, claim the listing), Smithery (publish by URL), mcp.so. A Postman public
      workspace from `/openapi.json` (public-apis needs its Run in Postman link). APIs.guru
- [ ] Day 2, pull requests: `punkpeye/awesome-remote-mcp-servers` (remote servers go there,
      not in awesome-mcp-servers), `public-apis/public-apis` twice (the REST API as
      "India Data" in Open Data, since names may not end in "API"; then the MCP server once
      Glama or Anthropic lists it), `ripienaar/free-for-dev`
- [ ] Day 3, r/developersIndia (check the sidebar for the showcase flair and days)
- [ ] Day 4, Show HN, Tuesday to Thursday evening IST, with the prepared first comment
- [ ] Day 5, LinkedIn, an X thread, the dev.to article
- [ ] Week 2, Anthropic's directory: plugin bundle from `plugins/india-api`, and the MCP
      server as a connector (developer portal, needs a paid claude.ai plan); r/golang (the
      technical post), r/mcp, r/ClaudeAI, a day apart
- [ ] Later, Product Hunt; then answer real questions on Stack Overflow and Reddit, saying
      that you built it

The texts, steps and checklist are in the "India API Launch Kit" page in Dharun's claude.ai
artifacts (private), with a copy at `.cache/launch-kit.html` on his machine (git-ignored).
Prepared for the directories already: the plugin README and content version, read-only
annotations on every MCP tool, `claude plugin validate --strict` passing.

From the first brainstorm, not done yet:

- [ ] List it: `public-apis/public-apis`, free-for.dev, the MCP registries, RapidAPI's free tier
- [ ] Thin client packages on PyPI and npm, which also show up in code search
- [ ] A public status page (Uptime Kuma in its own container)
- [ ] Launch posts with a three-line example: r/developersIndia, r/TamilnaduFoss, Show HN, LinkedIn
- [ ] Answer real questions (Reddit, Stack Overflow) with a working `curl` line
- [ ] Put the tunnel's cache in front: almost every answer carries a long `Cache-Control`,
      so cached answers keep working through a short outage

Already in place for AI tools: `/openapi.json`, `/llms.txt`, the MCP server at `/mcp`.
Never break `/v1`.

## 8. Not building

| Idea | Reason |
|---|---|
| PAN, Aadhaar or vehicle owner lookup | Personal data |
| IRCTC PNR status | The source prohibits scraping |
| GSTIN verification against the GST portal | Captcha-protected government site |
| Live or delayed share prices | Licensed by the exchanges |
| Open web search or scraping | Uses the server's connection to scrape third parties; search engines block it |
| State and district boundary files | Publishing a map of India whose boundary differs from the Survey of India's is an offence; DataMeet already offers the files free |

Turned down on 2026-10-14. The rule behind them: nothing that lets strangers publish
content under `dharun.dev`, or use the server's address as their own, is built. Anything
that only returns data to the caller can be.

| Idea | Reason |
|---|---|
| Temporary email inbox | Throwaway signups under the domain get it blocklisted |
| Dynamic DNS subdomains (`*.dyn.dharun.dev`) | Strangers' pages under the domain; phishing |
| Scheduled URL calls and uptime pings for others | Sends strangers' traffic from the server's address on a timer |
| Hosted mock APIs from uploaded specs | Strangers' content served at a live URL under the domain |
| Tunnels (ngrok-style) | Phishing; the domain gets flagged by Google Safe Browsing and Cloudflare |
| Temporary file sharing | Malware hosting |
| Free VPN or proxy | Abuse traffic from the server's address |
| Public speed test server | Uses up the home connection |

## 9. How data reaches the server

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
| Satellite orbit elements | celestrak.org GP data, `stations` and `amateur` groups | Every four hours | Server (never in a release) |
| Space weather | services.swpc.noaa.gov | Every 50 minutes; a failure is retried after one interval | Server (never in a release) |
| Pincodes | data.gov.in file saved by hand | When Dharun runs it | Repository seed |
| Central government holidays | DoPT memorandum, typed in | Yearly, around July | Repository seed |
| Logos and company details | TradingView scraper | When Dharun runs it | Repository seed |
| Thirukkural, phone series, constituencies | See section 4 | Rarely | Repository seed |

## 10. Questions settled on 2026-10-13

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

## 11. First production deploy

Live since 2026-10-14 (set up from the private homelab repository). Steps:

- [x] Create the container: Debian 13, unprivileged, with `nesting=1` (the service's
      systemd sandboxing needs its own mount namespaces), 2 GB of memory (the API's
      `MemoryMax` is 768 MB, the refresh job's 512 MB), a few GB of disk for the datasets
- [x] Run `deploy/install.sh`. In an unprivileged container it opens ICMP sockets to the
      service's group (`/etc/sysctl.d/60-dharun-api.conf`), which the traceroute needs;
      the install test runs it that way
- [x] Put in `/etc/dharun-api/env`: `CLOUDFLARE_API_TOKEN` (with Zone > Cache Purge
      added to it) and `CLOUDFLARE_ZONE_ID`, for the purge after each deploy
- [x] Wait for the first refresh run (the timer starts it within minutes of install):
      the IP database, satellites and space weather have no seed, and their endpoints
      answer 503 until then. `apidata status` shows each copy
- [x] Try `/v1/mtr/1.1.1.1` from outside: hops on the LAN and the ISP's shared address
      range come back without their address
- [x] Tunnel route for `api.dharun.dev` to the service port
- [x] Prometheus scrape job, alert rules and Grafana dashboard from `deploy/monitoring/`
- [x] **Where requests come from, and unique visitors** (2026-10-15). Each request's
      address is looked up in the IP database (Cloudflare's country header when it has no
      answer) and then dropped. Metrics: requests by country and, for India, by state;
      unique visitors per day, per month and per country and day (India time), counted
      with HyperLogLog sketches over addresses hashed with a key replaced each period.
      The sketches and current keys are kept in `STATE_DIR/visitors.json` (mode 600) so a
      deploy does not reset the count. The request log gains country, region, city with
      its coordinates, AS number and network; Grafana reads cities and networks from Loki.
      New "Visitors and where they come from" row on the dashboard; `/privacy` and the FAQ
      say what is logged and counted. Dashboard install: request in the homelab.
- [ ] **Count what Cloudflare serves from its cache.** The server only sees requests that
      reach it, so once a Cache Rule is added the dashboard undercounts. Asked for on
      2026-10-14: show the full picture in Grafana. Plan: the API asks Cloudflare's
      analytics API once a minute for requests to `api.dharun.dev` by cache status and
      publishes them on its own metrics port, then the dashboard gets panels for requests
      at the edge, cached against passed to the server, and the hit ratio. No extra
      container. Dharun made a separate read-only token for this (Zone, Analytics, Read)
      and put it with the zone ID in `.cache/cloudflare.env`, which git ignores; on the
      server both go in `/etc/dharun-api/env`. To check first: whether the plan allows
      statistics per hostname and cache status. Not started. Without a Cache Rule
      nothing is cached and the numbers already match.
- [x] Ship logs; add an uptime monitor on `/healthz`
- [x] Firewall: service port from the tunnel only, metrics port from Prometheus only
- [x] Check the first automatic refresh run and the first push-triggered deploy (first
      refresh run 2026-10-14: IP database, NAV, satellites and space weather fetched;
      first push deploy 1.1.8 to 1.1.9 the same day, checked live by CI)
- [x] CI waits for each release to be live on the public site and checks it there
      (`Production` workflow, added 2026-10-14)

## 12. Decisions

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
| 2026-10-14 | Brainstorm of new ideas recorded: lookup and compute ideas Open; traceroute and TLS fingerprint join round 4, a small LLM joins round 3; sandboxes and disposable services Open, with the decisions they would reopen listed |
| 2026-10-14 | Security first. Temporary inbox, dynamic DNS, URL pinger, hosted mocks, tunnels, file sharing, VPN and speed test are not built |
| 2026-10-14 | Sandbox-type services get their own monitoring: resources, abuse alerts, audit log, kill switch |
| 2026-10-14 | The Cloudflare analytics token is extended with write permissions (Turnstile, cache purge, edge rate limits, edge blocking). Blocking must not over-block and never targets hosting platforms such as Vercel |
| 2026-10-14 | Built now: satellite passes, HF propagation, sun and moon, regex tester, traceroute (ahead of round 4), cache purge after a deploy; a design write-up for the sandbox |
| 2026-10-14 | Satellite passes publish computed results only, never the orbit elements, credited to CelesTrak and USSPACECOM |
| 2026-10-14 | Regex tester: RE2 and JavaScript. HF propagation: NOAA's indices plus a per-band rule of thumb, labelled as one |
| 2026-10-14 | Licence: code AGPL-3.0; code examples, guides' code, the skill and the plugin MIT-0. Contributions by issue only, no outside pull requests |
| 2026-10-14 | Name "India API by dharun.dev"; MCP server and skill `india-api` |
| 2026-10-14 | Website stays on api.dharun.dev, split into pages; plain white minimal design; guides live under `/guides`. The trust pages say nothing about hosting |
| 2026-10-14 | AI packaging: Agent Skill, Claude Code plugin (both in this repository), MCP Registry entry, install links. Every API change updates them in the same change |
| 2026-10-14 | Trust signals: security policy and security.txt, privacy page, release provenance, OpenSSF Scorecard, Dependabot monthly |
| 2026-10-14 | The launch (listings, pull requests, posts) waits until the remaining roadmap rounds are built |
| 2026-10-14 | Every API change updates every description of it in the same change: checklist in CLAUDE.md, "Keeping every description in step" |
| 2026-10-14 | Round 2 (uploads) builds every tool, model-based ones included: 10 MB uploads, body or multipart, no URL fetching, nothing stored or logged; libvips for images; apt packages plus programs in the release plus server-fetched models; all Piper voices for Indian languages, licence named per voice; IS-Net for background removal |
| 2026-10-15 | The API container gets 10 CPUs and 5 GB (the host has cores to spare), the service `MemoryMax=3G`; speech keeps a 3,000-character limit. Give more hardware before cutting a feature |
| 2026-10-15 | The GPL source of the espeak-ng inside Piper is attached to every release (`espeak-ng-source.tar.gz`) rather than offered on request |
| 2026-10-15 | Infrastructure first, then the round 2 push: the release waits until the server has the packages, memory and disk |
| 2026-10-15 | Monitoring shows where requests come from (country, Indian state, city, network) and unique visitors per day, month and country per day. Addresses are still never written; the visitor sketches and their period keys are saved so deploys do not reset the counts |
