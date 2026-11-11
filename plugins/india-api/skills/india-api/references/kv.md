# Agent notes API

> A small private key-value store, for agents and scripts that must remember between runs.

Make a store, then write and read notes by name over HTTP: plain text or JSON, up to 16 KB a note and 1 MB a store, kept 30 days after the last write (they survive restarts). Private to the secret, sent as X-Secret. The store demo (secret demo) is read-only. Needs a free API key once keys are required (see /keys).

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Make a store

`POST /v1/kv`

Send: Nothing, or {} .

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/kv?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## List notes

`GET /v1/kv/{id}`

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The store.
- `prefix` (query): Only names starting with this.

Example: https://api.dharun.dev/v1/kv/demo?secret=demo&utm_medium=skill

## Read or write a note

`GET /v1/kv/{id}/{key}`, also `POST`

GET reads the note; POST writes the body as the note.

POST body: The note: text or JSON, up to 16 KB. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The store.
- `key` (path, required): The note's name, up to 200 characters.

Example: https://api.dharun.dev/v1/kv/demo/greeting?secret=demo&utm_medium=skill

## Delete a note

`POST /v1/kv/{id}/{key}/delete`

Send: Nothing, or {} .

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/kv/demo/greeting/delete?secret=demo&utm_medium=skill

Example body (invoice.json):

```json
{}
```
