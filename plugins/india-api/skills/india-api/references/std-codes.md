# STD codes API

> The landline STD code of every town and area in India, and which area a landline number belongs to.

All 2,645 Short Distance Charging Areas of the Department of Telecommunications' National Numbering Plan, each with its STD (trunk) code, the long distance area and the telecom circle. Look a code or a landline number up, or find a town's code. Circles are the 2003 service areas, which are not always states.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## The area of an STD code or a landline number

`GET /v1/std-codes/{code}`

A code (44, 0422) must match whole; a landline number (0422 2345678, +91 44 2345 6789) is matched by its longest code. A whole number is kept out of the log.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `code` (path, required): An STD code or a landline number, with or without 0 or +91.

Example: https://api.dharun.dev/v1/std-codes/0422?utm_medium=skill

## Find the STD code of a town

`GET /v1/std-codes`

Areas whose names (or long distance area) contain the words, or every area of a circle; shortest codes first.

Parameters:

- `q` (query): 3 to 60 letters of a town's name.
- `circle` (query): A circle code such as TN, KT, MH, UPE, NE.

Example: https://api.dharun.dev/v1/std-codes?q=pollachi&utm_medium=skill
