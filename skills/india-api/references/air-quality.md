# Air quality API

> The air quality index at CPCB's monitoring stations across India, updated hourly.

The latest readings of the Central Pollution Control Board's continuous monitoring stations (PM2.5, PM10, NO2, SO2, CO, ozone, NH3), from data.gov.in, refreshed hourly, with the National Air Quality Index and its band (Good to Severe) worked out per CPCB's rule. Find stations by city, state, name, or nearest to a point.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Stations with their AQI and pollutants

`GET /v1/air-quality`

Every station that matches, each with its AQI, category, the pollutant that sets it, and each pollutant's sub-index. With near, the five nearest stations and their distance.

Parameters:

- `city` (query): As CPCB names it: Delhi, Mumbai, Bengaluru, Chennai ...
- `state` (query): Such as Tamil Nadu.
- `station` (query): Part of a station's name.
- `near` (query): latitude,longitude: the five nearest stations.

Example: https://api.dharun.dev/v1/air-quality?city=Chennai&utm_medium=skill
