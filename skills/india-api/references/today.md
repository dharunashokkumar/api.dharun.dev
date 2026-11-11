# Today in India API

> India's "now" in one call: date, financial year, working days, holidays, market and panchang.

For agents and widgets: the date and time in IST, the financial year, quarter and assessment year, whether today is a bank (per state), market and central government working day, today's holidays, whether the market is open, and with a pincode the sunrise and the day's panchang in brief.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Today in India

`GET /v1/today`

Without date it is about now, and cached for a minute. The panchang (with pincode) counts as 10 more units.

Parameters:

- `state` (query): State code or name, for example TN or Tamil Nadu.
- `pincode` (query): Adds sunrise, sunset and the panchang for that place.
- `date` (query): Another day, YYYY-MM-DD, instead of today.

Example: https://api.dharun.dev/v1/today?state=TN&pincode=600001&utm_medium=skill
