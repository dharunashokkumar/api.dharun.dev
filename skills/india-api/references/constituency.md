# Constituencies API

> Which assembly and Lok Sabha constituency a place is in.

The assembly (MLA) and parliamentary (MP) constituency of a point or a pincode, with number, name, reserved category, district and state. From the DataMeet community's maps; boundaries are approximate near their edges, assembly seats in six states predate the latest delimitation, and sitting members are not given.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Constituencies of a place

`GET /v1/constituency`

Parameters:

- `pincode` (query): Indian pincode (the average position of its post offices).
- `lat` (query): Latitude, if no pincode.
- `lng` (query): Longitude, if no pincode.

Example: https://api.dharun.dev/v1/constituency?pincode=637001&utm_medium=skill
