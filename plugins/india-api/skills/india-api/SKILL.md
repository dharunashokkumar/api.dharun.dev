---
name: india-api
description: Look up Indian reference data with the free India API (api.dharun.dev): pincodes and post offices, IFSC codes and bank branches, GSTIN and PAN format checks, bank, market and government holidays and working days, RTO vehicle codes, NSE and BSE company logos, mutual fund NAVs, HSN and SAC codes, UPI QR codes, numbers in words (lakh, crore), transliteration between Indian scripts, panchang, GST tax invoices as PDF, OCR in Indian languages, PDF and image tools, text to speech and speech to text, translation between English and 22 Indian languages, Roman typing to Indian scripts (vanakkam to வணக்கம்), moderation including Hinglish and Tanglish, multilingual embeddings and reranking, whois (RDAP), TLS certificate and website speed checks from India, and more. Use when building an app that needs Indian data, Indian-language AI or document tools, or when the user asks about an Indian pincode, IFSC, GSTIN, PAN, holiday, vehicle registration or listed company. No API key needed.
license: MIT-0
metadata:
  homepage: https://api.dharun.dev
  source: https://github.com/dharunashokkumar/api.dharun.dev
---

# India API

Free JSON APIs for Indian data at https://api.dharun.dev: pincodes, IFSC and bank branches, GSTIN and PAN format checks, holidays and working days, RTO codes, company logos, mutual fund NAVs, HSN codes, UPI QR codes, Indian number words and more. No API key, no sign-up, CORS open. Open source (https://github.com/dharunashokkumar/api.dharun.dev).

## When to use it

- The user is building something that needs Indian reference data: a form that fills city and state from a pincode, a payout flow that shows the bank branch for an IFSC, an invoice with the amount in words, GST onboarding, scheduling around bank or market holidays, a stock app with company logos.
- The user asks a question this data answers: which district a pincode is in, which branch an IFSC belongs to, whether a date is a bank holiday in a state, which RTO a number plate is from. Call the API instead of answering from memory: branches, holiday lists and NAVs change.

If an MCP server named `india-api` is connected (https://api.dharun.dev/mcp), its tools return the same data and can be used directly.

## How to call it

- Every endpoint is a GET under `https://api.dharun.dev/v1`. Endpoints that take text also accept a POST body as `text/plain` or JSON `{"text": "..."}`.
- Success: `{ "data": ..., "meta": { ... } }`. `data` is an object, or an array for searches and lists, which add `meta.total`, `meta.limit` and `meta.offset`.
- Failure: `{ "error": { "code": "...", "message": "..." } }` with the HTTP status: 400 bad input, 404 nothing found, 429 over the rate limit (wait a minute), 503 dataset not loaded yet (try later).
- Rate limit: 600 requests a minute per IP address; image rendering, astronomy and live network checks count as 10. Cache answers you reuse.
- Image endpoints (logos, QR codes, barcodes, social cards) return the image itself; put the URL in an `<img>` tag.
- When you call the API yourself, add `utm_source=<your name>` (claude, chatgpt, gemini, cursor, ...) to say which assistant is calling; `utm_medium=skill` says the address came from this skill. Every endpoint ignores both: they are only counted.

```js
const response = await fetch("https://api.dharun.dev/v1/pincode/637001?utm_medium=skill");
const body = await response.json();
if (!response.ok) throw new Error(body.error.message);
const { district, state } = body.data;
```

```python
import requests

response = requests.get("https://api.dharun.dev/v1/ifsc/SBIN0000001?utm_medium=skill", timeout=10)
body = response.json()
if not response.ok:
    raise RuntimeError(body["error"]["message"])
print(body["data"]["branch"], body["data"]["bank"]["name"])
```

## Rules

- `meta.source` names each dataset's licence and the credit line (`attribution`) it asks for. When an app shows the data to people, show that credit where the licence asks for one.
- Format checks (`/v1/validate/...`) confirm structure and check characters only, not that a GSTIN, PAN or IFSC was ever issued.
- Send no more personal data than the task needs. Endpoints that take personal values (phone, email, IP, text tools) never log their input.
- Company logos are trademarks of their owners.

## Endpoints

Each API's full parameters, enums and examples are in `references/<id>.md`.

### Money and business

**Company logos** ([references/logos.md](references/logos.md)): Logos and details for companies listed on NSE and BSE.

- `GET /v1/logos/{symbol}`: Logo image for a listed company. Example: `/v1/logos/TCS?utm_medium=skill`
- `GET /v1/companies/{symbol}`: Details of a listed company. Example: `/v1/companies/RELIANCE?utm_medium=skill`
- `GET /v1/companies`: Search listed companies. Example: `/v1/companies?q=tata&limit=5&utm_medium=skill`

**IFSC and banks** ([references/ifsc.md](references/ifsc.md)): IFSC to bank and branch.

- `GET /v1/ifsc/{code}`: Branch for an IFSC. Example: `/v1/ifsc/SBIN0000001?utm_medium=skill`
- `GET /v1/ifsc`: Search a bank's branches. Example: `/v1/ifsc?bank=SBIN&state=TN&q=namakkal&utm_medium=skill`
- `GET /v1/banks/{code}`: Bank for a four-letter code. Example: `/v1/banks/HDFC?utm_medium=skill`
- `GET /v1/banks`: Search banks. Example: `/v1/banks?q=gramin&utm_medium=skill`

**QR codes and barcodes** ([references/codes.md](references/codes.md)): QR codes, UPI payment QR codes and barcodes as SVG or PNG, and reading them from a photo.

- `GET /v1/qr`: QR code for any text. Example: `/v1/qr?text=https://api.dharun.dev&format=png&size=256&utm_medium=skill`
- `GET /v1/upi-qr`: UPI payment QR code. Example: `/v1/upi-qr?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250&tn=Order%2042&utm_medium=skill`
- `GET /v1/barcode/{type}`: Barcode. Example: `/v1/barcode/ean13?text=890123456789&utm_medium=skill`
- `GET /v1/barcode/read`: Read QR codes and barcodes from a picture. Example: `/v1/barcode/read?utm_medium=skill`

**HSN and SAC codes** ([references/hsn.md](references/hsn.md)): GST classification codes for goods and services.

- `GET /v1/hsn/{code}`: A code, its parents and children. Example: `/v1/hsn/0902?utm_medium=skill`
- `GET /v1/hsn`: Search codes. Example: `/v1/hsn?q=green%20tea&utm_medium=skill`

**Mutual fund NAVs** ([references/mutual-funds.md](references/mutual-funds.md)): Latest net asset value of every Indian mutual fund scheme.

- `GET /v1/mutual-funds/{code}`: A scheme and its NAV. Example: `/v1/mutual-funds/122639?utm_medium=skill`
- `GET /v1/mutual-funds`: Search schemes. Example: `/v1/mutual-funds?q=flexi%20cap&plan=direct&option=growth&utm_medium=skill`
- `GET /v1/mutual-funds/fund-houses`: Fund houses. Example: `/v1/mutual-funds/fund-houses?utm_medium=skill`

**Number to words** ([references/words.md](references/words.md)): Numbers and rupee amounts in words, the Indian way.

- `GET /v1/words/{number}`: Spell out a number. Example: `/v1/words/1,25,000.50?style=currency&utm_medium=skill`

**Format checks** ([references/validate.md](references/validate.md)): Check the format of a GSTIN, PAN or IFSC.

- `GET /v1/validate/{kind}/{value}`: Check an identifier's format. Example: `/v1/validate/gstin/27AAPFU0939F1ZV?utm_medium=skill`

**GST invoice** ([references/invoice.md](references/invoice.md)): A GST tax invoice as a PDF from JSON, with the tax worked out.

- `GET /v1/invoice`: Make a GST tax invoice. Example: `/v1/invoice?utm_medium=skill`

### Places and transport

**Pincodes** ([references/pincode.md](references/pincode.md)): Pincode to post office, district and state.

- `GET /v1/pincode/{pincode}`: Post offices for a pincode. Example: `/v1/pincode/637001?utm_medium=skill`
- `GET /v1/pincode`: Search post offices. Example: `/v1/pincode?q=namakkal&state=TN&utm_medium=skill`
- `GET /v1/pincode/near`: Post offices near a point. Example: `/v1/pincode/near?lat=11.2189&lng=78.1674&radius=5&utm_medium=skill`

**RTO codes** ([references/rto.md](references/rto.md)): Vehicle registration code to transport office.

- `GET /v1/rto/{code}`: Office for an RTO code. Example: `/v1/rto/TN-28?utm_medium=skill`
- `GET /v1/rto`: List or search RTO codes. Example: `/v1/rto?state=TN&q=chennai&utm_medium=skill`

**Railway stations** ([references/stations.md](references/stations.md)): Indian Railways station codes to stations, and back.

- `GET /v1/stations/{code}`: Station for a code. Example: `/v1/stations/MAS?utm_medium=skill`
- `GET /v1/stations`: Search stations. Example: `/v1/stations?q=chennai&utm_medium=skill`
- `GET /v1/stations/zones`: Railway zones. Example: `/v1/stations/zones?utm_medium=skill`

**Constituencies** ([references/constituency.md](references/constituency.md)): Which assembly and Lok Sabha constituency a place is in.

- `GET /v1/constituency`: Constituencies of a place. Example: `/v1/constituency?pincode=637001&utm_medium=skill`

**Airports** ([references/airports.md](references/airports.md)): Airport codes to airports, and airports by city.

- `GET /v1/airports/{code}`: Airport for a code. Example: `/v1/airports/MAA?utm_medium=skill`
- `GET /v1/airports`: Search airports. Example: `/v1/airports?q=chennai&utm_medium=skill`

**States** ([references/states.md](references/states.md)): States and union territories with their codes.

- `GET /v1/states`: All states and union territories. Example: `/v1/states?utm_medium=skill`
- `GET /v1/states/{code}`: One state. Example: `/v1/states/TN?utm_medium=skill`

### Dates and calendars

**Holidays** ([references/holidays.md](references/holidays.md)): Bank, government and stock market holidays.

- `GET /v1/holidays/{year}`: Holidays in a year. Example: `/v1/holidays/2026?type=market&utm_medium=skill`

**Working days** ([references/working-days.md](references/working-days.md)): Bank, market, settlement and government working days.

- `GET /v1/working-days/{date}`: Is a date a working day. Example: `/v1/working-days/2026-01-16?calendar=bank&state=TN&add=3&utm_medium=skill`
- `GET /v1/working-days`: Count working days between two dates. Example: `/v1/working-days?from=2026-01-01&to=2026-01-31&calendar=market&utm_medium=skill`
- `GET /v1/market/status`: Is the stock market open. Example: `/v1/market/status?at=2026-01-26T10:00:00%2B05:30&utm_medium=skill`

**Panchang** ([references/panchang.md](references/panchang.md)): Hindu almanac for any date and place: tithi, nakshatra, Rahu kalam and more.

- `GET /v1/panchang/{date}`: Panchang for a date. Example: `/v1/panchang/2026-01-14?pincode=600001&utm_medium=skill`

**Sun and moon** ([references/sun-moon.md](references/sun-moon.md)): Sunrise, sunset, twilight, moonrise and the moon's phase for any date and place.

- `GET /v1/sun-moon/{date}`: Sun and moon for a date. Example: `/v1/sun-moon/2026-10-14?pincode=600001&utm_medium=skill`

### Language and text

**Thirukkural** ([references/thirukkural.md](references/thirukkural.md)): All 1,330 kurals in Tamil with English translations.

- `GET /v1/thirukkural/{number}`: A kural by number. Example: `/v1/thirukkural/1?utm_medium=skill`
- `GET /v1/thirukkural`: Search kurals. Example: `/v1/thirukkural?q=rain&utm_medium=skill`
- `GET /v1/thirukkural/today`: Kural of the day. Example: `/v1/thirukkural/today?date=2026-10-13&utm_medium=skill`
- `GET /v1/thirukkural/random`: A random kural. Example: `/v1/thirukkural/random?utm_medium=skill`
- `GET /v1/thirukkural/chapters`: All 133 chapters. Example: `/v1/thirukkural/chapters?utm_medium=skill`
- `GET /v1/thirukkural/chapters/{number}`: One chapter. Example: `/v1/thirukkural/chapters/2?utm_medium=skill`

**Text tools** ([references/text.md](references/text.md)): Chunk documents, redact identifiers and names, moderate, count tokens, detect language.

- `GET /v1/text/chunk`: Split a document into chunks. Example: `/v1/text/chunk?size=200&overlap=20&text=%23+Setup%0A%0AInstall+Go.+Clone+the+repository.%0A%0A%23+Deploy%0A%0APush+to+main.&utm_medium=skill`
- `GET /v1/text/redact`: Redact Indian personal identifiers. Example: `/v1/text/redact?text=Call+98765+43210%2C+PAN+ABCPK1234F&utm_medium=skill`
- `GET /v1/text/tokens`: Count tokens. Example: `/v1/text/tokens?model=gpt-4o&text=%E0%AE%B5%E0%AE%A3%E0%AE%95%E0%AF%8D%E0%AE%95%E0%AE%AE%E0%AF%8D%2C+how+many+tokens+is+this%3F&utm_medium=skill`
- `GET /v1/text/language`: Detect the language. Example: `/v1/text/language?text=%E0%AE%85%E0%AE%95%E0%AE%B0+%E0%AE%AE%E0%AF%81%E0%AE%A4%E0%AE%B2+%E0%AE%8E%E0%AE%B4%E0%AF%81%E0%AE%A4%E0%AF%8D%E0%AE%A4%E0%AF%86%E0%AE%B2%E0%AF%8D%E0%AE%B2%E0%AE%BE%E0%AE%AE%E0%AF%8D+%E0%AE%86%E0%AE%A4%E0%AE%BF+%E0%AE%AA%E0%AE%95%E0%AE%B5%E0%AE%A9%E0%AF%8D+%E0%AE%AE%E0%AF%81%E0%AE%A4%E0%AE%B1%E0%AF%8D%E0%AE%B1%E0%AF%87+%E0%AE%89%E0%AE%B2%E0%AE%95%E0%AF%81&utm_medium=skill`
- `GET /v1/text/moderate`: Score text for abuse, Hinglish and Tanglish included. Example: `/v1/text/moderate?text=tu+bahut+bakwas+aadmi+hai&utm_medium=skill`

**Transliteration** ([references/transliterate.md](references/transliterate.md)): Rewrite text between Indian scripts and Roman letters, or turn Roman typing into an Indian script.

- `GET /v1/transliterate`: Transliterate text. Example: `/v1/transliterate?to=tamil&text=%E0%A4%A8%E0%A4%AE%E0%A4%B8%E0%A5%8D%E0%A4%A4%E0%A5%87+%E0%A4%AD%E0%A4%BE%E0%A4%B0%E0%A4%A4&utm_medium=skill`
- `GET /v1/transliterate/suggest`: Roman typing to an Indian script (type English, get Tamil). Example: `/v1/transliterate/suggest?lang=ta&text=vanakkam+chennai&utm_medium=skill`

**Tamil legacy encodings** ([references/tamil-font.md](references/tamil-font.md)): Old Tamil text that shows as accented Latin letters, to Unicode and back.

- `GET /v1/tamil-font/convert`: Convert Tamil text. Example: `/v1/tamil-font/convert?from=tscii&text=%C2%BE%C3%81%C2%A2%C3%BA+%C3%85%C2%BD%C3%AC%C2%B8%C3%B5&utm_medium=skill`
- `GET /v1/tamil-font/encodings`: Encodings offered. Example: `/v1/tamil-font/encodings?utm_medium=skill`

**Speech** ([references/speech.md](references/speech.md)): Speech to text, text to speech in Hindi, Telugu, Malayalam and English, and noise removal.

- `GET /v1/stt`: Speech to text, with subtitles. Example: `/v1/stt?utm_medium=skill`
- `GET /v1/tts`: Text to speech. Example: `/v1/tts?text=Namaste%2C%20welcome%20to%20India%20API.&voice=en_GB-cori-medium&utm_medium=skill`
- `GET /v1/tts/voices`: Voices. Example: `/v1/tts/voices?utm_medium=skill`
- `GET /v1/audio/denoise`: Remove background noise. Example: `/v1/audio/denoise?utm_medium=skill`

**Embeddings and reranking** ([references/embeddings.md](references/embeddings.md)): Vectors for semantic search in Indian languages and English, and a reranker to order results.

- `GET /v1/embed`: Text embeddings. Example: `/v1/embed?type=query&text=How+do+I+renew+my+passport%3F&utm_medium=skill`
- `GET /v1/rerank`: Rerank documents for a query. Example: `/v1/rerank?query=capital+of+Tamil+Nadu&document=Chennai+is+the+capital+of+Tamil+Nadu.&document=Mumbai+is+the+financial+capital+of+India.&utm_medium=skill`

**Translation** ([references/translate.md](references/translate.md)): Translate between English and 22 Indian languages with IndicTrans2.

- `GET /v1/translate`: Translate text. Example: `/v1/translate?from=en&to=ta&text=The+train+to+Chennai+leaves+at+6+pm.+Please+reach+the+station+early.&utm_medium=skill`
- `GET /v1/translate/languages`: Languages. Example: `/v1/translate/languages?utm_medium=skill`

### Network and contact checks

**IP address location** ([references/ip.md](references/ip.md)): IP address to country, state, city and network.

- `GET /v1/ip/{address}`: Location of an IP address. Example: `/v1/ip/49.204.0.1?utm_medium=skill`
- `GET /v1/ip`: Location of your own address. Example: `/v1/ip?utm_medium=skill`

**Email checks** ([references/email.md](references/email.md)): Is an email address well formed, throwaway, and able to receive mail.

- `GET /v1/email/{address}`: Check an email address. Example: `/v1/email/someone@mailinator.com?utm_medium=skill`

**DNS check** ([references/dns.md](references/dns.md)): A domain's DNS records at four public resolvers at once.

- `GET /v1/dns/{name}`: Look up DNS records. Example: `/v1/dns/dharun.dev?type=MX&utm_medium=skill`

**Phone numbers** ([references/phone.md](references/phone.md)): Check and format an Indian phone number, and see whose series it is.

- `GET /v1/phone/{number}`: Check a phone number. Example: `/v1/phone/+91%2095001%2012345?utm_medium=skill`

**Traceroute from India** ([references/mtr.md](references/mtr.md)): The network path from a server in India to any host, with loss and latency per hop.

- `GET /v1/mtr/{host}`: Trace the route to a host. Example: `/v1/mtr/1.1.1.1?utm_medium=skill`

**Whois** ([references/whois.md](references/whois.md)): Who registered a domain, an IP network or an AS number, through RDAP.

- `GET /v1/whois/{query}`: Look up a registration. Example: `/v1/whois/google.in?utm_medium=skill`

**TLS check** ([references/tls.md](references/tls.md)): A website's certificate and TLS setup, checked from India.

- `GET /v1/tls/{host}`: Check a host's TLS. Example: `/v1/tls/dharun.dev?utm_medium=skill`

**IPv6 check** ([references/ipv6.md](references/ipv6.md)): Is a domain ready for IPv6: website, nameservers and mail.

- `GET /v1/ipv6/{domain}`: Check a domain's IPv6. Example: `/v1/ipv6/dharun.dev?utm_medium=skill`

**HTTP probe from India** ([references/probe.md](references/probe.md)): How fast a URL answers from India: DNS, connect, TLS, first byte and the CDN edge.

- `GET /v1/probe`: Time a URL from India. Example: `/v1/probe?url=https://dharun.dev&utm_medium=skill`

**Website icons** ([references/icon.md](references/icon.md)): The icon of any website as a square PNG, for lists of links and bookmarks.

- `GET /v1/icon/{domain}`: Icon for a website. Example: `/v1/icon/github.com?utm_medium=skill`

### Files, documents and media

**OCR** ([references/ocr.md](references/ocr.md)): Text from photos, scans and scanned PDFs, in English and ten Indian languages.

- `GET /v1/ocr`: Read the text in a picture or PDF. Example: `/v1/ocr?lang=eng+tam&utm_medium=skill`

**Image tools** ([references/image-tools.md](references/image-tools.md)): Resize and convert pictures, remove backgrounds, and compare pictures by how they look.

- `GET /v1/image/convert`: Resize and convert a picture. Example: `/v1/image/convert?format=webp&width=400&utm_medium=skill`
- `GET /v1/image/info`: Size, format and metadata of a picture. Example: `/v1/image/info?utm_medium=skill`
- `GET /v1/image/remove-background`: Remove the background. Example: `/v1/image/remove-background?utm_medium=skill`
- `GET /v1/image/hash`: Perceptual hash of a picture. Example: `/v1/image/hash?utm_medium=skill`
- `GET /v1/image/compare`: Compare two pictures. Example: `/v1/image/compare?utm_medium=skill`

**PDF tools** ([references/pdf.md](references/pdf.md)): Merge, split, rotate and compress PDFs, read their text and draw their pages.

- `GET /v1/pdf/info`: Pages, size and details of a PDF. Example: `/v1/pdf/info?utm_medium=skill`
- `GET /v1/pdf/text`: Text of a PDF, page by page. Example: `/v1/pdf/text?utm_medium=skill`
- `GET /v1/pdf/merge`: Merge PDFs into one. Example: `/v1/pdf/merge?utm_medium=skill`
- `GET /v1/pdf/pages`: Pick, reorder or split pages. Example: `/v1/pdf/pages?pages=2,1&utm_medium=skill`
- `GET /v1/pdf/rotate`: Rotate pages. Example: `/v1/pdf/rotate?angle=90&pages=1&utm_medium=skill`
- `GET /v1/pdf/compress`: Make a PDF smaller. Example: `/v1/pdf/compress?utm_medium=skill`
- `GET /v1/pdf/render`: Draw a page as a picture. Example: `/v1/pdf/render?page=1&width=600&utm_medium=skill`

**Typst to PDF** ([references/typst.md](references/typst.md)): Turn a Typst document into a PDF, with Indian scripts set correctly.

- `GET /v1/typst`: Compile Typst to PDF. Example: `/v1/typst?text=%23set%20page%28width%3A%2012cm%2C%20height%3A%20auto%2C%20margin%3A%201cm%29%0A=%20வணக்கம்%2C%20India%20API%0ATypst%20sets%20%2ATamil%2A%2C%20हिन्दी%20and%20English%20on%20one%20page.&utm_medium=skill`

### Developer tools

**Regex tester** ([references/regex.md](references/regex.md)): Run a regular expression against a text and see every match, explained.

- `GET /v1/regex`: Test a regular expression. Example: `/v1/regex?flags=i&pattern=%28%3F%3Cstate%3E%5BA-Z%5D%7B2%7D%29%5B+-%5D%3F%28%5Cd%7B2%7D%29&text=Cars+TN+37%2C+ka-05+and+MH12&utm_medium=skill`

**Fake test data** ([references/fake.md](references/fake.md)): Made-up Indian people, companies and addresses for testing apps.

- `GET /v1/fake/{kind}`: Generate fake records. Example: `/v1/fake/people?count=3&state=TN&seed=42&utm_medium=skill`

**Generated images** ([references/images.md](references/images.md)): Social preview images and code screenshots from a URL.

- `GET /v1/og-image`: Social preview image. Example: `/v1/og-image?title=Free%20APIs%20for%20Indian%20developers&subtitle=Pincodes%2C%20IFSC%2C%20holidays%20and%20more&site=api.dharun.dev&utm_medium=skill`
- `GET /v1/code-image`: Code as an image. Example: `/v1/code-image?language=go&title=main.go&code=package+main%0A%0Aimport+%22fmt%22%0A%0Afunc+main%28%29+%7B%0A%09fmt.Println%28%22%E0%AE%B5%E0%AE%A3%E0%AE%95%E0%AF%8D%E0%AE%95%E0%AE%AE%E0%AF%8D%22%29%0A%7D&utm_medium=skill`

### Sky and radio

**Satellite passes** ([references/satellites.md](references/satellites.md)): When the ISS and amateur radio satellites pass over you, and where they are now.

- `GET /v1/sat-pass`: Passes over a place. Example: `/v1/sat-pass?sat=ISS&pincode=600001&utm_medium=skill`
- `GET /v1/satellites/{id}`: Where a satellite is now. Example: `/v1/satellites/ISS?utm_medium=skill`
- `GET /v1/satellites`: Tracked satellites. Example: `/v1/satellites?group=stations&utm_medium=skill`

**HF propagation** ([references/propagation.md](references/propagation.md)): Space weather and a rough rating of shortwave radio bands, day and night.

- `GET /v1/propagation`: Space weather and band conditions. Example: `/v1/propagation?utm_medium=skill`

More: OpenAPI at https://api.dharun.dev/openapi.json, guides at https://api.dharun.dev/guides, the full documentation in one file at https://api.dharun.dev/llms-full.txt.
