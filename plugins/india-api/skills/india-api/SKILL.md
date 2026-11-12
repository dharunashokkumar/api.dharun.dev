---
name: india-api
description: Indian reference data from the free India API (api.dharun.dev): pincodes, IFSC and bank branches, GSTIN and PAN checks, holidays and working days, festivals, RTO codes, NSE and BSE companies and logos, mutual fund NAVs and returns, gold, silver and RBI rates, HSN codes, UPI QR codes, numbers in lakh and crore, money calculators (SIP, EMI, GST, income tax), transliteration, panchang, GST invoices as PDF, reading invoices, receipts, cheques and bank statements, PDF to Markdown, Aadhaar masking, OCR in Indian languages, PDF and image tools, speech in and out, translation for 22 Indian languages, Roman typing to Indian scripts, moderation including Hinglish, embeddings and reranking, a small English LLM (OpenAI-compatible chat), Census 2011, STD codes, card BIN lookup, and more. Use when an app needs Indian data, Indian-language AI or document tools, or the user asks about an Indian pincode, IFSC, GSTIN, PAN, holiday, vehicle code, listed company, income tax or EMI. No API key needed for most endpoints.
license: MIT-0
metadata:
  homepage: https://api.dharun.dev
  source: https://github.com/dharunashokkumar/api.dharun.dev
---

# India API

Free JSON APIs for Indian data at https://api.dharun.dev: pincodes, IFSC and bank branches, GSTIN and PAN format checks, holidays and working days, RTO codes, company logos, mutual fund NAVs, HSN codes, UPI QR codes, Indian number words and more. No API key needed for most endpoints, CORS open. A free key from https://api.dharun.dev/signup, sent as `Authorization: Bearer <key>`, raises the limits; the model endpoints (OCR, speech, translation, embeddings, moderation, Roman typing, background removal) will need one after a notice period: their answers carry an `X-Key-Notice` header until then. Open source (https://github.com/dharunashokkumar/api.dharun.dev).

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

Each API's endpoints, methods, parameters, enums and examples are in `references/<id>.md`; read it before calling an API you have not used.

### Money and business

- **Company logos** ([references/logos.md](references/logos.md)): Logos and details for companies listed on NSE and BSE. `/v1/logos/{symbol}`, `/v1/companies/{symbol}`, `/v1/companies`, `/v1/sectors`. Example: `/v1/logos/TCS?utm_medium=skill`
- **Brand, stock and asset logos** ([references/world-logos.md](references/world-logos.md)): Logos for unlisted Indian brands, global stocks, crypto, indices, ETFs and commodities. `/v1/brand-logos/{name}`, `/v1/brand-logos`, `/v1/stock-logos/{symbol}`, `/v1/stock-logos`, `/v1/asset-logos/{symbol}`, `/v1/asset-logos`. Example: `/v1/brand-logos/phonepe?utm_medium=skill`
- **IFSC and banks** ([references/ifsc.md](references/ifsc.md)): IFSC to bank and branch. `/v1/ifsc/{code}`, `/v1/ifsc`, `/v1/banks/{code}`, `/v1/banks`, `/v1/micr/{code}`, `/v1/swift/{code}`. Example: `/v1/ifsc/SBIN0000001?utm_medium=skill`
- **QR codes and barcodes** ([references/codes.md](references/codes.md)): QR codes, UPI payment QR codes and barcodes as SVG or PNG, and reading them from a photo. `/v1/qr`, `/v1/upi-qr`, `/v1/barcode/{type}`, `/v1/barcode/read`. Example: `/v1/qr?text=https://api.dharun.dev&format=png&size=256&utm_medium=skill`
- **HSN and SAC codes** ([references/hsn.md](references/hsn.md)): GST classification codes for goods and services. `/v1/hsn/{code}`, `/v1/hsn`. Example: `/v1/hsn/0902?utm_medium=skill`
- **Mutual fund NAVs** ([references/mutual-funds.md](references/mutual-funds.md)): Latest net asset value of every Indian mutual fund scheme. `/v1/mutual-funds/{code}`, `/v1/mutual-funds`, `/v1/mutual-funds/fund-houses`, `/v1/mutual-funds/{code}/returns`, `/v1/mutual-funds/{code}/history`. Example: `/v1/mutual-funds/122639?utm_medium=skill`
- **Number to words** ([references/words.md](references/words.md)): Numbers and rupee amounts in words, the Indian way. `/v1/words/{number}`. Example: `/v1/words/1,25,000.50?style=currency&utm_medium=skill`
- **Format checks** ([references/validate.md](references/validate.md)): Check the format of a GSTIN, PAN or IFSC. `/v1/validate/{kind}/{value}`. Example: `/v1/validate/gstin/27AAPFU0939F1ZV?utm_medium=skill`
- **GST invoice** ([references/invoice.md](references/invoice.md)): A GST tax invoice as a PDF from JSON, with the tax worked out. `/v1/invoice`. Example: `/v1/invoice?utm_medium=skill`
- **Money calculators** ([references/calculators.md](references/calculators.md)): SIP, EMI, GST, income tax, HRA, gratuity, PPF, capital gains and trade charges. `/v1/calc/sip`, `/v1/calc/lumpsum`, `/v1/calc/emi`, `/v1/calc/gst`, `/v1/calc/income-tax`, `/v1/calc/hra`, `/v1/calc/gratuity`, `/v1/calc/ppf`, `/v1/calc/capital-gains`, `/v1/calc/trade`. Example: `/v1/calc/sip?monthly=10000&rate=12&years=10&utm_medium=skill`
- **Indian number formatting** ([references/format.md](references/format.md)): Lakh and crore grouping, 1.2 Cr short forms, rupees and Indian-script digits. `/v1/format/number/{number}`. Example: `/v1/format/number/123456789.5?utm_medium=skill`
- **UPI handles and payment QR codes** ([references/upi.md](references/upi.md)): Which app and bank is behind @okaxis, and what a UPI or Bharat QR code says. `/v1/upi-handles`, `/v1/upi-handles/{handle}`, `/v1/qr/parse`. Example: `/v1/upi-handles?utm_medium=skill`
- **RBI rates and exchange rates** ([references/rates.md](references/rates.md)): Repo rate, CRR and SLR, FBIL reference exchange rates, lending and deposit rates, and bond yields. `/v1/rates`, `/v1/fx`. Example: `/v1/rates?utm_medium=skill`
- **Derivative expiries** ([references/expiries.md](references/expiries.md)): NSE and BSE futures and options expiry dates, moved for holidays. `/v1/expiries`. Example: `/v1/expiries?exchange=nse&year=2026&month=3&utm_medium=skill`
- **Gold and silver rates** ([references/bullion.md](references/bullion.md)): Today's gold (24K, 22K, 18K...), silver and platinum rates from IBJA, and their history. `/v1/bullion`, `/v1/bullion/history`. Example: `/v1/bullion?utm_medium=skill`
- **Index constituents** ([references/indices.md](references/indices.md)): Which companies are in Nifty 50, Bank Nifty, Nifty 500 and 28 more NSE indices, and which indices a stock is in. `/v1/indices`, `/v1/indices/{id}`. Example: `/v1/indices?symbol=HDFCBANK&utm_medium=skill`
- **Mandi prices** ([references/mandi.md](references/mandi.md)): Today's wholesale prices of vegetables, fruit, grain and pulses in India's markets, from Agmarknet. `/v1/mandi`, `/v1/mandi/commodities`. Example: `/v1/mandi?commodity=Tomato&state=Tamil%20Nadu&utm_medium=skill`
- **Card BIN lookup** ([references/bin.md](references/bin.md)): Which network, card type and bank a card's first six digits belong to: Visa, Mastercard, RuPay, debit or credit. `/v1/bin/{prefix}`. Example: `/v1/bin/605067?utm_medium=skill`

### Places and transport

- **Pincodes** ([references/pincode.md](references/pincode.md)): Pincode to post office, district and state. `/v1/pincode/{pincode}`, `/v1/pincode`, `/v1/pincode/near`. Example: `/v1/pincode/637001?utm_medium=skill`
- **RTO codes** ([references/rto.md](references/rto.md)): Vehicle registration code to transport office. `/v1/rto/{code}`, `/v1/rto`. Example: `/v1/rto/TN-28?utm_medium=skill`
- **Railway stations** ([references/stations.md](references/stations.md)): Indian Railways station codes to stations, and back. `/v1/stations/{code}`, `/v1/stations`, `/v1/stations/zones`. Example: `/v1/stations/MAS?utm_medium=skill`
- **Constituencies** ([references/constituency.md](references/constituency.md)): Which assembly and Lok Sabha constituency a place is in. `/v1/constituency`. Example: `/v1/constituency?pincode=637001&utm_medium=skill`
- **Airports** ([references/airports.md](references/airports.md)): Airport codes to airports, and airports by city. `/v1/airports/{code}`, `/v1/airports`. Example: `/v1/airports/MAA?utm_medium=skill`
- **States** ([references/states.md](references/states.md)): States and union territories with their codes. `/v1/states`, `/v1/states/{code}`. Example: `/v1/states?utm_medium=skill`
- **Address reader** ([references/address.md](references/address.md)): A free-text Indian address split into house, street, locality, city, state and pincode, checked. `/v1/address/parse`. Example: `/v1/address/parse?text=12%2C%20Anna%20Salai%2C%20Mount%20Road%2C%20Chennai%2C%20Tamil%20Nadu%20600002&utm_medium=skill`
- **Helplines** ([references/helplines.md](references/helplines.md)): National emergency and helpline numbers. `/v1/helplines`. Example: `/v1/helplines?q=cyber&utm_medium=skill`
- **Air quality** ([references/air-quality.md](references/air-quality.md)): The air quality index at CPCB's monitoring stations across India, updated hourly. `/v1/air-quality`. Example: `/v1/air-quality?city=Chennai&utm_medium=skill`
- **Villages and LGD codes** ([references/lgd.md](references/lgd.md)): Every state, district, sub-district and village of India with its LGD and Census 2011 codes, and village PIN codes. `/v1/lgd/states`, `/v1/lgd/districts`, `/v1/lgd/subdistricts`, `/v1/lgd/villages`, `/v1/lgd/villages/{code}`. Example: `/v1/lgd/states?utm_medium=skill`
- **Hospitals** ([references/hospitals.md](references/hospitals.md)): About 30,000 hospitals across India with their kind, system of medicine, phone and position, from the National Hospital Directory. `/v1/hospitals`. Example: `/v1/hospitals?pincode=637001&utm_medium=skill`
- **Census 2011 figures** ([references/census.md](references/census.md)): Population, households, literacy, sex ratio and workers for India, every state, district, sub-district and town, from the 2011 census. `/v1/census/{level}/{code}`, `/v1/census`. Example: `/v1/census/district/632?utm_medium=skill`

### Dates and calendars

- **Holidays** ([references/holidays.md](references/holidays.md)): Bank, government and stock market holidays. `/v1/holidays/{year}`, `/v1/holidays/{year}.ics`. Example: `/v1/holidays/2026?type=market&utm_medium=skill`
- **Working days** ([references/working-days.md](references/working-days.md)): Bank, market, settlement and government working days. `/v1/working-days/{date}`, `/v1/working-days`, `/v1/market/status`. Example: `/v1/working-days/2026-01-16?calendar=bank&state=TN&add=3&utm_medium=skill`
- **Panchang** ([references/panchang.md](references/panchang.md)): Hindu almanac for any date and place: tithi, nakshatra, Rahu kalam and more. `/v1/panchang/{date}`. Example: `/v1/panchang/2026-01-14?pincode=600001&utm_medium=skill`
- **Sun and moon** ([references/sun-moon.md](references/sun-moon.md)): Sunrise, sunset, twilight, moonrise and the moon's phase for any date and place. `/v1/sun-moon/{date}`. Example: `/v1/sun-moon/2026-10-14?pincode=600001&utm_medium=skill`
- **Today in India** ([references/today.md](references/today.md)): India's "now" in one call: date, financial year, working days, holidays, market and panchang. `/v1/today`. Example: `/v1/today?state=TN&pincode=600001&utm_medium=skill`
- **Indian date tools** ([references/dates.md](references/dates.md)): Financial and assessment year, quarter, the Saka, Tamil and Hijri dates, and ages. `/v1/dates/{date}`, `/v1/dates/diff`. Example: `/v1/dates/2026-11-11?utm_medium=skill`
- **Festivals** ([references/festivals.md](references/festivals.md)): Hindu, Tamil, Sikh, Jain, Buddhist, Islamic and Christian festival dates for any year, computed. `/v1/festivals/{year}`, `/v1/festivals/{year}.ics`. Example: `/v1/festivals/2026?month=11&utm_medium=skill`

### Language and text

- **Thirukkural** ([references/thirukkural.md](references/thirukkural.md)): All 1,330 kurals in Tamil with English translations. `/v1/thirukkural/{number}`, `/v1/thirukkural`, `/v1/thirukkural/today`, `/v1/thirukkural/random`, `/v1/thirukkural/chapters`, `/v1/thirukkural/chapters/{number}`. Example: `/v1/thirukkural/1?utm_medium=skill`
- **Text tools** ([references/text.md](references/text.md)): Chunk documents, redact identifiers and names, moderate, count tokens, detect language. `/v1/text/chunk`, `/v1/text/redact`, `/v1/text/tokens`, `/v1/text/language`, `/v1/text/moderate`. Example: `/v1/text/chunk?size=200&overlap=20&text=%23+Setup%0A%0AInstall+Go.+Clone+the+repository.%0A%0A%23+Deploy%0A%0APush+to+main.&utm_medium=skill`
- **Transliteration** ([references/transliterate.md](references/transliterate.md)): Rewrite text between Indian scripts and Roman letters, or turn Roman typing into an Indian script. `/v1/transliterate`, `/v1/transliterate/suggest`. Example: `/v1/transliterate?to=tamil&text=%E0%A4%A8%E0%A4%AE%E0%A4%B8%E0%A5%8D%E0%A4%A4%E0%A5%87+%E0%A4%AD%E0%A4%BE%E0%A4%B0%E0%A4%A4&utm_medium=skill`
- **Tamil legacy encodings** ([references/tamil-font.md](references/tamil-font.md)): Old Tamil text that shows as accented Latin letters, to Unicode and back. `/v1/tamil-font/convert`, `/v1/tamil-font/encodings`. Example: `/v1/tamil-font/convert?from=tscii&text=%C2%BE%C3%81%C2%A2%C3%BA+%C3%85%C2%BD%C3%AC%C2%B8%C3%B5&utm_medium=skill`
- **Speech** ([references/speech.md](references/speech.md)): Speech to text, text to speech in Hindi, Telugu, Malayalam and English, and noise removal. `/v1/stt`, `/v1/tts`, `/v1/tts/voices`, `/v1/audio/denoise`. Example: `/v1/stt?utm_medium=skill`
- **Embeddings and reranking** ([references/embeddings.md](references/embeddings.md)): Vectors for semantic search in Indian languages and English, and a reranker to order results. `/v1/embed`, `/v1/rerank`. Example: `/v1/embed?type=query&text=How+do+I+renew+my+passport%3F&utm_medium=skill`
- **Translation** ([references/translate.md](references/translate.md)): Translate between English and 22 Indian languages with IndicTrans2. `/v1/translate`, `/v1/translate/languages`. Example: `/v1/translate?from=en&to=ta&text=The+train+to+Chennai+leaves+at+6+pm.+Please+reach+the+station+early.&utm_medium=skill`
- **Indic text tools** ([references/indic.md](references/indic.md)): Letters as readers count them, dictionary order, look-alike fixes, braille and keyboard layouts. `/v1/indic/aksharas`, `/v1/indic/sort`, `/v1/indic/normalise`, `/v1/braille`, `/v1/keyboard/convert`. Example: `/v1/indic/aksharas?text=%E0%A4%95%E0%A5%8D%E0%A4%B7%E0%A4%A4%E0%A5%8D%E0%A4%B0%E0%A4%BF%E0%A4%AF&utm_medium=skill`
- **Search your own documents** ([references/rag.md](references/rag.md)): Upload documents, ask a question, get the best passages back: retrieval for your own AI app. `/v1/rag`, `/v1/rag/{id}/documents`, `/v1/rag/{id}/query`, `/v1/rag/{id}/delete`. Example: `/v1/rag?utm_medium=skill`
- **IPC to BNS section finder** ([references/law.md](references/law.md)): Which section of the Bharatiya Nyaya Sanhita replaced IPC 302, and the same for CrPC to BNSS and the Evidence Act to BSA. `/v1/law/{law}/{section}`, `/v1/law/{law}`. Example: `/v1/law/ipc/302?utm_medium=skill`
- **Indian classics** ([references/classics.md](references/classics.md)): Aathichoodi, the Bhagavad Gita in Sanskrit and Kabir's couplets, a line or verse at a time, with transliteration. `/v1/classics`, `/v1/classics/{text}`, `/v1/classics/{text}/{ref}`. Example: `/v1/classics?utm_medium=skill`
- **Small LLM (English)** ([references/llm.md](references/llm.md)): Summarise, classify, extract to JSON and rewrite English text with a small open model, and an OpenAI-compatible chat. `/v1/llm/summarise`, `/v1/llm/classify`, `/v1/llm/extract`, `/v1/llm/rewrite`, `/v1/chat/completions`, `/v1/models`. Example: `/v1/llm/summarise?utm_medium=skill`

### Network and contact checks

- **IP address location** ([references/ip.md](references/ip.md)): IP address to country, state, city and network. `/v1/ip/{address}`, `/v1/ip`. Example: `/v1/ip/49.204.0.1?utm_medium=skill`
- **Email checks** ([references/email.md](references/email.md)): Is an email address well formed, throwaway, and able to receive mail. `/v1/email/{address}`. Example: `/v1/email/someone@mailinator.com?utm_medium=skill`
- **DNS check** ([references/dns.md](references/dns.md)): A domain's DNS records at four public resolvers at once. `/v1/dns/{name}`. Example: `/v1/dns/dharun.dev?type=MX&utm_medium=skill`
- **Phone numbers** ([references/phone.md](references/phone.md)): Check and format an Indian phone number, and see whose series it is. `/v1/phone/{number}`. Example: `/v1/phone/+91%2095001%2012345?utm_medium=skill`
- **Traceroute from India** ([references/mtr.md](references/mtr.md)): The network path from a server in India to any host, with loss and latency per hop. `/v1/mtr/{host}`. Example: `/v1/mtr/1.1.1.1?utm_medium=skill`
- **Whois** ([references/whois.md](references/whois.md)): Who registered a domain, an IP network or an AS number, through RDAP. `/v1/whois/{query}`. Example: `/v1/whois/google.in?utm_medium=skill`
- **TLS check** ([references/tls.md](references/tls.md)): A website's certificate and TLS setup, checked from India. `/v1/tls/{host}`. Example: `/v1/tls/dharun.dev?utm_medium=skill`
- **IPv6 check** ([references/ipv6.md](references/ipv6.md)): Is a domain ready for IPv6: website, nameservers and mail. `/v1/ipv6/{domain}`. Example: `/v1/ipv6/dharun.dev?utm_medium=skill`
- **HTTP probe from India** ([references/probe.md](references/probe.md)): How fast a URL answers from India: DNS, connect, TLS, first byte and the CDN edge. `/v1/probe`. Example: `/v1/probe?url=https://dharun.dev&utm_medium=skill`
- **Website icons** ([references/icon.md](references/icon.md)): The icon of any website as a square PNG, for lists of links and bookmarks. `/v1/icon/{domain}`. Example: `/v1/icon/github.com?utm_medium=skill`
- **Web pages** ([references/web.md](references/web.md)): Link previews, a page as Markdown for agents, and HTML and Markdown to each other. `/v1/unfurl`, `/v1/read`, `/v1/markdown`. Example: `/v1/unfurl?url=https://dharun.dev&utm_medium=skill`
- **STD codes** ([references/std-codes.md](references/std-codes.md)): The landline STD code of every town and area in India, and which area a landline number belongs to. `/v1/std-codes/{code}`, `/v1/std-codes`. Example: `/v1/std-codes/0422?utm_medium=skill`

### Files, documents and media

- **OCR** ([references/ocr.md](references/ocr.md)): Text from photos, scans and scanned PDFs, in English and ten Indian languages. `/v1/ocr`. Example: `/v1/ocr?lang=eng+tam&utm_medium=skill`
- **Image tools** ([references/image-tools.md](references/image-tools.md)): Resize and convert pictures, remove backgrounds, and compare pictures by how they look. `/v1/image/convert`, `/v1/image/info`, `/v1/image/remove-background`, `/v1/image/hash`, `/v1/image/compare`, `/v1/image/thumbhash`, `/v1/image/icons`, `/v1/image/svg-to-png`, `/v1/image/watermark`. Example: `/v1/image/convert?format=webp&width=400&utm_medium=skill`
- **PDF tools** ([references/pdf.md](references/pdf.md)): Merge, split, rotate and compress PDFs, read their text and draw their pages. `/v1/pdf/info`, `/v1/pdf/text`, `/v1/pdf/merge`, `/v1/pdf/pages`, `/v1/pdf/rotate`, `/v1/pdf/compress`, `/v1/pdf/render`, `/v1/pdf/watermark`, `/v1/pdf/page-numbers`, `/v1/pdf/from-images`, `/v1/pdf/unlock`. Example: `/v1/pdf/info?utm_medium=skill`
- **Typst to PDF** ([references/typst.md](references/typst.md)): Turn a Typst document into a PDF, with Indian scripts set correctly. `/v1/typst`. Example: `/v1/typst?text=%23set%20page%28width%3A%2012cm%2C%20height%3A%20auto%2C%20margin%3A%201cm%29%0A=%20வணக்கம்%2C%20India%20API%0ATypst%20sets%20%2ATamil%2A%2C%20हिन्दी%20and%20English%20on%20one%20page.&utm_medium=skill`
- **Audio and video tools** ([references/media.md](references/media.md)): Convert, trim, draw a waveform or take a frame, with ffmpeg. `/v1/media/convert`, `/v1/media/trim`, `/v1/media/waveform`, `/v1/media/thumbnail`. Example: `/v1/media/convert?format=wav&utm_medium=skill`
- **Document reading** ([references/documents.md](references/documents.md)): PDFs and scans to Markdown and tables; invoices, receipts, cheques and bank statements read and checked; Aadhaar masking and redaction. `/v1/doc/markdown`, `/v1/doc/tables`, `/v1/doc/invoice`, `/v1/doc/einvoice-qr`, `/v1/doc/mask-aadhaar`, `/v1/doc/receipt`, `/v1/doc/cheque`, `/v1/doc/statement`, `/v1/doc/redact`. Example: `/v1/doc/markdown?utm_medium=skill`

### Developer tools

- **Regex tester** ([references/regex.md](references/regex.md)): Run a regular expression against a text and see every match, explained. `/v1/regex`. Example: `/v1/regex?flags=i&pattern=%28%3F%3Cstate%3E%5BA-Z%5D%7B2%7D%29%5B+-%5D%3F%28%5Cd%7B2%7D%29&text=Cars+TN+37%2C+ka-05+and+MH12&utm_medium=skill`
- **Fake test data** ([references/fake.md](references/fake.md)): Made-up Indian people, companies and addresses for testing apps. `/v1/fake/{kind}`. Example: `/v1/fake/people?count=3&state=TN&seed=42&utm_medium=skill`
- **Generated images** ([references/images.md](references/images.md)): Share images, code screenshots, avatars, placeholders and charts from a URL. `/v1/og-image`, `/v1/code-image`, `/v1/avatar/{name}`, `/v1/placeholder/{size}`, `/v1/chart`. Example: `/v1/og-image?title=Free%20APIs%20for%20Indian%20developers&subtitle=Pincodes%2C%20IFSC%2C%20holidays%20and%20more&site=api.dharun.dev&utm_medium=skill`
- **Batch lookups** ([references/batch.md](references/batch.md)): Up to 1,000 lookups in one request. `/v1/batch`. Example: `/v1/batch?utm_medium=skill`
- **README badges** ([references/badges.md](references/badges.md)): Small SVG badges: market open or closed, the next holiday, a dataset's freshness, or your own. `/v1/badge`, `/v1/badge/{kind}`, `/v1/badge/dataset/{name}`. Example: `/v1/badge?label=made%20in&message=India&color=orange&utm_medium=skill`
- **Data format tools** ([references/data-tools.md](references/data-tools.md)): JSON, YAML, TOML, CSV and XML to each other; text and JSON differences; spreadsheets to JSON and back. `/v1/convert`, `/v1/diff`, `/v1/sheet/to-json`, `/v1/sheet/from-json`. Example: `/v1/convert?to=yaml&text=%7B%22name%22%3A%22India%20API%22%2C%22tags%22%3A%5B%22a%22%2C%22b%22%5D%7D&utm_medium=skill`
- **Developer utilities** ([references/dev-utils.md](references/dev-utils.md)): Cron in words, UUIDs and ULIDs, hashes and HMACs, JWT decoding, user agents and time zones. `/v1/cron`, `/v1/uuid`, `/v1/uuid/{id}`, `/v1/hash`, `/v1/jwt/decode`, `/v1/user-agent`, `/v1/time`. Example: `/v1/cron?expr=30%209%20*%20*%201-5&utm_medium=skill`
- **Colours** ([references/colors.md](references/colors.md)): Colour conversions, WCAG contrast checks and the palette of a picture. `/v1/color/{color}`, `/v1/color/contrast`, `/v1/color/palette`. Example: `/v1/color/ff9933?utm_medium=skill`
- **Request bins** ([references/hooks.md](references/hooks.md)): A private address that records what is sent to it, for testing webhooks. `/v1/hooks`, `/v1/hook/{id}`, `/v1/hooks/{id}`, `/v1/hooks/{id}/delete`. Example: `/v1/hooks?utm_medium=skill`
- **Throwaway databases** ([references/db.md](references/db.md)): A private SQLite database for a day, over HTTP. `/v1/db`, `/v1/db/{id}/query`, `/v1/db/{id}/delete`. Example: `/v1/db?utm_medium=skill`
- **MCP tools** ([references/mcp-tools.md](references/mcp-tools.md)): Find the right tool among the MCP server's, and connect to only the sets you need. `/v1/mcp-tools`. Example: `/v1/mcp-tools?q=gold%20rate&utm_medium=skill`
- **Agent notes** ([references/kv.md](references/kv.md)): A small private key-value store, for agents and scripts that must remember between runs. `/v1/kv`, `/v1/kv/{id}`, `/v1/kv/{id}/{key}`, `/v1/kv/{id}/{key}/delete`. Example: `/v1/kv?utm_medium=skill`

### Sky and radio

- **Satellite passes** ([references/satellites.md](references/satellites.md)): When the ISS and amateur radio satellites pass over you, and where they are now. `/v1/sat-pass`, `/v1/satellites/{id}`, `/v1/satellites`. Example: `/v1/sat-pass?sat=ISS&pincode=600001&utm_medium=skill`
- **HF propagation** ([references/propagation.md](references/propagation.md)): Space weather and a rough rating of shortwave radio bands, day and night. `/v1/propagation`. Example: `/v1/propagation?utm_medium=skill`

More: OpenAPI at https://api.dharun.dev/openapi.json, guides at https://api.dharun.dev/guides, the full documentation in one file at https://api.dharun.dev/llms-full.txt.
