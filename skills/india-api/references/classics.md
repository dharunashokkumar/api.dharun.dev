# Indian classics API

> Aathichoodi, the Bhagavad Gita in Sanskrit and Kabir's couplets, a line or verse at a time, with transliteration.

Public domain classics beside the Thirukkural: Avvaiyar's Aathichoodi (109 Tamil maxims), the Bhagavad Gita (700 Sanskrit verses) and 284 of Kabir's sakhis (Hindi couplets) from the Kabir Granthavali, as transcribed on Wikisource. Each line comes with a Roman transliteration (ISO 15919). No translations: those still under copyright are not used.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## The texts

`GET /v1/classics`

Example: https://api.dharun.dev/v1/classics?utm_medium=skill

## A text and its parts

`GET /v1/classics/{text}`

Parameters:

- `text` (path, required): Which text. One of: aathichoodi, gita, kabir.

Example: https://api.dharun.dev/v1/classics/gita?utm_medium=skill

## A verse, a whole part, or a random one

`GET /v1/classics/{text}/{ref}`

ref is part.number (gita 2.47, kabir 1.3), a part alone for all of it (gita 2), random, or for Aathichoodi a line number (1 to 109).

Parameters:

- `text` (path, required): Which text. One of: aathichoodi, gita, kabir.
- `ref` (path, required): 2.47, 2, random, or 7 for Aathichoodi.

Example: https://api.dharun.dev/v1/classics/gita/2.47?utm_medium=skill
