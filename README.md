# India API

**Free, open source APIs for Indian data.** Pincodes, IFSC and bank branches, GSTIN and PAN
checks, bank and market holidays, RTO codes, company logos, UPI QR codes, numbers in lakh
and crore, GST invoices as PDF, OCR in Indian languages, PDF and image tools, translation,
speech to text and text to speech, money calculators and more: 48 APIs behind one consistent JSON shape. No API key
needed for most endpoints (a free key raises the limits), open CORS, free tool pages, a playground, and an MCP server
so AI assistants can use it too.

[![CI](https://github.com/dharunashokkumar/api.dharun.dev/actions/workflows/ci.yml/badge.svg)](https://github.com/dharunashokkumar/api.dharun.dev/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/dharunashokkumar/api.dharun.dev/badge)](https://scorecard.dev/viewer/?uri=github.com/dharunashokkumar/api.dharun.dev)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/15520/badge)](https://www.bestpractices.dev/projects/15520)
[![Licence: AGPL-3.0](https://img.shields.io/badge/licence-AGPL--3.0-blue)](LICENSE)

**Documentation: [api.dharun.dev](https://api.dharun.dev)** · [API reference](https://api.dharun.dev/docs) · [Playground](https://api.dharun.dev/play) · [Free tools](https://api.dharun.dev/tools) · [API keys](https://api.dharun.dev/keys) · [Guides](https://api.dharun.dev/guides) · [Use with AI](https://api.dharun.dev/ai) · [OpenAPI](https://api.dharun.dev/openapi.json) · [llms.txt](https://api.dharun.dev/llms.txt)

```bash
curl https://api.dharun.dev/v1/pincode/637001
```

```json
{
  "data": { "pincode": "637001", "district": "Namakkal", "stateCode": "TN", "state": "Tamil Nadu", "offices": [ ... ] },
  "meta": { "source": { "name": "...", "licence": "...", "attribution": "..." } }
}
```

## APIs

Each name links to its page, with every parameter, a real response and code in curl,
JavaScript and Python.

| API | Example | What you get |
|---|---|---|
| [Company logos](https://api.dharun.dev/docs/logos) | `/v1/logos/TCS`, `/v1/companies/RELIANCE` | SVG or PNG logo of an NSE or BSE listed company, REITs and InvITs included; an unknown symbol gets an initials tile, never a broken image, after the brand, global stock and asset sets are tried; sector, ISIN, market cap, indexes; old tickers still work |
| [Brand, stock and asset logos](https://api.dharun.dev/docs/world-logos) | `/v1/brand-logos/phonepe`, `/v1/stock-logos/AAPL`, `/v1/asset-logos/BTC` | Logos of unlisted Indian brands (payments, fintech, insurers, mutual fund houses, apps, government services), global stocks (US and nine major markets) and crypto, indices, ETFs and commodities; `/v1/logos` falls through to them |
| [Pincodes](https://api.dharun.dev/docs/pincode) | `/v1/pincode/637001`, `/v1/pincode/near?lat=&lng=` | Post offices, district, state, coordinates; pincodes near a point |
| [IFSC and banks](https://api.dharun.dev/docs/ifsc) | `/v1/ifsc/SBIN0000001`, `/v1/banks?q=gramin` | Bank, branch, address, MICR, payment systems; banks and their codes by name |
| [RTO codes](https://api.dharun.dev/docs/rto) | `/v1/rto/TN-28` | Transport office for a vehicle registration |
| [Holidays](https://api.dharun.dev/docs/holidays) | `/v1/holidays/2026?type=market`, `/v1/holidays/2026.ics?state=TN` | Bank, central government and stock market holidays; a calendar feed to subscribe to |
| [Today in India](https://api.dharun.dev/docs/today) | `/v1/today?state=TN&pincode=600001` | Date and time in IST, financial year and quarter, working days, today's holidays, market status, panchang in brief |
| [Money calculators](https://api.dharun.dev/docs/calculators) | `/v1/calc/income-tax?income=1500000`, `/v1/calc/emi?amount=1000000&rate=8.5&years=20` | SIP, lump sum, EMI with schedule, GST split, income tax under both regimes, HRA, gratuity, PPF, capital gains, share trade charges; each names its rule |
| [Working days](https://api.dharun.dev/docs/working-days) | `/v1/working-days/2026-01-16?calendar=bank&state=TN` | Working day or not, the next one, N working days ahead, T+1 settlement dates; whether NSE is open |
| [HSN and SAC](https://api.dharun.dev/docs/hsn) | `/v1/hsn/0902`, `/v1/hsn?q=tea` | Tariff codes with descriptions and their chapter |
| [Mutual funds](https://api.dharun.dev/docs/mutual-funds) | `/v1/mutual-funds/122639` | Latest NAV of a scheme, search by name or fund house |
| [Railway stations](https://api.dharun.dev/docs/stations) | `/v1/stations/MAS` | Station name, zone, state |
| [Airports](https://api.dharun.dev/docs/airports) | `/v1/airports/MAA` | IATA and ICAO codes, city, coordinates |
| [Phone numbers](https://api.dharun.dev/docs/phone) | `/v1/phone/9500112345` | Valid or not, mobile or landline, original operator and circle |
| [Constituencies](https://api.dharun.dev/docs/constituency) | `/v1/constituency?pincode=637001` | Assembly and parliamentary constituency for a place |
| [Panchang](https://api.dharun.dev/docs/panchang) | `/v1/panchang/2026-01-14?pincode=600001` | Tithi, nakshatra, yoga, karana, sunrise, Rahu kalam, Tamil month |
| [Sun and moon](https://api.dharun.dev/docs/sun-moon) | `/v1/sun-moon/today?pincode=600001` | Sunrise, sunset, twilight, moonrise, moon phase and illumination |
| [Satellite passes](https://api.dharun.dev/docs/satellites) | `/v1/sat-pass?sat=ISS&pincode=600001` | When the ISS or an amateur radio satellite passes over, and whether it can be seen |
| [HF propagation](https://api.dharun.dev/docs/propagation) | `/v1/propagation` | Solar flux, K and A index, NOAA scales, and a day and night rating per band |
| [Thirukkural](https://api.dharun.dev/docs/thirukkural) | `/v1/thirukkural/1` | Couplet, transliteration, two English translations; search and random |
| [Transliteration](https://api.dharun.dev/docs/transliterate) | `/v1/transliterate?to=tamil&text=...`, `/v1/transliterate/suggest?lang=ta&text=vanakkam` | Between nine Indian scripts, ISO 15919, IAST and ASCII; Roman typing to 21 Indian scripts with a model |
| [Translation](https://api.dharun.dev/docs/translate) | `/v1/translate?from=en&to=ta&text=...` | Between English and 22 Indian languages with IndicTrans2 |
| [Embeddings and reranking](https://api.dharun.dev/docs/embeddings) | `/v1/embed?text=...`, `POST /v1/rerank` | Multilingual vectors for semantic search, and a reranker to order results |
| [Tamil legacy encodings](https://api.dharun.dev/docs/tamil-font) | `/v1/tamil-font/convert?from=tscii&text=...` | TSCII text to Unicode and back |
| [IP lookup](https://api.dharun.dev/docs/ip) | `/v1/ip/8.8.8.8`, `/v1/ip` | Country, city and network of an address, or of the caller |
| [Email check](https://api.dharun.dev/docs/email) | `/v1/email/someone@example.com` | Syntax, disposable domain, whether the domain takes mail |
| [DNS check](https://api.dharun.dev/docs/dns) | `/v1/dns/example.com?type=MX` | Records as several public resolvers see them |
| [Traceroute from India](https://api.dharun.dev/docs/mtr) | `/v1/mtr/example.com` | MTR-style path from a server in India: network, loss and latency per hop |
| [Whois](https://api.dharun.dev/docs/whois) | `/v1/whois/example.co.in`, `/v1/whois/AS55836` | Registrar, dates, status and nameservers of a domain; holder and abuse contact of an IP network or AS number, through RDAP |
| [TLS check](https://api.dharun.dev/docs/tls) | `/v1/tls/example.com` | Certificate validity, chain, days left, TLS versions and weak ciphers, as a list of problems |
| [IPv6 check](https://api.dharun.dev/docs/ipv6) | `/v1/ipv6/example.com` | IPv6 addresses for the site, its nameservers and mail servers |
| [HTTP probe from India](https://api.dharun.dev/docs/probe) | `/v1/probe?url=https://example.com` | DNS, connect, TLS and first-byte times from India, redirects, headers and the CDN edge city |
| [Website icons](https://api.dharun.dev/docs/icon) | `/v1/icon/github.com?size=64` | Any site's icon as a square PNG, or an initials tile |
| [QR, UPI QR and barcodes](https://api.dharun.dev/docs/codes) | `/v1/qr?text=hello`, `/v1/upi-qr?pa=shop@okaxis&am=250`, `POST /v1/barcode/read` | QR codes, UPI payment QR codes and links, barcodes; reading codes from a photo |
| [GST invoice](https://api.dharun.dev/docs/invoice) | `POST /v1/invoice` | A GST tax invoice PDF from JSON: CGST and SGST or IGST, HSN summary, amount in words, UPI QR |
| [OCR](https://api.dharun.dev/docs/ocr) | `POST /v1/ocr?lang=eng+tam` | Text from photos, scans and scanned PDFs in English and ten Indian languages |
| [Document reading](https://api.dharun.dev/docs/documents) | `POST /v1/doc/markdown`, `/doc/tables`, `/doc/invoice`, `/doc/einvoice-qr`, `/doc/receipt`, `/doc/cheque`, `/doc/statement`, `/doc/redact`, `/doc/mask-aadhaar` | PDFs and scans to Markdown and tables; GST invoices, e-invoice QR codes, receipts, cheques and bank statements read and checked; identifiers redacted and Aadhaar numbers masked |
| [Image tools](https://api.dharun.dev/docs/image-tools) | `POST /v1/image/convert?format=webp&width=800` | Resize and convert (WebP, AVIF, HEIC in), remove backgrounds, perceptual hashes |
| [PDF tools](https://api.dharun.dev/docs/pdf) | `POST /v1/pdf/merge`, `/pages`, `/rotate`, `/compress`, `/text`, `/render` | Merge, split, rotate, compress, read text, draw pages |
| [Typst to PDF](https://api.dharun.dev/docs/typst) | `/v1/typst?text=...` | Typst documents to PDF or PNG, with Indian scripts |
| [Speech](https://api.dharun.dev/docs/speech) | `POST /v1/stt`, `/v1/tts?text=...&voice=hi`, `POST /v1/audio/denoise` | Speech to text and subtitles with Whisper; text to speech in Hindi, Telugu, Malayalam and English; noise removal |
| [Generated images](https://api.dharun.dev/docs/images) | `/v1/og-image?title=Hello`, `/v1/avatar/Priya%20Raman`, `/v1/chart?values=1,3,2` | Link preview cards, code images, initials avatars, placeholders and bar, line, pie and sparkline charts, in ten scripts |
| [Batch lookups](https://api.dharun.dev/docs/batch) | `POST /v1/batch` | Up to 1,000 pincode, IFSC, phone, GSTIN and other lookups in one request |
| [Text tools](https://api.dharun.dev/docs/text) | `/v1/text/chunk`, `/redact`, `/moderate`, `/tokens`, `/language` | Chunking for retrieval, PII and name redaction, moderation with Hinglish and Tanglish, token counts, language detection |
| [Regex tester](https://api.dharun.dev/docs/regex) | `/v1/regex?pattern=...&text=...` | Every match with its groups, replacement, the pattern explained |
| [Fake Indian data](https://api.dharun.dev/docs/fake) | `/v1/fake/companies?count=5` | Test people and companies with PAN and GSTIN that pass format checks |
| [Number to words](https://api.dharun.dev/docs/words) | `/v1/words/1,25,000?lang=ta` | Lakh and crore spelling in English, Hindi, Tamil |
| [Format checks](https://api.dharun.dev/docs/validate) | `/v1/validate/gstin/27AAPFU0939F1ZV` | GSTIN, PAN and IFSC structure and check characters |
| [States](https://api.dharun.dev/docs/states) | `/v1/states/TN` | State codes, GST codes, vehicle prefixes |
| [Number formatting](https://api.dharun.dev/docs/format) | `/v1/format/number/123456789.5` | Lakh and crore grouping, 12.35 Cr short forms, rupees, words, Indian-script digits |
| [Indian date tools](https://api.dharun.dev/docs/dates) | `/v1/dates/2026-11-11`, `/v1/dates/diff?from=1995-08-15` | Financial and assessment year, quarter, Saka, Tamil and Hijri dates, ages |
| [UPI handles and payment QR](https://api.dharun.dev/docs/upi) | `/v1/upi-handles/okhdfcbank`, `/v1/qr/parse?text=upi://...` | The app and bank behind a handle; a UPI or Bharat QR code's fields with the checksum checked |
| [README badges](https://api.dharun.dev/docs/badges) | `/v1/badge/market`, `/v1/badge?label=&message=` | Market open or closed, next holiday, dataset freshness, or your own |
| [Address reader](https://api.dharun.dev/docs/address) | `/v1/address/parse?text=...` | A written address split into house, street, city, state and pincode, checked against the post offices |
| [Data format tools](https://api.dharun.dev/docs/data-tools) | `POST /v1/convert?to=yaml`, `POST /v1/diff`, `POST /v1/sheet/to-json` | JSON, YAML, TOML, CSV and XML to each other; text and JSON differences; XLSX and CSV to JSON and back |
| [Developer utilities](https://api.dharun.dev/docs/dev-utils) | `/v1/cron?expr=...`, `/v1/uuid?kind=uuid7`, `/v1/time?at=3pm&to=PST` | Cron in words with next runs, UUIDs and ULIDs, hashes and HMACs, JWT decoding, user agents, time zones |
| [Colours](https://api.dharun.dev/docs/colors) | `/v1/color/ff9933`, `/v1/color/contrast?fg=&bg=` | Hex, RGB, HSL and OKLCH, WCAG contrast with a passing suggestion, a picture's palette |
| [Web pages](https://api.dharun.dev/docs/web) | `/v1/unfurl?url=`, `/v1/read?url=` | Link previews, a page as Markdown for agents (robots.txt respected) |
| [Audio and video tools](https://api.dharun.dev/docs/media) | `POST /v1/media/convert?format=mp3`, `/trim`, `/waveform`, `/thumbnail` | Convert, cut, draw a waveform, take a frame |
| [Request bins](https://api.dharun.dev/docs/hooks) | `POST /v1/hooks` | A private address that records webhooks for a day |
| [Throwaway databases](https://api.dharun.dev/docs/db) | `POST /v1/db` | A private SQLite database for a day, SQL over HTTPS |
| [Festivals](https://api.dharun.dev/docs/festivals) | `/v1/festivals/2026`, `/v1/festivals/2026.ics?state=TN` | Hindu, Tamil, Sikh, Jain, Buddhist, Islamic and Christian festival dates 1950 to 2100, computed from the panchang with each rule given |
| [Indic text tools](https://api.dharun.dev/docs/indic) | `/v1/indic/aksharas?text=...`, `/v1/braille?text=...`, `/v1/keyboard/convert?layout=tamil99` | Letters as readers count them, dictionary order, look-alike fixes, Bharati braille, InScript and Tamil99 keys |
| [Helplines](https://api.dharun.dev/docs/helplines) | `/v1/helplines?q=cyber` | National emergency and helpline numbers |
| [Index constituents](https://api.dharun.dev/docs/indices) | `/v1/indices/nifty-50`, `/v1/indices?symbol=TCS` | Members of 31 NSE indices with industry, ISIN and logo; the indices a stock is in |
| [Gold and silver rates](https://api.dharun.dev/docs/bullion) | `/v1/bullion`, `/v1/bullion/history?purity=916` | IBJA's daily gold (24K to 14K), silver and platinum rates, per gram too, and their history |
| [Mutual fund returns](https://api.dharun.dev/docs/mutual-funds) | `/v1/mutual-funds/122639/returns`, `/history` | 1 month to 10 year returns (CAGR), SIP XIRR, month-end NAV history |
| [RBI rates and exchange rates](https://api.dharun.dev/docs/rates) | `/v1/rates`, `/v1/fx?currency=USD` | Repo rate, CRR, SLR, MCLR range, bond yields; FBIL reference rates with history |
| [Mandi prices](https://api.dharun.dev/docs/mandi) | `/v1/mandi?commodity=Tomato&state=Tamil%20Nadu`, `/v1/mandi/commodities` | Daily wholesale prices of farm produce in each market, from Agmarknet via data.gov.in |
| [Air quality](https://api.dharun.dev/docs/air-quality) | `/v1/air-quality?city=Chennai`, `?near=lat,lon` | AQI and pollutants at CPCB's monitoring stations, hourly, via data.gov.in |
| [Villages and LGD codes](https://api.dharun.dev/docs/lgd) | `/v1/lgd/states`, `/v1/lgd/villages?pincode=637001` | Every state, district, sub-district and village with its LGD and Census 2011 codes, and village PIN codes, from the Local Government Directory |
| [Hospitals](https://api.dharun.dev/docs/hospitals) | `/v1/hospitals?pincode=637001`, `?near=lat,lon` | About 30,000 hospitals with phone, specialties and position, from the National Hospital Directory (May 2016) |
| [IPC to BNS](https://api.dharun.dev/docs/law) | `/v1/law/ipc/302`, `/v1/law/bns?q=murder` | Section correspondence between the new criminal laws and the old ones: BNS and IPC, BNSS and CrPC, BSA and the Evidence Act |
| [Derivative expiries](https://api.dharun.dev/docs/expiries) | `/v1/expiries?exchange=nse&year=2026` | NSE and BSE weekly and monthly expiry dates, moved for holidays |
| [MICR and SWIFT](https://api.dharun.dev/docs/ifsc) | `/v1/micr/400240003`, `/v1/swift/HDFCINBB` | The branch behind a cheque's MICR code or a SWIFT code |
| [Sectors](https://api.dharun.dev/docs/logos) | `/v1/sectors?sector=...` | Sectors and industries of listed companies, with counts and the largest companies |
| [Search your own documents](https://api.dharun.dev/docs/rag) | `POST /v1/rag`, `/documents`, `/query` | Upload text, ask questions, get the best passages back, embedded and reranked on the server |
| [Agent notes](https://api.dharun.dev/docs/kv) | `POST /v1/kv`, `/v1/kv/{id}/{key}` | A small private key-value store for agents, kept 30 days |
| [MCP tools](https://api.dharun.dev/docs/mcp-tools) | `/v1/mcp-tools?q=gold`, `/mcp?tools=money,dates` | Find a tool by task; connect to only the tool sets you need |

## One shape everywhere

- Every JSON response is `{ "data": ..., "meta": { ... } }`. `data` is an object, or an
  array for list endpoints, which add `meta.total`, `meta.limit` and `meta.offset`.
- `meta.source` names the dataset, its version, its licence and the credit it asks for.
- Errors are `{ "error": { "code", "message" } }` with a matching HTTP status.
- Each address has an allowance of 600 units a minute; a [free API key](https://api.dharun.dev/keys)
  gives 3,000 a minute per key. A lookup costs 1; a request that takes real CPU work (image
  rendering, astronomy, live DNS) costs 10, a file upload 15, and a neural network job
  (speech, speech to text, translation, background removal, denoise) 30. Embeddings,
  reranking and moderation count as CPU work. The `RateLimit-*` headers say what is left.
- The model endpoints (OCR, speech, translation, embeddings, reranking, moderation, Roman
  typing, background removal, denoise, name detection) will need a free key after a notice
  period; until then their answers carry an `X-Key-Notice` header. Send a key as
  `Authorization: Bearer <key>` or `X-API-Key`.
- Text endpoints take `?text=`, or a POST body as `text/plain` or JSON `{"text": "..."}`.
- Image endpoints return the image itself, ready for an `<img>` tag; PDF and audio
  endpoints return the file.
- File endpoints take a POST of the file as the body, or as `multipart/form-data`, up to
  10 MB. Files are deleted when the answer is sent; nothing is kept.

## Use it from AI assistants

One MCP server, one Agent Skill and one Claude Code plugin cover every API.

```bash
# Claude Code: the MCP server
claude mcp add --transport http india-api https://api.dharun.dev/mcp

# Claude Code: the plugin (skill and MCP server together)
/plugin marketplace add dharunashokkumar/api.dharun.dev
/plugin install india-api@dharun-dev

# Any agent that reads Agent Skills (Claude Code, Codex, Copilot, Cursor, Gemini CLI)
npx skills add dharunashokkumar/api.dharun.dev
```

- **MCP server:** `https://api.dharun.dev/mcp`, Streamable HTTP, stateless, no
  authentication. Each tool returns exactly what the matching REST endpoint returns.
  Setup for Claude, ChatGPT, VS Code, Cursor, Windsurf, Gemini CLI and Codex:
  [api.dharun.dev/ai](https://api.dharun.dev/ai). In the MCP Registry as
  `io.github.dharunashokkumar/india-api`.
- **Agent Skill:** [`skills/india-api`](skills/india-api/SKILL.md).
- **Claude Code plugin:** [`plugins/india-api`](plugins/india-api), with the marketplace in
  [`.claude-plugin/marketplace.json`](.claude-plugin/marketplace.json).
- **For crawlers and agents:** [`/llms.txt`](https://api.dharun.dev/llms.txt),
  [`/llms-full.txt`](https://api.dharun.dev/llms-full.txt), every page as Markdown (add
  `.md` or send `Accept: text/markdown`), [`/openapi.json`](https://api.dharun.dev/openapi.json),
  an MCP server card, an AI catalog and an API catalog under `/.well-known/`.

The skill, the plugin and `server.json` are generated from the catalog in
`internal/api/catalog.go`; a test fails when the committed copies are out of date.

## Licence

- **Calling the API** puts no licence on your app.
- **The code** is licensed under the [GNU Affero General Public License v3.0](LICENSE)
  (AGPL-3.0-only). If you run a modified copy as a service for others, you must offer them
  your changes.
- **Code examples, the guides' code, the Agent Skill and the Claude Code plugin** are
  [MIT-0](skills/india-api/LICENSE.txt): use them anywhere, no credit needed.
- **Data** keeps the licence of its source (below); every response repeats the credit.
- **Third-party code and fonts** compiled into the server keep their licences, such as the
  SGP4 code in `internal/orbit` (BSD-2-Clause) and the Noto fonts (OFL-1.1). The release
  bundle's `NOTICES` lists every one.
- **Company, brand, stock and asset logos** are trademarks of their owners. Brand artwork from
  Wikimedia Commons keeps its own free licence, named per brand; the crypto icons from
  spothq are CC0; the commodity icons were drawn for this project.

## Security and privacy

Report vulnerabilities privately: see [SECURITY.md](SECURITY.md). Client IP addresses are
never logged, and endpoints that take personal values never log their input; the full
list is at [api.dharun.dev/privacy](https://api.dharun.dev/privacy). Each release carries a
build provenance attestation:

```bash
gh attestation verify dharun-api-linux-amd64.tar.gz --repo dharunashokkumar/api.dharun.dev
```

## Contributing

Bug reports, corrections to data and requests for new APIs are welcome as
[issues](https://github.com/dharunashokkumar/api.dharun.dev/issues). Pull requests are not
accepted; see [CONTRIBUTING.md](CONTRIBUTING.md).

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
| Constituencies | [DataMeet](https://github.com/datameet/maps) community maps | Assembly CC BY 2.5 India; parliamentary CC0 | 4,182 assembly, 543 parliamentary |
| Thirukkural | [tk120404/thirukkural](https://github.com/tk120404/thirukkural); translations by G.U. Pope and by Drew and Lazarus | Public domain text; compilation Apache-2.0 | 1,330 couplets |
| IP addresses | [DB-IP](https://db-ip.com) Lite | CC BY 4.0 | fetched by each server |
| Whois registries | [IANA RDAP bootstrap registries](https://www.iana.org/assignments/rdap-dns/) (RFC 9224); each registration is fetched live from its registry | CC0 1.0 | 1,201 top-level domains, refreshed weekly |
| Orbit elements | [CelesTrak](https://celestrak.org/NORAD/elements/), from US Space Force data via Space-Track.org; only computed passes and positions are published | None stated; redistribution with credit allowed by USSPACECOM | fetched by each server, 111 satellites |
| Space weather | [NOAA Space Weather Prediction Center](https://www.swpc.noaa.gov/) | Public domain (US government work) | fetched by each server, hourly |
| Speech voices | [Piper voices](https://huggingface.co/rhasspy/piper-voices), each trained on its own recordings | Per voice: public domain, CC BY 4.0, IIT Madras Indic TTS licence, or CC BY-NC-SA 4.0 (two Hindi voices, non-commercial) | fetched by each server, 10 voices |
| Background removal model | [IS-Net](https://github.com/xuebinqin/DIS) (Qin et al.), ONNX export by rembg | Apache-2.0 | fetched by each server |
| Embeddings and reranking | [multilingual-e5-small](https://huggingface.co/intfloat/multilingual-e5-small) (Microsoft); [bge-reranker-v2-m3](https://huggingface.co/BAAI/bge-reranker-v2-m3) (BAAI) | MIT; Apache-2.0 | fetched by each server |
| Speech recognition | [Whisper](https://github.com/openai/whisper) small (OpenAI), through whisper.cpp | MIT | fetched by each server |
| Translation | [IndicTrans2](https://github.com/AI4Bharat/IndicTrans2) distilled 200M (AI4Bharat) | MIT | fetched by each server |
| Transliteration model | [IndicXlit](https://github.com/AI4Bharat/IndicXlit) (AI4Bharat), converted to ONNX here | MIT | fetched by each server |
| Name detection | [IndicNER](https://huggingface.co/ai4bharat/IndicNER) (AI4Bharat); [bert-base-NER](https://huggingface.co/dslim/bert-base-NER) | MIT | fetched by each server |
| Moderation | [Detoxify multilingual](https://huggingface.co/unitary/multilingual-toxic-xlm-roberta) (Unitary); [Hinglish toxicity](https://huggingface.co/darelphilip/hinglish-toxicity-classifier) (Darel Philip, on L3Cube HingRoBERTa, CC BY 4.0); [Tanglish offensive language](https://huggingface.co/seanbenhur/tanglish-offensive-language-identification) (Sean Benhur) | Apache-2.0; MIT; Apache-2.0 | fetched by each server |
| Disposable email domains | [disposable-email-domains](https://github.com/disposable-email-domains/disposable-email-domains) | CC0 | 9,881 domains |
| Logos and company details | Collected from TradingView | Logos are trademarks of the companies they identify | 5,599 companies, 5,354 with a logo |
| Indian brand logos | [Wikimedia Commons](https://commons.wikimedia.org/) files under a free licence, else the brand site's own icon (`tools/fetch_brand_logos.py`) | Trademarks of the brands; Commons files keep their licence, named per brand | 294 brands, 234 with a logo |
| Global stock logos | Collected from TradingView (`tools/scrape_world_logos.py`) | Logos are trademarks of the companies they identify | 8,799 stocks, 8,630 with a logo |
| Crypto, index, ETF and commodity logos | TradingView; [cryptocurrency-icons](https://github.com/spothq/cryptocurrency-icons) (spothq); commodity icons drawn here | Marks belong to their owners; spothq icons CC0; drawn icons with the code | 1,586 assets, 1,548 with a logo |

Exact versions, retrieval dates and checksums are in [`data/sources.json`](data/sources.json)
(the models: pinned by revision and checksum in `internal/dataset/aimodels.go`), and every
response repeats the credit in `meta.source`. Models published only as PyTorch weights or in
full precision were converted to ONNX and int8 by [`tools/models/convert.py`](tools/models/convert.py)
and attached to the `models-2026-10` release.

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
cmd/accounts       the account system (PocketBase as a library), localhost only
internal/api       routing, endpoints, the website, MCP, metrics, keys
internal/api/assets      the scripts of the interactive pages (try-it, playground, tools)
internal/accounts  accounts, API keys and their collections on PocketBase
internal/apikey    the key format shared by accounts and the API
internal/calc      the money calculators and their tax rules
internal/api/guides      the guides, in Markdown
internal/api/templates   the Markdown forms of the pages and the skill
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
skills/            the Agent Skill (generated from the catalog)
plugins/           the Claude Code plugin (generated); marketplace in .claude-plugin/
server.json        the MCP Registry entry (generated)
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
python tools/scrape_logos.py --dry-run                   # logos: see new, renamed and delisted companies
python tools/scrape_logos.py                             # then download and write them
python tools/scrape_world_logos.py --crypto-icons DIR    # global stocks, crypto, indices, ETFs
python tools/fetch_brand_logos.py                        # Indian brands (Commons, then site icons)
go run ./cmd/apidata status                              # which copy of each dataset is in use
```

Central government holidays are typed into `data/holidays/central/<year>.json` from the
yearly memorandum, usually published in July for the following year.

## Release and deploy

Deployment is pull-based: the server fetches releases, nothing connects in to it.

1. Every push to `main` runs CI: tests, shell lint, a full release build, a smoke test of
   the bundle against the full datasets, and an install of it on a fresh Debian 13.
2. When it passes, the bundle is published as release `v1.1.<run number>`.
3. On the server, `dharun-api-update.timer` checks every 2 minutes. It downloads the new
   release, verifies the checksum, switches to it, and checks `/healthz`. If the new
   version does not come up, it switches back and marks that release as failed.
4. When CI has finished, the `Production` workflow waits (up to 20 minutes) for the public
   `/healthz` to report the new version, then runs the smoke test's route checks against
   the live site. A release that was never installed, or was rolled back, fails the run.
   The run shows on GitHub as a deployment to `production`.

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
