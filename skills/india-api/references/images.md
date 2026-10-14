# Generated images API

> Social preview images and code screenshots from a URL.

Pictures made from text, with no browser involved: link-preview (Open Graph) cards and syntax-coloured code images. Latin, Devanagari, Bengali, Gurmukhi, Gujarati, Odia, Tamil, Telugu, Kannada and Malayalam are shaped properly (conjuncts, vowel signs) and can be mixed in one line; other scripts come out as empty boxes. PNG by default; format=svg gives the same picture as an SVG whose text is drawn as outlines, so it needs no fonts where it is shown. Text and colours only: no images are fetched from anywhere. Each request counts as 10 against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

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

Example: https://api.dharun.dev/v1/og-image?title=Free%20APIs%20for%20Indian%20developers&subtitle=Pincodes%2C%20IFSC%2C%20holidays%20and%20more&site=api.dharun.dev (returns the image itself)

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

Example: https://api.dharun.dev/v1/code-image?language=go&title=main.go&code=package+main%0A%0Aimport+%22fmt%22%0A%0Afunc+main%28%29+%7B%0A%09fmt.Println%28%22%E0%AE%B5%E0%AE%A3%E0%AE%95%E0%AF%8D%E0%AE%95%E0%AE%AE%E0%AF%8D%22%29%0A%7D (returns the image itself)
