"""Client for the India API, https://api.dharun.dev/docs

Generated from the API's catalog; do not edit. MIT-0: copy it into anything.
https://github.com/dharunashokkumar/api.dharun.dev/tree/main/clients/python
"""

from __future__ import annotations

import json as _json
import mimetypes as _mimetypes
import os as _os
import uuid as _uuid
from typing import Any, Dict, Iterable, Optional, Union
from urllib import error as _error
from urllib import parse as _parse
from urllib import request as _request

CATALOG = "100f2618e8c4"

__all__ = ["IndiaAPI", "IndiaAPIError", "FileAnswer", "CATALOG"]

FileInput = Union[str, "_os.PathLike[str]", bytes, bytearray]


class IndiaAPIError(Exception):
    """An answer other than 2xx. code is the API's error code, such as not_found or rate_limited."""

    def __init__(self, status: int, code: Optional[str], message: str):
        super().__init__(message or f"HTTP {status}")
        self.status = status
        self.code = code


class FileAnswer:
    """A file the API answered with: an image, a PDF or audio."""

    def __init__(self, content_type: str, data: bytes, headers: Dict[str, str]):
        self.content_type = content_type
        self.data = data
        self.headers = headers

    def save(self, path: Union[str, "_os.PathLike[str]"]) -> None:
        with open(path, "wb") as out:
            out.write(self.data)

    def __repr__(self) -> str:
        return f"FileAnswer({self.content_type!r}, {len(self.data)} bytes)"


def _enc(value: Any) -> str:
    return _parse.quote(str(value), safe="")


def _read_file(file: FileInput) -> tuple:
    if isinstance(file, (bytes, bytearray)):
        return bytes(file), "file"
    path = _os.fspath(file)
    with open(path, "rb") as handle:
        return handle.read(), _os.path.basename(path)


class IndiaAPI:
    """The India API. Every method returns the {"data", "meta"} envelope as a dict, or a
    FileAnswer for images, PDFs and audio, and raises IndiaAPIError otherwise."""

    def __init__(self, base_url: str = "https://api.dharun.dev", token: Optional[str] = None,
                 headers: Optional[Dict[str, str]] = None, timeout: float = 120):
        self.base_url = base_url.rstrip("/")
        self.headers = dict(headers or {})
        if token:
            self.headers["Authorization"] = "Bearer " + token
        self.timeout = timeout

    def request(self, method: str, path: str, query: Optional[Dict[str, Any]] = None,
                body: Any = None, content_type: Optional[str] = None) -> Any:
        """Any request. A str body is sent as text, bytes as they are, anything else as JSON."""
        url = self.base_url + path
        pairs = {name: _value(value) for name, value in (query or {}).items() if value is not None}
        if pairs:
            url += "?" + _parse.urlencode(pairs)
        headers = {"Accept": "application/json, */*", "User-Agent": "india-api-python/" + CATALOG}
        headers.update(self.headers)
        data = None
        if body is not None:
            if isinstance(body, str):
                data = body.encode("utf-8")
                headers["Content-Type"] = content_type or "text/plain; charset=utf-8"
            elif isinstance(body, (bytes, bytearray)):
                data = bytes(body)
                headers["Content-Type"] = content_type or "application/octet-stream"
            else:
                data = _json.dumps(body).encode("utf-8")
                headers["Content-Type"] = "application/json"
        req = _request.Request(url, data=data, method=method, headers=headers)
        try:
            with _request.urlopen(req, timeout=self.timeout) as response:
                return _answer(response.status, dict(response.headers), response.read())
        except _error.HTTPError as failed:
            return _answer(failed.code, dict(failed.headers), failed.read())

    def get(self, path: str, **query: Any) -> Any:
        """GET any path, such as "/v1/pincode/600001"."""
        return self.request("GET", path, query)

    def post(self, path: str, body: Any = None, **query: Any) -> Any:
        """POST to any path: a str as text, a dict as JSON, bytes as they are."""
        return self.request("POST", path, query, body)

    def _upload(self, path: str, files: Iterable[FileInput], query: Dict[str, Any]) -> Any:
        boundary = "india-api-" + _uuid.uuid4().hex
        chunks = []
        for file in files:
            data, name = _read_file(file)
            kind = _mimetypes.guess_type(name)[0] or "application/octet-stream"
            chunks.append(
                f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"{name}\"\r\n"
                f"Content-Type: {kind}\r\n\r\n".encode("utf-8") + data + b"\r\n")
        chunks.append(f"--{boundary}--\r\n".encode("utf-8"))
        return self.request("POST", path, query, b"".join(chunks), "multipart/form-data; boundary=" + boundary)

    def logos_by_symbol(self, symbol: str, **params: Any) -> Any:
        """Logo image for a listed company. Returns the image itself, ready for an <img> tag. A
        listed company without a published logo gets an initials tile, marked with the
        X-Logo-Fallback: monogram header. Unknown symbols return 404 unless
        fallback=monogram is set.

        https://api.dharun.dev/docs/logos
        """
        return self.request("GET", f"/v1/logos/{_enc(symbol)}", params)

    def companies_by_symbol(self, symbol: str, **params: Any) -> Any:
        """Details of a listed company. Name, sector, industry, ISIN, market capitalisation,
        index memberships and logo URLs. listed is false for a delisted company (with
        delistedOn, the date it was first missed); aliases holds earlier tickers; hasLogo is
        false when the logo URL serves an initials tile.

        https://api.dharun.dev/docs/logos
        """
        return self.request("GET", f"/v1/companies/{_enc(symbol)}", params)

    def companies(self, **params: Any) -> Any:
        """Search listed companies. Searches ticker, earlier tickers, company name, sector,
        industry, ISIN and index names. Best match first. Delisted companies are left out
        unless delisted=true.

        https://api.dharun.dev/docs/logos
        """
        return self.request("GET", "/v1/companies", params)

    def pincode_by_pincode(self, pincode: str, **params: Any) -> Any:
        """Post offices for a pincode. Returns the district and state of the pincode and every
        post office that uses it.

        https://api.dharun.dev/docs/pincode
        """
        return self.request("GET", f"/v1/pincode/{_enc(pincode)}", params)

    def pincode(self, **params: Any) -> Any:
        """Search post offices. Find post offices by the start of their name, by the start of a
        pincode, or list a district.

        https://api.dharun.dev/docs/pincode
        """
        return self.request("GET", "/v1/pincode", params)

    def pincode_near(self, **params: Any) -> Any:
        """Post offices near a point. Post offices within a radius of a latitude and longitude,
        nearest first, with the distance. Positions come from the Department of Posts and
        some are approximate.

        https://api.dharun.dev/docs/pincode
        """
        return self.request("GET", "/v1/pincode/near", params)

    def ifsc_by_code(self, code: str, **params: Any) -> Any:
        """Branch for an IFSC.

        https://api.dharun.dev/docs/ifsc
        """
        return self.request("GET", f"/v1/ifsc/{_enc(code)}", params)

    def ifsc(self, **params: Any) -> Any:
        """Search a bank's branches. Lists branches of one bank, filtered by state, city or
        text in the branch name or address.

        https://api.dharun.dev/docs/ifsc
        """
        return self.request("GET", "/v1/ifsc", params)

    def banks_by_code(self, code: str, **params: Any) -> Any:
        """Bank for a four-letter code.

        https://api.dharun.dev/docs/ifsc
        """
        return self.request("GET", f"/v1/banks/{_enc(code)}", params)

    def banks(self, **params: Any) -> Any:
        """Search banks. Finds banks by name or code, largest branch network first.

        https://api.dharun.dev/docs/ifsc
        """
        return self.request("GET", "/v1/banks", params)

    def rto_by_code(self, code: str, **params: Any) -> Any:
        """Office for an RTO code.

        https://api.dharun.dev/docs/rto
        """
        return self.request("GET", f"/v1/rto/{_enc(code)}", params)

    def rto(self, **params: Any) -> Any:
        """List or search RTO codes.

        https://api.dharun.dev/docs/rto
        """
        return self.request("GET", "/v1/rto", params)

    def holidays_by_year(self, year: str, **params: Any) -> Any:
        """Holidays in a year.

        https://api.dharun.dev/docs/holidays
        """
        return self.request("GET", f"/v1/holidays/{_enc(year)}", params)

    def working_days_by_date(self, date: str, **params: Any) -> Any:
        """Is a date a working day. Whether the date is a working day and why not, the previous
        and next working days, and with add=N the date N working days later (negative for
        earlier). A T+1 settlement date is calendar=settlement&add=1.

        https://api.dharun.dev/docs/working-days
        """
        return self.request("GET", f"/v1/working-days/{_enc(date)}", params)

    def working_days(self, **params: Any) -> Any:
        """Count working days between two dates. Counts working days from one date to another,
        both included, and lists each day off with its reason. At most 366 days.

        https://api.dharun.dev/docs/working-days
        """
        return self.request("GET", "/v1/working-days", params)

    def market_status(self, **params: Any) -> Any:
        """Is the stock market open. NSE equities session at a moment: pre_open, open, closing
        or closed, the reason it is closed, and when it next opens. Without at, the current
        moment.

        https://api.dharun.dev/docs/working-days
        """
        return self.request("GET", "/v1/market/status", params)

    def qr(self, body: Optional[str] = None, **params: Any) -> Any:
        """QR code for any text. Returns the image itself. Text up to about 2,900 bytes at
        error correction L.

        https://api.dharun.dev/docs/codes
        """
        return self.request("POST" if body is not None else "GET", "/v1/qr", params, body)

    def upi_qr(self, **params: Any) -> Any:
        """UPI payment QR code. A QR code any UPI app can scan to pay, built to NPCI's
        upi://pay link format. format=json returns the link instead of the image. Nothing is
        checked with a bank.

        https://api.dharun.dev/docs/codes
        """
        return self.request("GET", "/v1/upi-qr", params)

    def barcode_by_type(self, type_: str, body: Optional[str] = None, **params: Any) -> Any:
        """Barcode. Linear barcodes (code128, code39, code93, ean13, ean8, itf, codabar) and 2D
        codes (datamatrix, pdf417, aztec). Linear codes are sized with scale and height, 2D
        codes with size. No human-readable text is printed under the bars.

        https://api.dharun.dev/docs/codes
        """
        return self.request("POST" if body is not None else "GET", f"/v1/barcode/{_enc(type_)}", params, body)

    def barcode_read(self, file: FileInput, **params: Any) -> Any:
        """Read QR codes and barcodes from a picture. Finds every QR code in a photo or
        screenshot, and Data Matrix, Aztec, EAN, UPC, Code 128, Code 39, Code 93, ITF and
        Codabar codes. Not PDF417. Returns each code's kind, text and position. Options go
        in the query string, or as form fields when the file is sent as multipart/form-data.

        https://api.dharun.dev/docs/codes
        """
        return self._upload("/v1/barcode/read", [file], params)

    def hsn_by_code(self, code: str, **params: Any) -> Any:
        """A code, its parents and children.

        https://api.dharun.dev/docs/hsn
        """
        return self.request("GET", f"/v1/hsn/{_enc(code)}", params)

    def hsn(self, **params: Any) -> Any:
        """Search codes. Every word of q must appear in the description; digits search by code
        prefix. Broader codes come first.

        https://api.dharun.dev/docs/hsn
        """
        return self.request("GET", "/v1/hsn", params)

    def mutual_funds_by_code(self, code: str, **params: Any) -> Any:
        """A scheme and its NAV.

        https://api.dharun.dev/docs/mutual-funds
        """
        return self.request("GET", f"/v1/mutual-funds/{_enc(code)}", params)

    def mutual_funds(self, **params: Any) -> Any:
        """Search schemes. Every word of q must appear in the scheme name, in any order. Only
        active schemes unless all=true.

        https://api.dharun.dev/docs/mutual-funds
        """
        return self.request("GET", "/v1/mutual-funds", params)

    def mutual_funds_fund_houses(self, **params: Any) -> Any:
        """Fund houses.

        https://api.dharun.dev/docs/mutual-funds
        """
        return self.request("GET", "/v1/mutual-funds/fund-houses", params)

    def ip_by_address(self, address: str, **params: Any) -> Any:
        """Location of an IP address.

        https://api.dharun.dev/docs/ip
        """
        return self.request("GET", f"/v1/ip/{_enc(address)}", params)

    def ip(self, **params: Any) -> Any:
        """Location of your own address.

        https://api.dharun.dev/docs/ip
        """
        return self.request("GET", "/v1/ip", params)

    def email_by_address(self, address: str, **params: Any) -> Any:
        """Check an email address.

        https://api.dharun.dev/docs/email
        """
        return self.request("GET", f"/v1/email/{_enc(address)}", params)

    def dns_by_name(self, name: str, **params: Any) -> Any:
        """Look up DNS records.

        https://api.dharun.dev/docs/dns
        """
        return self.request("GET", f"/v1/dns/{_enc(name)}", params)

    def stations_by_code(self, code: str, **params: Any) -> Any:
        """Station for a code.

        https://api.dharun.dev/docs/stations
        """
        return self.request("GET", f"/v1/stations/{_enc(code)}", params)

    def stations(self, **params: Any) -> Any:
        """Search stations. By name (an exact code comes first, then names starting with the
        text), state or zone.

        https://api.dharun.dev/docs/stations
        """
        return self.request("GET", "/v1/stations", params)

    def stations_zones(self, **params: Any) -> Any:
        """Railway zones.

        https://api.dharun.dev/docs/stations
        """
        return self.request("GET", "/v1/stations/zones", params)

    def phone_by_number(self, number: str, **params: Any) -> Any:
        """Check a phone number.

        https://api.dharun.dev/docs/phone
        """
        return self.request("GET", f"/v1/phone/{_enc(number)}", params)

    def constituency(self, **params: Any) -> Any:
        """Constituencies of a place.

        https://api.dharun.dev/docs/constituency
        """
        return self.request("GET", "/v1/constituency", params)

    def airports_by_code(self, code: str, **params: Any) -> Any:
        """Airport for a code.

        https://api.dharun.dev/docs/airports
        """
        return self.request("GET", f"/v1/airports/{_enc(code)}", params)

    def airports(self, **params: Any) -> Any:
        """Search airports. By name, city or code; airports with scheduled flights and larger
        airports first.

        https://api.dharun.dev/docs/airports
        """
        return self.request("GET", "/v1/airports", params)

    def thirukkural_by_number(self, number: str, **params: Any) -> Any:
        """A kural by number.

        https://api.dharun.dev/docs/thirukkural
        """
        return self.request("GET", f"/v1/thirukkural/{_enc(number)}", params)

    def thirukkural(self, **params: Any) -> Any:
        """Search kurals. Finds kurals containing the text, in Tamil, the transliteration or
        either translation; or lists a chapter or a paal.

        https://api.dharun.dev/docs/thirukkural
        """
        return self.request("GET", "/v1/thirukkural", params)

    def thirukkural_today(self, **params: Any) -> Any:
        """Kural of the day. The same kural for everyone on a day in India, a different one the
        next day.

        https://api.dharun.dev/docs/thirukkural
        """
        return self.request("GET", "/v1/thirukkural/today", params)

    def thirukkural_random(self, **params: Any) -> Any:
        """A random kural.

        https://api.dharun.dev/docs/thirukkural
        """
        return self.request("GET", "/v1/thirukkural/random", params)

    def thirukkural_chapters(self, **params: Any) -> Any:
        """All 133 chapters.

        https://api.dharun.dev/docs/thirukkural
        """
        return self.request("GET", "/v1/thirukkural/chapters", params)

    def thirukkural_chapters_by_number(self, number: str, **params: Any) -> Any:
        """One chapter.

        https://api.dharun.dev/docs/thirukkural
        """
        return self.request("GET", f"/v1/thirukkural/chapters/{_enc(number)}", params)

    def text_chunk(self, body: Optional[str] = None, **params: Any) -> Any:
        """Split a document into chunks. Splits at headings, paragraphs, sentences and words,
        in that order of preference, so chunks stay readable. Markdown chunks never cross a
        heading and carry their heading path. Each chunk is an exact slice of the input with
        its character offsets.

        https://api.dharun.dev/docs/text
        """
        return self.request("POST" if body is not None else "GET", "/v1/text/chunk", params, body)

    def text_redact(self, body: Optional[str] = None, **params: Any) -> Any:
        """Redact Indian personal identifiers. Masks Aadhaar numbers (Verhoeff-checked), PAN,
        GSTIN (checksum-checked), card numbers (Luhn-checked), Indian mobile numbers, email
        addresses, UPI IDs, vehicle numbers, voter IDs, passport numbers and IP addresses.
        Returns where each was found, never the values. With names=true a model also masks
        people's names: IndicNER for Indian scripts, bert-base-NER for English. Street
        addresses are not detected.

        https://api.dharun.dev/docs/text
        """
        return self.request("POST" if body is not None else "GET", "/v1/text/redact", params, body)

    def text_tokens(self, body: Optional[str] = None, **params: Any) -> Any:
        """Count tokens. Exact token counts with OpenAI tokenizers, by encoding or model name.
        Other vendors do not publish their tokenizers, so their counts are not offered.

        https://api.dharun.dev/docs/text
        """
        return self.request("POST" if body is not None else "GET", "/v1/text/tokens", params, body)

    def text_language(self, body: Optional[str] = None, **params: Any) -> Any:
        """Detect the language. Language and script, with a confidence. Covers about 80
        languages including Hindi, Bengali, Marathi, Gujarati, Punjabi, Tamil, Telugu,
        Kannada, Malayalam, Odia and Urdu. Romanised Indian text (Hinglish, Tanglish) is
        flagged as such.

        https://api.dharun.dev/docs/text
        """
        return self.request("POST" if body is not None else "GET", "/v1/text/language", params, body)

    def text_moderate(self, body: Optional[str] = None, **params: Any) -> Any:
        """Score text for abuse, Hinglish and Tanglish included. Three classifiers each score
        the text from 0 to 1: a general multilingual one (toxic), one for Hindi typed in
        Roman letters (profanity, harassment, hate_speech, caste, communal, regional,
        misogyny) and one for Tamil typed in Roman letters (offensive). flagged says whether
        any score reached the threshold. Up to 4,000 characters; the first 256 tokens are
        read. Scores are a model's guess: use them to sort or to ask a person, not to decide
        alone.

        https://api.dharun.dev/docs/text
        """
        return self.request("POST" if body is not None else "GET", "/v1/text/moderate", params, body)

    def regex(self, body: Optional[str] = None, **params: Any) -> Any:
        """Test a regular expression. An invalid pattern answers 422 with code invalid_pattern
        and the engine's message; a JavaScript pattern that runs out of time answers 422
        with code timeout. A group that took no part in a match has a null text, which is
        different from an empty one.

        https://api.dharun.dev/docs/regex
        """
        return self.request("POST" if body is not None else "GET", "/v1/regex", params, body)

    def transliterate(self, body: Optional[str] = None, **params: Any) -> Any:
        """Transliterate text.

        https://api.dharun.dev/docs/transliterate
        """
        return self.request("POST" if body is not None else "GET", "/v1/transliterate", params, body)

    def transliterate_suggest(self, body: Optional[str] = None, **params: Any) -> Any:
        """Roman typing to an Indian script (type English, get Tamil). Guesses how words typed
        in Roman letters are written in an Indian script with IndicXlit (AI4Bharat), giving
        up to eight suggestions per word, best first, and the text with the best guess for
        each word. 21 languages. Up to 60 words; anything that is not Roman letters is kept
        as it is.

        https://api.dharun.dev/docs/transliterate
        """
        return self.request("POST" if body is not None else "GET", "/v1/transliterate/suggest", params, body)

    def tamil_font_convert(self, body: Optional[str] = None, **params: Any) -> Any:
        """Convert Tamil text. Give the text as it looks without its font (the Windows-1252
        characters), or POST the raw bytes as application/octet-stream. Converting to an
        8-bit encoding answers with the same kind of text, and with the bytes in base64.
        Bytes the encoding does not assign are counted in undefined; characters it has no
        code for are listed in unmapped.

        https://api.dharun.dev/docs/tamil-font
        """
        return self.request("POST" if body is not None else "GET", "/v1/tamil-font/convert", params, body)

    def tamil_font_encodings(self, **params: Any) -> Any:
        """Encodings offered. The encodings that can be converted, with fonts that use each, to
        help recognise what a text is in.

        https://api.dharun.dev/docs/tamil-font
        """
        return self.request("GET", "/v1/tamil-font/encodings", params)

    def panchang_by_date(self, date: str, **params: Any) -> Any:
        """Panchang for a date. For a pincode, a latitude and longitude, or (by default) New
        Delhi. Times are in the time zone asked for.

        https://api.dharun.dev/docs/panchang
        """
        return self.request("GET", f"/v1/panchang/{_enc(date)}", params)

    def sun_moon_by_date(self, date: str, **params: Any) -> Any:
        """Sun and moon for a date. For a pincode, a latitude and longitude, or (by default)
        New Delhi. A twilight time is null where the sun does not sink that far, and
        moonrise or moonset is null on the day each month the moon does not rise or set. The
        phase names a principal phase (new moon, first quarter, full moon, last quarter) for
        the whole date it falls on.

        https://api.dharun.dev/docs/sun-moon
        """
        return self.request("GET", f"/v1/sun-moon/{_enc(date)}", params)

    def mtr_by_host(self, host: str, **params: Any) -> Any:
        """Trace the route to a host. Lists hops up to the one where the host answered, or one
        past the last router that answered. A hop with 100% loss is common: many routers
        forward traffic without answering probes. otherAddresses lists further routers seen
        at the same hop, where traffic is balanced over several paths.

        https://api.dharun.dev/docs/mtr
        """
        return self.request("GET", f"/v1/mtr/{_enc(host)}", params)

    def whois_by_query(self, query: str, **params: Any) -> Any:
        """Look up a registration. A name below a registered domain (www.example.co.in) is
        looked up as that domain (example.co.in). daysToExpiry counts from now. An IP lookup
        is treated as private: not logged, not cached.

        https://api.dharun.dev/docs/whois
        """
        return self.request("GET", f"/v1/whois/{_enc(query)}", params)

    def tls_by_host(self, host: str, **params: Any) -> Any:
        """Check a host's TLS. valid is false when any problem is an error: expired, not yet
        valid, name_mismatch, self_signed, incomplete_chain, untrusted, weak_key or
        sha1_signature. Warnings (expires_soon, old_versions, no_tls13, weak_cipher,
        chain_order) leave it true. Each version is tried with its own handshake offering
        every cipher Go supports; SSL 3 is not tested.

        https://api.dharun.dev/docs/tls
        """
        return self.request("GET", f"/v1/tls/{_enc(host)}", params)

    def ipv6_by_domain(self, domain: str, **params: Any) -> Any:
        """Check a domain's IPv6. ready is true when the site (the name, or www when the name
        has no address) has a public IPv6 address, at least one nameserver does, and every
        mail server does (a domain without mail servers passes that part). website.network
        names the network of the first IPv6 address, from DB-IP.

        https://api.dharun.dev/docs/ipv6
        """
        return self.request("GET", f"/v1/ipv6/{_enc(domain)}", params)

    def probe(self, **params: Any) -> Any:
        """Time a URL from India. With rounds above 1 the URL is fetched that many times and
        summary gives the fastest, median and slowest total; the other fields describe the
        first round. A redirect into private address space, to another port or to anything
        other than http or https is refused. The URL may carry a token, so the request is
        not logged and the answer is not cached.

        https://api.dharun.dev/docs/probe
        """
        return self.request("GET", "/v1/probe", params)

    def icon_by_domain(self, domain: str, **params: Any) -> Any:
        """Icon for a website. The X-Icon-Source header names the file the icon was drawn from,
        the first time it is fetched. An icon already fetched counts as 1 request against
        the rate limit; one that has to be fetched counts as 20.

        https://api.dharun.dev/docs/icon
        """
        return self.request("GET", f"/v1/icon/{_enc(domain)}", params)

    def sat_pass(self, **params: Any) -> Any:
        """Passes over a place. Passes from now for the next days, soonest first. Directions
        are compass points and azimuths in degrees from true north; elevations are degrees
        above the horizon. A satellite can be named by catalogue number, by name, by the
        designation in its name's brackets (SO-50, AO-7), or as ISS, CSS or QO-100.

        https://api.dharun.dev/docs/satellites
        """
        return self.request("GET", "/v1/sat-pass", params)

    def satellites_by_id(self, id_: str, **params: Any) -> Any:
        """Where a satellite is now. The point on Earth below the satellite at the moment of
        the request, its altitude and speed, whether it is in sunlight, and how old its
        orbit elements are.

        https://api.dharun.dev/docs/satellites
        """
        return self.request("GET", f"/v1/satellites/{_enc(id_)}", params)

    def satellites(self, **params: Any) -> Any:
        """Tracked satellites. Every tracked satellite with its catalogue number, groups,
        orbital period and inclination.

        https://api.dharun.dev/docs/satellites
        """
        return self.request("GET", "/v1/satellites", params)

    def propagation(self, **params: Any) -> Any:
        """Space weather and band conditions. Rules behind the band ratings: 80m-40m: fair by
        day (D-layer absorption), good at night. 30m-20m: good by day; at night good from a
        solar flux of 120, fair from 80, else poor. 17m-15m: by day good from a solar flux
        of 110, fair from 80, else poor; at night fair from 150, else poor. 12m-10m: by day
        good from a solar flux of 150, fair from 110, else poor; at night fair from 200,
        else poor. A K-index of 4 lowers every night rating one step; 5 or more lowers every
        rating one step, and 7 or more two. An M-class X-ray flux lowers the day rating of
        80m-40m and …

        https://api.dharun.dev/docs/propagation
        """
        return self.request("GET", "/v1/propagation", params)

    def fake_by_kind(self, kind: str, **params: Any) -> Any:
        """Generate fake records. people have a name, gender, birth date, email (on example
        domains), phone, PAN and address; companies a name, PAN, GSTIN, email, phone and
        address.

        https://api.dharun.dev/docs/fake
        """
        return self.request("GET", f"/v1/fake/{_enc(kind)}", params)

    def og_image(self, **params: Any) -> Any:
        """Social preview image. A 1200 by 630 card for og:image and twitter:image: a title
        (set as large as fits in three lines, cut with an ellipsis beyond that), an optional
        subtitle of up to two lines, and a site name and tag along the bottom. The same URL
        always gives the same picture, so it can be used directly in a meta tag.

        https://api.dharun.dev/docs/images
        """
        return self.request("GET", "/v1/og-image", params)

    def code_image(self, body: Optional[str] = None, **params: Any) -> Any:
        """Code as an image. A code snippet as a window with syntax colouring. Up to 150 lines
        of 140 characters. PNGs are drawn at twice the pixel density for sharp text (very
        large ones at single). The language is guessed from the title's file extension or
        the code when not given, and reported in the X-Code-Language header.

        https://api.dharun.dev/docs/images
        """
        return self.request("POST" if body is not None else "GET", "/v1/code-image", params, body)

    def words_by_number(self, number: str, **params: Any) -> Any:
        """Spell out a number.

        https://api.dharun.dev/docs/words
        """
        return self.request("GET", f"/v1/words/{_enc(number)}", params)

    def validate_by_kind_by_value(self, kind: str, value: str, **params: Any) -> Any:
        """Check an identifier's format.

        https://api.dharun.dev/docs/validate
        """
        return self.request("GET", f"/v1/validate/{_enc(kind)}/{_enc(value)}", params)

    def states(self, **params: Any) -> Any:
        """All states and union territories.

        https://api.dharun.dev/docs/states
        """
        return self.request("GET", "/v1/states", params)

    def states_by_code(self, code: str, **params: Any) -> Any:
        """One state.

        https://api.dharun.dev/docs/states
        """
        return self.request("GET", f"/v1/states/{_enc(code)}", params)

    def ocr(self, file: FileInput, **params: Any) -> Any:
        """Read the text in a picture or PDF. Returns the text, Tesseract's confidence and,
        with words=true, every word with its box. A PDF is read up to 10 pages at a time at
        300 dpi; a PDF that already holds text is read faster by /v1/pdf/text. Options go in
        the query string, or as form fields when the file is sent as multipart/form-data.

        https://api.dharun.dev/docs/ocr
        """
        return self._upload("/v1/ocr", [file], params)

    def image_convert(self, file: FileInput, **params: Any) -> Any:
        """Resize and convert a picture. Returns the picture itself, turned upright from its
        EXIF orientation, without metadata. fit=inside keeps the whole picture within width
        and height and never enlarges it; cover fills the box and crops the middle; fill
        stretches. Options go in the query string, or as form fields when the file is sent
        as multipart/form-data.

        https://api.dharun.dev/docs/image-tools
        """
        return self._upload("/v1/image/convert", [file], params)

    def image_info(self, file: FileInput, **params: Any) -> Any:
        """Size, format and metadata of a picture. Format, size, colour space, pages,
        orientation, the camera, and whether the picture carries EXIF data or a GPS
        position.

        https://api.dharun.dev/docs/image-tools
        """
        return self._upload("/v1/image/info", [file], params)

    def image_remove_background(self, file: FileInput, **params: Any) -> Any:
        """Remove the background. Returns the picture as a PNG with the background transparent,
        cut out by the IS-Net model. Works best on one clear subject: a product, a person,
        an animal. Takes a few seconds; pictures larger than 4096 pixels are made smaller.
        Options go in the query string, or as form fields when the file is sent as
        multipart/form-data.

        https://api.dharun.dev/docs/image-tools
        """
        return self._upload("/v1/image/remove-background", [file], params)

    def image_hash(self, file: FileInput, **params: Any) -> Any:
        """Perceptual hash of a picture. Three 64-bit fingerprints (average, difference and
        DCT-based perceptual hashes) that stay nearly the same when a picture is resized,
        recompressed or slightly edited. Pictures whose perceptual hashes differ by 10 bits
        or fewer look alike.

        https://api.dharun.dev/docs/image-tools
        """
        return self._upload("/v1/image/hash", [file], params)

    def image_compare(self, files: Iterable[FileInput], **params: Any) -> Any:
        """Compare two pictures. Send two pictures as form fields. Returns the bits by which
        each hash differs, a score from 0 to 1, and whether they look like the same picture.

        https://api.dharun.dev/docs/image-tools
        """
        return self._upload("/v1/image/compare", files, params)

    def pdf_info(self, file: FileInput, **params: Any) -> Any:
        """Pages, size and details of a PDF. Page count, PDF version, title, author, dates,
        page sizes (named when A4, Letter and so on), and whether it has forms, signatures
        or attachments.

        https://api.dharun.dev/docs/pdf
        """
        return self._upload("/v1/pdf/info", [file], params)

    def pdf_text(self, file: FileInput, **params: Any) -> Any:
        """Text of a PDF, page by page. The text of each page, in reading order, or laid out as
        on the page with layout=true. needs_ocr is true when the pages hold almost no text,
        as in a scan: send those to /v1/ocr. Options go in the query string, or as form
        fields when the file is sent as multipart/form-data.

        https://api.dharun.dev/docs/pdf
        """
        return self._upload("/v1/pdf/text", [file], params)

    def pdf_merge(self, files: Iterable[FileInput], **params: Any) -> Any:
        """Merge PDFs into one. Send two to twenty PDFs as form fields, in order; up to 25 MB
        together. Returns one PDF.

        https://api.dharun.dev/docs/pdf
        """
        return self._upload("/v1/pdf/merge", files, params)

    def pdf_pages(self, file: FileInput, **params: Any) -> Any:
        """Pick, reorder or split pages. Returns a PDF of the pages named, in the order named;
        a page may be named twice. To split a PDF, ask for each part in turn. Options go in
        the query string, or as form fields when the file is sent as multipart/form-data.

        https://api.dharun.dev/docs/pdf
        """
        return self._upload("/v1/pdf/pages", [file], params)

    def pdf_rotate(self, file: FileInput, **params: Any) -> Any:
        """Rotate pages. Turns pages clockwise by a multiple of 90 degrees. Options go in the
        query string, or as form fields when the file is sent as multipart/form-data.

        https://api.dharun.dev/docs/pdf
        """
        return self._upload("/v1/pdf/rotate", [file], params)

    def pdf_compress(self, file: FileInput, **params: Any) -> Any:
        """Make a PDF smaller. Removes duplicate fonts and images and packs the file's
        structure. Pictures are not re-compressed, so a scanned PDF shrinks little. The
        original size is in the X-Original-Bytes header; when nothing was gained, the
        original comes back.

        https://api.dharun.dev/docs/pdf
        """
        return self._upload("/v1/pdf/compress", [file], params)

    def pdf_render(self, file: FileInput, **params: Any) -> Any:
        """Draw a page as a picture. Returns one page as PNG or JPEG, for previews and
        thumbnails. Options go in the query string, or as form fields when the file is sent
        as multipart/form-data.

        https://api.dharun.dev/docs/pdf
        """
        return self._upload("/v1/pdf/render", [file], params)

    def invoice(self, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Make a GST tax invoice. Fields: number (required, up to 16 letters, digits, - and
        /), date and due_date (YYYY-MM-DD, date defaults to today), title (default Tax
        Invoice), place_of_supply (state code, name or GST code; defaults to the buyer's
        state from the GSTIN or state), reverse_charge, seller and buyer (name, address,
        gstin, state, phone, email; the seller's GSTIN is required), ship_to, items
        (description, hsn, quantity (default 1), unit (default Nos), rate before tax,
        discount as an amount, gst_rate in percent), bank (name, account, ifsc, branch,
        holder), upi, notes and terms. Unknown …

        https://api.dharun.dev/docs/invoice
        """
        return self.request("POST", "/v1/invoice", params, document if document is not None else {})

    def typst(self, body: Optional[str] = None, **params: Any) -> Any:
        """Compile Typst to PDF. Returns the PDF itself, or one page as an image. A mistake in
        the document returns 422 with Typst's message, line and column.

        https://api.dharun.dev/docs/typst
        """
        return self.request("POST" if body is not None else "GET", "/v1/typst", params, body)

    def stt(self, file: FileInput, **params: Any) -> Any:
        """Speech to text, with subtitles. Send a recording (MP3, WAV, Ogg, M4A, WebM, video
        files too) and get the text with timed segments, as JSON, SRT or WebVTT subtitles,
        or plain text. Whisper small recognises about a hundred languages, Indian ones
        included; it is good on clear English and fair on Indian languages. task=translate
        gives English text whatever was spoken. The first five minutes are heard. Options go
        in the query string, or as form fields when the file is sent as multipart/form-data.

        https://api.dharun.dev/docs/speech
        """
        return self._upload("/v1/stt", [file], params)

    def tts(self, body: Optional[str] = None, **params: Any) -> Any:
        """Text to speech. Returns the audio itself; X-Voice and X-Voice-Licence headers name
        the voice and its licence. Up to 3,000 characters; split longer text. Write the text
        in its own script: Hindi in Devanagari, Telugu in Telugu, Malayalam in Malayalam.

        https://api.dharun.dev/docs/speech
        """
        return self.request("POST" if body is not None else "GET", "/v1/tts", params, body)

    def tts_voices(self, **params: Any) -> Any:
        """Voices. Every voice with its language, speaker, licence and the credit the licence
        asks for.

        https://api.dharun.dev/docs/speech
        """
        return self.request("GET", "/v1/tts/voices", params)

    def audio_denoise(self, file: FileInput, **params: Any) -> Any:
        """Remove background noise. Send a recording (MP3, WAV, Ogg, M4A, WebM, video files
        too) and get it back with fans, traffic and hum lowered, as mono 48 kHz. The first
        five minutes are cleaned. Options go in the query string, or as form fields when the
        file is sent as multipart/form-data.

        https://api.dharun.dev/docs/speech
        """
        return self._upload("/v1/audio/denoise", [file], params)

    def embed(self, body: Optional[str] = None, **params: Any) -> Any:
        """Text embeddings. Up to 64 texts a request (GET: one or more text parameters; POST:
        JSON {"texts": [...], "type": ...}). Each comes back as 384 numbers scaled to length
        1, so a dot product is the cosine similarity. Embed questions with type=query and
        the documents searched with type=passage, as the model was trained. Each text is
        read up to 512 tokens.

        https://api.dharun.dev/docs/embeddings
        """
        return self.request("POST" if body is not None else "GET", "/v1/embed", params, body)

    def rerank(self, body: Optional[str] = None, **params: Any) -> Any:
        """Rerank documents for a query. Scores how well each document answers the query, from
        0 to 1, and returns them best first with their place in the request. Up to 100
        documents of up to 8,000 characters; each pair is read up to 512 tokens. GET takes
        query and repeated document parameters; POST takes JSON.

        https://api.dharun.dev/docs/embeddings
        """
        return self.request("POST" if body is not None else "GET", "/v1/rerank", params, body)

    def translate(self, body: Optional[str] = None, **params: Any) -> Any:
        """Translate text. Translates a sentence at a time and returns the whole translation
        and each sentence. Up to 2,000 characters and 30 sentences. Write Indian languages
        in their own script.

        https://api.dharun.dev/docs/translate
        """
        return self.request("POST" if body is not None else "GET", "/v1/translate", params, body)

    def translate_languages(self, **params: Any) -> Any:
        """Languages. Every language with its short code and FLORES-200 code; either can be
        used as from and to.

        https://api.dharun.dev/docs/translate
        """
        return self.request("GET", "/v1/translate/languages", params)

    def sandbox_languages(self, **params: Any) -> Any:
        """Languages, versions and limits. The languages the code image runs, with the version
        of each, the session images, and every limit. Needs no token.

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("GET", "/v1/sandbox/languages", params)

    def sandbox_run(self, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Run code once. language (python, javascript, go, c, cpp, ruby, php, bash, or an
        alias such as py, js, node, golang, c++, sh), code (up to 64 KB), and optionally
        stdin, args and files ([{path, content, encoding}] with encoding text or base64; up
        to 20 files of 1 MB each, relative to the working directory /work). Returns stdout,
        stderr, exitCode, durationMs, cpuMs and limitsHit (time, memory, output). Needs a
        sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("POST", "/v1/sandbox/run", params, document if document is not None else {})

    def sandbox_sessions(self, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Start a session. image is code (default) or a distribution: debian, alpine, arch,
        fedora. network: true lets a code session reach its languages' package registries
        through a proxy (500 MB of downloads), and nothing else; distributions have no
        network. Returns the session's id, when it expires and links to exec, run, files,
        stop and tty. An account may have two sessions at once. Needs a sandbox token
        (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("POST", "/v1/sandbox/sessions", params, document if document is not None else {})

    def sandbox_sessions_by_id(self, id_: str, **params: Any) -> Any:
        """Describe a session. Its image, network and expiry. Needs a sandbox token
        (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("GET", f"/v1/sandbox/sessions/{_enc(id_)}", params)

    def sandbox_sessions_by_id_exec(self, id_: str, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Run a command in a session. command (a shell line) or argv (a list), and optionally
        stdin, cwd (default /work), env and timeoutMs (up to 60 seconds). Returns the same
        as a run. Counts as a run against the daily quota. Needs a sandbox token
        (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("POST", f"/v1/sandbox/sessions/{_enc(id_)}/exec", params, document if document is not None else {})

    def sandbox_sessions_by_id_run(self, id_: str, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Run code in a session. As /v1/sandbox/run, in the session: files written before and
        packages installed stay. Code sessions only. Needs a sandbox token (Authorization:
        Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("POST", f"/v1/sandbox/sessions/{_enc(id_)}/run", params, document if document is not None else {})

    def sandbox_sessions_by_id_upload(self, id_: str, file: FileInput, **params: Any) -> Any:
        """Put a file in a session. Writes the body, or the first file of a multipart form, to
        path: relative to /work, or absolute. Up to 1 MB. Needs a sandbox token
        (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self._upload(f"/v1/sandbox/sessions/{_enc(id_)}/upload", [file], params)

    def sandbox_sessions_by_id_download(self, id_: str, **params: Any) -> Any:
        """Read a file from a session. Returns the file at path, up to 1 MB. A directory
        (list=true, a path ending in /, or no path for /work) is listed as JSON: name, size,
        dir and mode of each entry. Needs a sandbox token (Authorization: Bearer <token>;
        sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("GET", f"/v1/sandbox/sessions/{_enc(id_)}/download", params)

    def sandbox_sessions_by_id_stop(self, id_: str, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Stop a session now. Ends the session and destroys its microVM and everything in it.
        Needs a sandbox token (Authorization: Bearer <token>; sign in at POST
        /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("POST", f"/v1/sandbox/sessions/{_enc(id_)}/stop", params, document if document is not None else {})

    def sandbox_sessions_by_id_tty(self, id_: str, **params: Any) -> Any:
        """Terminal of a session (WebSocket). A WebSocket: binary messages carry the terminal
        both ways, and a text message {"cols": 120, "rows": 32} resizes it. Only for
        sessions started from the terminal page at /terminal, which passes a browser check;
        programs use exec. The token may be given as ?token=, as browsers cannot set headers
        on a WebSocket. Needs a sandbox token (Authorization: Bearer <token>; sign in at
        POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox
        """
        return self.request("GET", f"/v1/sandbox/sessions/{_enc(id_)}/tty", params)

    def sandbox_login(self, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Start a GitHub sign-in. Returns userCode, verificationUri, deviceCode, interval
        (seconds between polls) and expiresIn.

        https://api.dharun.dev/docs/sandbox-account
        """
        return self.request("POST", "/v1/sandbox/login", params, document if document is not None else {})

    def sandbox_login_poll(self, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Finish a GitHub sign-in. status is pending until the person approves, slow_down when
        polled too often, or ok with the token, its expiry and the account's quota. 403 when
        the sign-in was refused or the code expired.

        https://api.dharun.dev/docs/sandbox-account
        """
        return self.request("POST", "/v1/sandbox/login/poll", params, document if document is not None else {})

    def sandbox_account(self, **params: Any) -> Any:
        """Today's quota. The login, the daily quota, what has been used today and what is
        left, and when it resets. Needs a sandbox token (Authorization: Bearer <token>; sign
        in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox-account
        """
        return self.request("GET", "/v1/sandbox/account", params)

    def sandbox_logout(self, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Revoke a token. The token stops working at once. Needs a sandbox token
        (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/sandbox-account
        """
        return self.request("POST", "/v1/sandbox/logout", params, document if document is not None else {})

    def browser_screenshot(self, body: Optional[str] = None, **params: Any) -> Any:
        """Screenshot of a page. Returns the image. GET with the options in the query, or POST
        JSON with url or html (up to 4 MB) and the same options.

        https://api.dharun.dev/docs/browser
        """
        return self.request("POST" if body is not None else "GET", "/v1/browser/screenshot", params, body)

    def browser_pdf(self, body: Optional[str] = None, **params: Any) -> Any:
        """A page or HTML as PDF. Returns the PDF. The page's print styles apply. POST html to
        make a PDF from your own HTML and CSS (images inline or at public URLs).

        https://api.dharun.dev/docs/browser
        """
        return self.request("POST" if body is not None else "GET", "/v1/browser/pdf", params, body)

    def browser_text(self, body: Optional[str] = None, **params: Any) -> Any:
        """Text of a page. url, title, language, the HTTP status and text: what a reader sees
        (innerText), or with format=markdown the page as Markdown with headings, lists,
        links, code and tables. 403 robots_disallowed when the site's robots.txt does not
        allow it.

        https://api.dharun.dev/docs/browser
        """
        return self.request("POST" if body is not None else "GET", "/v1/browser/text", params, body)

    def db(self, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Make a database. Returns name, user, password (shown only now), websocket, a
        connectionString for the bridge, and expiresAt. Needs a sandbox token
        (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/db
        """
        return self.request("POST", "/v1/db", params, document if document is not None else {})

    def db_by_name(self, name: str, **params: Any) -> Any:
        """Describe a database. Its size, limit and expiry. Needs a sandbox token
        (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/db
        """
        return self.request("GET", f"/v1/db/{_enc(name)}", params)

    def db_by_name_drop(self, name: str, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Delete a database now. Cuts off its connections and deletes it. Needs a sandbox
        token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

        https://api.dharun.dev/docs/db
        """
        return self.request("POST", f"/v1/db/{_enc(name)}/drop", params, document if document is not None else {})

    def db_ws(self, **params: Any) -> Any:
        """Postgres over WebSocket. A WebSocket carrying the Postgres protocol to the server,
        as Neon's serverless driver expects (set its wsProxy to this address). Postgres
        checks the user and password; no token is needed here.

        https://api.dharun.dev/docs/db
        """
        return self.request("GET", "/v1/db/ws", params)

    def hooks(self, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Make a request bin. Returns the bin's id, url (where to send requests), readUrl and
        token. The token is shown only in this answer; only its hash is kept.

        https://api.dharun.dev/docs/hooks
        """
        return self.request("POST", "/v1/hooks", params, document if document is not None else {})

    def hook_by_id(self, id_: str, body: Optional[str] = None, **params: Any) -> Any:
        """Send a request to a bin. Any method (GET, HEAD, POST, PUT, PATCH, DELETE) and any
        path after the id (/v1/hook/{id}/payments?attempt=1) is recorded. The answer is
        always the same small JSON: a bin does not imitate an API. The documentation's bin,
        example, accepts requests and keeps none.

        https://api.dharun.dev/docs/hooks
        """
        return self.request("POST" if body is not None else "GET", f"/v1/hook/{_enc(id_)}", params, body)

    def hooks_by_id(self, id_: str, **params: Any) -> Any:
        """Read what a bin received. Newest first. body is text when it is valid UTF-8,
        otherwise base64 (bodyEncoding says which); truncated is true when the body was over
        64 KB. Poll with since=<the highest seq you have> to get only new requests. POST
        /v1/hooks/{id}/delete with the same token deletes the bin at once. The example bin's
        token is example.

        https://api.dharun.dev/docs/hooks
        """
        return self.request("GET", f"/v1/hooks/{_enc(id_)}", params)

    def hooks_by_id_delete(self, id_: str, document: Optional[Dict[str, Any]] = None, **params: Any) -> Any:
        """Delete a bin now. Deletes the bin and everything it received. Needs the bin's token.

        https://api.dharun.dev/docs/hooks
        """
        return self.request("POST", f"/v1/hooks/{_enc(id_)}/delete", params, document if document is not None else {})


def _value(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def _answer(status: int, headers: Dict[str, str], raw: bytes) -> Any:
    lowered = {name.lower(): value for name, value in headers.items()}
    kind = lowered.get("content-type", "")
    if kind.startswith("application/json"):
        decoded = _json.loads(raw.decode("utf-8")) if raw else {}
        if status >= 300:
            error = decoded.get("error") or {}
            raise IndiaAPIError(status, error.get("code"), error.get("message", ""))
        return decoded
    if status >= 300:
        raise IndiaAPIError(status, f"http_{status}", raw.decode("utf-8", "replace"))
    return FileAnswer(kind, raw, headers)
