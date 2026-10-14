# Satellite passes API

> When the ISS and amateur radio satellites pass over you, and where they are now.

Pass predictions for the space stations (ISS, Tiangong) and about ninety amateur radio satellites, from any Indian pincode or any latitude and longitude: when each rises, how high it gets and where, when it sets, and whether it can be seen with the eye. Computed with SGP4 from CelesTrak's orbit elements, refreshed a few times a day; the elements themselves are not published here. Pass predictions count as 10 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Passes over a place

`GET /v1/sat-pass`

Passes from now for the next days, soonest first. Directions are compass points and azimuths in degrees from true north; elevations are degrees above the horizon. A satellite can be named by catalogue number, by name, by the designation in its name's brackets (SO-50, AO-7), or as ISS, CSS or QO-100.

Parameters:

- `sat` (query): Catalogue number or name. Default ISS (25544).
- `pincode` (query): Indian pincode of the observer.
- `lat` (query): Latitude, if no pincode. Default New Delhi.
- `lng` (query): Longitude, if no pincode.
- `days` (query): How far ahead, 1 to 10. Default 3.
- `minElevation` (query): Lowest peak elevation listed, in degrees. Default 10; 0 lists every pass.
- `visible` (query): true lists only passes that can be seen with the eye. One of: true, false.
- `tz` (query): IANA time zone for the times. Default Asia/Kolkata.

Example: https://api.dharun.dev/v1/sat-pass?sat=ISS&pincode=600001

## Where a satellite is now

`GET /v1/satellites/{id}`

The point on Earth below the satellite at the moment of the request, its altitude and speed, whether it is in sunlight, and how old its orbit elements are.

Parameters:

- `id` (path, required): Catalogue number or name.

Example: https://api.dharun.dev/v1/satellites/ISS

## Tracked satellites

`GET /v1/satellites`

Every tracked satellite with its catalogue number, groups, orbital period and inclination.

Parameters:

- `q` (query): Part of a name, or a catalogue number.
- `group` (query): Limit to one group. One of: stations, amateur.

Example: https://api.dharun.dev/v1/satellites?group=stations
