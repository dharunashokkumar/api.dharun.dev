# Website icons API

> The icon of any website as a square PNG, for lists of links and bookmarks.

Give a domain and get the icon the site names for itself: its Apple touch icon, the icons in its web app manifest, its favicon. The best one is fetched once, drawn again here as a square PNG of the size asked and kept in memory, so the next request is served without asking the site. SVG icons are not used. A site without a usable icon, or one that cannot be reached, gets an initials tile marked with the X-Logo-Fallback: monogram header, so an <img> tag always shows something. For listed Indian companies, the company logos API has better artwork.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Icon for a website

`GET /v1/icon/{domain}`

The X-Icon-Source header names the file the icon was drawn from, the first time it is fetched. An icon already fetched counts as 1 request against the rate limit; one that has to be fetched counts as 20.

Parameters:

- `domain` (path, required): Domain name; a .png suffix is allowed.
- `size` (query): Width and height in pixels, 16 to 256. Default 64.

Example: https://api.dharun.dev/v1/icon/github.com?utm_medium=skill (returns the image itself)
