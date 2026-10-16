# HF propagation API

> Space weather and a rough rating of shortwave radio bands, day and night.

The numbers amateur radio operators check before getting on the air, from NOAA's Space Weather Prediction Center and refreshed every hour: 10.7 cm solar flux, sunspot number, the planetary K and A indices, the X-ray class and latest flare, solar wind speed and NOAA's radio blackout, radiation and geomagnetic storm scales. From those, each band group gets a good, fair or poor rating for day and for night. The ratings are a rule of thumb, and the rules are returned with them.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Space weather and band conditions

`GET /v1/propagation`

Rules behind the band ratings: 80m-40m: fair by day (D-layer absorption), good at night. 30m-20m: good by day; at night good from a solar flux of 120, fair from 80, else poor. 17m-15m: by day good from a solar flux of 110, fair from 80, else poor; at night fair from 150, else poor. 12m-10m: by day good from a solar flux of 150, fair from 110, else poor; at night fair from 200, else poor. A K-index of 4 lowers every night rating one step; 5 or more lowers every rating one step, and 7 or more two. An M-class X-ray flux lowers the day rating of 80m-40m and 30m-20m one step; X-class makes every day rating poor. Readings NOAA did not publish at the last fetch are null.

Example: https://api.dharun.dev/v1/propagation?utm_medium=skill
