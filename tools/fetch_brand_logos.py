#!/usr/bin/env python3
"""Update data/brand-logos/: artwork for the unlisted Indian brands in brands.json.

brands.json is kept by hand (slug, name, domain, category, aliases). For each brand the
artwork is, in order (Dharun, 2026-11-11):

1. a file pinned in brands.json as "commons": "File:....svg" (checked by a person);
   "commons": false skips Commons for a brand whose name it uses for something else;
2. an SVG on Wikimedia Commons whose title starts with the brand's name and says "logo",
   under a licence that allows reuse (public domain, CC0, CC BY, CC BY-SA), with the
   author and licence recorded for the credit;
3. the brand site's own icon, drawn again as a 256 px PNG by this API's /v1/icon code
   (--api, the live API by default), unless "site_icon": false.

A press-kit file can be added by hand: put it in svg/ or png/ and pin it in brands.json
as "file" with "artwork_url" and "licence". Brands with none of these get the monogram.
The origin of every file is written to metadata.json. Run with --dry-run first.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode
from urllib.request import Request, urlopen

sys.path.insert(0, str(Path(__file__).resolve().parent))
from scrape_logos import clean_svg  # noqa: E402

COMMONS_API = "https://commons.wikimedia.org/w/api.php"
USER_AGENT = "india-api-logo-update/1 (https://api.dharun.dev)"
ALLOWED_LICENCE = re.compile(r"(?i)^(public domain|pd\b|cc0|cc[- ]by(-sa)?[- ]\d)")
MAX_SVG = 300_000
TITLE_REST = re.compile(r"(logo|official|new|app|india|vector|wordmark|icon|svg|present|\d{4})*")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", default=".", help="Repository root. Defaults to the current directory.")
    parser.add_argument("--api", default="https://api.dharun.dev", help="The India API that draws site icons.")
    parser.add_argument("--resvg", help="resvg executable: an SVG that does not render is not used.")
    parser.add_argument("--recheck", action="store_true", help="Look again for brands that already have artwork.")
    parser.add_argument("--dry-run", action="store_true", help="Report what would be used without writing.")
    parser.add_argument("--only", help="Comma-separated slugs to look at.")
    return parser.parse_args()


def get(url: str, attempts: int = 3, agent: str = USER_AGENT) -> tuple[bytes, dict[str, str]]:
    request = Request(url, headers={"User-Agent": agent})
    for attempt in range(1, attempts + 1):
        try:
            with urlopen(request, timeout=60) as response:
                return response.read(), {k.lower(): v for k, v in response.headers.items()}
        except HTTPError as exc:
            if exc.code in (400, 403, 404, 422):
                raise
            error: Exception = exc
        except (URLError, TimeoutError) as exc:
            error = exc
        if attempt == attempts:
            raise RuntimeError(f"{url}: {error}") from error
        time.sleep(2 * attempt)
    raise AssertionError("unreachable")


def loose(text: str) -> str:
    return re.sub(r"[^a-z0-9]", "", text.lower())


def strip_html(text: str) -> str:
    return re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", text or "")).strip()


def commons_files(query: str) -> list[dict[str, Any]]:
    params = {"action": "query", "format": "json", "generator": "search", "gsrnamespace": "6",
              "gsrsearch": query, "gsrlimit": "20", "prop": "imageinfo",
              "iiprop": "url|mime|size|extmetadata"}
    body, _ = get(COMMONS_API + "?" + urlencode(params))
    pages = (json.loads(body).get("query") or {}).get("pages") or {}
    return list(pages.values())


def commons_info(title: str) -> dict[str, Any] | None:
    params = {"action": "query", "format": "json", "titles": title, "prop": "imageinfo",
              "iiprop": "url|mime|size|extmetadata"}
    body, _ = get(COMMONS_API + "?" + urlencode(params))
    pages = (json.loads(body).get("query") or {}).get("pages") or {}
    return next(iter(pages.values()), None)


def usable(page: dict[str, Any]) -> dict[str, Any] | None:
    info = (page.get("imageinfo") or [None])[0]
    if not info or info.get("mime") != "image/svg+xml" or info.get("size", 0) > MAX_SVG:
        return None
    meta = info.get("extmetadata") or {}
    licence = strip_html((meta.get("LicenseShortName") or {}).get("value", ""))
    if not ALLOWED_LICENCE.match(licence) or (meta.get("NonFree") or {}).get("value"):
        return None
    return {"url": info["url"], "page": info.get("descriptionurl"), "licence": licence,
            "author": strip_html((meta.get("Artist") or {}).get("value", ""))[:200] or None}


def renders(svg: bytes, resvg: str | None, scratch: Path) -> bool:
    if not resvg:
        return True
    source, target = scratch / "check.svg", scratch / "check.png"
    source.write_bytes(svg)
    return subprocess.run([resvg, "--width", "64", str(source), str(target)], capture_output=True).returncode == 0


def find_commons(brand: dict[str, Any]) -> dict[str, Any] | None:
    if brand.get("commons") is False:  # a person found that Commons has only other things by this name
        return None
    if brand.get("commons"):
        page = commons_info(brand["commons"])
        return usable(page) if page else None
    names = [loose(brand["name"])] + [loose(a) for a in brand.get("aliases", []) if len(loose(a)) >= 4]
    candidates = []
    for page in commons_files(f'intitle:"{brand["name"]}" logo filetype:drawing'):
        title = loose(re.sub(r"(?i)^file:|\.svg$", "", page.get("title", "")))
        # After the name only words like "logo", "official" or a year may follow, so
        # "Josh Shapiro logo" is not Josh and "Porter-Cable logo" is not Porter.
        if "logo" not in title or not any(n and title.startswith(n) and TITLE_REST.fullmatch(title[len(n):])
                                          for n in names):
            continue
        found = usable(page)
        if found:
            candidates.append((len(title), page["title"], found))
    if not candidates:
        return None
    candidates.sort(key=lambda c: (c[0], c[1]))
    _, title, found = candidates[0]
    found["title"] = title
    return found


def main() -> int:
    args = parse_args()
    root = Path(args.root).resolve()
    folder = root / "data" / "brand-logos"
    brands = json.loads((folder / "brands.json").read_text(encoding="utf-8"))
    metadata_path = folder / "metadata.json"
    previous = {}
    if metadata_path.exists():
        previous = {b["slug"]: b for b in json.loads(metadata_path.read_text(encoding="utf-8"))["logos"]}
    only = set(args.only.split(",")) if args.only else None
    scratch = folder / ".scratch"
    scratch.mkdir(exist_ok=True)

    records, counts = [], {"commons": 0, "site-icon": 0, "pinned": 0, "none": 0}
    for brand in brands:
        old = previous.get(brand["slug"])
        record = {"slug": brand["slug"], "name": brand["name"], "domain": brand["domain"],
                  "category": brand["category"], "aliases": brand.get("aliases", []),
                  "file": None, "artwork": "none", "artwork_url": None, "licence": None, "author": None}
        keep = old and old.get("file") and (folder / old["file"]).exists() and not args.recheck
        if keep or (only and brand["slug"] not in only):
            if old:
                record.update({k: old.get(k) for k in ("file", "artwork", "artwork_url", "licence", "author")})
            records.append(record)
            counts[record["artwork"] if record["artwork"] in counts else "none"] += 1
            continue

        if brand.get("file"):
            record.update(file=brand["file"], artwork="pinned", artwork_url=brand.get("artwork_url"),
                          licence=brand.get("licence"))
        if record["file"] is None:
            try:
                found = find_commons(brand)
            except (HTTPError, RuntimeError) as exc:
                print(f"{brand['slug']}: Commons lookup failed: {exc}", file=sys.stderr)
                found = None
            if found:
                svg = clean_svg(get(found["url"])[0])
                if svg and renders(svg, args.resvg, scratch):
                    record.update(file=f"svg/{brand['slug']}.svg", artwork="commons", artwork_url=found["page"],
                                  licence=found["licence"], author=found["author"])
                    if not args.dry_run:
                        (folder / "svg").mkdir(exist_ok=True)
                        (folder / record["file"]).write_bytes(svg)
        if record["file"] is None and brand.get("site_icon", True):
            try:
                png, headers = get(f"{args.api}/v1/icon/{quote(brand['domain'])}?size=256", attempts=2,
                                   agent="india-api-check/1")  # counted as the API's own traffic
                if not headers.get("x-logo-fallback"):
                    record.update(file=f"png/{brand['slug']}.png", artwork="site-icon",
                                  artwork_url=headers.get("x-icon-source") or f"https://{brand['domain']}/")
                    if not args.dry_run:
                        (folder / "png").mkdir(exist_ok=True)
                        (folder / record["file"]).write_bytes(png)
            except (HTTPError, RuntimeError) as exc:
                print(f"{brand['slug']}: site icon failed: {exc}", file=sys.stderr)
        counts[record["artwork"]] += 1
        print(f"{brand['slug']}: {record['artwork']} {record['artwork_url'] or ''}", file=sys.stderr)
        records.append(record)
        time.sleep(1)

    for path in scratch.glob("*"):
        path.unlink()
    scratch.rmdir()
    if args.dry_run:
        print(f"Dry run: {counts}")
        return 0
    used = {r["file"] for r in records if r["file"]}
    for path in list(folder.glob("svg/*.svg")) + list(folder.glob("png/*.png")):
        if path.relative_to(folder).as_posix() not in used:
            path.unlink()
    output = {
        "generated_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "source": {"provider": "Wikimedia Commons (files under a free licence); the brands' own site icons",
                   "note": "brands.json is kept by hand. Updated with tools/fetch_brand_logos.py."},
        "counts": {"brands": len(records), **counts},
        "logos": records,
    }
    with metadata_path.open("w", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps(output, ensure_ascii=False, indent=1) + "\n")
    print(f"Wrote {metadata_path}: {counts}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
