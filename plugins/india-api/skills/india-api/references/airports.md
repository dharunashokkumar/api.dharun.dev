# Airports API

> Airport codes to airports, and airports by city.

Every airport in the world with an IATA code, plus every Indian airfield and heliport, from OurAirports. Indian airports carry their state code.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Airport for a code

`GET /v1/airports/{code}`

Parameters:

- `code` (path, required): IATA code (MAA) or ICAO code (VOMM).

Example: https://api.dharun.dev/v1/airports/MAA

## Search airports

`GET /v1/airports`

By name, city or code; airports with scheduled flights and larger airports first.

Parameters:

- `q` (query): Name, city or code, 2 or more characters.
- `country` (query): Two-letter ISO country code, such as IN.
- `state` (query): State code or name, for example TN or Tamil Nadu.
- `type` (query): Kind of airfield. One of: large_airport, medium_airport, small_airport, heliport, seaplane_base.
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/airports?q=chennai
