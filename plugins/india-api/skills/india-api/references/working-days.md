# Working days API

> Bank, market, settlement and government working days.

Is a date a working day, what is the next one, and what date is N working days away. Bank days follow the Reserve Bank's holidays for a state plus Sundays and the second and fourth Saturdays; market and settlement days follow NSE's trading and clearing holidays. Only years with a published holiday list are answered.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Is a date a working day

`GET /v1/working-days/{date}`

Whether the date is a working day and why not, the previous and next working days, and with add=N the date N working days later (negative for earlier). A T+1 settlement date is calendar=settlement&add=1.

Parameters:

- `date` (path, required): Date, YYYY-MM-DD.
- `calendar` (query): bank (default; needs state), market (NSE trading), settlement (NSE clearing) or central (central government offices). One of: bank, market, settlement, central.
- `state` (query): State code or name. Required for the bank calendar.
- `office` (query): Bank calendar only. One Reserve Bank office's list, such as Srinagar or Kochi, for a state with several offices (Maharashtra, Uttar Pradesh, Jammu and Kashmir, Kerala). Without it, a day off at any of the state's offices counts.
- `add` (query): Working days to add, -366 to 366.

Example: https://api.dharun.dev/v1/working-days/2026-01-16?calendar=bank&state=TN&add=3&utm_medium=skill

## Count working days between two dates

`GET /v1/working-days`

Counts working days from one date to another, both included, and lists each day off with its reason. At most 366 days.

Parameters:

- `from` (query, required): First date, YYYY-MM-DD.
- `to` (query, required): Last date, YYYY-MM-DD.
- `calendar` (query): bank (default; needs state), market (NSE trading), settlement (NSE clearing) or central (central government offices). One of: bank, market, settlement, central.
- `state` (query): State code or name. Required for the bank calendar.
- `office` (query): Bank calendar only. One Reserve Bank office's list, such as Srinagar or Kochi, for a state with several offices (Maharashtra, Uttar Pradesh, Jammu and Kashmir, Kerala). Without it, a day off at any of the state's offices counts.

Example: https://api.dharun.dev/v1/working-days?from=2026-01-01&to=2026-01-31&calendar=market&utm_medium=skill

## Is the stock market open

`GET /v1/market/status`

NSE equities session at a moment: pre_open, open, closing or closed, the reason it is closed, and when it next opens. Without at, the current moment.

Parameters:

- `at` (query): Moment with its offset, RFC 3339, for example 2026-10-13T10:30:00+05:30.

Example: https://api.dharun.dev/v1/market/status?at=2026-01-26T10:00:00%2B05:30&utm_medium=skill

## Guides

- [Bank holidays, working days and T+1 settlement dates](https://api.dharun.dev/guides/working-days.md): Find whether a date is a bank holiday in a state, the next working day, N working days ahead, and NSE settlement dates, in Python.
