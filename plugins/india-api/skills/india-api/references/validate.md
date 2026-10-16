# Format checks API

> Check the format of a GSTIN, PAN or IFSC.

Structural checks only: character pattern, state code and, for GSTIN, the check character. Nothing is looked up with a government system, and these values are never written to the request log.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Check an identifier's format

`GET /v1/validate/{kind}/{value}`

What you send is never written to the log, and the answer is not cached.

Parameters:

- `kind` (path, required): Which identifier. One of: gstin, pan, ifsc.
- `value` (path, required): The identifier to check.

Example: https://api.dharun.dev/v1/validate/gstin/27AAPFU0939F1ZV?utm_medium=skill

## Guides

- [Check a GSTIN or PAN before you save it](https://api.dharun.dev/guides/gstin-pan-check.md): Catch typing mistakes in GSTINs and PANs with the check character, and read the state and PAN out of a GSTIN, in JavaScript.
- [Show the bank and branch for an IFSC code](https://api.dharun.dev/guides/ifsc-bank-branch.md): Turn an IFSC into a bank name, branch and address, and check that the branch takes NEFT, RTGS, IMPS or UPI before you send money there.
