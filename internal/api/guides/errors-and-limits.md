---
title: Handle India API errors, rate limits and retries
description: Every error has the same JSON shape; rate limits are weighted per endpoint and told in RateLimit headers. A reusable client in Python and JavaScript with timeouts, retries and backoff.
apis: pincode, ifsc
published: 2026-11-12
---

Every endpoint answers errors the same way, so one small wrapper handles them all. This guide shows what can go wrong, what each status means, and a client that retries only what is worth retrying.

## One error shape

A successful answer is `{ "data": ..., "meta": ... }`. An error is always:

```json
{"error": {"code": "not_found", "message": "No post office found for pincode 999999."}}
```

`code` is for your program, `message` for people (it says what to fix). That one is the answer to `GET /v1/pincode/999999`, a pincode that does not exist.

| Status | Codes | What to do |
|---|---|---|
| 400 | `bad_request` | Your input is wrong; show the message, do not retry |
| 401 | `key_required`, `invalid_key` | Send a key (`Authorization: Bearer <key>` or `X-API-Key`); do not retry |
| 404 | `not_found` | The thing does not exist (an unknown pincode or IFSC); do not retry |
| 405 | `method_not_allowed` | Wrong method; fix the code |
| 413 | `too_large` | The body is over the endpoint's limit; send less |
| 429 | `rate_limited`, `quota_exceeded`, `token_quota_exceeded` | Wait for `Retry-After` seconds, then retry |
| 503 | `busy`, `dataset_unavailable`, `not_available` | Retry after `Retry-After`; a model or dataset is loading or the server is busy |
| 500 | `internal_error` | Retry once later; report it if it stays |

## Weighted rate limits

Without a key each address has 600 units a minute; a [free key](https://api.dharun.dev/signup) has 3,000. A lookup costs 1 unit, heavier work more (a file upload 15, a model 30). Every charged answer says where you stand:

```text
RateLimit-Limit: 600
RateLimit-Remaining: 559
RateLimit-Reset: 50
```

`RateLimit-Reset` is the seconds until the minute's allowance refills. Model endpoints also send `X-Quota-Remaining` (requests left today) and the LLM endpoints `X-Token-Remaining`.

## Cache what does not change

A pincode or an IFSC rarely changes, and answers carry `Cache-Control` and an `ETag`. Keep answers for a day in your own cache, and send `If-None-Match` with the ETag to get a free `304 Not Modified` when nothing changed. A logo answered from cache does not count against the limit at all.

## A client in Python

```python
import random
import time
import requests

BASE = "https://api.dharun.dev/v1"
RETRY = {429, 500, 502, 503, 504}

class IndiaAPIError(Exception):
    def __init__(self, status, code, message):
        super().__init__(f"{status} {code}: {message}")
        self.status, self.code = status, code

def get(path, params=None, key=None, tries=4):
    headers = {"X-API-Key": key} if key else {}
    for attempt in range(tries):
        try:
            r = requests.get(f"{BASE}{path}", params=params, headers=headers, timeout=10)
        except requests.RequestException:
            if attempt == tries - 1:
                raise
            time.sleep(2 ** attempt + random.random())
            continue
        if r.ok:
            return r.json()["data"]
        if r.status_code in RETRY and attempt < tries - 1:
            wait = float(r.headers.get("Retry-After", 2 ** attempt))
            time.sleep(min(wait, 60) + random.random())
            continue
        error = r.json().get("error", {}) if r.headers.get("Content-Type", "").startswith("application/json") else {}
        raise IndiaAPIError(r.status_code, error.get("code", "http_error"), error.get("message", r.text[:200]))

try:
    print(get("/pincode/637001")[0]["district"])
except IndiaAPIError as e:
    if e.code == "not_found":
        print("no such pincode")
    else:
        raise
```

## A client in JavaScript

```js
const BASE = "https://api.dharun.dev/v1";
const RETRY = new Set([429, 500, 502, 503, 504]);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

export async function get(path, { key, tries = 4, signal } = {}) {
  for (let attempt = 0; attempt < tries; attempt++) {
    let r;
    try {
      r = await fetch(BASE + path, {
        headers: key ? { "X-API-Key": key } : {},
        signal: signal ?? AbortSignal.timeout(10_000),
      });
    } catch (e) {
      if (e.name === "AbortError" && signal?.aborted) throw e; // the caller gave up
      if (attempt === tries - 1) throw e;
      await sleep(1000 * 2 ** attempt + Math.random() * 500);
      continue;
    }
    if (r.ok) return (await r.json()).data;
    if (RETRY.has(r.status) && attempt < tries - 1) {
      const wait = Number(r.headers.get("Retry-After")) || 2 ** attempt;
      await sleep(Math.min(wait, 60) * 1000 + Math.random() * 500);
      continue;
    }
    const body = await r.json().catch(() => ({}));
    const err = new Error(body.error?.message ?? r.statusText);
    err.status = r.status;
    err.code = body.error?.code;
    throw err;
  }
}

// const branch = await get("/ifsc/SBIN0000001");
```

## Check an IFSC with it

```sh
curl "https://api.dharun.dev/v1/ifsc/SBIN0000001"
```

A 404 here means the code is not in the Reserve Bank's list: tell the user to check it, do not retry. A 429 or 503 is the only kind worth a second try.

## In a browser

CORS is open, and the `RateLimit-*`, `Retry-After`, `X-Quota-*` and `X-Token-*` headers are exposed to scripts, so the same code works in a web page. Keep keys out of public pages: the endpoints most pages call need no key.
