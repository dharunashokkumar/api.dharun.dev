---
title: See exactly what a webhook sends, with a request bin
description: Point Razorpay, GitHub or your own code at a throwaway URL and read every request it receives as JSON, with no account and nothing to install.
apis: hooks
published: 2026-10-17
---

Before you write the handler for a payment or Git webhook, you want to see what it really sends: which headers, which signature, what the body looks like. A request bin is a URL that keeps every request sent to it.

## Make a bin

```sh
API=https://api.dharun.dev
curl -X POST "$API/v1/hooks"
```

```json
{
  "data": {
    "id": "k3x9q2m7pa4tbw",
    "url": "…/v1/hook/k3x9q2m7pa4tbw",
    "readUrl": "…/v1/hooks/k3x9q2m7pa4tbw",
    "token": "N0t-a-real-token",
    "expiresAt": "2026-10-18T09:00:00Z",
    "keeps": 100
  }
}
```

Give `url` (in full, starting with `https://api.dharun.dev`) to the service as its webhook address. Any method works, and so does any path after the id, so `…/v1/hook/k3x9q2m7pa4tbw/payments` is fine too.

## Read what arrived

```sh
curl "$API/v1/hooks/$ID" -H "Authorization: Bearer $TOKEN"
```

Each request comes back with its method, path, query, headers and body, newest first. Here is what the documentation's own example bin holds:

```sh
curl "https://api.dharun.dev/v1/hooks/example?token=example&utm_medium=guide"
```

To wait for new requests, poll with `since` set to the highest `seq` you have seen. A body that is not text comes back as base64, and `bodyEncoding` says which.

## Check a signature

Razorpay signs each webhook with `X-Razorpay-Signature`, an HMAC-SHA256 of the raw body. Take the body exactly as the bin returns it and check it the way your handler will:

```js
import crypto from "node:crypto";

const { data } = await (await fetch(readUrl, { headers: { Authorization: `Bearer ${token}` } })).json();
const latest = data.requests[0];
const expected = crypto.createHmac("sha256", process.env.RAZORPAY_WEBHOOK_SECRET).update(latest.body).digest("hex");
console.log(expected === latest.headers["X-Razorpay-Signature"][0] ? "signature ok" : "signature mismatch");
```

## What is kept

A bin keeps the last 100 requests for 24 hours, then it and everything in it is deleted; `POST /v1/hooks/{id}/delete` deletes it at once. The headers that carry the sender's address are dropped before anything is stored, nothing is logged, and only the token's holder can read the bin.
