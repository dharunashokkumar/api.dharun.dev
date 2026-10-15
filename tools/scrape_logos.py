#!/usr/bin/env python3
"""Update data/logos/ from TradingView: every company listed on NSE, and those listed only on BSE.

Fetches the current listings from the TradingView India scanner, downloads the logo of
every company that is new or whose logo changed, and rewrites data/logos/metadata.json.
Companies are followed by ISIN: a new ticker with a known ISIN is a rename (the old
ticker is kept as an alias), and a company whose ISIN is no longer listed is kept and
marked delisted. Run with --dry-run first to see what would change.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import time
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

SCAN_URL = "https://scanner.tradingview.com/india/scan"
LOGO_URL = "https://s3-symbol-logo.tradingview.com/{}.svg"
USER_AGENT = "Mozilla/5.0 (india-api logo update; +https://api.dharun.dev)"
COLUMNS = [
    "name", "description", "exchange", "sector", "industry", "type", "subtype", "isin",
    "logoid", "currency", "market_cap_basic", "fundamental_currency_code", "indexes",
]
PAGE = 2000

# The same checks as prepareSVG in internal/dataset/logos.go, so a bad file is reported
# here instead of failing the build.
UNSAFE_SVG = [
    re.compile(r"(?i)<script"),
    re.compile(r"(?i)\son[a-z]+\s*="),
    re.compile(r"(?i)javascript:"),
    re.compile(r"(?i)<(foreignObject|iframe|object|embed)\b"),
    re.compile(r"""(?i)(href|src)\s*=\s*["']\s*(https?:)?//"""),
]
SVG_PREAMBLE = re.compile(r"^\s*(<\?xml[^>]*\?>\s*|<!--.*?-->\s*)*", re.S)
SVG_SIZE = re.compile(r"""<svg\b[^>]*(\sviewBox\s*=|\swidth="\d)""")
NOT_A_TRUST = re.compile(r"(?i)fixed (horizon|maturity)|interval fund|\bFMP\b|\bseries\b")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", default=".", help="Repository root. Defaults to the current directory.")
    parser.add_argument("--dry-run", action="store_true", help="Report what would change without downloading or writing.")
    parser.add_argument("--recheck", action="store_true", help="Download every logo again, not only new or changed ones.")
    parser.add_argument("--workers", type=int, default=4, help="Logo downloads at a time. Default 4.")
    return parser.parse_args()


def post_json(url: str, payload: dict[str, Any]) -> dict[str, Any]:
    body = json.dumps(payload, separators=(",", ":")).encode()
    request = Request(url, data=body, method="POST",
                      headers={"Content-Type": "application/json", "User-Agent": USER_AGENT})
    for attempt in range(1, 4):
        try:
            with urlopen(request, timeout=60) as response:
                return json.loads(response.read().decode())
        except (HTTPError, URLError, TimeoutError) as exc:
            if attempt == 3:
                raise RuntimeError(f"scanner request failed: {exc}") from exc
            time.sleep(2 * attempt)
    raise AssertionError("unreachable")


def scan(exchange: str) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    while True:
        result = post_json(SCAN_URL, {
            "filter": [
                {"left": "exchange", "operation": "equal", "right": exchange},
                {"left": "type", "operation": "in_range", "right": ["stock", "fund"]},
            ],
            "options": {"lang": "en"},
            "columns": COLUMNS,
            "sort": {"sortBy": "name", "sortOrder": "asc"},
            "range": [len(rows), len(rows) + PAGE],
        })
        if result.get("error"):
            raise RuntimeError(f"scanner error: {result['error']}")
        page = result.get("data") or []
        for item in page:
            row = dict(zip(COLUMNS, item.get("d") or []))
            row["id"] = item["s"]
            rows.append(row)
        if not page or len(rows) >= result.get("totalCount", 0):
            break
    print(f"{exchange}: {len(rows)} listings", file=sys.stderr)
    return rows


def is_company(row: dict[str, Any]) -> bool:
    """Shares, REITs and InvITs; not ETFs, mutual funds or fixed-maturity plans."""
    if row.get("type") == "stock":
        return True
    if row.get("type") == "fund" and row.get("subtype") in ("reit", "closedend"):
        name = row.get("description") or ""
        return bool(re.search(r"(?i)invit|trust|\breit\b", name)) and not NOT_A_TRUST.search(name)
    return False


def clean_svg(raw: bytes) -> bytes | None:
    """Strips TradingView's comment and returns the SVG, or None if it fails the build's checks."""
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        return None
    text = SVG_PREAMBLE.sub("", text, count=1).strip()
    if not text.startswith("<svg") or any(p.search(text) for p in UNSAFE_SVG) or not SVG_SIZE.search(text):
        return None
    return text.encode("utf-8")


def download(logoid: str) -> bytes | None:
    request = Request(LOGO_URL.format(logoid), headers={"User-Agent": USER_AGENT})
    for attempt in range(1, 4):
        try:
            with urlopen(request, timeout=30) as response:
                return clean_svg(response.read())
        except HTTPError as exc:
            if exc.code in (403, 404):
                return None
            error: Exception = exc
        except (URLError, TimeoutError) as exc:
            error = exc
        if attempt == 3:
            raise RuntimeError(f"logo {logoid}: {error}") from error
        time.sleep(2 * attempt)
    raise AssertionError("unreachable")


def one_liner(name: str | None, sector: str | None, industry: str | None) -> str | None:
    if name and sector and industry:
        return f"{name} operates in the {industry} industry within the {sector} sector."
    if name and (industry or sector):
        kind = "industry" if industry else "sector"
        return f"{name} operates in the {industry or sector} {kind}."
    return None


def record(row: dict[str, Any], aliases: list[str]) -> dict[str, Any]:
    exchange, ticker = row["id"].split(":", 1)
    indexes = [{"name": i.get("name"), "symbol": i.get("proname")}
               for i in row.get("indexes") or [] if isinstance(i, dict) and i.get("name")]
    return {
        "id": row["id"],
        "exchange": exchange,
        "ticker": ticker,
        "file": None,
        "company_name": row.get("description"),
        "sector": row.get("sector"),
        "industry": row.get("industry"),
        "one_liner": one_liner(row.get("description"), row.get("sector"), row.get("industry")),
        "isin": row.get("isin") or None,
        "currency": row.get("currency") or row.get("fundamental_currency_code"),
        "market_cap": row.get("market_cap_basic"),
        "security_type": row.get("type"),
        "security_subtype": row.get("subtype"),
        "is_index_constituent": bool(indexes),
        "indexes": indexes,
        "listed": True,
        "delisted_on": None,
        "aliases": aliases,
        "logo_id": row.get("logoid") or None,
    }


RECORD_KEYS = list(record({"id": "NSE:X"}, []))


def issuer(isin: str) -> str:
    """INE467B01029 -> INE467B: the country, issuer kind and issuer number, without the security."""
    return isin[:7]


def file_for(rec: dict[str, Any]) -> str:
    return f"{rec['exchange'].lower()}/{rec['exchange']}_{rec['ticker']}.svg"


def main() -> int:
    args = parse_args()
    root = Path(args.root).resolve()
    logos_dir = root / "data" / "logos"
    metadata_path = logos_dir / "metadata.json"
    today = datetime.now(timezone.utc).date().isoformat()

    previous: dict[str, dict[str, Any]] = {}
    if metadata_path.exists():
        for old in json.loads(metadata_path.read_text(encoding="utf-8"))["logos"]:
            old.setdefault("listed", True)
            old.setdefault("delisted_on", None)
            old.setdefault("aliases", [])
            previous[old["id"]] = old

    listings = {ex: [r for r in scan(ex) if is_company(r)] for ex in ("NSE", "BSE")}
    nse_isins = {r["isin"] for r in listings["NSE"] if r.get("isin")}
    current: dict[str, dict[str, Any]] = {r["id"]: r for r in listings["NSE"]}
    for row in listings["BSE"]:
        # A company on both exchanges is served from NSE, unless it was already here as BSE.
        if row.get("isin") not in nse_isins or row["id"] in previous:
            current[row["id"]] = row

    every = {r["id"]: r for rows in listings.values() for r in rows}
    by_isin: dict[tuple[str, str], str] = {}
    by_issuer: dict[tuple[str, str], list[str]] = {}
    for row_id, row in every.items():
        if row.get("isin"):
            exchange = row_id.split(":")[0]
            by_isin.setdefault((exchange, row["isin"]), row_id)
            by_issuer.setdefault((exchange, issuer(row["isin"])), []).append(row_id)

    # A ticker that is gone, with its ISIN now under a ticker we did not have, was renamed.
    renamed_from: dict[str, dict[str, Any]] = {}
    for old_id, old in previous.items():
        if old_id in current or not old.get("isin"):
            continue
        new_id = by_isin.get((old["exchange"], old["isin"]))
        if not new_id:
            # A split or bonus gives the share a new ISIN under the same issuer code.
            same = by_issuer.get((old["exchange"], issuer(old["isin"])), [])
            new_id = same[0] if len(same) == 1 else None
        if new_id and new_id not in previous and new_id not in renamed_from:
            renamed_from[new_id] = old
            current[new_id] = every[new_id]

    records: dict[str, dict[str, Any]] = {}
    added, renamed, relisted, dropped = [], [], [], []
    for row_id, row in sorted(current.items()):
        old = previous.get(row_id) or renamed_from.get(row_id)
        if old and old.get("isin") and row.get("isin") and issuer(old["isin"]) != issuer(row["isin"]):
            # The ticker now belongs to a different company; the old one is not kept.
            dropped.append(f"{row_id} ({old.get('company_name')})")
            old = None
        aliases = list(old["aliases"]) if old else []
        if row_id in renamed_from:
            aliases.append(old["ticker"])
            renamed.append(f"{old['id']} -> {row_id}")
        elif not old:
            added.append(row_id)
        elif not old["listed"]:
            relisted.append(row_id)
        rec = record(row, sorted(set(aliases)))
        if old and not rec["logo_id"]:
            rec["logo_id"] = old.get("logo_id")
        rec["_old"] = old
        records[row_id] = rec

    delisted = []
    moved = {old["id"] for old in renamed_from.values()}
    for old_id, old in previous.items():
        if old_id in records or old_id in moved:
            continue
        if old["listed"]:
            delisted.append(old_id)
        rec = {key: old.get(key) for key in RECORD_KEYS}
        rec.update(listed=False, delisted_on=old["delisted_on"] or today, aliases=old["aliases"],
                   market_cap=None, is_index_constituent=False, indexes=[])
        rec["_old"] = old
        records[old_id] = rec

    # An alias must not hide a company that holds that ticker today.
    tickers = {(r["exchange"], r["ticker"].upper()) for r in records.values()}
    for rec in records.values():
        rec["aliases"] = [a for a in rec["aliases"] if (rec["exchange"], a.upper()) not in tickers]

    seen_files: dict[str, str] = {}
    for rec in records.values():
        name = file_for(rec).lower()
        if name in seen_files:
            raise RuntimeError(f"{rec['id']} and {seen_files[name]} would share a file name")
        seen_files[name] = rec["id"]

    # Logos: keep the file when the logo is unchanged, download otherwise.
    wanted: dict[str, list[dict[str, Any]]] = {}
    for rec in records.values():
        old = rec["_old"]
        old_file = logos_dir / old["file"] if old and old.get("file") else None
        unchanged = old_file and old_file.exists() and old.get("logo_id") == rec["logo_id"]
        rec["_source"] = old_file if old_file and old_file.exists() else None
        if rec["listed"] and rec["logo_id"] and (args.recheck or not unchanged):
            wanted.setdefault(rec["logo_id"], []).append(rec)

    print(f"\nCompanies: {len(records)} "
          f"(NSE {sum(r['exchange'] == 'NSE' for r in records.values())}, "
          f"BSE {sum(r['exchange'] == 'BSE' for r in records.values())}), "
          f"listed {sum(r['listed'] for r in records.values())}")
    print(f"Previously: {len(previous)}")
    for label, items in (("Added", added), ("Renamed", renamed), ("Relisted", relisted),
                         ("Delisted", delisted), ("Dropped", dropped)):
        shown = ", ".join(items[:40]) + (f", ... {len(items) - 40} more" if len(items) > 40 else "")
        print(f"{label}: {len(items)}" + (f"\n  {shown}" if items else ""))
    print(f"Logos to download: {len(wanted)}; listed companies TradingView has no logo for: "
          f"{sum(1 for r in records.values() if r['listed'] and not r['logo_id'])}")

    if args.dry_run:
        print("\nDry run: nothing downloaded or written.")
        return 0

    fetched: dict[str, bytes | None] = {}
    with ThreadPoolExecutor(max_workers=max(1, args.workers)) as pool:
        for done, (logoid, svg) in enumerate(zip(wanted, pool.map(download, wanted)), 1):
            fetched[logoid] = svg
            if done % 250 == 0 or done == len(wanted):
                print(f"Downloaded {done}/{len(wanted)} logos", file=sys.stderr)

    changed, unavailable = 0, []
    contents: dict[str, bytes] = {}
    for rec in records.values():
        svg = fetched.get(rec["logo_id"]) if rec["logo_id"] in wanted else None
        source = rec.pop("_source")
        rec.pop("_old")
        if svg is None and source is not None:
            svg = source.read_bytes()
        if svg is None:
            if rec["listed"] and rec["logo_id"] in wanted:
                unavailable.append(rec["id"])
            rec["file"] = None
            continue
        rec["file"] = file_for(rec)
        contents[rec["file"]] = svg

    for folder in ("nse", "bse"):
        (logos_dir / folder).mkdir(parents=True, exist_ok=True)
    for name, svg in contents.items():
        path = logos_dir / name
        if not path.exists() or path.read_bytes() != svg:
            path.write_bytes(svg)
            changed += 1
    removed = 0
    for path in sorted(logos_dir.glob("*/*.svg")):
        if path.relative_to(logos_dir).as_posix() not in contents:
            path.unlink()
            removed += 1

    ordered = sorted(records.values(), key=lambda r: (r["exchange"] != "NSE", r["ticker"]))
    output = {
        "generated_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "source": {
            "provider": "TradingView India scanner",
            "url": SCAN_URL,
            "note": "one_liner is written from the sector and industry. Updated with tools/scrape_logos.py.",
        },
        "counts": {
            "companies": len(ordered),
            "listed": sum(r["listed"] for r in ordered),
            "with_logo": sum(1 for r in ordered if r["file"]),
            "by_exchange": {ex: sum(r["exchange"] == ex for r in ordered) for ex in ("NSE", "BSE")},
        },
        "logos": ordered,
    }
    with metadata_path.open("w", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps(output, ensure_ascii=False, indent=1) + "\n")

    print(f"\nWrote {metadata_path}: {len(ordered)} companies, {output['counts']['with_logo']} with a logo")
    print(f"SVG files written or changed: {changed}, removed: {removed}")
    if unavailable:
        print(f"Logos that would not download or failed the checks ({len(unavailable)}): "
              + ", ".join(unavailable[:30]))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
