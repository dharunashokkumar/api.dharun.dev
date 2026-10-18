// Types for the India API client. Generated from the catalog; do not edit. MIT-0.

export declare const CATALOG: string;

export interface Envelope<T = any> {
  data: T;
  meta: {
    total?: number;
    limit?: number;
    offset?: number;
    note?: string;
    source?: { name: string; url: string; licence: string; version?: string; retrieved?: string; attribution?: string };
  };
}

/** A file the API answered with: an image, a PDF or audio. */
export interface FileAnswer {
  contentType: string;
  bytes: Uint8Array;
  headers: Headers;
}

export type Answer<T = any> = Envelope<T> | FileAnswer;
export type FileInput = string | Blob | Uint8Array | ArrayBuffer;
type Query = Record<string, string | number | boolean | undefined | null>;

export declare class IndiaAPIError extends Error {
  status: number;
  code?: string;
}

export interface IndiaAPIOptions {
  baseUrl?: string;
  token?: string;
  headers?: Record<string, string>;
  fetch?: typeof fetch;
}

export declare class IndiaAPI {
  constructor(options?: IndiaAPIOptions);
  request(method: string, path: string, options?: { query?: Query; body?: unknown; contentType?: string; headers?: Record<string, string> }): Promise<Answer>;
  get(path: string, query?: Query): Promise<Answer>;
  post(path: string, body?: unknown, query?: Query): Promise<Answer>;
  /** Logo image for a listed company. https://api.dharun.dev/docs/logos */
  logosBySymbol(symbol: string, params?: { "format"?: "svg" | "png"; "size"?: number; "fallback"?: "monogram"; "exchange"?: "nse" | "bse"; [name: string]: unknown }): Promise<Answer>;
  /** Details of a listed company. https://api.dharun.dev/docs/logos */
  companiesBySymbol(symbol: string, params?: { "exchange"?: "nse" | "bse"; [name: string]: unknown }): Promise<Answer>;
  /** Search listed companies. https://api.dharun.dev/docs/logos */
  companies(params?: { "q"?: string; "exchange"?: "nse" | "bse"; "sector"?: string; "index"?: string; "delisted"?: "true" | "false"; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Post offices for a pincode. https://api.dharun.dev/docs/pincode */
  pincodeByPincode(pincode: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Search post offices. https://api.dharun.dev/docs/pincode */
  pincode(params?: { "q"?: string; "state"?: string; "district"?: string; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Post offices near a point. https://api.dharun.dev/docs/pincode */
  pincodeNear(params?: { "lat": string; "lng": string; "radius"?: number; "limit"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Branch for an IFSC. https://api.dharun.dev/docs/ifsc */
  ifscByCode(code: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Search a bank's branches. https://api.dharun.dev/docs/ifsc */
  ifsc(params?: { "bank": string; "q"?: string; "state"?: string; "city"?: string; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Bank for a four-letter code. https://api.dharun.dev/docs/ifsc */
  banksByCode(code: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Search banks. https://api.dharun.dev/docs/ifsc */
  banks(params?: { "q"?: string; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Office for an RTO code. https://api.dharun.dev/docs/rto */
  rtoByCode(code: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** List or search RTO codes. https://api.dharun.dev/docs/rto */
  rto(params?: { "state"?: string; "q"?: string; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Holidays in a year. https://api.dharun.dev/docs/holidays */
  holidaysByYear(year: string, params?: { "type"?: "bank" | "central_gazetted" | "central_restricted" | "market" | "clearing"; "state"?: string; "month"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Is a date a working day. https://api.dharun.dev/docs/working-days */
  workingDaysByDate(date: string, params?: { "calendar"?: "bank" | "market" | "settlement" | "central"; "state"?: string; "office"?: string; "add"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Count working days between two dates. https://api.dharun.dev/docs/working-days */
  workingDays(params?: { "from": string; "to": string; "calendar"?: "bank" | "market" | "settlement" | "central"; "state"?: string; "office"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Is the stock market open. https://api.dharun.dev/docs/working-days */
  marketStatus(params?: { "at"?: string; [name: string]: unknown }): Promise<Answer>;
  /** QR code for any text. https://api.dharun.dev/docs/codes */
  qr(params?: { "text"?: string; "ecc"?: "L" | "M" | "Q" | "H"; "format"?: "svg" | "png"; "size"?: number; "margin"?: number; "fg"?: string; "bg"?: string; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** UPI payment QR code. https://api.dharun.dev/docs/codes */
  upiQr(params?: { "pa": string; "pn"?: string; "am"?: string; "tn"?: string; "tr"?: string; "mc"?: string; "format"?: "svg" | "png"; "size"?: number; "margin"?: number; "fg"?: string; "bg"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Barcode. https://api.dharun.dev/docs/codes */
  barcodeByType(type: string, params?: { "text"?: string; "scale"?: number; "height"?: number; "format"?: "svg" | "png"; "size"?: number; "margin"?: number; "fg"?: string; "bg"?: string; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Read QR codes and barcodes from a picture. https://api.dharun.dev/docs/codes */
  barcodeRead(file: FileInput, params?: { [name: string]: unknown }): Promise<Answer>;
  /** A code, its parents and children. https://api.dharun.dev/docs/hsn */
  hsnByCode(code: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Search codes. https://api.dharun.dev/docs/hsn */
  hsn(params?: { "q": string; "kind"?: "hsn" | "sac"; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** A scheme and its NAV. https://api.dharun.dev/docs/mutual-funds */
  mutualFundsByCode(code: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Search schemes. https://api.dharun.dev/docs/mutual-funds */
  mutualFunds(params?: { "q"?: string; "fundHouse"?: string; "category"?: string; "plan"?: "direct" | "regular"; "option"?: "growth" | "idcw" | "bonus"; "all"?: "true"; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Fund houses. https://api.dharun.dev/docs/mutual-funds */
  mutualFundsFundHouses(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Location of an IP address. https://api.dharun.dev/docs/ip */
  ipByAddress(address: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Location of your own address. https://api.dharun.dev/docs/ip */
  ip(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Check an email address. https://api.dharun.dev/docs/email */
  emailByAddress(address: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Look up DNS records. https://api.dharun.dev/docs/dns */
  dnsByName(name: string, params?: { "type"?: "A" | "AAAA" | "CNAME" | "MX" | "NS" | "TXT" | "CAA" | "SOA" | "SRV" | "PTR"; [name: string]: unknown }): Promise<Answer>;
  /** Station for a code. https://api.dharun.dev/docs/stations */
  stationsByCode(code: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Search stations. https://api.dharun.dev/docs/stations */
  stations(params?: { "q"?: string; "state"?: string; "zone"?: string; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Railway zones. https://api.dharun.dev/docs/stations */
  stationsZones(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Check a phone number. https://api.dharun.dev/docs/phone */
  phoneByNumber(number: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Constituencies of a place. https://api.dharun.dev/docs/constituency */
  constituency(params?: { "pincode"?: string; "lat"?: string; "lng"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Airport for a code. https://api.dharun.dev/docs/airports */
  airportsByCode(code: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Search airports. https://api.dharun.dev/docs/airports */
  airports(params?: { "q"?: string; "country"?: string; "state"?: string; "type"?: "large_airport" | "medium_airport" | "small_airport" | "heliport" | "seaplane_base"; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** A kural by number. https://api.dharun.dev/docs/thirukkural */
  thirukkuralByNumber(number: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Search kurals. https://api.dharun.dev/docs/thirukkural */
  thirukkural(params?: { "q"?: string; "chapter"?: number; "paal"?: number; "limit"?: number; "offset"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Kural of the day. https://api.dharun.dev/docs/thirukkural */
  thirukkuralToday(params?: { "date"?: string; [name: string]: unknown }): Promise<Answer>;
  /** A random kural. https://api.dharun.dev/docs/thirukkural */
  thirukkuralRandom(params?: { [name: string]: unknown }): Promise<Answer>;
  /** All 133 chapters. https://api.dharun.dev/docs/thirukkural */
  thirukkuralChapters(params?: { [name: string]: unknown }): Promise<Answer>;
  /** One chapter. https://api.dharun.dev/docs/thirukkural */
  thirukkuralChaptersByNumber(number: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Split a document into chunks. https://api.dharun.dev/docs/text */
  textChunk(params?: { "text"?: string; "size"?: number; "overlap"?: number; "format"?: "auto" | "markdown" | "text"; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Redact Indian personal identifiers. https://api.dharun.dev/docs/text */
  textRedact(params?: { "text"?: string; "types"?: string; "style"?: "label" | "partial"; "names"?: "true" | "false"; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Count tokens. https://api.dharun.dev/docs/text */
  textTokens(params?: { "text"?: string; "model"?: string; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Detect the language. https://api.dharun.dev/docs/text */
  textLanguage(params?: { "text"?: string; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Score text for abuse, Hinglish and Tanglish included. https://api.dharun.dev/docs/text */
  textModerate(params?: { "text"?: string; "threshold"?: string; "models"?: string; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Test a regular expression. https://api.dharun.dev/docs/regex */
  regex(params?: { "pattern"?: string; "text"?: string; "flavor"?: "js" | "re2"; "flags"?: string; "replace"?: string; "limit"?: number; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Transliterate text. https://api.dharun.dev/docs/transliterate */
  transliterate(params?: { "text"?: string; "to"?: string; "from"?: string; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Roman typing to an Indian script (type English, get Tamil). https://api.dharun.dev/docs/transliterate */
  transliterateSuggest(params?: { "text"?: string; "lang"?: string; "count"?: number; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Convert Tamil text. https://api.dharun.dev/docs/tamil-font */
  tamilFontConvert(params?: { "text"?: string; "from"?: "unicode" | "tscii"; "to"?: "unicode" | "tscii"; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Encodings offered. https://api.dharun.dev/docs/tamil-font */
  tamilFontEncodings(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Panchang for a date. https://api.dharun.dev/docs/panchang */
  panchangByDate(date: string, params?: { "pincode"?: string; "lat"?: string; "lng"?: string; "tz"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Sun and moon for a date. https://api.dharun.dev/docs/sun-moon */
  sunMoonByDate(date: string, params?: { "pincode"?: string; "lat"?: string; "lng"?: string; "tz"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Trace the route to a host. https://api.dharun.dev/docs/mtr */
  mtrByHost(host: string, params?: { "rounds"?: number; "v"?: "4" | "6"; [name: string]: unknown }): Promise<Answer>;
  /** Look up a registration. https://api.dharun.dev/docs/whois */
  whoisByQuery(query: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Check a host's TLS. https://api.dharun.dev/docs/tls */
  tlsByHost(host: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Check a domain's IPv6. https://api.dharun.dev/docs/ipv6 */
  ipv6ByDomain(domain: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Time a URL from India. https://api.dharun.dev/docs/probe */
  probe(params?: { "url": string; "rounds"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Icon for a website. https://api.dharun.dev/docs/icon */
  iconByDomain(domain: string, params?: { "size"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Passes over a place. https://api.dharun.dev/docs/satellites */
  satPass(params?: { "sat"?: string; "pincode"?: string; "lat"?: string; "lng"?: string; "days"?: number; "minElevation"?: string; "visible"?: "true" | "false"; "tz"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Where a satellite is now. https://api.dharun.dev/docs/satellites */
  satellitesById(id: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Tracked satellites. https://api.dharun.dev/docs/satellites */
  satellites(params?: { "q"?: string; "group"?: "stations" | "amateur"; [name: string]: unknown }): Promise<Answer>;
  /** Space weather and band conditions. https://api.dharun.dev/docs/propagation */
  propagation(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Generate fake records. https://api.dharun.dev/docs/fake */
  fakeByKind(kind: string, params?: { "count"?: number; "state"?: string; "seed"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Social preview image. https://api.dharun.dev/docs/images */
  ogImage(params?: { "title": string; "subtitle"?: string; "site"?: string; "tag"?: string; "theme"?: "dark" | "light"; "bg"?: string; "fg"?: string; "accent"?: string; "format"?: "png" | "svg"; [name: string]: unknown }): Promise<Answer>;
  /** Code as an image. https://api.dharun.dev/docs/images */
  codeImage(params?: { "code"?: string; "language"?: string; "title"?: string; "theme"?: "dark" | "light"; "lineNumbers"?: "true"; "format"?: "png" | "svg"; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Spell out a number. https://api.dharun.dev/docs/words */
  wordsByNumber(number: string, params?: { "lang"?: "en" | "hi" | "ta"; "style"?: "number" | "currency"; [name: string]: unknown }): Promise<Answer>;
  /** Check an identifier's format. https://api.dharun.dev/docs/validate */
  validateByKindByValue(kind: string, value: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** All states and union territories. https://api.dharun.dev/docs/states */
  states(params?: { [name: string]: unknown }): Promise<Answer>;
  /** One state. https://api.dharun.dev/docs/states */
  statesByCode(code: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Read the text in a picture or PDF. https://api.dharun.dev/docs/ocr */
  ocr(file: FileInput, params?: { "lang"?: string; "words"?: "true" | "false"; "first"?: number; "last"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Resize and convert a picture. https://api.dharun.dev/docs/image-tools */
  imageConvert(file: FileInput, params?: { "format"?: "jpeg" | "png" | "webp" | "avif" | "gif" | "tiff"; "width"?: number; "height"?: number; "fit"?: "inside" | "cover" | "fill"; "quality"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Size, format and metadata of a picture. https://api.dharun.dev/docs/image-tools */
  imageInfo(file: FileInput, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Remove the background. https://api.dharun.dev/docs/image-tools */
  imageRemoveBackground(file: FileInput, params?: { "format"?: "png" | "webp" | "mask"; [name: string]: unknown }): Promise<Answer>;
  /** Perceptual hash of a picture. https://api.dharun.dev/docs/image-tools */
  imageHash(file: FileInput, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Compare two pictures. https://api.dharun.dev/docs/image-tools */
  imageCompare(files: FileInput[], params?: { [name: string]: unknown }): Promise<Answer>;
  /** Pages, size and details of a PDF. https://api.dharun.dev/docs/pdf */
  pdfInfo(file: FileInput, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Text of a PDF, page by page. https://api.dharun.dev/docs/pdf */
  pdfText(file: FileInput, params?: { "first"?: number; "last"?: number; "layout"?: "true" | "false"; [name: string]: unknown }): Promise<Answer>;
  /** Merge PDFs into one. https://api.dharun.dev/docs/pdf */
  pdfMerge(files: FileInput[], params?: { [name: string]: unknown }): Promise<Answer>;
  /** Pick, reorder or split pages. https://api.dharun.dev/docs/pdf */
  pdfPages(file: FileInput, params?: { "pages"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Rotate pages. https://api.dharun.dev/docs/pdf */
  pdfRotate(file: FileInput, params?: { "angle"?: "90" | "180" | "270" | "-90" | "-180" | "-270"; "pages"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Make a PDF smaller. https://api.dharun.dev/docs/pdf */
  pdfCompress(file: FileInput, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Draw a page as a picture. https://api.dharun.dev/docs/pdf */
  pdfRender(file: FileInput, params?: { "page"?: number; "width"?: number; "format"?: "png" | "jpeg"; [name: string]: unknown }): Promise<Answer>;
  /** Make a GST tax invoice. https://api.dharun.dev/docs/invoice */
  invoice(document?: object, params?: { "format"?: "pdf" | "png" | "json"; [name: string]: unknown }): Promise<Answer>;
  /** Compile Typst to PDF. https://api.dharun.dev/docs/typst */
  typst(params?: { "text"?: string; "format"?: "pdf" | "png" | "svg"; "page"?: number; "ppi"?: number; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Speech to text, with subtitles. https://api.dharun.dev/docs/speech */
  stt(file: FileInput, params?: { "language"?: string; "task"?: "transcribe" | "translate"; "format"?: "json" | "srt" | "vtt" | "text"; [name: string]: unknown }): Promise<Answer>;
  /** Text to speech. https://api.dharun.dev/docs/speech */
  tts(params?: { "text"?: string; "voice"?: string; "format"?: "mp3" | "ogg" | "wav"; "speed"?: string; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Voices. https://api.dharun.dev/docs/speech */
  ttsVoices(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Remove background noise. https://api.dharun.dev/docs/speech */
  audioDenoise(file: FileInput, params?: { "format"?: "mp3" | "ogg" | "wav"; "limit"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Text embeddings. https://api.dharun.dev/docs/embeddings */
  embed(params?: { "text"?: string; "type"?: "query" | "passage"; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Rerank documents for a query. https://api.dharun.dev/docs/embeddings */
  rerank(params?: { "query"?: string; "document"?: string; "top"?: number; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Translate text. https://api.dharun.dev/docs/translate */
  translate(params?: { "text"?: string; "from"?: string; "to"?: string; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Languages. https://api.dharun.dev/docs/translate */
  translateLanguages(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Languages, versions and limits. https://api.dharun.dev/docs/sandbox */
  sandboxLanguages(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Run code once. https://api.dharun.dev/docs/sandbox */
  sandboxRun(document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Start a session. https://api.dharun.dev/docs/sandbox */
  sandboxSessions(document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Describe a session. https://api.dharun.dev/docs/sandbox */
  sandboxSessionsById(id: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Run a command in a session. https://api.dharun.dev/docs/sandbox */
  sandboxSessionsByIdExec(id: string, document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Run code in a session. https://api.dharun.dev/docs/sandbox */
  sandboxSessionsByIdRun(id: string, document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Put a file in a session. https://api.dharun.dev/docs/sandbox */
  sandboxSessionsByIdUpload(id: string, file: FileInput, params?: { "path"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Read a file from a session. https://api.dharun.dev/docs/sandbox */
  sandboxSessionsByIdDownload(id: string, params?: { "path"?: string; "list"?: "true"; [name: string]: unknown }): Promise<Answer>;
  /** Stop a session now. https://api.dharun.dev/docs/sandbox */
  sandboxSessionsByIdStop(id: string, document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Terminal of a session (WebSocket). https://api.dharun.dev/docs/sandbox */
  sandboxSessionsByIdTty(id: string, params?: { "cols"?: number; "rows"?: number; "token"?: string; [name: string]: unknown }): Promise<Answer>;
  /** Start a GitHub sign-in. https://api.dharun.dev/docs/sandbox-account */
  sandboxLogin(document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Finish a GitHub sign-in. https://api.dharun.dev/docs/sandbox-account */
  sandboxLoginPoll(document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Today's quota. https://api.dharun.dev/docs/sandbox-account */
  sandboxAccount(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Revoke a token. https://api.dharun.dev/docs/sandbox-account */
  sandboxLogout(document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Screenshot of a page. https://api.dharun.dev/docs/browser */
  browserScreenshot(params?: { "url"?: string; "width"?: number; "height"?: number; "fullPage"?: "true"; "format"?: "png" | "jpeg" | "webp"; "mobile"?: "true"; "waitMs"?: number; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** A page or HTML as PDF. https://api.dharun.dev/docs/browser */
  browserPdf(params?: { "url"?: string; "paper"?: "a4" | "a3" | "a5" | "letter" | "legal"; "landscape"?: "true"; "background"?: "true"; "waitMs"?: number; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Text of a page. https://api.dharun.dev/docs/browser */
  browserText(params?: { "url"?: string; "format"?: "text" | "markdown"; "waitMs"?: number; [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Make a database. https://api.dharun.dev/docs/db */
  db(document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Describe a database. https://api.dharun.dev/docs/db */
  dbByName(name: string, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Delete a database now. https://api.dharun.dev/docs/db */
  dbByNameDrop(name: string, document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Postgres over WebSocket. https://api.dharun.dev/docs/db */
  dbWs(params?: { [name: string]: unknown }): Promise<Answer>;
  /** Make a request bin. https://api.dharun.dev/docs/hooks */
  hooks(document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
  /** Send a request to a bin. https://api.dharun.dev/docs/hooks */
  hookById(id: string, params?: { [name: string]: unknown }, options?: { body?: string }): Promise<Answer>;
  /** Read what a bin received. https://api.dharun.dev/docs/hooks */
  hooksById(id: string, params?: { "token"?: string; "since"?: number; "limit"?: number; [name: string]: unknown }): Promise<Answer>;
  /** Delete a bin now. https://api.dharun.dev/docs/hooks */
  hooksByIdDelete(id: string, document?: object, params?: { [name: string]: unknown }): Promise<Answer>;
}

export default IndiaAPI;
