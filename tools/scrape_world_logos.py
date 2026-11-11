#!/usr/bin/env python3
"""Update data/stock-logos/ and data/asset-logos/: global stocks, crypto, indices and ETFs.

Stocks: every share and depositary receipt listed first on NASDAQ, NYSE or NYSE American,
and the largest companies by market capitalisation on the other major markets
(--per-market, default 500 each). Assets: the largest coins, a fixed list of indices,
and the largest ETFs in India and the US. Names, tickers and logo files come from the
TradingView scanner, like tools/scrape_logos.py. Crypto icons from spothq's CC0 set are
preferred where it has the coin (--crypto-icons, an unpacked copy of
github.com/spothq/cryptocurrency-icons). Commodity icons are drawn here and kept by hand
in data/asset-logos/drawn/ with data/asset-logos/commodities.json; this script copies
them into the metadata and never changes them.

Logo files are stored once per TradingView logo, so share classes share a file. A file
already present is not downloaded again unless --recheck. Run with --dry-run first.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parent))
from scrape_logos import clean_svg, download, post_json  # noqa: E402

SCANNER = "https://scanner.tradingview.com/{}/scan"

# market, exchanges, how many (None for every listing), country
STOCK_MARKETS = [
    ("america", ["NASDAQ", "NYSE", "AMEX"], None, "United States"),
    ("uk", ["LSE"], "per", "United Kingdom"),
    ("japan", ["TSE"], "per", "Japan"),
    ("hongkong", ["HKEX"], "per", "Hong Kong"),
    ("germany", ["XETR", "FWB"], "per", "Germany"),
    ("france", ["EURONEXT"], "per", "Euronext"),
    ("canada", ["TSX"], "per", "Canada"),
    ("australia", ["ASX"], "per", "Australia"),
    ("switzerland", ["SIX"], "per", "Switzerland"),
]
STOCK_COLUMNS = ["name", "description", "exchange", "type", "logoid", "isin", "sector", "industry",
                 "country", "currency", "market_cap_basic"]

# TradingView ticker, the symbol served, name, other names it is asked for by.
INDICES = [
    ("NSE:NIFTY", "NIFTY50", "Nifty 50", ["NIFTY", "NIFTY 50", "CNXNIFTY"]),
    ("NSE:BANKNIFTY", "BANKNIFTY", "Nifty Bank", ["NIFTYBANK", "NIFTY BANK"]),
    ("NSE:CNXFINANCE", "FINNIFTY", "Nifty Financial Services", ["NIFTYFIN", "CNXFINANCE"]),
    ("NSE:NIFTY_MID_SELECT", "MIDCPNIFTY", "Nifty Midcap Select", ["NIFTYMIDSELECT"]),
    ("NSE:NIFTYJR", "NIFTYNEXT50", "Nifty Next 50", ["NIFTYJR", "NIFTY NEXT 50"]),
    ("NSE:CNX100", "NIFTY100", "Nifty 100", ["CNX100"]),
    ("NSE:CNX200", "NIFTY200", "Nifty 200", ["CNX200"]),
    ("NSE:CNX500", "NIFTY500", "Nifty 500", ["CNX500"]),
    ("NSE:NIFTYMIDCAP150", "NIFTYMIDCAP150", "Nifty Midcap 150", []),
    ("NSE:NIFTYSMLCAP250", "NIFTYSMALLCAP250", "Nifty Smallcap 250", ["NIFTYSMLCAP250"]),
    ("NSE:CNXIT", "NIFTYIT", "Nifty IT", ["CNXIT"]),
    ("NSE:CNXAUTO", "NIFTYAUTO", "Nifty Auto", ["CNXAUTO"]),
    ("NSE:CNXFMCG", "NIFTYFMCG", "Nifty FMCG", ["CNXFMCG"]),
    ("NSE:CNXPHARMA", "NIFTYPHARMA", "Nifty Pharma", ["CNXPHARMA"]),
    ("NSE:CNXMETAL", "NIFTYMETAL", "Nifty Metal", ["CNXMETAL"]),
    ("NSE:CNXREALTY", "NIFTYREALTY", "Nifty Realty", ["CNXREALTY"]),
    ("NSE:CNXENERGY", "NIFTYENERGY", "Nifty Energy", ["CNXENERGY"]),
    ("NSE:CNXMEDIA", "NIFTYMEDIA", "Nifty Media", ["CNXMEDIA"]),
    ("NSE:CNXPSUBANK", "NIFTYPSUBANK", "Nifty PSU Bank", ["CNXPSUBANK"]),
    ("NSE:NIFTYPVTBANK", "NIFTYPVTBANK", "Nifty Private Bank", []),
    ("NSE:CNXINFRA", "NIFTYINFRA", "Nifty Infrastructure", ["CNXINFRA"]),
    ("NSE:INDIAVIX", "INDIAVIX", "India VIX", ["VIXINDIA"]),
    ("BSE:SENSEX", "SENSEX", "BSE Sensex", ["BSESENSEX", "SENSEX30"]),
    ("BSE:BANKEX", "BANKEX", "BSE Bankex", []),
    ("BSE:BSE500", "BSE500", "BSE 500", []),
    ("SP:SPX", "SPX", "S&P 500", ["SP500", "S&P500", "GSPC", "INX"]),
    ("NASDAQ:NDX", "NDX", "Nasdaq 100", ["NASDAQ100", "NAS100"]),
    ("NASDAQ:IXIC", "IXIC", "Nasdaq Composite", ["NASDAQCOMPOSITE", "COMP"]),
    ("DJ:DJI", "DJI", "Dow Jones Industrial Average", ["DOW", "DOWJONES", "DJIA", "US30"]),
    ("TVC:RUT", "RUT", "Russell 2000", ["RUSSELL2000"]),
    ("CBOE:VIX", "VIX", "CBOE Volatility Index", []),
    ("TVC:UKX", "FTSE100", "FTSE 100", ["UKX", "FTSE"]),
    ("TVC:NI225", "NIKKEI225", "Nikkei 225", ["NI225", "N225", "NIKKEI"]),
    ("TVC:HSI", "HSI", "Hang Seng", ["HANGSENG"]),
    ("XETR:DAX", "DAX", "DAX", ["DAX40", "GDAXI"]),
    ("EURONEXT:PX1", "CAC40", "CAC 40", ["PX1", "FCHI"]),
    ("TVC:SX5E", "EUROSTOXX50", "Euro Stoxx 50", ["SX5E", "STOXX50E"]),
    ("TSX:TSX", "TSX", "S&P/TSX Composite", ["GSPTSE"]),
    ("ASX:XJO", "ASX200", "S&P/ASX 200", ["XJO", "AXJO"]),
    ("SSE:000001", "SSECOMPOSITE", "SSE Composite", ["SHCOMP", "000001.SS"]),
    ("KRX:KOSPI", "KOSPI", "KOSPI", ["KS11"]),
    ("TVC:STI", "STI", "Straits Times Index", ["STRAITSTIMES"]),
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", default=".", help="Repository root. Defaults to the current directory.")
    parser.add_argument("--dry-run", action="store_true", help="Report what would change without downloading or writing.")
    parser.add_argument("--recheck", action="store_true", help="Download every logo again.")
    parser.add_argument("--workers", type=int, default=4, help="Logo downloads at a time. Default 4.")
    parser.add_argument("--per-market", type=int, default=500, help="Companies per market outside the US. Default 500.")
    parser.add_argument("--coins", type=int, default=500, help="Coins by market capitalisation. Default 500.")
    parser.add_argument("--etfs", type=int, default=300, help="US ETFs by assets. Default 300 (every Indian ETF is kept).")
    parser.add_argument("--crypto-icons", help="An unpacked copy of spothq/cryptocurrency-icons (CC0).")
    parser.add_argument("--sets", default="stocks,assets", help="Which sets to update. Default stocks,assets.")
    return parser.parse_args()


def scan(market: str, filters: list[dict[str, Any]], columns: list[str], limit: int | None,
         sort: str | None = "market_cap_basic") -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    while True:
        size = 2000 if limit is None else min(2000, limit - len(rows))
        body: dict[str, Any] = {"filter": filters, "options": {"lang": "en"}, "columns": columns,
                                "range": [len(rows), len(rows) + size]}
        if sort:
            body["sort"] = {"sortBy": sort, "sortOrder": "desc"}
        result = post_json(SCANNER.format(market), body)
        if result.get("error"):
            raise RuntimeError(f"{market}: scanner error: {result['error']}")
        page = result.get("data") or []
        for item in page:
            row = dict(zip(columns, item.get("d") or []))
            row["id"] = item["s"]
            rows.append(row)
        if not page or len(rows) >= result.get("totalCount", 0) or (limit is not None and len(rows) >= limit):
            return rows


def lookup(tickers: list[str], columns: list[str]) -> dict[str, dict[str, Any]]:
    result = post_json(SCANNER.format("global"), {"symbols": {"tickers": tickers}, "columns": columns})
    return {item["s"]: dict(zip(columns, item.get("d") or [])) for item in result.get("data") or []}


def safe_name(logoid: str) -> str:
    return re.sub(r"[^a-z0-9._-]+", "_", logoid.lower()).strip("_")


def entry(**fields: Any) -> dict[str, Any]:
    base = {"id": None, "symbol": None, "exchange": None, "kind": None, "name": None, "aliases": [],
            "file": None, "country": None, "sector": None, "industry": None, "isin": None,
            "currency": None, "market_cap": None, "artwork": "tradingview", "logo_id": None}
    base.update(fields)
    return base


def collect_stocks(args: argparse.Namespace) -> list[dict[str, Any]]:
    entries = []
    for market, exchanges, limit, country in STOCK_MARKETS:
        rows = scan(market, [
            {"left": "exchange", "operation": "in_range", "right": exchanges},
            {"left": "type", "operation": "in_range", "right": ["stock", "dr"]},
            {"left": "is_primary", "operation": "equal", "right": True},
        ], STOCK_COLUMNS, args.per_market if limit == "per" else None)
        print(f"{market}: {len(rows)} listings", file=sys.stderr)
        for row in rows:
            exchange, ticker = row["id"].split(":", 1)
            entries.append(entry(
                id=row["id"], symbol=ticker, exchange=exchange, kind=row.get("type"),
                name=row.get("description"), country=row.get("country") or country,
                sector=row.get("sector"), industry=row.get("industry"), isin=row.get("isin") or None,
                currency=row.get("currency"), market_cap=row.get("market_cap_basic"),
                logo_id=row.get("logoid") or None))
    return entries


def collect_assets(args: argparse.Namespace, root: Path) -> list[dict[str, Any]]:
    entries = []

    spothq: dict[str, tuple[str, Path]] = {}
    if args.crypto_icons:
        icons = Path(args.crypto_icons)
        for coin in json.loads((icons / "manifest.json").read_text(encoding="utf-8")):
            path = icons / "svg" / "color" / f"{coin['symbol'].lower()}.svg"
            if path.exists() and not coin["symbol"].startswith("$"):
                spothq[coin["symbol"].upper()] = (coin["name"], path)
    coins = scan("coin", [], ["base_currency", "description", "base_currency_logoid", "market_cap_calc"],
                 args.coins, "market_cap_calc")
    print(f"coins: {len(coins)}", file=sys.stderr)
    seen = set()
    for row in coins:
        symbol = (row.get("base_currency") or "").upper()
        if not re.fullmatch(r"[A-Z0-9]{1,15}", symbol) or symbol in seen:
            continue
        seen.add(symbol)
        artwork, logo_id = "tradingview", row.get("base_currency_logoid") or None
        if symbol in spothq:
            artwork, logo_id = "spothq-cc0", None
        entries.append(entry(id="CRYPTO:" + symbol, symbol=symbol, kind="crypto", name=row.get("description"),
                             market_cap=row.get("market_cap_calc"), artwork=artwork, logo_id=logo_id))
    # Coins the CC0 set has that are no longer among the largest.
    for symbol, (name, _) in sorted(spothq.items()):
        if symbol not in seen and re.fullmatch(r"[A-Z0-9]{1,15}", symbol):
            entries.append(entry(id="CRYPTO:" + symbol, symbol=symbol, kind="crypto", name=name, artwork="spothq-cc0"))

    found = lookup([tv for tv, *_ in INDICES], ["description", "logoid"])
    for tv, symbol, name, aliases in INDICES:
        row = found.get(tv)
        if not row or not row.get("logoid"):
            print(f"index {tv}: no logo, left out", file=sys.stderr)
            continue
        entries.append(entry(id="INDEX:" + symbol, symbol=symbol, exchange=tv.split(":")[0], kind="index",
                             name=name, aliases=sorted(set(aliases)), logo_id=row["logoid"]))

    etf_columns = ["name", "description", "exchange", "logoid", "aum", "isin", "currency"]
    etf_filter = [{"left": "type", "operation": "equal", "right": "fund"},
                  {"left": "subtype", "operation": "equal", "right": "etf"},
                  {"left": "is_primary", "operation": "equal", "right": True}]
    india = scan("india", etf_filter + [{"left": "exchange", "operation": "equal", "right": "NSE"}], etf_columns, None, "aum")
    america = scan("america", etf_filter + [{"left": "exchange", "operation": "in_range", "right": ["NASDAQ", "NYSE", "AMEX"]}],
                   etf_columns, args.etfs, "aum")
    print(f"ETFs: India {len(india)}, US {len(america)}", file=sys.stderr)
    for row in india + america:
        exchange, ticker = row["id"].split(":", 1)
        entries.append(entry(id=row["id"], symbol=ticker, exchange=exchange, kind="etf", name=row.get("description"),
                             isin=row.get("isin") or None, currency=row.get("currency"), market_cap=row.get("aum"),
                             logo_id=row.get("logoid") or None))

    commodities = root / "data" / "asset-logos" / "commodities.json"
    for item in json.loads(commodities.read_text(encoding="utf-8")):
        entries.append(entry(id="COMMODITY:" + item["symbol"], symbol=item["symbol"], kind="commodity",
                             name=item["name"], aliases=item.get("aliases", []), file=item["file"], artwork="drawn"))

    spothq_files = {symbol: path for symbol, (_, path) in spothq.items()}
    for item in entries:
        if item["artwork"] == "spothq-cc0":
            item["_cc0"] = spothq_files[item["symbol"]]
    return entries


def write_set(name: str, entries: list[dict[str, Any]], args: argparse.Namespace, root: Path, source: dict[str, Any]) -> None:
    folder = root / "data" / name
    svg_dir, cc0_dir = folder / "svg", folder / "cc0"
    # Duplicate ids (a ticker seen twice) keep the first, which is the larger.
    unique: dict[str, dict[str, Any]] = {}
    for item in entries:
        unique.setdefault(item["id"], item)
    entries = list(unique.values())

    wanted = sorted({item["logo_id"] for item in entries if item["logo_id"] and item["artwork"] == "tradingview"
                     and (args.recheck or not (svg_dir / (safe_name(item["logo_id"]) + ".svg")).exists())})
    print(f"{name}: {len(entries)} entries, {len(wanted)} logos to download", file=sys.stderr)
    if args.dry_run:
        return

    svg_dir.mkdir(parents=True, exist_ok=True)
    with ThreadPoolExecutor(max_workers=max(1, args.workers)) as pool:
        for done, (logo_id, svg) in enumerate(zip(wanted, pool.map(download, wanted)), 1):
            path = svg_dir / (safe_name(logo_id) + ".svg")
            if svg is not None:
                path.write_bytes(svg)
            elif path.exists():
                path.unlink()
            if done % 500 == 0 or done == len(wanted):
                print(f"{name}: downloaded {done}/{len(wanted)}", file=sys.stderr)

    used = set()
    for item in entries:
        cc0 = item.pop("_cc0", None)
        if cc0 is not None:
            svg = clean_svg(cc0.read_bytes())
            if svg is None:
                item["artwork"], item["file"] = "none", None
                continue
            cc0_dir.mkdir(parents=True, exist_ok=True)
            item["file"] = f"cc0/{item['symbol'].lower()}.svg"
            (folder / item["file"]).write_bytes(svg)
        elif item["artwork"] == "tradingview":
            path = svg_dir / (safe_name(item["logo_id"]) + ".svg") if item["logo_id"] else None
            item["file"] = f"svg/{path.name}" if path and path.exists() else None
        if item["file"]:
            used.add(item["file"])
    removed = 0
    for path in list(svg_dir.glob("*.svg")) + list(cc0_dir.glob("*.svg")):
        if path.relative_to(folder).as_posix() not in used:
            path.unlink()
            removed += 1

    output = {
        "generated_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "source": source,
        "counts": {"entries": len(entries), "with_logo": sum(1 for e in entries if e["file"]),
                   "files": len(used), "by_kind": {k: sum(e["kind"] == k for e in entries)
                                                   for k in sorted({e["kind"] for e in entries})}},
        "logos": sorted(entries, key=lambda e: (e["kind"], e["exchange"] or "", e["symbol"])),
    }
    with (folder / "metadata.json").open("w", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps(output, ensure_ascii=False, indent=1) + "\n")
    print(f"{name}: wrote {len(entries)} entries, {output['counts']['with_logo']} with a logo, "
          f"{len(used)} files, {removed} files removed", file=sys.stderr)


def main() -> int:
    args = parse_args()
    root = Path(args.root).resolve()
    sets = set(args.sets.split(","))
    if "stocks" in sets:
        write_set("stock-logos", collect_stocks(args), args, root, {
            "provider": "TradingView scanner",
            "url": SCANNER.format("america"),
            "note": f"Every primary share and depositary receipt on NASDAQ, NYSE and NYSE American; the "
                    f"{args.per_market} largest by market capitalisation on LSE, TSE, HKEX, Xetra and Frankfurt, "
                    f"Euronext, TSX, ASX and SIX. Updated with tools/scrape_world_logos.py.",
        })
    if "assets" in sets:
        if not args.crypto_icons:
            print("--crypto-icons is needed for the assets set", file=sys.stderr)
            return 2
        write_set("asset-logos", collect_assets(args, root), args, root, {
            "provider": "TradingView scanner; spothq/cryptocurrency-icons (CC0 1.0); commodity icons drawn for this API",
            "url": SCANNER.format("coin"),
            "note": f"The {args.coins} largest coins, a fixed list of indices, every NSE ETF and the {args.etfs} "
                    f"largest US ETFs. Updated with tools/scrape_world_logos.py.",
        })
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
