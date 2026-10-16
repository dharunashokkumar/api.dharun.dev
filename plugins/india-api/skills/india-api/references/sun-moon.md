# Sun and moon API

> Sunrise, sunset, twilight, moonrise and the moon's phase for any date and place.

The astronomy of a day without the almanac: sunrise, sunset, solar noon and day length; civil, nautical and astronomical twilight; moonrise and moonset; the moon's phase, how much of it is lit and its age; and the next new, first quarter, full and last quarter moons. Computed with the same code as the panchang, and checked against NASA's JPL ephemeris.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Sun and moon for a date

`GET /v1/sun-moon/{date}`

For a pincode, a latitude and longitude, or (by default) New Delhi. A twilight time is null where the sun does not sink that far, and moonrise or moonset is null on the day each month the moon does not rise or set. The phase names a principal phase (new moon, first quarter, full moon, last quarter) for the whole date it falls on.

Parameters:

- `date` (path, required): YYYY-MM-DD, 1900 to 2100, or today.
- `pincode` (query): Indian pincode of the place.
- `lat` (query): Latitude, -66 to 66, if no pincode.
- `lng` (query): Longitude, if no pincode.
- `tz` (query): IANA time zone for the times. Default Asia/Kolkata.

Example: https://api.dharun.dev/v1/sun-moon/2026-10-14?pincode=600001&utm_medium=skill
