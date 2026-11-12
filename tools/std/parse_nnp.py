#!/usr/bin/env python3
"""Reads the list of SDCA (STD) codes from the National Numbering Plan 2003.

The Department of Telecommunications' National Numbering Plan (2003), Annex II, lists
every Short Distance Charging Area with its trunk code, its Long Distance Charging Area
and its licensed service area. dot.gov.in refuses most programs, so the copy read is the
Internet Archive's of 8 March 2022 (pinned below by checksum).

    curl -L -o nnp.pdf "https://web.archive.org/web/20220308090551id_/https://dot.gov.in/sites/default/files/nnp2003_0_0.pdf"
    python3 -I tools/std/parse_nnp.py nnp.pdf data/std-codes/sdca.json

Needs pdftotext (poppler-utils). Code: AGPL-3.0-only like the rest of the repository.
"""
import hashlib
import json
import re
import subprocess
import sys

PDF_SHA256 = "5037b8a360191bb7be53e2a30022c0bcd04677a57d8a00bd9b3372afbf9cad9a"

# Licensed service areas as the plan abbreviates them.
CIRCLES = {
    "AN": "Andaman and Nicobar", "AP": "Andhra Pradesh", "AS": "Assam", "BR": "Bihar", "BY": "Mumbai",
    "GJ": "Gujarat", "HA": "Haryana", "HP": "Himachal Pradesh", "JK": "Jammu and Kashmir", "KL": "Kerala",
    "KT": "Karnataka", "MH": "Maharashtra", "MP": "Madhya Pradesh", "ND": "Delhi", "NE": "North East",
    "OR": "Odisha", "PB": "Punjab", "RJ": "Rajasthan", "TN": "Tamil Nadu", "UPE": "Uttar Pradesh (East)",
    "UPW": "Uttar Pradesh (West)", "WB": "West Bengal", "KO": "Kolkata", "CH": "Chennai",
}

ROW = re.compile(r"^\s*(\d+)\s*o?\s+([A-Z]{2,3})\s+(.*?)\s+(\d{2,5})\s*$")


def clean(name):
    name = re.sub(r"\s+", " ", name).strip()
    name = name.replace("( ", "(").replace(" )", ")")
    if name.count("(") > name.count(")"):
        name += ")"
    if name.count(")") > name.count("(") and " " in name:
        # "ANANTPUR GUNTAKAL)" in the source, for "ANANTPUR (GUNTAKAL)".
        head, last = name.rsplit(" ", 1)
        name = head + " (" + last
    return name


def main(pdf, out):
    raw = open(pdf, "rb").read()
    if hashlib.sha256(raw).hexdigest() != PDF_SHA256:
        sys.exit("the PDF is not the pinned copy of the National Numbering Plan 2003")
    text = subprocess.run(["pdftotext", "-layout", pdf, "-"], capture_output=True, check=True).stdout.decode()
    lines = text.splitlines()
    start = next(i for i, l in enumerate(lines) if "ANNEX-II" in l and "III" not in l)
    end = next(i for i, l in enumerate(lines) if "ANNEX-III" in l)
    rows, ldcas = [], {}
    sdca_col = 40
    for line in lines[start:end]:
        if "SDCA NAME" in line:
            sdca_col = line.index("SDCA NAME")
            continue
        if "National Numbering Plan" in line or not line.strip():
            continue
        m = ROW.match(line)
        if m:
            number, circle, middle, code = m.groups()
            parts = re.split(r"\s{2,}", middle.strip())
            if len(parts) != 2:
                # The LDCA ran into the SDCA column: split after a known LDCA name of the
                # circle, else at the column.
                known = sorted(ldcas.get(circle, ()), key=len, reverse=True)
                hit = next((k for k in known if middle.strip().startswith(k + " ")), None)
                if hit:
                    parts = [hit, middle.strip()[len(hit):]]
                elif ") " in middle.strip():
                    left, right = middle.strip().split(") ", 1)
                    parts = [left + ")", right]
                else:
                    at = sdca_col - line.index(middle)
                    parts = [middle[:at], middle[at:]] if 0 < at < len(middle) else [middle, middle]
            ldca, sdca = clean(parts[0]), clean(parts[1])
            ldcas.setdefault(circle, set()).add(ldca)
            rows.append({"n": int(number), "circle": circle, "ldca": ldca, "sdca": sdca, "code": code})
            continue
        # A name that wrapped onto the next line, in the LDCA or the SDCA column.
        if rows and line.strip().startswith("("):
            indent = len(line) - len(line.lstrip(" "))
            key = "sdca" if indent >= sdca_col - 4 else "ldca"
            if line.strip() not in rows[-1][key]:
                rows[-1][key] = clean(rows[-1][key] + " " + line.strip())
    numbers = [r["n"] for r in rows]
    missing = sorted(set(range(1, max(numbers) + 1)) - set(numbers))
    if missing:
        print("rows not read:", missing[:50], file=sys.stderr)
    out_rows = []
    for r in rows:
        if r["circle"] not in CIRCLES:
            sys.exit("unknown circle " + r["circle"])
        out_rows.append({"code": r["code"], "sdca": r["sdca"].title(), "ldca": r["ldca"].title(),
                         "circle": r["circle"], "circle_name": CIRCLES[r["circle"]]})
    with open(out, "w", encoding="utf-8") as f:
        json.dump(out_rows, f, ensure_ascii=False, indent=0)
        f.write("\n")
    print(f"{len(out_rows)} SDCAs, {len(set(r['code'] for r in out_rows))} codes", file=sys.stderr)


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
