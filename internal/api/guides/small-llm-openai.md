---
title: Pull fields out of customer messages with a free small LLM
description: Turn free-text enquiries into JSON that always matches your schema, classify them, and use the OpenAI SDK against India API's small hosted model by changing the base URL.
apis: llm
published: 2026-11-12
---

Leads arrive as free text: "Hi, I am Ravi Kumar from Coimbatore, call me on 98765 43210 about the 2 BHK flat." India API runs a small open model (Qwen 2.5 1.5B Instruct) on its own server for exactly this kind of narrow job. Its answers to `extract` and `classify` are forced to match the shape you ask for, so your code never has to parse a chatty reply.

It is English only, it can be wrong, and it needs a [free API key](https://api.dharun.dev/signup): each key has 100,000 tokens a day. Nothing you send is logged or kept.

## Extract fields as JSON

POST the text and the fields you want. Each field comes back as a string, empty when the text does not say:

```python
import os
import requests

BASE = "https://api.dharun.dev/v1"
KEY = os.environ["INDIA_API_KEY"]

def extract(text, fields):
    r = requests.post(f"{BASE}/llm/extract",
                      json={"text": text, "fields": fields},
                      headers={"Authorization": f"Bearer {KEY}"}, timeout=60)
    if r.status_code == 429:
        raise RuntimeError("out of tokens or rate limited: " + r.json()["error"]["message"])
    r.raise_for_status()
    return r.json()["data"]["data"]

lead = extract("Hi, I am Ravi Kumar from Coimbatore, call me on 98765 43210 about the 2 BHK flat.",
               ["name", "city", "phone", "requirement"])
print(lead)
```

What the model answered for that message:

```json
{"name": "Ravi Kumar", "city": "Coimbatore", "phone": "98765 43210", "requirement": "2 BHK flat"}
```

For numbers, lists or nested objects, send a JSON Schema instead of `fields`:

```json
{
  "text": "Need 3 cartons of A4 paper and 2 toner cartridges by Friday.",
  "schema": {
    "type": "object",
    "properties": {
      "items": {"type": "array", "items": {"type": "object",
        "properties": {"name": {"type": "string"}, "quantity": {"type": "integer"}},
        "required": ["name", "quantity"]}},
      "deadline": {"type": "string"}
    },
    "required": ["items", "deadline"]
  }
}
```

Decoding is constrained to the schema, so the JSON always parses and has the fields you asked for. Whether the values are right is another matter: check a phone number with `/v1/phone`, a pincode with `/v1/pincode`, before you trust it.

## Classify into your own labels

```js
const BASE = "https://api.dharun.dev/v1";

async function classify(text, labels) {
  const r = await fetch(`${BASE}/llm/classify`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${process.env.INDIA_API_KEY}` },
    body: JSON.stringify({ text, labels }),
  });
  if (!r.ok) throw new Error((await r.json()).error.message);
  return (await r.json()).data.label; // always one of labels
}

await classify("The delivery came two days late and the box was torn.", ["positive", "negative", "neutral"]);
// "negative"
```

Add `"multi": true` for every label that fits, and `instructions` to say what a label means ("billing: anything about payments or refunds").

## Use the OpenAI SDK

`/v1/chat/completions` speaks OpenAI's format, streaming included, so existing code works by changing the base URL. The model list is public:

```sh
curl "https://api.dharun.dev/v1/models"
```

```python
from openai import OpenAI

client = OpenAI(base_url="https://api.dharun.dev/v1", api_key=os.environ["INDIA_API_KEY"])
reply = client.chat.completions.create(
    model="qwen2.5-1.5b-instruct",
    messages=[{"role": "user", "content": "Write a two-line welcome message for a food delivery app."}],
    max_tokens=80,
)
print(reply.choices[0].message.content)
```

The window is 4,096 tokens (prompt and answer), answers are capped at 1,024 tokens, and there are no tools, images or audio. `response_format` with a JSON schema works as in the task endpoints.

## When to use it, and when not

Good fits: pulling fields out of short messages, routing tickets, tidying the grammar of a reply, a one-line summary. Poor fits: facts the text does not contain (a small model invents them), anything in Hindi or Tamil (measured, and not good enough), long documents (split them first; `/v1/text/chunk` does that), and decisions about people that nobody checks.

For errors, limits and retries, see [Handle India API errors, rate limits and retries](https://api.dharun.dev/guides/errors-and-limits).
