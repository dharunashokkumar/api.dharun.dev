# Thirukkural API

> All 1,330 kurals in Tamil with English translations.

Thiruvalluvar's Thirukkural with a transliteration, G.U. Pope's verse translation (1886) and the prose translation by W.H. Drew and John Lazarus (1840, 1885), organised into 3 paals, 13 iyals and 133 chapters. Only text that is free to share is served; modern commentaries are added once their release into the public domain is confirmed.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## A kural by number

`GET /v1/thirukkural/{number}`

Parameters:

- `number` (path, required): 1 to 1330.

Example: https://api.dharun.dev/v1/thirukkural/1?utm_medium=skill

## Search kurals

`GET /v1/thirukkural`

Finds kurals containing the text, in Tamil, the transliteration or either translation; or lists a chapter or a paal.

Parameters:

- `q` (query): Text to find, 2 or more characters, Tamil or English.
- `chapter` (query): Chapter (அதிகாரம்), 1 to 133.
- `paal` (query): 1 virtue (அறத்துப்பால்), 2 wealth (பொருட்பால்), 3 love (காமத்துப்பால்).
- `limit` (query): Results per page, 1 to 100.
- `offset` (query): Results to skip.

Example: https://api.dharun.dev/v1/thirukkural?q=rain&utm_medium=skill

## Kural of the day

`GET /v1/thirukkural/today`

The same kural for everyone on a day in India, a different one the next day.

Parameters:

- `date` (query): A date, YYYY-MM-DD. Default today in India.

Example: https://api.dharun.dev/v1/thirukkural/today?date=2026-10-13&utm_medium=skill

## A random kural

`GET /v1/thirukkural/random`

Example: https://api.dharun.dev/v1/thirukkural/random?utm_medium=skill

## All 133 chapters

`GET /v1/thirukkural/chapters`

Example: https://api.dharun.dev/v1/thirukkural/chapters?utm_medium=skill

## One chapter

`GET /v1/thirukkural/chapters/{number}`

Parameters:

- `number` (path, required): 1 to 133.

Example: https://api.dharun.dev/v1/thirukkural/chapters/2?utm_medium=skill
