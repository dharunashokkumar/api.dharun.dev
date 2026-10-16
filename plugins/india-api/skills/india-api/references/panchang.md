# Panchang API

> Hindu almanac for any date and place: tithi, nakshatra, Rahu kalam and more.

Computed, not looked up, from the positions of the Sun and the Moon (within seconds of NASA JPL Horizons), with the Lahiri ayanamsa. Gives the five elements with the moments they change, sunrise, sunset, moonrise and moonset, Rahu kalam, Yamagandam, Gulika kalam and Abhijit muhurta, the Tamil month and day, the lunar month both ways it is reckoned, and the Shaka and Vikram years. Names come in English transliteration and, where the Tamil calendar has its own, in Tamil. Festival dates are not given. Counts as 10 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Panchang for a date

`GET /v1/panchang/{date}`

For a pincode, a latitude and longitude, or (by default) New Delhi. Times are in the time zone asked for.

Parameters:

- `date` (path, required): YYYY-MM-DD, 1900 to 2100, or today.
- `pincode` (query): Indian pincode of the place.
- `lat` (query): Latitude, if no pincode.
- `lng` (query): Longitude, if no pincode.
- `tz` (query): IANA time zone for the times. Default Asia/Kolkata.

Example: https://api.dharun.dev/v1/panchang/2026-01-14?pincode=600001&utm_medium=skill
