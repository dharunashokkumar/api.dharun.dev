# Railway stations API

> Indian Railways station codes to stations, and back.

About 4,000 stations with their code, state, railway zone and elevation, from Wikipedia's list of railway stations in India. No official open list exists; this one is kept by volunteers, so some codes are out of date and many entries lack a state or zone.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Station for a code

`GET /v1/stations/{code}`

Parameters:

- `code` (path, required): Station code, such as MAS or NDLS.

Example: https://api.dharun.dev/v1/stations/MAS?utm_medium=skill

## Search stations

`GET /v1/stations`

By name (an exact code comes first, then names starting with the text), state or zone.

Parameters:

- `q` (query): Part of a station name, or a code.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `zone` (query): Zone code, such as SR. See /v1/stations/zones.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/stations?q=chennai&utm_medium=skill

## Railway zones

`GET /v1/stations/zones`

Example: https://api.dharun.dev/v1/stations/zones?utm_medium=skill
