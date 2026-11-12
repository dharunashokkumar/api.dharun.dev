#!/usr/bin/env python3
"""Fetches three public domain classics from Wikisource into data/classics/.

  aathichoodi.json  Avvaiyar's Aathichoodi (Tamil, 109 lines and the invocation), from
                    ta.wikisource.org "ஆத்திசூடி". Only the original lines: the page's
                    modern glosses are left out.
  gita.json         The Bhagavad Gita in Sanskrit, 700 verses in 18 chapters, from
                    sa.wikisource.org "भगवद्गीता/<chapter>". Only the verses: the
                    commentaries on the same pages are left out.
  kabir.json        Kabir's sakhis (couplets), from hi.wikisource.org "कबीर ग्रंथावली"
                    (Shyamsundar Das's edition), the 17 angas the site has. Only the
                    couplets: the later commentary (सन्दर्भ, भावार्थ, शब्दार्थ) is left out.

The texts are centuries old and in the public domain; Wikisource is credited as the
transcription. Each page's revision id is recorded, so a rerun can be compared.

    python3 -I tools/classics/fetch_classics.py data/classics
"""
import html
import json
import re
import sys
import urllib.parse
import urllib.request

AGENT = "india-api-data/1 (https://api.dharun.dev/about)"


def api(lang, **params):
    params["format"] = "json"
    params["formatversion"] = "2"
    url = f"https://{lang}.wikisource.org/w/api.php?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={"User-Agent": AGENT})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.load(r)


def wikitext(lang, title):
    d = api(lang, action="parse", page=title, prop="wikitext|revid")["parse"]
    return d["wikitext"], d["revid"]


def rendered(lang, title):
    d = api(lang, action="parse", page=title, prop="text|revid")["parse"]
    t = d["text"]
    t = re.sub(r"<style.*?</style>", "", t, flags=re.S)
    t = re.sub(r"<br\s*/?>", "\n", t)
    t = re.sub(r"</(p|div|dd|li)>", "\n", t)
    t = re.sub(r"<[^>]+>", "", t)
    return html.unescape(t).replace("​", ""), d["revid"]


TAMIL_DIGITS = str.maketrans("०१२३४५६७८९", "0123456789")


def aathichoodi():
    text, rev = wikitext("ta", "ஆத்திசூடி")
    lines, section = [], ""
    invocation = re.findall(r"\{\{green\|:(.+?)\}\}", text)
    for line in text.splitlines():
        h = re.match(r"^===\s*(.+?)\s*===", line)
        if h:
            section = h.group(1)
            continue
        m = re.match(r"^(\d+)\.\s*\{\{green\|(.+?)\}\}", line)
        if m:
            lines.append({"number": int(m.group(1)), "section": section, "text": m.group(2).strip()})
    numbers = [l["number"] for l in lines]
    if numbers != list(range(1, 110)):
        sys.exit(f"Aathichoodi: expected lines 1 to 109, got {len(lines)}")
    return {"title": "ஆத்திசூடி", "title_en": "Aathichoodi", "author": "Avvaiyar", "language": "ta",
            "invocation": [i.strip() for i in invocation],
            "source": "https://ta.wikisource.org/wiki/ஆத்திசூடி", "revision": rev, "lines": lines}


GITA_CHAPTERS = ["अर्जुनविषादयोगः", "साङ्ख्ययोगः", "कर्मयोगः", "ज्ञानकर्मसंन्यासयोगः", "कर्मसंन्यासयोगः",
                 "आत्मसंयमयोगः", "ज्ञानविज्ञानयोगः", "अक्षरब्रह्मयोगः", "राजविद्याराजगुह्ययोगः", "विभूतियोगः",
                 "विश्वरूपदर्शनयोगः", "भक्तियोगः", "क्षेत्रक्षेत्रज्ञविभागयोगः", "गुणत्रयविभागयोगः",
                 "पुरुषोत्तमयोगः", "दैवासुरसम्पद्विभागयोगः", "श्रद्धात्रयविभागयोगः", "मोक्षसंन्यासयोगः"]
GITA_COUNTS = [47, 72, 43, 42, 29, 47, 30, 28, 34, 42, 55, 20, 34, 27, 20, 24, 28, 78]


def gita():
    chapters, revisions = [], {}
    for n, name in enumerate(GITA_CHAPTERS, 1):
        text, rev = wikitext("sa", "भगवद्गीता/" + name)
        revisions[name] = rev
        verses = []
        for poem in re.findall(r"<poem>(.*?)</poem>", text, flags=re.S):
            body = []
            for line in poem.splitlines():
                line = line.strip()
                if not line:
                    continue
                plain = re.sub(r"'''", "", line).strip()
                if plain in ("ॐ", "श्रीपरमात्मने नमः") or (plain.startswith("अथ ") and ("ध्याय" in plain or "गीता" in plain)) or plain.startswith("ॐ तत्सदिति") or (len(plain) < 25 and plain.endswith("ध्यायः")):
                    continue
                body.append(plain)
            joined = "\n".join(body)
            # Verses end with ॥3- 1॥ (chapter-verse); a poem block holds one or more.
            for part in re.finditer(r"(.+?)॥\s*([०-९\d]+)\s*[-–.]\s*([०-९\d]+)\s*॥", joined, flags=re.S):
                verse = int(part.group(3).translate(TAMIL_DIGITS))
                lines = []
                for l in part.group(1).strip().split("\n"):
                    l = l.strip(" ।")
                    if not l:
                        continue
                    # "अर्जुन उवाच", "श्रीभगवानुवाच": the transcription names the speaker
                    # only in places, so speakers are left out rather than guessed.
                    if len(l) < 30 and re.search(r"(उवाच|ुवाच)$", l):
                        continue
                    lines.append(l)
                if verses and verses[-1]["verse"] == verse:
                    continue
                verses.append({"verse": verse, "text": "\n".join(lines)})
        got = [v["verse"] for v in verses]
        if got != list(range(1, GITA_COUNTS[n - 1] + 1)):
            missing = sorted(set(range(1, GITA_COUNTS[n - 1] + 1)) - set(got))
            sys.exit(f"Gita chapter {n}: {len(verses)} verses read, missing {missing[:10]}")
        chapters.append({"chapter": n, "name": name, "verses": verses})
    return {"title": "श्रीमद्भगवद्गीता", "title_en": "Bhagavad Gita", "language": "sa",
            "source": "https://sa.wikisource.org/wiki/भगवद्गीता", "revisions": revisions, "chapters": chapters}


KABIR_ANGAS = ["(१) गुरुदेव कौ अंग", "(२) सुमिरन कौ अंग", "(३) बिरह कौ अंग", "(४) ग्यान विरह कौ अंग", "(५) परचा कौ अंग",
               "(६) रस कौ अंग", "(७) लाबि कौ अंग", "(८) जर्णा कौ अंग", "(९) हैरान कौ अंग", "(१०) लै कौ अंग",
               "(११) निहकर्मी पतिव्रता कौ अंग", "(१२) चितावणी कौ अंग", "(१३) मन को अंग", "(१४) सूषिम मारग कौ अंग",
               "(१५) सूषिम जनम कौ अंग", "(१६) माया कौ अंग", "(१७) चाँणक कौ अंग"]


def kabir():
    angas, revisions = [], {}
    for n, title in enumerate(KABIR_ANGAS, 1):
        text, rev = rendered("hi", "कबीर ग्रंथावली/" + title)
        revisions[title] = rev
        lines = [l.strip() for l in text.splitlines() if l.strip()]
        couplets = []
        for i, line in enumerate(lines):
            m = re.search(r"॥\s*([०-९\d]+)\s*॥\s*$", line)
            if not m or i == 0:
                continue
            first = lines[i - 1]
            # The first half ends with a danda or comma; commentary lines are prose.
            if any(first.startswith(p) for p in ("सन्दर्भ", "संदर्भ", "भावार्थ", "शब्दार्थ")) or len(first) > 90:
                continue
            number = int(m.group(1).translate(TAMIL_DIGITS))
            second = line[:m.start()].strip()
            # Numbers only rise; a jump far ahead is a misread number in the transcription.
            if couplets and (couplets[-1]["number"] >= number or number > couplets[-1]["number"] + 20):
                continue
            couplets.append({"number": number, "text": first.rstrip(" ।") + "।\n" + second + "॥"})
        if len(couplets) < 1:
            sys.exit(f"Kabir {title}: only {len(couplets)} couplets read")
        name = re.sub(r"^\([^)]*\)\s*", "", title)
        angas.append({"anga": n, "name": name, "couplets": couplets})
    return {"title": "कबीर साखी", "title_en": "Kabir's sakhis (couplets)", "author": "Kabir", "language": "hi",
            "edition": "Kabir Granthavali, ed. Shyamsundar Das (Nagari Pracharini Sabha)",
            "source": "https://hi.wikisource.org/wiki/कबीर_ग्रंथावली", "revisions": revisions, "angas": angas}


def main(out):
    for name, fn in (("aathichoodi", aathichoodi), ("gita", gita), ("kabir", kabir)):
        data = fn()
        with open(f"{out}/{name}.json", "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=1)
            f.write("\n")
        print(name, "written", file=sys.stderr)


if __name__ == "__main__":
    main(sys.argv[1])
