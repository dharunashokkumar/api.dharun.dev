#!/usr/bin/env python3
"""Build metadata for NSE/BSE SVG logos from public market scanner data."""

from __future__ import annotations

import argparse
import json
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


TRADINGVIEW_SCAN_URL = "https://scanner.tradingview.com/india/scan"
TRADINGVIEW_COLUMNS = [
    "name",
    "description",
    "exchange",
    "sector",
    "industry",
    "type",
    "subtype",
    "typespecs",
    "isin",
    "logoid",
    "country",
    "market",
    "currency",
    "market_cap_basic",
    "fundamental_currency_code",
    "indexes",
]


@dataclass(frozen=True)
class LogoFile:
    exchange: str
    ticker: str
    path: Path

    @property
    def source_symbol(self) -> str:
        return f"{self.exchange}:{self.ticker}"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Scrape NSE/BSE company metadata for SVG logo files."
    )
    parser.add_argument(
        "--root",
        default=".",
        help="Repository root containing logos/nse/ and logos/bse/. Defaults to current directory.",
    )
    parser.add_argument(
        "--output",
        default="data/logo_metadata.full.json",
        help="Output JSON path, relative to --root unless absolute.",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=400,
        help="Number of symbols to request per TradingView scanner call.",
    )
    parser.add_argument(
        "--cache",
        default=".metadata_cache/tradingview_logo_metadata_cache.json",
        help="Raw scanner cache path, relative to --root unless absolute.",
    )
    parser.add_argument(
        "--refresh",
        action="store_true",
        help="Ignore any existing scanner cache and fetch fresh data.",
    )
    return parser.parse_args()


def resolve_path(root: Path, value: str) -> Path:
    path = Path(value)
    return path if path.is_absolute() else root / path


def discover_logos(root: Path) -> list[LogoFile]:
    logos: list[LogoFile] = []
    for folder, exchange in (("nse", "NSE"), ("bse", "BSE")):
        directory = root / "logos" / folder
        if not directory.exists():
            continue

        prefix = f"{exchange}_"
        for file_path in sorted(directory.glob("*.svg")):
            stem = file_path.stem
            if not stem.upper().startswith(prefix):
                continue
            ticker = stem[len(prefix) :]
            logos.append(LogoFile(exchange=exchange, ticker=ticker, path=file_path))
    return logos


def request_scanner_batch(symbols: list[str], retries: int = 3) -> dict[str, Any]:
    payload = {
        "options": {"lang": "en"},
        "symbols": {"tickers": symbols},
        "columns": TRADINGVIEW_COLUMNS,
        "range": [0, len(symbols)],
    }
    data = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    request = Request(
        TRADINGVIEW_SCAN_URL,
        data=data,
        headers={
            "Content-Type": "application/json",
            "User-Agent": "Mozilla/5.0 metadata-builder",
        },
        method="POST",
    )

    last_error: Exception | None = None
    for attempt in range(1, retries + 1):
        try:
            with urlopen(request, timeout=60) as response:
                return json.loads(response.read().decode("utf-8"))
        except (HTTPError, URLError, TimeoutError) as exc:
            last_error = exc
            if attempt < retries:
                time.sleep(1.5 * attempt)

    raise RuntimeError(f"TradingView scanner request failed: {last_error}") from last_error


def fetch_scanner_data(symbols: list[str], batch_size: int) -> dict[str, dict[str, Any]]:
    rows: dict[str, dict[str, Any]] = {}
    for start in range(0, len(symbols), batch_size):
        batch = symbols[start : start + batch_size]
        result = request_scanner_batch(batch)
        if result.get("error"):
            raise RuntimeError(f"TradingView scanner error: {result['error']}")

        for item in result.get("data") or []:
            symbol = item.get("s")
            values = item.get("d") or []
            if not symbol:
                continue
            rows[symbol] = dict(zip(TRADINGVIEW_COLUMNS, values))

        print(
            f"Fetched {min(start + batch_size, len(symbols))}/{len(symbols)} symbols",
            file=sys.stderr,
        )
    return rows


def load_or_fetch_cache(
    cache_path: Path, symbols: list[str], batch_size: int, refresh: bool
) -> dict[str, dict[str, Any]]:
    if cache_path.exists() and not refresh:
        with cache_path.open("r", encoding="utf-8") as handle:
            cached = json.load(handle)
        cached_rows = cached.get("rows", cached)
        if isinstance(cached_rows, dict):
            return cached_rows

    rows = fetch_scanner_data(symbols, batch_size)
    cache_path.parent.mkdir(parents=True, exist_ok=True)
    with cache_path.open("w", encoding="utf-8") as handle:
        json.dump(
            {
                "generated_at": datetime.now(timezone.utc).isoformat(),
                "source_url": TRADINGVIEW_SCAN_URL,
                "columns": TRADINGVIEW_COLUMNS,
                "rows": rows,
            },
            handle,
            ensure_ascii=False,
            indent=2,
        )
        handle.write("\n")
    return rows


def normalize_indexes(raw_indexes: Any) -> list[dict[str, str | None]]:
    if not isinstance(raw_indexes, list):
        return []

    normalized: list[dict[str, str | None]] = []
    for item in raw_indexes:
        if isinstance(item, dict):
            normalized.append(
                {
                    "name": item.get("name"),
                    "symbol": item.get("proname"),
                }
            )
    return normalized


def make_one_liner(company_name: str | None, sector: str | None, industry: str | None) -> str | None:
    if company_name and sector and industry:
        return f"{company_name} operates in the {industry} industry within the {sector} sector."
    if company_name and industry:
        return f"{company_name} operates in the {industry} industry."
    if company_name and sector:
        return f"{company_name} operates in the {sector} sector."
    return None


def build_record(root: Path, logo: LogoFile, scanner_row: dict[str, Any] | None) -> dict[str, Any]:
    relative_path = logo.path.relative_to(root / "logos").as_posix()
    if not scanner_row:
        return {
            "id": logo.source_symbol,
            "exchange": logo.exchange,
            "ticker": logo.ticker,
            "file": relative_path,
            "matched": False,
            "company_name": None,
            "sector": None,
            "industry": None,
            "one_liner": None,
            "one_liner_source": None,
            "isin": None,
            "country": None,
            "currency": None,
            "market_cap": None,
            "security_type": None,
            "security_subtype": None,
            "is_index_symbol": False,
            "is_index_constituent": False,
            "indexes": [],
            "source": "tradingview",
            "source_symbol": logo.source_symbol,
        }

    company_name = scanner_row.get("description")
    sector = scanner_row.get("sector")
    industry = scanner_row.get("industry")
    indexes = normalize_indexes(scanner_row.get("indexes"))
    security_type = scanner_row.get("type")

    return {
        "id": logo.source_symbol,
        "exchange": logo.exchange,
        "ticker": logo.ticker,
        "file": relative_path,
        "matched": True,
        "company_name": company_name,
        "sector": sector,
        "industry": industry,
        "one_liner": make_one_liner(company_name, sector, industry),
        "one_liner_source": "derived_from_sector_industry",
        "isin": scanner_row.get("isin"),
        "country": scanner_row.get("country"),
        "currency": scanner_row.get("currency") or scanner_row.get("fundamental_currency_code"),
        "market_cap": scanner_row.get("market_cap_basic"),
        "security_type": security_type,
        "security_subtype": scanner_row.get("subtype"),
        "typespecs": scanner_row.get("typespecs") or [],
        "is_index_symbol": security_type == "index",
        "is_index_constituent": bool(indexes),
        "indexes": indexes,
        "logo_id": scanner_row.get("logoid"),
        "source": "tradingview",
        "source_symbol": logo.source_symbol,
    }


def main() -> int:
    args = parse_args()
    root = Path(args.root).resolve()
    output_path = resolve_path(root, args.output)
    cache_path = resolve_path(root, args.cache)

    logos = discover_logos(root)
    if not logos:
        print("No NSE/BSE SVG logos found.", file=sys.stderr)
        return 1

    symbols = sorted({logo.source_symbol for logo in logos})
    scanner_rows = load_or_fetch_cache(
        cache_path=cache_path,
        symbols=symbols,
        batch_size=args.batch_size,
        refresh=args.refresh,
    )

    records = [build_record(root, logo, scanner_rows.get(logo.source_symbol)) for logo in logos]
    matched = sum(1 for record in records if record["matched"])
    by_exchange = {
        exchange: sum(1 for record in records if record["exchange"] == exchange)
        for exchange in ("NSE", "BSE")
    }
    unmatched = [
        {
            "exchange": record["exchange"],
            "ticker": record["ticker"],
            "file": record["file"],
            "source_symbol": record["source_symbol"],
        }
        for record in records
        if not record["matched"]
    ]

    output = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "source": {
            "provider": "TradingView India scanner",
            "url": TRADINGVIEW_SCAN_URL,
            "note": "Company one_liner values are derived from scraped sector and industry fields.",
        },
        "counts": {
            "total_logos": len(records),
            "matched": matched,
            "unmatched": len(records) - matched,
            "by_exchange": by_exchange,
        },
        "logos": records,
        "unmatched": unmatched,
    }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8") as handle:
        json.dump(output, handle, ensure_ascii=False, indent=2)
        handle.write("\n")

    print(f"Wrote {output_path}")
    print(f"Matched {matched}/{len(records)} logos")
    if unmatched:
        print(f"Unmatched symbols: {len(unmatched)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
