# Indian date tools API

> Financial and assessment year, quarter, the Saka, Tamil and Hijri dates, and ages.

For a date: the financial year (April to March), its quarter and half, the assessment year, the ISO week, the Indian national (Saka) date, the Tamil solar date, the Vikram Samvat year and the tabular Hijri date. And the span between two dates in years, months and days, for ages and tenures.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## One date in Indian calendars

`GET /v1/dates/{date}`

Parameters:

- `date` (path, required): YYYY-MM-DD, or today.

Example: https://api.dharun.dev/v1/dates/2026-11-11?utm_medium=skill

## Age and the span between dates

`GET /v1/dates/diff`

Whole years, months and days from one date to another, the total days and weeks, and the next anniversary (a birthday, say).

Parameters:

- `from` (query, required): Start date, YYYY-MM-DD (a date of birth for an age).
- `to` (query): End date, YYYY-MM-DD. Default today in India.

Example: https://api.dharun.dev/v1/dates/diff?from=1995-08-15&to=2026-11-11&utm_medium=skill
