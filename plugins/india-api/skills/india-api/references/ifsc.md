# IFSC and banks API

> IFSC to bank and branch.

Branch details for every IFSC in the Reserve Bank's NEFT and RTGS lists: bank, branch, address, MICR and which payment systems the branch supports.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

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

## Guides

- [Show the bank and branch for an IFSC code](https://api.dharun.dev/guides/ifsc-bank-branch.md): Turn an IFSC into a bank name, branch and address, and check that the branch takes NEFT, RTGS, IMPS or UPI before you send money there.
