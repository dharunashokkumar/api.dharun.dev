# Colours API

> Colour conversions, WCAG contrast checks and the palette of a picture.

Hex, RGB, HSL and OKLCH for any colour; whether a text colour is readable on a background, with a nearby colour that is; the main colours of a picture.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## One colour every way

`GET /v1/color/{color}`

Parameters:

- `color` (path, required): Hex (with or without #, written %23), rgb(...), hsl(...) or a name such as saffron.

Example: https://api.dharun.dev/v1/color/ff9933?utm_medium=skill

## Contrast check

`GET /v1/color/contrast`

Parameters:

- `fg` (query, required): Text colour.
- `bg` (query, required): Background colour.

Example: https://api.dharun.dev/v1/color/contrast?fg=999999&bg=ffffff&utm_medium=skill

## Palette of a picture

`POST /v1/color/palette`

Its main colours with their shares and a readable text colour for each. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/photo.jpg

What you send is never written to the log, and the answer is not cached.

Parameters:

- `count` (query): Colours, 1 to 16. Default 6.

Example: `POST` https://api.dharun.dev/v1/color/palette?count=5&utm_medium=skill
