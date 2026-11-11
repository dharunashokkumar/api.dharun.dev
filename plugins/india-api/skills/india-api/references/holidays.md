# Holidays API

> Bank, government and stock market holidays.

Holidays for a year, filterable by kind and state. Each entry says who observes it.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Holidays in a year

`GET /v1/holidays/{year}`

Parameters:

- `year` (path, required): Four-digit year.
- `type` (query): Kind of holiday. clearing days are ones on which the market trades but nothing settles. One of: bank, central_gazetted, central_restricted, market, clearing.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `month` (query): Month number, 1 to 12.

Example: https://api.dharun.dev/v1/holidays/2026?type=market&utm_medium=skill

## Holidays as a calendar to subscribe to

`GET /v1/holidays/{year}.ics`

The same list as an iCalendar feed (RFC 5545): one all-day event per holiday. Add the address to Google Calendar (Other calendars, From URL), Outlook or a phone, and holidays added later appear by themselves. The same filters as the JSON list.

Parameters:

- `year` (path, required): Four-digit year, followed by .ics.
- `type` (query): Kind of holiday. One of: bank, central_gazetted, central_restricted, market, clearing.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `month` (query): Month number, 1 to 12.

Example: https://api.dharun.dev/v1/holidays/2026.ics?type=bank&state=TN&utm_medium=skill (returns an iCalendar file (text/calendar) to subscribe to)

## Guides

- [Bank holidays, working days and T+1 settlement dates](https://api.dharun.dev/guides/working-days.md): Find whether a date is a bank holiday in a state, the next working day, N working days ahead, and NSE settlement dates, in Python.
