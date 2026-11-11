# Generated images API

> Share images, code screenshots, avatars, placeholders and charts from a URL.

Pictures made from text, with no browser involved: link-preview (Open Graph) cards and syntax-coloured code images. Latin, Devanagari, Bengali, Gurmukhi, Gujarati, Odia, Tamil, Telugu, Kannada and Malayalam are shaped properly (conjuncts, vowel signs) and can be mixed in one line; other scripts come out as empty boxes. PNG by default; format=svg gives the same picture as an SVG whose text is drawn as outlines, so it needs no fonts where it is shown. Text and colours only: no images are fetched from anywhere. Each request counts as 10 against the rate limit.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Social preview image

`GET /v1/og-image`

A 1200 by 630 card for og:image and twitter:image: a title (set as large as fits in three lines, cut with an ellipsis beyond that), an optional subtitle of up to two lines, and a site name and tag along the bottom. The same URL always gives the same picture, so it can be used directly in a meta tag.

Parameters:

- `title` (query, required): Up to 200 characters, in any script.
- `subtitle` (query): Up to 300 characters; two lines are shown.
- `site` (query): Site name, bottom left.
- `tag` (query): Short label, bottom right.
- `theme` (query): Default dark. One of: dark, light.
- `bg` (query): Background colour, hex.
- `fg` (query): Title colour, hex.
- `accent` (query): Accent colour, hex.
- `format` (query): Default png. One of: png, svg.

Example: https://api.dharun.dev/v1/og-image?title=Free%20APIs%20for%20Indian%20developers&subtitle=Pincodes%2C%20IFSC%2C%20holidays%20and%20more&site=api.dharun.dev&utm_medium=skill (returns the image itself)

## Code as an image

`GET /v1/code-image`, also `POST`

A code snippet as a window with syntax colouring. Up to 150 lines of 140 characters. PNGs are drawn at twice the pixel density for sharp text (very large ones at single). The language is guessed from the title's file extension or the code when not given, and reported in the X-Code-Language header.

POST body: The code. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `code` (query): The code (or POST it).
- `language` (query): Language name or file extension, such as go, python, js, sql.
- `title` (query): File name shown in the title bar.
- `theme` (query): Default dark. One of: dark, light.
- `lineNumbers` (query): Show line numbers. One of: true.
- `format` (query): Default png. One of: png, svg.

Example: https://api.dharun.dev/v1/code-image?language=go&title=main.go&code=package+main%0A%0Aimport+%22fmt%22%0A%0Afunc+main%28%29+%7B%0A%09fmt.Println%28%22%E0%AE%B5%E0%AE%A3%E0%AE%95%E0%AF%8D%E0%AE%95%E0%AE%AE%E0%AF%8D%22%29%0A%7D&utm_medium=skill (returns the image itself)

## Avatar with initials

`GET /v1/avatar/{name}`

A round (or square) tile with a person's initials, its colour fixed by the name, for user lists and comments. Initials in Indian scripts keep their vowel signs and conjuncts. SVG by default; a .png suffix or format=png gives PNG.

Parameters:

- `name` (path, required): The name, in any script. The first letters of its first and last words are drawn.
- `size` (query): Width and height in pixels, 16 to 1024. Default 128.
- `shape` (query): Default circle. One of: circle, square, rounded.
- `bg` (query): Background colour as hex, such as 2563eb. Default: picked from the name.
- `fg` (query): Letter colour as hex. Default white.
- `initials` (query): Draw these letters instead (up to 4).
- `format` (query): svg (default) or png. One of: svg, png.

Example: https://api.dharun.dev/v1/avatar/Priya%20Raman?utm_medium=skill (returns the image itself)

## Placeholder image

`GET /v1/placeholder/{size}`

A plain box with its size (or your text) in the middle, for mock-ups and layouts.

Parameters:

- `size` (path, required): WIDTHxHEIGHT, or one number for a square; each 8 to 4000. A .png suffix gives PNG.
- `text` (query): Text to show instead of the size, up to 80 characters.
- `bg` (query): Background colour as hex. Default dde3ea.
- `fg` (query): Text colour as hex. Default 556070.
- `format` (query): svg (default) or png. One of: svg, png.

Example: https://api.dharun.dev/v1/placeholder/600x400?utm_medium=skill (returns the image itself)

## Chart image from numbers

`GET /v1/chart`, also `POST`

A bar, line, pie or sparkline chart as SVG or PNG, for emails, chat bots, READMEs and anywhere JavaScript charts cannot run. Axis values are written the Indian way (K, L, Cr). In the query: values=3,5,2 (repeat values for more series, with names to name them) and labels=Jan,Feb,Mar. Counts as 10 against the rate limit.

POST body: A chart as JSON: {"type": "bar", "title": "...", "labels": [...], "series": [{"name": "...", "values": [...], "color": "2563eb"}], "width": 800, "height": 450}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

Parameters:

- `type` (query): Default bar. One of: bar, line, pie, sparkline.
- `values` (query): Numbers separated by commas. Repeat the parameter for more series.
- `labels` (query): Labels for the values, separated by commas.
- `names` (query): A name for each series: repeated like values, or separated by commas.
- `title` (query): Title above the chart.
- `width` (query): Pixels, 40 to 3000. Default 800 (160 for a sparkline).
- `height` (query): Pixels, 20 to 3000. Default 450 (40 for a sparkline).
- `theme` (query): Default light. One of: light, dark.
- `format` (query): svg (default) or png. One of: svg, png.

Example: https://api.dharun.dev/v1/chart?type=bar&title=Sales%20by%20quarter&labels=Q1,Q2,Q3,Q4&values=12,19,9,15&utm_medium=skill (returns the image itself)
