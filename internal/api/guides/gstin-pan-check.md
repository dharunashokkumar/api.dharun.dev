---
title: Check a GSTIN or PAN before you save it
description: Catch typing mistakes in GSTINs and PANs with the check character, and read the state and PAN out of a GSTIN, in JavaScript.
apis: validate, states
published: 2026-10-14
---

A GSTIN has fifteen characters: a two-digit state code, the holder's PAN, an entity number, the letter Z and a check character computed from the other fourteen. One wrong character makes the check fail, so a format check catches nearly every typing mistake without asking any government system.

## The request

```sh
curl https://api.dharun.dev/v1/validate/gstin/27AAPFU0939F1ZV
```

```json
{
  "data": {
    "kind": "gstin",
    "normalized": "27AAPFU0939F1ZV",
    "valid": true,
    "parts": {
      "entityNumber": "1",
      "holderType": "Firm or Limited Liability Partnership",
      "pan": "AAPFU0939F",
      "state": "Maharashtra",
      "stateCode": "27"
    }
  },
  "meta": { "note": "This checks the format only. It does not confirm that the identifier was ever issued." }
}
```

A wrong character still answers `200`, with `valid: false` and the reason:

```sh
curl https://api.dharun.dev/v1/validate/gstin/27AAPFU0939F1ZX
```

```json
{ "data": { "kind": "gstin", "normalized": "27AAPFU0939F1ZX", "valid": false,
  "reason": "The check character does not match; one of the characters is wrong." } }
```

## In a form

```js
async function checkGstin(value) {
  const response = await fetch(
    `https://api.dharun.dev/v1/validate/gstin/${encodeURIComponent(value.trim())}`
  );
  const { data } = await response.json();
  if (!data.valid) return { ok: false, message: data.reason };
  return {
    ok: true,
    gstin: data.normalized, // upper case, spaces removed: store this one
    state: data.parts.state,
    pan: data.parts.pan,
  };
}

const result = await checkGstin("27aapfu0939f1zv");
console.log(result.ok ? `${result.gstin} is from ${result.state}` : result.message);
```

Compare `state` with the billing address's state: a GSTIN registered in another state is a common source of wrong tax on an invoice.

## PAN

The fourth letter of a PAN says what kind of holder it belongs to: P for a person, C for a company, F for a firm and so on.

```sh
curl https://api.dharun.dev/v1/validate/pan/ABCPK1234F
```

```json
{ "data": { "kind": "pan", "normalized": "ABCPK1234F", "valid": true, "parts": { "holderType": "Individual" } } }
```

## What it cannot tell you

These checks are structural. A well-formed GSTIN may never have been issued, or may have been cancelled; only the GST portal knows that. Values sent to the format checks are never written to the request log.
