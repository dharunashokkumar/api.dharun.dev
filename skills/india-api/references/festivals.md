# Festivals API

> Hindu, Tamil, Sikh, Jain, Buddhist, Islamic and Christian festival dates for any year, computed.

Festival dates for 1950 to 2100, worked out from the panchang by each festival's own rule (Diwali by the amavasya at sunset, Ganesh Chaturthi by the chaturthi at midday, Karthigai Deepam by the evening nakshatra), with the rule given beside each date. Islamic dates come from the tabular calendar and are marked expected. Also as a calendar feed. For official holidays, see Holidays.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Festivals in a year

`GET /v1/festivals/{year}`

Parameters:

- `year` (path, required): 1950 to 2100.
- `kind` (query): Only one kind. One of: hindu, tamil, sikh, jain, buddhist, islamic, christian, national.
- `state` (query): Leave out regional festivals not kept in this state.
- `month` (query): 1 to 12.

Example: https://api.dharun.dev/v1/festivals/2026?month=11&utm_medium=skill

## Festivals as a calendar to subscribe to

`GET /v1/festivals/{year}.ics`

Parameters:

- `year` (path, required): The year, followed by .ics.
- `kind` (query): Only one kind. One of: hindu, tamil, sikh, jain, buddhist, islamic, christian, national.
- `state` (query): State code or name, for example TN or Tamil Nadu.

Example: https://api.dharun.dev/v1/festivals/2026.ics?state=TN&utm_medium=skill (returns an iCalendar file (text/calendar) to subscribe to)
