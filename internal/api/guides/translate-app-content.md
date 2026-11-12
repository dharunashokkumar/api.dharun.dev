---
title: Translate app content into Indian languages
description: Translate app strings between English and 22 Indian languages with IndicTrans2, keep placeholders safe, translate at build time, and know when a person should review.
apis: translate
published: 2026-11-12
---

India API runs IndicTrans2 (AI4Bharat's open model) on its own server: English to and from 22 Indian languages, and between two Indian languages through English. Nothing you send is logged or kept. It is a model endpoint: from 15 December 2026 it needs a [free API key](https://api.dharun.dev/signup).

## The languages

```sh
curl "https://api.dharun.dev/v1/translate/languages"
```

Each language has a short code (`ta`, `hi`, `bn`...) and its FLORES-200 code (`tam_Taml`); either works in `from` and `to`. Write Indian languages in their own script, not in Roman letters (for Roman typing, `/v1/transliterate/suggest` turns "vanakkam" into வணக்கம் first).

## Translate a message

```python
import os
import requests

BASE = "https://api.dharun.dev/v1"
HEADERS = {"Authorization": f"Bearer {os.environ['INDIA_API_KEY']}"}

def translate(text, to, source="en"):
    r = requests.post(f"{BASE}/translate", params={"from": source, "to": to},
                      json={"text": text}, headers=HEADERS, timeout=60)
    r.raise_for_status()
    return r.json()["data"]["translation"]

print(translate("Your order has been shipped and will arrive by Friday.", "ta"))
print(translate("Your order has been shipped and will arrive by Friday.", "hi"))
```

What the model answered:

```text
உங்கள் ஆர்டர் அனுப்பப்பட்டு வெள்ளிக்கிழமைக்குள் வரும்.
आपका ऑर्डर भेज दिया गया है और शुक्रवार तक आ जाएगा।
```

The answer also has `sentences` (the text is translated a sentence at a time), `via_english` (true between two Indian languages) and `truncated` (sentences cut at the model's length limit). Up to 2,000 characters and 30 sentences a request.

## Keep placeholders and names safe

Templates such as `Hi {name}, your OTP is {otp}` must come back with the placeholders untouched. Swap them for tokens the model leaves alone, translate, and swap back:

```python
import re

def translate_template(template, to):
    names = re.findall(r"\{\w+\}", template)
    masked = template
    for i, n in enumerate(names):
        masked = masked.replace(n, f"X{i}X", 1)
    out = translate(masked, to)
    for i, n in enumerate(names):
        out = out.replace(f"X{i}X", n)
    missing = [n for n in names if n not in out]
    if missing:
        raise ValueError(f"placeholders lost in {to}: {missing}; keep the English for this string")
    return out
```

Do the same for brand and product names you never want translated. Always check that every placeholder survived, and fall back to English when one did not.

## Translate once, at build time

Translations of fixed strings do not change between requests. Translate your string files once (in CI or a script), save them as your app's locale files, and have a native speaker review the ones users see most. Calling the API on every page view wastes your daily quota and adds a second or two to each screen.

```js
// translate-strings.mjs: node translate-strings.mjs ta
import fs from "node:fs";

const BASE = "https://api.dharun.dev/v1";
const to = process.argv[2];
const en = JSON.parse(fs.readFileSync("locales/en.json", "utf8"));
const out = {};
for (const [key, text] of Object.entries(en)) {
  const r = await fetch(`${BASE}/translate?from=en&to=${to}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${process.env.INDIA_API_KEY}` },
    body: JSON.stringify({ text }),
  });
  if (!r.ok) throw new Error(`${key}: ${(await r.json()).error.message}`);
  out[key] = (await r.json()).data.translation;
}
fs.writeFileSync(`locales/${to}.json`, JSON.stringify(out, null, 2));
```

Requests with one key run one at a time, so a loop like this is the right shape.

## When a person should check

Machine translation is good for notifications, support replies and help pages. Have a person review legal text, prices and anything about money or health, slogans and wordplay, and short UI labels without context ("Save" can mean several things). Numbers, dates and amounts deserve a glance in every case.

Text read from a photo can be translated too: see [Read Tamil, Hindi and English text from photos](https://api.dharun.dev/guides/ocr-tamil-hindi).
