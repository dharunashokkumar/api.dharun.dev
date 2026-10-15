---
title: Mask Aadhaar, PAN and phone numbers before an LLM sees them
description: Mask Indian personal identifiers in text before it reaches a language model or a log, with check-digit validation so ordinary numbers are left alone.
apis: text
published: 2026-10-14
updated: 2026-10-15
---

Support tickets, chat messages and documents often carry Aadhaar numbers, PANs, phone numbers and bank card numbers. Masking them before the text goes to a language model, a vector store or a log keeps them out of places they should not be.

The redaction endpoint finds Indian identifiers by their structure and, where they have one, their check digit (Verhoeff for Aadhaar, Luhn for cards, the GSTIN check character), so a 12-digit order number is not mistaken for an Aadhaar number.

## The request

Send the text as the POST body:

```sh
curl -X POST https://api.dharun.dev/v1/text/redact \
  -H "Content-Type: text/plain" \
  --data "Call 98765 43210, PAN ABCPK1234F"
```

```json
{
  "data": {
    "text": "Call [PHONE], PAN [PAN]",
    "findings": [
      { "type": "phone", "start": 5, "end": 16 },
      { "type": "pan", "start": 22, "end": 32 }
    ],
    "counts": { "pan": 1, "phone": 1 }
  },
  "meta": { "note": "Finds identifiers by their structure and check digits. Names and street addresses are not detected." }
}
```

Short text can also go in the query: [`/v1/text/redact?text=Call%2098765%2043210`](https://api.dharun.dev/v1/text/redact?text=Call%2098765%2043210). The original values are never returned, and nothing you send is logged or kept.

## Before calling a model, in Python

```python
import requests


def redact(text: str) -> str:
    response = requests.post(
        "https://api.dharun.dev/v1/text/redact",
        data=text.encode("utf-8"),
        headers={"Content-Type": "text/plain; charset=utf-8"},
        timeout=10,
    )
    body = response.json()
    if not response.ok:
        raise RuntimeError(body["error"]["message"])
    return body["data"]["text"]


ticket = "Refund to my card please. PAN ABCPK1234F, call me on 98765 43210."
safe = redact(ticket)
# "Refund to my card please. PAN [PAN], call me on [PHONE]."
# Now send `safe` to the model instead of `ticket`.
```

## Options

- `style=partial` keeps the last characters, as a bank statement does: `XXXXX XXX10`.
- `types=aadhaar,pan,phone` limits what is masked. The kinds are `email`, `upi`, `gstin`, `card`, `aadhaar`, `pan`, `phone`, `vehicle`, `voter_id`, `passport` and `ip`.
- With JSON, put the options beside the text: `{"text": "...", "style": "partial"}`.

## What it does not catch

Names and street addresses have no fixed shape, so they are not detected. Treat redaction as one layer: still send models only the text a task needs.

To see how many tokens the redacted text will cost, [`/v1/text/tokens`](https://api.dharun.dev/v1/text/tokens?model=gpt-4o&text=Call%20%5BPHONE%5D) counts them with OpenAI's tokenizers.
