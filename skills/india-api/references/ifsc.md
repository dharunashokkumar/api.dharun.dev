# IFSC and banks API

> IFSC to bank and branch.

Branch details for every IFSC in the Reserve Bank's NEFT and RTGS lists: bank, branch, address, MICR and which payment systems the branch supports.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Branch for an IFSC

`GET /v1/ifsc/{code}`

Parameters:

- `code` (path, required): 11-character IFSC.

Example: https://api.dharun.dev/v1/ifsc/SBIN0000001?utm_medium=skill

## Search a bank's branches

`GET /v1/ifsc`

Lists branches of one bank, filtered by state, city or text in the branch name or address.

Parameters:

- `bank` (query, required): Four-letter bank code, for example SBIN.
- `q` (query): Text to find in the branch name, address or city.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `city` (query): Exact city name.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/ifsc?bank=SBIN&state=TN&q=namakkal&utm_medium=skill

## Bank for a four-letter code

`GET /v1/banks/{code}`

Parameters:

- `code` (path, required): Four-letter bank code.

Example: https://api.dharun.dev/v1/banks/HDFC?utm_medium=skill

## Search banks

`GET /v1/banks`

Finds banks by name or code, largest branch network first.

Parameters:

- `q` (query): Text to find in the bank name or code.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/banks?q=gramin&utm_medium=skill

## Branch from a MICR code

`GET /v1/micr/{code}`

The nine-digit code at the foot of a cheque, to the bank branch and its IFSC.

Parameters:

- `code` (path, required): Nine digits.

Example: https://api.dharun.dev/v1/micr/400240003?utm_medium=skill

## Branches with a SWIFT code

`GET /v1/swift/{code}`

Parameters:

- `code` (path, required): 8 or 11 characters, such as HDFCINBB.

Example: https://api.dharun.dev/v1/swift/HDFCINBB?utm_medium=skill

## Guides

- [Handle India API errors, rate limits and retries](https://api.dharun.dev/guides/errors-and-limits.md): Every error has the same JSON shape; rate limits are weighted per endpoint and told in RateLimit headers. A reusable client in Python and JavaScript with timeouts, retries and backoff.
- [IFSC lookup in Node.js with caching and clean errors](https://api.dharun.dev/guides/node-ifsc-lookup.md): Normalise what users type, check the format before calling, cache branches for a day, and tell apart a mistyped code, an unknown branch and a network failure, in Node.js with no dependencies.
- [Pincode autofill and IFSC lookup in a Flutter app](https://api.dharun.dev/guides/flutter-pincode-ifsc.md): Fill in district and state from a pincode, check an IFSC code and show a company logo in Flutter, with the http package and no API key.
- [Pincode autofill and IFSC lookup in an Android app (Kotlin)](https://api.dharun.dev/guides/android-kotlin-pincode-ifsc.md): Fill in district and state from a pincode and check an IFSC code in an Android app with Kotlin coroutines and HttpURLConnection, no library and no key.
- [Show the bank and branch for an IFSC code](https://api.dharun.dev/guides/ifsc-bank-branch.md): Turn an IFSC into a bank name, branch and address, and check that the branch takes NEFT, RTGS, IMPS or UPI before you send money there.
