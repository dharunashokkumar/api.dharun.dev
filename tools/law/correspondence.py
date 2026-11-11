#!/usr/bin/env python3
"""Reads the correspondence tables between India's new criminal laws and the old ones
(BNS and IPC, BNSS and CrPC, BSA and the Indian Evidence Act) from the Bureau of Police
Research and Development's comparison summaries, into data/law/<new>.json.

Only facts are taken: which section of the new law corresponds to which of the old one,
and each section's title, which is the Act's own text. The written summaries of what
changed are someone's work (the PDFs carry a copyright line) and are left out.

    python3 -I tools/law/correspondence.py <pdf> <bns|bnss|bsa> <out.json>

Needs poppler's pdftotext. Columns are found from the header row's word positions.
"""
import html
import json
import re
import subprocess
import sys

WORD = re.compile(r'<word xMin="([\d.]+)" yMin="([\d.]+)" xMax="([\d.]+)" yMax="([\d.]+)">(.*?)</word>')
PAGE = re.compile(r"<page ")
SECTION = re.compile(r"^\d+[A-Z]?(\(|$)")

LAYOUT = {
    # law: (header word of each column, in the order they stand on the page; their roles)
    "bns": (["Subject", "IPC", "Summary"], ["new", "subject", "old", "summary"]),
    "bnss": (["Subject", "CrPC", "Summary"], ["new", "subject", "old", "summary"]),
    "bsa": (["IEA", "Subject", "Summary"], ["new", "old", "subject", "summary"]),
}


def words(pdf):
    out = subprocess.run(["pdftotext", "-bbox", pdf, "-"], capture_output=True, text=True, check=True).stdout
    pages = PAGE.split(out)[1:]
    for number, page in enumerate(pages, 1):
        for m in WORD.finditer(page):
            yield number, float(m.group(1)), float(m.group(2)), float(m.group(3)), html.unescape(m.group(5))


def lines(pdf):
    """Words grouped into lines by page and height."""
    current, key = [], None
    for page, x0, y0, x1, text in sorted(words(pdf), key=lambda w: (w[0], round(w[2]), w[1])):
        k = (page, round(y0 / 3))
        if key is not None and (k[0] != key[0] or abs(y0 - current[0][2]) > 3):
            yield sorted(current, key=lambda w: w[1])
            current = []
        key = k
        current.append((page, x0, y0, x1, text))
    if current:
        yield sorted(current, key=lambda w: w[1])


def read(pdf, law):
    heads, roles = LAYOUT[law]
    edges = None
    entries = []
    for line in lines(pdf):
        texts = [w[4] for w in line]
        if edges is None:
            # The header: the first line holding every column's header word.
            found = {}
            for w in line:
                for h in heads:
                    if w[4].startswith(h) and h not in found:
                        found[h] = w[1]
            if len(found) == len(heads):
                xs = [found[h] for h in heads]
                edges = [x - 6 for x in xs]
            continue
        if texts and (texts[0].startswith("©") or texts[0] in ("BNS", "BNSS", "BSA")):
            continue
        cols = [[] for _ in roles]
        for w in line:
            i = sum(1 for e in edges if w[1] >= e)
            cols[i].append(w[4])
        cell = {r: " ".join(c).strip() for r, c in zip(roles, cols)}
        cell["new"] = cell["new"].replace(" ", "")
        if SECTION.match(cell["new"]):
            entries.append({r: cell[r] for r in roles})
        elif entries:
            last = entries[-1]
            for r in roles:
                if cell[r]:
                    joiner = "" if r == "new" or (r == "old" and last[r].endswith(("(", "-"))) else " "
                    last[r] = (last[r] + joiner + cell[r]).strip()
    out = []
    for e in entries:
        old = re.sub(r"\s+", " ", e["old"]).strip(" ]")
        new = re.sub(r"\s+", "", e["new"])
        part = ""
        m = re.match(r"(\d+(?:[A-Z](?![a-z]))?(?:\([^)]*\))*)(.*)$", new)
        if m and m.group(2):
            # "5Explanationtosection": a note on part of a section, its words run together.
            notes = {"explanationtosection": "Explanation to section", "provisotosection": "Proviso to section",
                     "firstprovisotosection": "First proviso to section", "secondprovisotosection": "Second proviso to section"}
            new, part = m.group(1), notes.get(m.group(2).lower(), m.group(2))
            if re.fullmatch(r"\(?[a-z]\)?", part):  # "197(1)d": a clause missing its brackets
                new, part = new + "(" + part.strip("()") + ")", ""
        is_new = old.lower() in ("new", "-", "—", "") or old.lower().startswith("new")
        out.append({
            "section": new,
            "part": part,
            "title": re.sub(r"\s+", " ", e["subject"]).strip().rstrip("."),
            "old": "" if is_new else old,
            "old_sections": [] if is_new else sorted(set(re.findall(r"\d+[A-Z]*", re.sub(r"para \d+", "", old))), key=lambda s: (int(re.match(r"\d+", s).group()), s)),
        })
    return out


def main():
    pdf, law, path = sys.argv[1:4]
    rows = read(pdf, law)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(rows, f, ensure_ascii=False, indent=0)
        f.write("\n")
    print(f"{law}: {len(rows)} rows")


if __name__ == "__main__":
    main()
