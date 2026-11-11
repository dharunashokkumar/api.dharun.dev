# Helplines API

> National emergency and helpline numbers.

112 for any emergency, and the national helplines for women, children, senior citizens, cyber fraud, mental health, railways, highways, consumers and more, with who runs each.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## National helplines

`GET /v1/helplines`

Parameters:

- `q` (query): Search the names and purposes.

Example: https://api.dharun.dev/v1/helplines?q=cyber&utm_medium=skill
