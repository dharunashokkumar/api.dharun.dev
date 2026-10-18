// Client for the India API, https://api.dharun.dev/docs
// Generated from the API's catalog; do not edit. MIT-0: copy it into anything.
// https://github.com/dharunashokkumar/api.dharun.dev/tree/main/clients/js

export const CATALOG = "100f2618e8c4";

const isNode = typeof process !== "undefined" && !!(process.versions && process.versions.node);

/** An answer other than 2xx. code is the API's error code, such as not_found or rate_limited. */
export class IndiaAPIError extends Error {
  constructor(status, code, message) {
    super(message || "HTTP " + status);
    this.name = "IndiaAPIError";
    this.status = status;
    this.code = code;
  }
}

const enc = (value) => encodeURIComponent(String(value));

async function fileBody(file) {
  if (typeof file === "string") {
    if (!isNode) throw new TypeError("A file path works only in Node; pass a Blob, File, Uint8Array or ArrayBuffer.");
    const { readFile } = await import("node:fs/promises");
    const { basename } = await import("node:path");
    return { blob: new Blob([await readFile(file)]), name: basename(file) };
  }
  if (typeof Blob !== "undefined" && file instanceof Blob) return { blob: file, name: file.name || "file" };
  if (file instanceof ArrayBuffer || ArrayBuffer.isView(file)) return { blob: new Blob([file]), name: "file" };
  throw new TypeError("Pass a file as a path (Node), Blob, File, Uint8Array or ArrayBuffer.");
}

export class IndiaAPI {
  /**
   * @param {object} [options]
   * @param {string} [options.baseUrl] Default https://api.dharun.dev.
   * @param {string} [options.token] A sandbox token (only the sandbox services need one).
   * @param {Record<string,string>} [options.headers] Sent with every request.
   * @param {typeof fetch} [options.fetch] A fetch to use instead of the global one.
   */
  constructor(options = {}) {
    this.baseUrl = (options.baseUrl || "https://api.dharun.dev").replace(/\/+$/, "");
    this.fetch = options.fetch || globalThis.fetch.bind(globalThis);
    this.headers = { ...(options.headers || {}) };
    if (options.token) this.headers.Authorization = "Bearer " + options.token;
  }

  /**
   * Any request. JSON answers resolve to the { data, meta } envelope; files (images,
   * PDFs, audio) to { contentType, bytes, headers }. Other than 2xx throws IndiaAPIError.
   */
  async request(method, path, { query, body, contentType, headers } = {}) {
    const url = new URL(this.baseUrl + path);
    for (const [name, value] of Object.entries(query || {})) {
      if (value !== undefined && value !== null) url.searchParams.set(name, String(value));
    }
    const init = { method, headers: { Accept: "application/json, */*", ...this.headers, ...(headers || {}) } };
    if (isNode) init.headers["User-Agent"] = "india-api-js/" + CATALOG;
    if (body !== undefined && body !== null) {
      if (typeof FormData !== "undefined" && body instanceof FormData) {
        init.body = body;
      } else if (typeof body === "string") {
        init.body = body;
        init.headers["Content-Type"] = contentType || "text/plain; charset=utf-8";
      } else if ((typeof Blob !== "undefined" && body instanceof Blob) || body instanceof ArrayBuffer || ArrayBuffer.isView(body)) {
        init.body = body;
        init.headers["Content-Type"] = contentType || body.type || "application/octet-stream";
      } else {
        init.body = JSON.stringify(body);
        init.headers["Content-Type"] = "application/json";
      }
    }
    const response = await this.fetch(url, init);
    const type = response.headers.get("content-type") || "";
    if (type.startsWith("application/json")) {
      const decoded = await response.json();
      if (!response.ok) {
        const error = (decoded && decoded.error) || {};
        throw new IndiaAPIError(response.status, error.code, error.message);
      }
      return decoded;
    }
    if (!response.ok) throw new IndiaAPIError(response.status, "http_" + response.status, await response.text());
    return { contentType: type, bytes: new Uint8Array(await response.arrayBuffer()), headers: response.headers };
  }

  /** GET any path, such as "/v1/pincode/600001". */
  get(path, query) {
    return this.request("GET", path, { query });
  }

  /** POST to any path: a string as text, an object as JSON, bytes or a Blob as they are. */
  post(path, body, query) {
    return this.request("POST", path, { body, query });
  }

  async _upload(path, file, query) {
    const { blob, name } = await fileBody(file);
    const form = new FormData();
    form.append("file", blob, name);
    return this.request("POST", path, { body: form, query });
  }

  async _uploadMany(path, files, query) {
    const form = new FormData();
    for (const file of files) {
      const { blob, name } = await fileBody(file);
      form.append("file", blob, name);
    }
    return this.request("POST", path, { body: form, query });
  }

  /**
   * Logo image for a listed company. Returns the image itself, ready for an <img> tag. A
   * listed company without a published logo gets an initials tile, marked with the
   * X-Logo-Fallback: monogram header. Unknown symbols return 404 unless fallback=monogram
   * is set.
   * https://api.dharun.dev/docs/logos
   */
  logosBySymbol(symbol, params = {}) {
    return this.request("GET", `/v1/logos/${enc(symbol)}`, { query: params });
  }

  /**
   * Details of a listed company. Name, sector, industry, ISIN, market capitalisation,
   * index memberships and logo URLs. listed is false for a delisted company (with
   * delistedOn, the date it was first missed); aliases holds earlier tickers; hasLogo is
   * false when the logo URL serves an initials tile.
   * https://api.dharun.dev/docs/logos
   */
  companiesBySymbol(symbol, params = {}) {
    return this.request("GET", `/v1/companies/${enc(symbol)}`, { query: params });
  }

  /**
   * Search listed companies. Searches ticker, earlier tickers, company name, sector,
   * industry, ISIN and index names. Best match first. Delisted companies are left out
   * unless delisted=true.
   * https://api.dharun.dev/docs/logos
   */
  companies(params = {}) {
    return this.request("GET", `/v1/companies`, { query: params });
  }

  /**
   * Post offices for a pincode. Returns the district and state of the pincode and every
   * post office that uses it.
   * https://api.dharun.dev/docs/pincode
   */
  pincodeByPincode(pincode, params = {}) {
    return this.request("GET", `/v1/pincode/${enc(pincode)}`, { query: params });
  }

  /**
   * Search post offices. Find post offices by the start of their name, by the start of a
   * pincode, or list a district.
   * https://api.dharun.dev/docs/pincode
   */
  pincode(params = {}) {
    return this.request("GET", `/v1/pincode`, { query: params });
  }

  /**
   * Post offices near a point. Post offices within a radius of a latitude and longitude,
   * nearest first, with the distance. Positions come from the Department of Posts and some
   * are approximate.
   * https://api.dharun.dev/docs/pincode
   */
  pincodeNear(params = {}) {
    return this.request("GET", `/v1/pincode/near`, { query: params });
  }

  /**
   * Branch for an IFSC.
   * https://api.dharun.dev/docs/ifsc
   */
  ifscByCode(code, params = {}) {
    return this.request("GET", `/v1/ifsc/${enc(code)}`, { query: params });
  }

  /**
   * Search a bank's branches. Lists branches of one bank, filtered by state, city or text
   * in the branch name or address.
   * https://api.dharun.dev/docs/ifsc
   */
  ifsc(params = {}) {
    return this.request("GET", `/v1/ifsc`, { query: params });
  }

  /**
   * Bank for a four-letter code.
   * https://api.dharun.dev/docs/ifsc
   */
  banksByCode(code, params = {}) {
    return this.request("GET", `/v1/banks/${enc(code)}`, { query: params });
  }

  /**
   * Search banks. Finds banks by name or code, largest branch network first.
   * https://api.dharun.dev/docs/ifsc
   */
  banks(params = {}) {
    return this.request("GET", `/v1/banks`, { query: params });
  }

  /**
   * Office for an RTO code.
   * https://api.dharun.dev/docs/rto
   */
  rtoByCode(code, params = {}) {
    return this.request("GET", `/v1/rto/${enc(code)}`, { query: params });
  }

  /**
   * List or search RTO codes.
   * https://api.dharun.dev/docs/rto
   */
  rto(params = {}) {
    return this.request("GET", `/v1/rto`, { query: params });
  }

  /**
   * Holidays in a year.
   * https://api.dharun.dev/docs/holidays
   */
  holidaysByYear(year, params = {}) {
    return this.request("GET", `/v1/holidays/${enc(year)}`, { query: params });
  }

  /**
   * Is a date a working day. Whether the date is a working day and why not, the previous
   * and next working days, and with add=N the date N working days later (negative for
   * earlier). A T+1 settlement date is calendar=settlement&add=1.
   * https://api.dharun.dev/docs/working-days
   */
  workingDaysByDate(date, params = {}) {
    return this.request("GET", `/v1/working-days/${enc(date)}`, { query: params });
  }

  /**
   * Count working days between two dates. Counts working days from one date to another,
   * both included, and lists each day off with its reason. At most 366 days.
   * https://api.dharun.dev/docs/working-days
   */
  workingDays(params = {}) {
    return this.request("GET", `/v1/working-days`, { query: params });
  }

  /**
   * Is the stock market open. NSE equities session at a moment: pre_open, open, closing or
   * closed, the reason it is closed, and when it next opens. Without at, the current
   * moment.
   * https://api.dharun.dev/docs/working-days
   */
  marketStatus(params = {}) {
    return this.request("GET", `/v1/market/status`, { query: params });
  }

  /**
   * QR code for any text. Returns the image itself. Text up to about 2,900 bytes at error
   * correction L.
   * https://api.dharun.dev/docs/codes
   */
  qr(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/qr`, { query: params }) : this.request("POST", `/v1/qr`, { body, query: params });
  }

  /**
   * UPI payment QR code. A QR code any UPI app can scan to pay, built to NPCI's upi://pay
   * link format. format=json returns the link instead of the image. Nothing is checked
   * with a bank.
   * https://api.dharun.dev/docs/codes
   */
  upiQr(params = {}) {
    return this.request("GET", `/v1/upi-qr`, { query: params });
  }

  /**
   * Barcode. Linear barcodes (code128, code39, code93, ean13, ean8, itf, codabar) and 2D
   * codes (datamatrix, pdf417, aztec). Linear codes are sized with scale and height, 2D
   * codes with size. No human-readable text is printed under the bars.
   * https://api.dharun.dev/docs/codes
   */
  barcodeByType(type, params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/barcode/${enc(type)}`, { query: params }) : this.request("POST", `/v1/barcode/${enc(type)}`, { body, query: params });
  }

  /**
   * Read QR codes and barcodes from a picture. Finds every QR code in a photo or
   * screenshot, and Data Matrix, Aztec, EAN, UPC, Code 128, Code 39, Code 93, ITF and
   * Codabar codes. Not PDF417. Returns each code's kind, text and position. Options go in
   * the query string, or as form fields when the file is sent as multipart/form-data.
   * https://api.dharun.dev/docs/codes
   */
  barcodeRead(file, params = {}) {
    return this._upload(`/v1/barcode/read`, file, params);
  }

  /**
   * A code, its parents and children.
   * https://api.dharun.dev/docs/hsn
   */
  hsnByCode(code, params = {}) {
    return this.request("GET", `/v1/hsn/${enc(code)}`, { query: params });
  }

  /**
   * Search codes. Every word of q must appear in the description; digits search by code
   * prefix. Broader codes come first.
   * https://api.dharun.dev/docs/hsn
   */
  hsn(params = {}) {
    return this.request("GET", `/v1/hsn`, { query: params });
  }

  /**
   * A scheme and its NAV.
   * https://api.dharun.dev/docs/mutual-funds
   */
  mutualFundsByCode(code, params = {}) {
    return this.request("GET", `/v1/mutual-funds/${enc(code)}`, { query: params });
  }

  /**
   * Search schemes. Every word of q must appear in the scheme name, in any order. Only
   * active schemes unless all=true.
   * https://api.dharun.dev/docs/mutual-funds
   */
  mutualFunds(params = {}) {
    return this.request("GET", `/v1/mutual-funds`, { query: params });
  }

  /**
   * Fund houses.
   * https://api.dharun.dev/docs/mutual-funds
   */
  mutualFundsFundHouses(params = {}) {
    return this.request("GET", `/v1/mutual-funds/fund-houses`, { query: params });
  }

  /**
   * Location of an IP address.
   * https://api.dharun.dev/docs/ip
   */
  ipByAddress(address, params = {}) {
    return this.request("GET", `/v1/ip/${enc(address)}`, { query: params });
  }

  /**
   * Location of your own address.
   * https://api.dharun.dev/docs/ip
   */
  ip(params = {}) {
    return this.request("GET", `/v1/ip`, { query: params });
  }

  /**
   * Check an email address.
   * https://api.dharun.dev/docs/email
   */
  emailByAddress(address, params = {}) {
    return this.request("GET", `/v1/email/${enc(address)}`, { query: params });
  }

  /**
   * Look up DNS records.
   * https://api.dharun.dev/docs/dns
   */
  dnsByName(name, params = {}) {
    return this.request("GET", `/v1/dns/${enc(name)}`, { query: params });
  }

  /**
   * Station for a code.
   * https://api.dharun.dev/docs/stations
   */
  stationsByCode(code, params = {}) {
    return this.request("GET", `/v1/stations/${enc(code)}`, { query: params });
  }

  /**
   * Search stations. By name (an exact code comes first, then names starting with the
   * text), state or zone.
   * https://api.dharun.dev/docs/stations
   */
  stations(params = {}) {
    return this.request("GET", `/v1/stations`, { query: params });
  }

  /**
   * Railway zones.
   * https://api.dharun.dev/docs/stations
   */
  stationsZones(params = {}) {
    return this.request("GET", `/v1/stations/zones`, { query: params });
  }

  /**
   * Check a phone number.
   * https://api.dharun.dev/docs/phone
   */
  phoneByNumber(number, params = {}) {
    return this.request("GET", `/v1/phone/${enc(number)}`, { query: params });
  }

  /**
   * Constituencies of a place.
   * https://api.dharun.dev/docs/constituency
   */
  constituency(params = {}) {
    return this.request("GET", `/v1/constituency`, { query: params });
  }

  /**
   * Airport for a code.
   * https://api.dharun.dev/docs/airports
   */
  airportsByCode(code, params = {}) {
    return this.request("GET", `/v1/airports/${enc(code)}`, { query: params });
  }

  /**
   * Search airports. By name, city or code; airports with scheduled flights and larger
   * airports first.
   * https://api.dharun.dev/docs/airports
   */
  airports(params = {}) {
    return this.request("GET", `/v1/airports`, { query: params });
  }

  /**
   * A kural by number.
   * https://api.dharun.dev/docs/thirukkural
   */
  thirukkuralByNumber(number, params = {}) {
    return this.request("GET", `/v1/thirukkural/${enc(number)}`, { query: params });
  }

  /**
   * Search kurals. Finds kurals containing the text, in Tamil, the transliteration or
   * either translation; or lists a chapter or a paal.
   * https://api.dharun.dev/docs/thirukkural
   */
  thirukkural(params = {}) {
    return this.request("GET", `/v1/thirukkural`, { query: params });
  }

  /**
   * Kural of the day. The same kural for everyone on a day in India, a different one the
   * next day.
   * https://api.dharun.dev/docs/thirukkural
   */
  thirukkuralToday(params = {}) {
    return this.request("GET", `/v1/thirukkural/today`, { query: params });
  }

  /**
   * A random kural.
   * https://api.dharun.dev/docs/thirukkural
   */
  thirukkuralRandom(params = {}) {
    return this.request("GET", `/v1/thirukkural/random`, { query: params });
  }

  /**
   * All 133 chapters.
   * https://api.dharun.dev/docs/thirukkural
   */
  thirukkuralChapters(params = {}) {
    return this.request("GET", `/v1/thirukkural/chapters`, { query: params });
  }

  /**
   * One chapter.
   * https://api.dharun.dev/docs/thirukkural
   */
  thirukkuralChaptersByNumber(number, params = {}) {
    return this.request("GET", `/v1/thirukkural/chapters/${enc(number)}`, { query: params });
  }

  /**
   * Split a document into chunks. Splits at headings, paragraphs, sentences and words, in
   * that order of preference, so chunks stay readable. Markdown chunks never cross a
   * heading and carry their heading path. Each chunk is an exact slice of the input with
   * its character offsets.
   * https://api.dharun.dev/docs/text
   */
  textChunk(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/text/chunk`, { query: params }) : this.request("POST", `/v1/text/chunk`, { body, query: params });
  }

  /**
   * Redact Indian personal identifiers. Masks Aadhaar numbers (Verhoeff-checked), PAN,
   * GSTIN (checksum-checked), card numbers (Luhn-checked), Indian mobile numbers, email
   * addresses, UPI IDs, vehicle numbers, voter IDs, passport numbers and IP addresses.
   * Returns where each was found, never the values. With names=true a model also masks
   * people's names: IndicNER for Indian scripts, bert-base-NER for English. Street
   * addresses are not detected.
   * https://api.dharun.dev/docs/text
   */
  textRedact(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/text/redact`, { query: params }) : this.request("POST", `/v1/text/redact`, { body, query: params });
  }

  /**
   * Count tokens. Exact token counts with OpenAI tokenizers, by encoding or model name.
   * Other vendors do not publish their tokenizers, so their counts are not offered.
   * https://api.dharun.dev/docs/text
   */
  textTokens(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/text/tokens`, { query: params }) : this.request("POST", `/v1/text/tokens`, { body, query: params });
  }

  /**
   * Detect the language. Language and script, with a confidence. Covers about 80 languages
   * including Hindi, Bengali, Marathi, Gujarati, Punjabi, Tamil, Telugu, Kannada,
   * Malayalam, Odia and Urdu. Romanised Indian text (Hinglish, Tanglish) is flagged as
   * such.
   * https://api.dharun.dev/docs/text
   */
  textLanguage(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/text/language`, { query: params }) : this.request("POST", `/v1/text/language`, { body, query: params });
  }

  /**
   * Score text for abuse, Hinglish and Tanglish included. Three classifiers each score the
   * text from 0 to 1: a general multilingual one (toxic), one for Hindi typed in Roman
   * letters (profanity, harassment, hate_speech, caste, communal, regional, misogyny) and
   * one for Tamil typed in Roman letters (offensive). flagged says whether any score
   * reached the threshold. Up to 4,000 characters; the first 256 tokens are read. Scores
   * are a model's guess: use them to sort or to ask a person, not to decide alone.
   * https://api.dharun.dev/docs/text
   */
  textModerate(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/text/moderate`, { query: params }) : this.request("POST", `/v1/text/moderate`, { body, query: params });
  }

  /**
   * Test a regular expression. An invalid pattern answers 422 with code invalid_pattern
   * and the engine's message; a JavaScript pattern that runs out of time answers 422 with
   * code timeout. A group that took no part in a match has a null text, which is different
   * from an empty one.
   * https://api.dharun.dev/docs/regex
   */
  regex(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/regex`, { query: params }) : this.request("POST", `/v1/regex`, { body, query: params });
  }

  /**
   * Transliterate text.
   * https://api.dharun.dev/docs/transliterate
   */
  transliterate(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/transliterate`, { query: params }) : this.request("POST", `/v1/transliterate`, { body, query: params });
  }

  /**
   * Roman typing to an Indian script (type English, get Tamil). Guesses how words typed in
   * Roman letters are written in an Indian script with IndicXlit (AI4Bharat), giving up to
   * eight suggestions per word, best first, and the text with the best guess for each
   * word. 21 languages. Up to 60 words; anything that is not Roman letters is kept as it
   * is.
   * https://api.dharun.dev/docs/transliterate
   */
  transliterateSuggest(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/transliterate/suggest`, { query: params }) : this.request("POST", `/v1/transliterate/suggest`, { body, query: params });
  }

  /**
   * Convert Tamil text. Give the text as it looks without its font (the Windows-1252
   * characters), or POST the raw bytes as application/octet-stream. Converting to an 8-bit
   * encoding answers with the same kind of text, and with the bytes in base64. Bytes the
   * encoding does not assign are counted in undefined; characters it has no code for are
   * listed in unmapped.
   * https://api.dharun.dev/docs/tamil-font
   */
  tamilFontConvert(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/tamil-font/convert`, { query: params }) : this.request("POST", `/v1/tamil-font/convert`, { body, query: params });
  }

  /**
   * Encodings offered. The encodings that can be converted, with fonts that use each, to
   * help recognise what a text is in.
   * https://api.dharun.dev/docs/tamil-font
   */
  tamilFontEncodings(params = {}) {
    return this.request("GET", `/v1/tamil-font/encodings`, { query: params });
  }

  /**
   * Panchang for a date. For a pincode, a latitude and longitude, or (by default) New
   * Delhi. Times are in the time zone asked for.
   * https://api.dharun.dev/docs/panchang
   */
  panchangByDate(date, params = {}) {
    return this.request("GET", `/v1/panchang/${enc(date)}`, { query: params });
  }

  /**
   * Sun and moon for a date. For a pincode, a latitude and longitude, or (by default) New
   * Delhi. A twilight time is null where the sun does not sink that far, and moonrise or
   * moonset is null on the day each month the moon does not rise or set. The phase names a
   * principal phase (new moon, first quarter, full moon, last quarter) for the whole date
   * it falls on.
   * https://api.dharun.dev/docs/sun-moon
   */
  sunMoonByDate(date, params = {}) {
    return this.request("GET", `/v1/sun-moon/${enc(date)}`, { query: params });
  }

  /**
   * Trace the route to a host. Lists hops up to the one where the host answered, or one
   * past the last router that answered. A hop with 100% loss is common: many routers
   * forward traffic without answering probes. otherAddresses lists further routers seen at
   * the same hop, where traffic is balanced over several paths.
   * https://api.dharun.dev/docs/mtr
   */
  mtrByHost(host, params = {}) {
    return this.request("GET", `/v1/mtr/${enc(host)}`, { query: params });
  }

  /**
   * Look up a registration. A name below a registered domain (www.example.co.in) is looked
   * up as that domain (example.co.in). daysToExpiry counts from now. An IP lookup is
   * treated as private: not logged, not cached.
   * https://api.dharun.dev/docs/whois
   */
  whoisByQuery(query, params = {}) {
    return this.request("GET", `/v1/whois/${enc(query)}`, { query: params });
  }

  /**
   * Check a host's TLS. valid is false when any problem is an error: expired, not yet
   * valid, name_mismatch, self_signed, incomplete_chain, untrusted, weak_key or
   * sha1_signature. Warnings (expires_soon, old_versions, no_tls13, weak_cipher,
   * chain_order) leave it true. Each version is tried with its own handshake offering
   * every cipher Go supports; SSL 3 is not tested.
   * https://api.dharun.dev/docs/tls
   */
  tlsByHost(host, params = {}) {
    return this.request("GET", `/v1/tls/${enc(host)}`, { query: params });
  }

  /**
   * Check a domain's IPv6. ready is true when the site (the name, or www when the name has
   * no address) has a public IPv6 address, at least one nameserver does, and every mail
   * server does (a domain without mail servers passes that part). website.network names
   * the network of the first IPv6 address, from DB-IP.
   * https://api.dharun.dev/docs/ipv6
   */
  ipv6ByDomain(domain, params = {}) {
    return this.request("GET", `/v1/ipv6/${enc(domain)}`, { query: params });
  }

  /**
   * Time a URL from India. With rounds above 1 the URL is fetched that many times and
   * summary gives the fastest, median and slowest total; the other fields describe the
   * first round. A redirect into private address space, to another port or to anything
   * other than http or https is refused. The URL may carry a token, so the request is not
   * logged and the answer is not cached.
   * https://api.dharun.dev/docs/probe
   */
  probe(params = {}) {
    return this.request("GET", `/v1/probe`, { query: params });
  }

  /**
   * Icon for a website. The X-Icon-Source header names the file the icon was drawn from,
   * the first time it is fetched. An icon already fetched counts as 1 request against the
   * rate limit; one that has to be fetched counts as 20.
   * https://api.dharun.dev/docs/icon
   */
  iconByDomain(domain, params = {}) {
    return this.request("GET", `/v1/icon/${enc(domain)}`, { query: params });
  }

  /**
   * Passes over a place. Passes from now for the next days, soonest first. Directions are
   * compass points and azimuths in degrees from true north; elevations are degrees above
   * the horizon. A satellite can be named by catalogue number, by name, by the designation
   * in its name's brackets (SO-50, AO-7), or as ISS, CSS or QO-100.
   * https://api.dharun.dev/docs/satellites
   */
  satPass(params = {}) {
    return this.request("GET", `/v1/sat-pass`, { query: params });
  }

  /**
   * Where a satellite is now. The point on Earth below the satellite at the moment of the
   * request, its altitude and speed, whether it is in sunlight, and how old its orbit
   * elements are.
   * https://api.dharun.dev/docs/satellites
   */
  satellitesById(id, params = {}) {
    return this.request("GET", `/v1/satellites/${enc(id)}`, { query: params });
  }

  /**
   * Tracked satellites. Every tracked satellite with its catalogue number, groups, orbital
   * period and inclination.
   * https://api.dharun.dev/docs/satellites
   */
  satellites(params = {}) {
    return this.request("GET", `/v1/satellites`, { query: params });
  }

  /**
   * Space weather and band conditions. Rules behind the band ratings: 80m-40m: fair by day
   * (D-layer absorption), good at night. 30m-20m: good by day; at night good from a solar
   * flux of 120, fair from 80, else poor. 17m-15m: by day good from a solar flux of 110,
   * fair from 80, else poor; at night fair from 150, else poor. 12m-10m: by day good from
   * a solar flux of 150, fair from 110, else poor; at night fair from 200, else poor. A
   * K-index of 4 lowers every night rating one step; 5 or more lowers every rating one
   * step, and 7 or more two. An M-class X-ray flux lowers the day rating of 80m-40m and
   * …
   * https://api.dharun.dev/docs/propagation
   */
  propagation(params = {}) {
    return this.request("GET", `/v1/propagation`, { query: params });
  }

  /**
   * Generate fake records. people have a name, gender, birth date, email (on example
   * domains), phone, PAN and address; companies a name, PAN, GSTIN, email, phone and
   * address.
   * https://api.dharun.dev/docs/fake
   */
  fakeByKind(kind, params = {}) {
    return this.request("GET", `/v1/fake/${enc(kind)}`, { query: params });
  }

  /**
   * Social preview image. A 1200 by 630 card for og:image and twitter:image: a title (set
   * as large as fits in three lines, cut with an ellipsis beyond that), an optional
   * subtitle of up to two lines, and a site name and tag along the bottom. The same URL
   * always gives the same picture, so it can be used directly in a meta tag.
   * https://api.dharun.dev/docs/images
   */
  ogImage(params = {}) {
    return this.request("GET", `/v1/og-image`, { query: params });
  }

  /**
   * Code as an image. A code snippet as a window with syntax colouring. Up to 150 lines of
   * 140 characters. PNGs are drawn at twice the pixel density for sharp text (very large
   * ones at single). The language is guessed from the title's file extension or the code
   * when not given, and reported in the X-Code-Language header.
   * https://api.dharun.dev/docs/images
   */
  codeImage(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/code-image`, { query: params }) : this.request("POST", `/v1/code-image`, { body, query: params });
  }

  /**
   * Spell out a number.
   * https://api.dharun.dev/docs/words
   */
  wordsByNumber(number, params = {}) {
    return this.request("GET", `/v1/words/${enc(number)}`, { query: params });
  }

  /**
   * Check an identifier's format.
   * https://api.dharun.dev/docs/validate
   */
  validateByKindByValue(kind, value, params = {}) {
    return this.request("GET", `/v1/validate/${enc(kind)}/${enc(value)}`, { query: params });
  }

  /**
   * All states and union territories.
   * https://api.dharun.dev/docs/states
   */
  states(params = {}) {
    return this.request("GET", `/v1/states`, { query: params });
  }

  /**
   * One state.
   * https://api.dharun.dev/docs/states
   */
  statesByCode(code, params = {}) {
    return this.request("GET", `/v1/states/${enc(code)}`, { query: params });
  }

  /**
   * Read the text in a picture or PDF. Returns the text, Tesseract's confidence and, with
   * words=true, every word with its box. A PDF is read up to 10 pages at a time at 300
   * dpi; a PDF that already holds text is read faster by /v1/pdf/text. Options go in the
   * query string, or as form fields when the file is sent as multipart/form-data.
   * https://api.dharun.dev/docs/ocr
   */
  ocr(file, params = {}) {
    return this._upload(`/v1/ocr`, file, params);
  }

  /**
   * Resize and convert a picture. Returns the picture itself, turned upright from its EXIF
   * orientation, without metadata. fit=inside keeps the whole picture within width and
   * height and never enlarges it; cover fills the box and crops the middle; fill
   * stretches. Options go in the query string, or as form fields when the file is sent as
   * multipart/form-data.
   * https://api.dharun.dev/docs/image-tools
   */
  imageConvert(file, params = {}) {
    return this._upload(`/v1/image/convert`, file, params);
  }

  /**
   * Size, format and metadata of a picture. Format, size, colour space, pages,
   * orientation, the camera, and whether the picture carries EXIF data or a GPS position.
   * https://api.dharun.dev/docs/image-tools
   */
  imageInfo(file, params = {}) {
    return this._upload(`/v1/image/info`, file, params);
  }

  /**
   * Remove the background. Returns the picture as a PNG with the background transparent,
   * cut out by the IS-Net model. Works best on one clear subject: a product, a person, an
   * animal. Takes a few seconds; pictures larger than 4096 pixels are made smaller.
   * Options go in the query string, or as form fields when the file is sent as
   * multipart/form-data.
   * https://api.dharun.dev/docs/image-tools
   */
  imageRemoveBackground(file, params = {}) {
    return this._upload(`/v1/image/remove-background`, file, params);
  }

  /**
   * Perceptual hash of a picture. Three 64-bit fingerprints (average, difference and
   * DCT-based perceptual hashes) that stay nearly the same when a picture is resized,
   * recompressed or slightly edited. Pictures whose perceptual hashes differ by 10 bits or
   * fewer look alike.
   * https://api.dharun.dev/docs/image-tools
   */
  imageHash(file, params = {}) {
    return this._upload(`/v1/image/hash`, file, params);
  }

  /**
   * Compare two pictures. Send two pictures as form fields. Returns the bits by which each
   * hash differs, a score from 0 to 1, and whether they look like the same picture.
   * https://api.dharun.dev/docs/image-tools
   */
  imageCompare(files, params = {}) {
    return this._uploadMany(`/v1/image/compare`, files, params);
  }

  /**
   * Pages, size and details of a PDF. Page count, PDF version, title, author, dates, page
   * sizes (named when A4, Letter and so on), and whether it has forms, signatures or
   * attachments.
   * https://api.dharun.dev/docs/pdf
   */
  pdfInfo(file, params = {}) {
    return this._upload(`/v1/pdf/info`, file, params);
  }

  /**
   * Text of a PDF, page by page. The text of each page, in reading order, or laid out as
   * on the page with layout=true. needs_ocr is true when the pages hold almost no text, as
   * in a scan: send those to /v1/ocr. Options go in the query string, or as form fields
   * when the file is sent as multipart/form-data.
   * https://api.dharun.dev/docs/pdf
   */
  pdfText(file, params = {}) {
    return this._upload(`/v1/pdf/text`, file, params);
  }

  /**
   * Merge PDFs into one. Send two to twenty PDFs as form fields, in order; up to 25 MB
   * together. Returns one PDF.
   * https://api.dharun.dev/docs/pdf
   */
  pdfMerge(files, params = {}) {
    return this._uploadMany(`/v1/pdf/merge`, files, params);
  }

  /**
   * Pick, reorder or split pages. Returns a PDF of the pages named, in the order named; a
   * page may be named twice. To split a PDF, ask for each part in turn. Options go in the
   * query string, or as form fields when the file is sent as multipart/form-data.
   * https://api.dharun.dev/docs/pdf
   */
  pdfPages(file, params = {}) {
    return this._upload(`/v1/pdf/pages`, file, params);
  }

  /**
   * Rotate pages. Turns pages clockwise by a multiple of 90 degrees. Options go in the
   * query string, or as form fields when the file is sent as multipart/form-data.
   * https://api.dharun.dev/docs/pdf
   */
  pdfRotate(file, params = {}) {
    return this._upload(`/v1/pdf/rotate`, file, params);
  }

  /**
   * Make a PDF smaller. Removes duplicate fonts and images and packs the file's structure.
   * Pictures are not re-compressed, so a scanned PDF shrinks little. The original size is
   * in the X-Original-Bytes header; when nothing was gained, the original comes back.
   * https://api.dharun.dev/docs/pdf
   */
  pdfCompress(file, params = {}) {
    return this._upload(`/v1/pdf/compress`, file, params);
  }

  /**
   * Draw a page as a picture. Returns one page as PNG or JPEG, for previews and
   * thumbnails. Options go in the query string, or as form fields when the file is sent as
   * multipart/form-data.
   * https://api.dharun.dev/docs/pdf
   */
  pdfRender(file, params = {}) {
    return this._upload(`/v1/pdf/render`, file, params);
  }

  /**
   * Make a GST tax invoice. Fields: number (required, up to 16 letters, digits, - and /),
   * date and due_date (YYYY-MM-DD, date defaults to today), title (default Tax Invoice),
   * place_of_supply (state code, name or GST code; defaults to the buyer's state from the
   * GSTIN or state), reverse_charge, seller and buyer (name, address, gstin, state, phone,
   * email; the seller's GSTIN is required), ship_to, items (description, hsn, quantity
   * (default 1), unit (default Nos), rate before tax, discount as an amount, gst_rate in
   * percent), bank (name, account, ifsc, branch, holder), upi, notes and terms. Unknown
   * …
   * https://api.dharun.dev/docs/invoice
   */
  invoice(document = {}, params = {}) {
    return this.request("POST", `/v1/invoice`, { body: document, query: params });
  }

  /**
   * Compile Typst to PDF. Returns the PDF itself, or one page as an image. A mistake in
   * the document returns 422 with Typst's message, line and column.
   * https://api.dharun.dev/docs/typst
   */
  typst(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/typst`, { query: params }) : this.request("POST", `/v1/typst`, { body, query: params });
  }

  /**
   * Speech to text, with subtitles. Send a recording (MP3, WAV, Ogg, M4A, WebM, video
   * files too) and get the text with timed segments, as JSON, SRT or WebVTT subtitles, or
   * plain text. Whisper small recognises about a hundred languages, Indian ones included;
   * it is good on clear English and fair on Indian languages. task=translate gives English
   * text whatever was spoken. The first five minutes are heard. Options go in the query
   * string, or as form fields when the file is sent as multipart/form-data.
   * https://api.dharun.dev/docs/speech
   */
  stt(file, params = {}) {
    return this._upload(`/v1/stt`, file, params);
  }

  /**
   * Text to speech. Returns the audio itself; X-Voice and X-Voice-Licence headers name the
   * voice and its licence. Up to 3,000 characters; split longer text. Write the text in
   * its own script: Hindi in Devanagari, Telugu in Telugu, Malayalam in Malayalam.
   * https://api.dharun.dev/docs/speech
   */
  tts(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/tts`, { query: params }) : this.request("POST", `/v1/tts`, { body, query: params });
  }

  /**
   * Voices. Every voice with its language, speaker, licence and the credit the licence
   * asks for.
   * https://api.dharun.dev/docs/speech
   */
  ttsVoices(params = {}) {
    return this.request("GET", `/v1/tts/voices`, { query: params });
  }

  /**
   * Remove background noise. Send a recording (MP3, WAV, Ogg, M4A, WebM, video files too)
   * and get it back with fans, traffic and hum lowered, as mono 48 kHz. The first five
   * minutes are cleaned. Options go in the query string, or as form fields when the file
   * is sent as multipart/form-data.
   * https://api.dharun.dev/docs/speech
   */
  audioDenoise(file, params = {}) {
    return this._upload(`/v1/audio/denoise`, file, params);
  }

  /**
   * Text embeddings. Up to 64 texts a request (GET: one or more text parameters; POST:
   * JSON {"texts": [...], "type": ...}). Each comes back as 384 numbers scaled to length
   * 1, so a dot product is the cosine similarity. Embed questions with type=query and the
   * documents searched with type=passage, as the model was trained. Each text is read up
   * to 512 tokens.
   * https://api.dharun.dev/docs/embeddings
   */
  embed(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/embed`, { query: params }) : this.request("POST", `/v1/embed`, { body, query: params });
  }

  /**
   * Rerank documents for a query. Scores how well each document answers the query, from 0
   * to 1, and returns them best first with their place in the request. Up to 100 documents
   * of up to 8,000 characters; each pair is read up to 512 tokens. GET takes query and
   * repeated document parameters; POST takes JSON.
   * https://api.dharun.dev/docs/embeddings
   */
  rerank(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/rerank`, { query: params }) : this.request("POST", `/v1/rerank`, { body, query: params });
  }

  /**
   * Translate text. Translates a sentence at a time and returns the whole translation and
   * each sentence. Up to 2,000 characters and 30 sentences. Write Indian languages in
   * their own script.
   * https://api.dharun.dev/docs/translate
   */
  translate(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/translate`, { query: params }) : this.request("POST", `/v1/translate`, { body, query: params });
  }

  /**
   * Languages. Every language with its short code and FLORES-200 code; either can be used
   * as from and to.
   * https://api.dharun.dev/docs/translate
   */
  translateLanguages(params = {}) {
    return this.request("GET", `/v1/translate/languages`, { query: params });
  }

  /**
   * Languages, versions and limits. The languages the code image runs, with the version of
   * each, the session images, and every limit. Needs no token.
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxLanguages(params = {}) {
    return this.request("GET", `/v1/sandbox/languages`, { query: params });
  }

  /**
   * Run code once. language (python, javascript, go, c, cpp, ruby, php, bash, or an alias
   * such as py, js, node, golang, c++, sh), code (up to 64 KB), and optionally stdin, args
   * and files ([{path, content, encoding}] with encoding text or base64; up to 20 files of
   * 1 MB each, relative to the working directory /work). Returns stdout, stderr, exitCode,
   * durationMs, cpuMs and limitsHit (time, memory, output). Needs a sandbox token
   * (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxRun(document = {}, params = {}) {
    return this.request("POST", `/v1/sandbox/run`, { body: document, query: params });
  }

  /**
   * Start a session. image is code (default) or a distribution: debian, alpine, arch,
   * fedora. network: true lets a code session reach its languages' package registries
   * through a proxy (500 MB of downloads), and nothing else; distributions have no
   * network. Returns the session's id, when it expires and links to exec, run, files, stop
   * and tty. An account may have two sessions at once. Needs a sandbox token
   * (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxSessions(document = {}, params = {}) {
    return this.request("POST", `/v1/sandbox/sessions`, { body: document, query: params });
  }

  /**
   * Describe a session. Its image, network and expiry. Needs a sandbox token
   * (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxSessionsById(id, params = {}) {
    return this.request("GET", `/v1/sandbox/sessions/${enc(id)}`, { query: params });
  }

  /**
   * Run a command in a session. command (a shell line) or argv (a list), and optionally
   * stdin, cwd (default /work), env and timeoutMs (up to 60 seconds). Returns the same as
   * a run. Counts as a run against the daily quota. Needs a sandbox token (Authorization:
   * Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxSessionsByIdExec(id, document = {}, params = {}) {
    return this.request("POST", `/v1/sandbox/sessions/${enc(id)}/exec`, { body: document, query: params });
  }

  /**
   * Run code in a session. As /v1/sandbox/run, in the session: files written before and
   * packages installed stay. Code sessions only. Needs a sandbox token (Authorization:
   * Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxSessionsByIdRun(id, document = {}, params = {}) {
    return this.request("POST", `/v1/sandbox/sessions/${enc(id)}/run`, { body: document, query: params });
  }

  /**
   * Put a file in a session. Writes the body, or the first file of a multipart form, to
   * path: relative to /work, or absolute. Up to 1 MB. Needs a sandbox token
   * (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxSessionsByIdUpload(id, file, params = {}) {
    return this._upload(`/v1/sandbox/sessions/${enc(id)}/upload`, file, params);
  }

  /**
   * Read a file from a session. Returns the file at path, up to 1 MB. A directory
   * (list=true, a path ending in /, or no path for /work) is listed as JSON: name, size,
   * dir and mode of each entry. Needs a sandbox token (Authorization: Bearer <token>; sign
   * in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxSessionsByIdDownload(id, params = {}) {
    return this.request("GET", `/v1/sandbox/sessions/${enc(id)}/download`, { query: params });
  }

  /**
   * Stop a session now. Ends the session and destroys its microVM and everything in it.
   * Needs a sandbox token (Authorization: Bearer <token>; sign in at POST
   * /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxSessionsByIdStop(id, document = {}, params = {}) {
    return this.request("POST", `/v1/sandbox/sessions/${enc(id)}/stop`, { body: document, query: params });
  }

  /**
   * Terminal of a session (WebSocket). A WebSocket: binary messages carry the terminal
   * both ways, and a text message {"cols": 120, "rows": 32} resizes it. Only for sessions
   * started from the terminal page at /terminal, which passes a browser check; programs
   * use exec. The token may be given as ?token=, as browsers cannot set headers on a
   * WebSocket. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST
   * /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox
   */
  sandboxSessionsByIdTty(id, params = {}) {
    return this.request("GET", `/v1/sandbox/sessions/${enc(id)}/tty`, { query: params });
  }

  /**
   * Start a GitHub sign-in. Returns userCode, verificationUri, deviceCode, interval
   * (seconds between polls) and expiresIn.
   * https://api.dharun.dev/docs/sandbox-account
   */
  sandboxLogin(document = {}, params = {}) {
    return this.request("POST", `/v1/sandbox/login`, { body: document, query: params });
  }

  /**
   * Finish a GitHub sign-in. status is pending until the person approves, slow_down when
   * polled too often, or ok with the token, its expiry and the account's quota. 403 when
   * the sign-in was refused or the code expired.
   * https://api.dharun.dev/docs/sandbox-account
   */
  sandboxLoginPoll(document = {}, params = {}) {
    return this.request("POST", `/v1/sandbox/login/poll`, { body: document, query: params });
  }

  /**
   * Today's quota. The login, the daily quota, what has been used today and what is left,
   * and when it resets. Needs a sandbox token (Authorization: Bearer <token>; sign in at
   * POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox-account
   */
  sandboxAccount(params = {}) {
    return this.request("GET", `/v1/sandbox/account`, { query: params });
  }

  /**
   * Revoke a token. The token stops working at once. Needs a sandbox token (Authorization:
   * Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/sandbox-account
   */
  sandboxLogout(document = {}, params = {}) {
    return this.request("POST", `/v1/sandbox/logout`, { body: document, query: params });
  }

  /**
   * Screenshot of a page. Returns the image. GET with the options in the query, or POST
   * JSON with url or html (up to 4 MB) and the same options.
   * https://api.dharun.dev/docs/browser
   */
  browserScreenshot(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/browser/screenshot`, { query: params }) : this.request("POST", `/v1/browser/screenshot`, { body, query: params });
  }

  /**
   * A page or HTML as PDF. Returns the PDF. The page's print styles apply. POST html to
   * make a PDF from your own HTML and CSS (images inline or at public URLs).
   * https://api.dharun.dev/docs/browser
   */
  browserPdf(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/browser/pdf`, { query: params }) : this.request("POST", `/v1/browser/pdf`, { body, query: params });
  }

  /**
   * Text of a page. url, title, language, the HTTP status and text: what a reader sees
   * (innerText), or with format=markdown the page as Markdown with headings, lists, links,
   * code and tables. 403 robots_disallowed when the site's robots.txt does not allow it.
   * https://api.dharun.dev/docs/browser
   */
  browserText(params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/browser/text`, { query: params }) : this.request("POST", `/v1/browser/text`, { body, query: params });
  }

  /**
   * Make a database. Returns name, user, password (shown only now), websocket, a
   * connectionString for the bridge, and expiresAt. Needs a sandbox token (Authorization:
   * Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/db
   */
  db(document = {}, params = {}) {
    return this.request("POST", `/v1/db`, { body: document, query: params });
  }

  /**
   * Describe a database. Its size, limit and expiry. Needs a sandbox token (Authorization:
   * Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/db
   */
  dbByName(name, params = {}) {
    return this.request("GET", `/v1/db/${enc(name)}`, { query: params });
  }

  /**
   * Delete a database now. Cuts off its connections and deletes it. Needs a sandbox token
   * (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).
   * https://api.dharun.dev/docs/db
   */
  dbByNameDrop(name, document = {}, params = {}) {
    return this.request("POST", `/v1/db/${enc(name)}/drop`, { body: document, query: params });
  }

  /**
   * Postgres over WebSocket. A WebSocket carrying the Postgres protocol to the server, as
   * Neon's serverless driver expects (set its wsProxy to this address). Postgres checks
   * the user and password; no token is needed here.
   * https://api.dharun.dev/docs/db
   */
  dbWs(params = {}) {
    return this.request("GET", `/v1/db/ws`, { query: params });
  }

  /**
   * Make a request bin. Returns the bin's id, url (where to send requests), readUrl and
   * token. The token is shown only in this answer; only its hash is kept.
   * https://api.dharun.dev/docs/hooks
   */
  hooks(document = {}, params = {}) {
    return this.request("POST", `/v1/hooks`, { body: document, query: params });
  }

  /**
   * Send a request to a bin. Any method (GET, HEAD, POST, PUT, PATCH, DELETE) and any path
   * after the id (/v1/hook/{id}/payments?attempt=1) is recorded. The answer is always the
   * same small JSON: a bin does not imitate an API. The documentation's bin, example,
   * accepts requests and keeps none.
   * https://api.dharun.dev/docs/hooks
   */
  hookById(id, params = {}, { body } = {}) {
    return body === undefined ? this.request("GET", `/v1/hook/${enc(id)}`, { query: params }) : this.request("POST", `/v1/hook/${enc(id)}`, { body, query: params });
  }

  /**
   * Read what a bin received. Newest first. body is text when it is valid UTF-8, otherwise
   * base64 (bodyEncoding says which); truncated is true when the body was over 64 KB. Poll
   * with since=<the highest seq you have> to get only new requests. POST
   * /v1/hooks/{id}/delete with the same token deletes the bin at once. The example bin's
   * token is example.
   * https://api.dharun.dev/docs/hooks
   */
  hooksById(id, params = {}) {
    return this.request("GET", `/v1/hooks/${enc(id)}`, { query: params });
  }

  /**
   * Delete a bin now. Deletes the bin and everything it received. Needs the bin's token.
   * https://api.dharun.dev/docs/hooks
   */
  hooksByIdDelete(id, document = {}, params = {}) {
    return this.request("POST", `/v1/hooks/${enc(id)}/delete`, { body: document, query: params });
  }
}

export default IndiaAPI;
