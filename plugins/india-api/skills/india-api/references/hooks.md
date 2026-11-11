# Request bins API

> A private address that records what is sent to it, for testing webhooks.

Make a bin, point a webhook at its address, read back every request (method, headers, query and body). The caller's address is never kept. The bin demo (secret demo) is public, for trying. Each caller may hold 5 bins and databases at a time; each lasts a day, in memory or a temporary file, and is gone after a restart.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Make a request bin

`POST /v1/hooks`

Answers with the bin's address, the address to read it from and its secret (shown once). Needs a free API key once keys are required (see /keys).

Send: Nothing, or {} .

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `status` (query): The HTTP status the bin answers with. Default 200.

Example: `POST` https://api.dharun.dev/v1/hooks?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Send to a bin

`GET /v1/hook/{id}`, also `POST`

GET or POST anything: it is recorded.

POST body: Anything, up to 64 KB kept. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The bin.

Example: https://api.dharun.dev/v1/hook/demo?order=42&utm_medium=skill

## Read a bin

`GET /v1/hooks/{id}`

Every request it caught, oldest first. Send the secret as an X-Secret header (or secret=).

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The bin.
- `secret` (query): Its secret; better as X-Secret.

Example: https://api.dharun.dev/v1/hooks/demo?secret=demo&utm_medium=skill

## Delete a bin

`POST /v1/hooks/{id}/delete`

Deletes the bin and everything it caught. Send the secret as an X-Secret header.

Send: Nothing, or {} .

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/hooks/demo/delete?secret=demo&utm_medium=skill

Example body (invoice.json):

```json
{}
```
