# Part 3: the website

Dharun, 2026-11-10: add a try-it panel on every API page, a full playground, free tool
pages for normal people at `api.dharun.dev/tools` (English), and visual demos. This
replaces "documentation-only website".

Why it matters (from the data): about 60 endpoints are nearly unused, only 191 requests
in a month came from someone copying a docs example, and Google fetched 61 pages. The
endpoints exist; people cannot find or try them.

## Rules that stay

- One Go service serves everything. No JavaScript framework and no build step: plain
  JavaScript and CSS files served from the binary, as the fonts are.
- Nothing loaded from other hosts (the one possible exception is the bot check, 03).
- Every page works without JavaScript as far as it can: the docs page keeps its static
  sample; a tool page explains itself and shows the API call.
- Generated from the catalog wherever possible, with tests, like the docs pages.
- No third-party analytics. Our own tagged links (`utm_medium`) get new values:
  `try`, `play`, `tool`, `demo`.
- The trust pages still say nothing about hosting.
- No map that draws India's outline (see `ROADMAP.md` section 8). Demos that show
  places use points on a plain grid or a globe without political borders.

## 1. Try-it panel (every API page)

Generated from each endpoint's catalog entry (parameters, `Example`, `Upload`, `Output`).

- Fields for the path and query parameters, filled with the example. Run sends the
  request from the visitor's browser.
- Shows status, time taken, the rate-limit headers, and the answer: JSON
  pretty-printed and foldable, images shown, PDFs in a frame, audio in a player.
- The curl, JavaScript and Python samples below follow the fields as they change.
- Upload endpoints: the sample file by default, or the visitor's own file.
- Private endpoints: nothing typed is stored or put in the address bar.
- Errors are shown as teaching material: what a 404, 429 or 422 from this endpoint
  looks like, with a button that triggers each one on purpose.

## 2. Playground (`/play`)

One workbench for the whole API.

- **Find:** search across all endpoints by what they do ("pincode", "pdf", "tamil").
- **Build:** the same form as the try-it panel, plus headers and a raw body editor.
- **Share:** the request is encoded in the part of the address after `#`, which
  browsers never send to a server, so shared links work and nothing private reaches the
  log.
- **History:** the last requests, kept in the browser only.
- **Chains:** send one answer into the next request. Ready-made chains show what the
  API can do together:
  - photo of a Tamil notice → OCR → translation → speech
  - pincode → sunrise and panchang → a share image
  - invoice JSON → PDF → page image
  - GSTIN → format check → state → that state's bank holidays
  - a domain → DNS → TLS → whois → probe
- **Simulate:** a switch that answers from the documented sample without calling the
  server. Lets a student explore offline-style, shows how the cost units drain a
  per-minute allowance, and lets them see a 429 and a 503 and how `Retry-After` works.
- **For agents:** each request also shown as the MCP tool call and as a sentence to
  type into an assistant.
- **With a key:** a logged-in visitor's requests use their key; the page shows the
  quota left.

## 3. Tool pages (`/tools/...`) for normal people

Each tool is a real page a person can use without knowing what an API is, with a short
explanation, the questions people ask about the task, and one line at the bottom:
"Developers: this page runs on this API" with the link.

**The conflict to settle (open question):** several tools call endpoints that will
need a key (decision 10), and a normal visitor has no key. Proposed: the site gives a
visitor a short-lived token after a silent browser check, tied to their address, with a
small daily allowance for the heavy tools (for example 10 OCR pages, 5 minutes of
speech). Past that, the page invites a free signup.

First ten (chosen for likely search demand and because the endpoint exists; these are
editorial guesses, to be checked against Search Console once pages are indexed):

| Tool | Address | Built on |
|---|---|---|
| GST invoice maker | `/tools/gst-invoice` | `/v1/invoice`: a form, live preview, PDF download |
| UPI QR code maker | `/tools/upi-qr` | `/v1/upi-qr` |
| IFSC code finder | `/tools/ifsc` | `/v1/ifsc`, `/v1/banks`: bank → state → city → branch |
| Pincode finder | `/tools/pincode` | `/v1/pincode`, nearby |
| Amount in words | `/tools/amount-in-words` | `/v1/words`: rupees for cheques, English, Hindi, Tamil |
| Bank holidays calendar | `/tools/bank-holidays` | `/v1/holidays`: by state and year, with a calendar file to subscribe to (05, set A) |
| Today's panchang | `/tools/panchang` | `/v1/panchang`: by city or pincode |
| Image to text (Indian languages) | `/tools/image-to-text` | `/v1/ocr` |
| Tamil and Hindi typing | `/tools/indian-language-typing` | `/v1/transliterate/suggest`: type in English letters, get the script |
| PDF tools | `/tools/pdf` | merge, split, rotate, compress, to text |

Next twenty, in no fixed order: GSTIN and PAN format check, working days calculator,
is the market open, sunrise and sunset, image convert and compress, background remover,
QR and barcode maker and reader, text to speech (Hindi, Telugu, Malayalam, English),
translate, TSCII to Unicode, HSN code finder, RTO code finder, mutual fund NAV, ISS and
satellite passes tonight, Thirukkural of the day, what is my IP, DNS, TLS and whois
checks, regex tester, token counter, fake Indian test data, code to image, share image
maker, and the calculators in 05 (SIP, EMI, income tax, GST).

A tool that a spreadsheet user would love, drawn from the batch usage in the data:
**CSV enrich**. Upload a CSV with a column of pincodes, IFSC codes, phone numbers or
GSTINs; get the same file back with district, state, bank, branch, operator or
validity columns added. Runs on the batch endpoints (05, set A).

## 4. Demos (`/demos/...`)

Made to be looked at and shared. Each links to the endpoint behind it.

| Demo | Shows |
|---|---|
| Satellite tracker | Where the ISS and amateur satellites are now, the ground track, the next pass over a pincode (computed positions only, never orbit elements) |
| Path from India | A traceroute drawn hop by hop with network names and delays; private hops stay hidden as in the API |
| Market clock | Open or closed now, time to the bell, the next holiday, special sessions |
| Logo wall | Nifty 50 or any index as a wall or treemap of logos by sector; a logo picker for designers |
| Panchang dial | Tithi, nakshatra and yoga as rings that turn through the day, sun and moon rise marked |
| Day length across India | Sunrise and day length for a row of cities from Kibithu to Kutch, through the year |
| Radio weather | Solar flux, K index and the band estimates as a small dashboard for radio amateurs |
| Script switcher | Type once, see nine Indian scripts side by side |
| Invoice live | The invoice JSON on the left, the PDF on the right, changing as you type |
| Under the hood | One request followed through the service: cost charged, dataset copy used, time per step, cache headers. A teaching page for students |

## 5. Search work

From `seo_audit.md` (still open) and the data:

- [ ] **Search titles and descriptions per page** (`SEOTitle`, `SEODescription` in the
      catalog), written for the task, not truncated from the summary.
- [ ] **Typed response schemas in OpenAPI**, shared with MCP `outputSchema`. Also the
      base for generated client packages.
- [ ] **Sitemap dates from documentation edits**, not dataset dates. Tool, demo and
      logo pages join the sitemap.
- [ ] **Visible byline** on guides, a fuller `/about`, and a link from dharun.dev to the
      API (the personal site did not mention it when audited).
- [ ] **Check Search Console first:** with 61 Googlebot fetches in a month, find out
      whether pages are indexed at all before writing more of them.
- [ ] **A changelog page** (`/changelog`) written from the releases: what changed for
      users, by date. Fresh content, and it tells callers about contract changes such
      as the logo fallback and the key requirement.
- [ ] **Guides for the developers who are already here:** Flutter (Dart made 1,800
      requests) and Android or Kotlin (Java made 5,900): pincode autofill, IFSC
      lookup, logos in a list. Then the twelve guides proposed in the audit.
- [ ] **Client packages** once the schemas are typed: npm, PyPI, and pub.dev for Dart.

### Pages made from data (open question)

One page per record can bring search visitors, and can also read as thin, mass-made
content, which the audit warned against. Recommended order, deciding each step from
Search Console:

1. **Company logo pages**, `/logos/{SYMBOL}` (about 5,600): logo preview, SVG and PNG
   download, embed code, company details, the API address. This is the search form of
   the one product with proven demand. Each page has real, different content. The
   trademark note stays visible.
2. **Holiday pages** by year and state, and by exchange: a real calendar people look
   for every year.
3. **Bank pages** (one per bank, branches grouped by state and city), not one per
   branch.
4. Only if those earn visits: RTO offices, pincodes. One page per IFSC branch
   (170,000 pages) is not proposed.

## 6. Order inside this part

1. Try-it panel (it is generated, and every later piece reuses its form).
2. The first ten tool pages and the site token.
3. Playground with chains and simulate.
4. Company logo pages.
5. Demos, one at a time.
6. The search items, alongside.

## 7. Tests

- The panel and playground scripts are generated from the catalog: a test checks every
  endpoint has a form and that the form's example equals the catalog's `Example`.
- `TestSitePages` covers the new pages (title, canonical, one H1, JSON-LD, Markdown
  copy, works without JavaScript).
- A headless browser check in CI for the three scripted pieces is worth its cost only
  if regressions appear; start without.
- Smoke checks for one tool page, `/play` and one demo.

## Open questions

1. The site token for heavy tools (section 3): yes, and what allowance.
2. Pages made from data: start with company logo pages only?
3. Which demos first.
4. Tool pages in Hindi and Tamil later (each needs a human review), or English only for good.

## Build status (2026-11-11)

Built, tested and committed, not pushed. Dharun's answers of 2026-11-11: the site
token, yes (10 model requests a day per address once keys are required); company logo
pages, yes, now.

- [x] Try-it panel under every endpoint (`tryit.go`, `assets/try.js`): fields from the
      example, live curl/JavaScript/Python, answer with status, time, limit headers,
      JSON, image, PDF or audio; error buttons (404, 400). A test checks every endpoint
      has a spec with path values.
- [x] Playground `/play`: find, build, headers, share through `#`, history (browser
      only, never private endpoints), five chains (`playChains`), simulate from
      `/play/samples.json` with the allowance drawn, a sentence for an assistant, the
      visitor's key when one was made on this browser.
- [x] The first ten tool pages at `/tools` (`site_tools.go`, `assets/tools.js`), each with
      questions, JSON-LD and the API line; model tools send the site token.
- [x] Company logo pages `/logos`, `/logos/{SYMBOL}` (every listed company, in the
      sitemap, other spellings redirect), with Markdown forms.
- [x] Demos (4 of 10): market clock, script switcher, logo wall, under the hood.
- [x] `/changelog` (by hand, `templates/changelog.md`).
- [x] Nav: Tools, Playground, API key; footer links.
- [ ] The other six demos (satellite tracker, path from India, panchang dial, day length,
      radio weather, invoice live).
- [ ] The next twenty tool pages, and CSV enrich on `/v1/batch`.
- [ ] Section 5 search items: per-page SEO titles, typed schemas, sitemap dates from
      documentation edits, Search Console first, Flutter and Android guides, client packages.
- [ ] A headless browser check, only if regressions appear. The scripts were checked with
      `node --check` and against a local server's answers, not in a browser.
