---
title: IFSC lookup in Node.js with caching and clean errors
description: Normalise what users type, check the format before calling, cache branches for a day, and tell apart a mistyped code, an unknown branch and a network failure, in Node.js with no dependencies.
apis: ifsc
published: 2026-11-12
---

When a user enters an IFSC for a payout, you want the bank and branch on screen before they confirm. This is a small module for a Node.js server (Node 18 or later, no packages) that does the lookup properly: it cleans the input, refuses impossible codes without a request, caches answers, and returns errors your UI can show.

## What a lookup returns

```sh
curl "https://api.dharun.dev/v1/ifsc/HDFC0000001"
```

```json
{
  "data": {
    "ifsc": "HDFC0000001",
    "bank": {"code": "HDFC", "name": "HDFC Bank"},
    "branch": "TULSIANI CHMBRS - NARIMAN PT",
    "city": "Greater Mumbai",
    "district": "Mumbai",
    "state": "Maharashtra",
    "micr": "400240003",
    "swift": "HDFCINBB",
    "imps": true, "neft": true, "rtgs": true, "upi": true
  }
}
```

(Shortened; the address and contact come too.) Lowercase works as well: `hdfc0000001` finds the same branch.

## The module

```js
// ifsc.js
const BASE = "https://api.dharun.dev/v1";
const DAY = 24 * 60 * 60 * 1000;
const cache = new Map(); // ifsc -> { at, branch }

export class IfscError extends Error {
  constructor(kind, message) {
    super(message);
    this.kind = kind; // "format", "unknown", "unavailable"
  }
}

// Users paste codes with spaces, dashes and the letter O for zero.
export function normaliseIfsc(raw) {
  let code = String(raw).toUpperCase().replace(/[\s-]/g, "");
  if (code.length === 11 && code[4] === "O") code = code.slice(0, 4) + "0" + code.slice(5);
  return code;
}

export async function lookupIfsc(raw) {
  const code = normaliseIfsc(raw);
  if (!/^[A-Z]{4}0[A-Z0-9]{6}$/.test(code)) {
    throw new IfscError("format", "An IFSC is 11 characters: four letters, then 0, then six letters or digits.");
  }
  const hit = cache.get(code);
  if (hit && Date.now() - hit.at < DAY) return hit.branch;

  let r;
  try {
    r = await fetch(`${BASE}/ifsc/${code}`, { signal: AbortSignal.timeout(8000) });
  } catch {
    throw new IfscError("unavailable", "Could not check the IFSC right now. You can continue; we will verify it later.");
  }
  if (r.status === 404) {
    throw new IfscError("unknown", `No branch has the IFSC ${code}. Check it on the cheque book or passbook.`);
  }
  if (!r.ok) {
    throw new IfscError("unavailable", "Could not check the IFSC right now. You can continue; we will verify it later.");
  }
  const branch = (await r.json()).data;
  cache.set(code, { at: Date.now(), branch });
  return branch;
}
```

## Using it in a route

```js
import express from "express";
import { lookupIfsc, IfscError } from "./ifsc.js";

const app = express();
app.get("/payout/ifsc/:code", async (req, res) => {
  try {
    const b = await lookupIfsc(req.params.code);
    res.json({ bank: b.bank.name, branch: b.branch, city: b.city, neft: b.neft, imps: b.imps });
  } catch (e) {
    if (!(e instanceof IfscError)) throw e;
    const status = { format: 400, unknown: 404, unavailable: 503 }[e.kind];
    res.status(status).json({ error: e.message });
  }
});
```

## Decisions worth copying

- **The format check is local.** A code that cannot exist never costs a request, and the user gets the message at once. The API answers the same rule with a 400 if you skip it.
- **404 is an answer, not a failure.** It means the Reserve Bank's list has no such branch: show it, do not retry.
- **Network trouble should not block a payout form.** Let the user continue and verify the code again before money moves.
- **Cache for a day.** Branch details change rarely; the list behind the API is refreshed weekly. One process-wide `Map` is enough for most servers; use Redis if you run several.
- **Check the payment system.** `neft`, `imps`, `rtgs` and `upi` say what the branch takes; a branch without NEFT should be refused before a NEFT payout.

Find a bank's code by name with `GET /v1/banks?q=...`, and see [Handle India API errors, rate limits and retries](https://api.dharun.dev/guides/errors-and-limits) for a general client.
