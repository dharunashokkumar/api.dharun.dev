# Holidays API

> Bank, government and stock market holidays.

Holidays for a year, filterable by kind and state. Each entry says who observes it.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Holidays in a year

`GET /v1/holidays/{year}`

Parameters:

- `year` (path, required): Four-digit year.
- `type` (query): Kind of holiday. clearing days are ones on which the market trades but nothing settles. One of: bank, central_gazetted, central_restricted, market, clearing.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `month` (query): Month number, 1 to 12.

Example: https://api.dharun.dev/v1/holidays/2026?type=market&utm_medium=skill

## Guides

- [Bank holidays, working days and T+1 settlement dates](https://api.dharun.dev/guides/working-days.md): Find whether a date is a bank holiday in a state, the next working day, N working days ahead, and NSE settlement dates, in Python.
