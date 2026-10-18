# Request bin API

> A URL that records every request sent to it, for testing webhooks.

Make a bin and get a URL. Point a webhook at it (Razorpay, GitHub, a form, your own code) or call it with any method and any path below it; each request is kept with its method, path, query, headers and body (the first 64 KB). Read them back as JSON with the bin's token. A bin keeps the last 100 requests and lasts 24 hours, then it and everything in it is deleted. Headers that carry the sender's address are dropped before anything is stored, nothing is logged, and what a bin holds is only ever returned as JSON to the token's holder. Making a bin counts as 10 requests against the rate limit; each request a bin records counts as one for its sender, and a bin records up to 60 a minute.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Make a request bin

`POST /v1/hooks`

Returns the bin's id, url (where to send requests), readUrl and token. The token is shown only in this answer; only its hash is kept.

Send: Nothing: an empty body or {}.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/hooks?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Send a request to a bin

`GET /v1/hook/{id}`, also `POST`

Any method (GET, HEAD, POST, PUT, PATCH, DELETE) and any path after the id (/v1/hook/{id}/payments?attempt=1) is recorded. The answer is always the same small JSON: a bin does not imitate an API. The documentation's bin, example, accepts requests and keeps none.

POST body: Anything, up to 64 KB kept (the rest is counted, not kept). Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The bin's id.

Example: https://api.dharun.dev/v1/hook/example?utm_medium=skill

## Read what a bin received

`GET /v1/hooks/{id}`

Newest first. body is text when it is valid UTF-8, otherwise base64 (bodyEncoding says which); truncated is true when the body was over 64 KB. Poll with since=<the highest seq you have> to get only new requests. POST /v1/hooks/{id}/delete with the same token deletes the bin at once. The example bin's token is example.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The bin's id.
- `token` (query): The bin's token, if not sent as Authorization: Bearer <token>.
- `since` (query): Only requests with a higher seq.
- `limit` (query): At most this many, 1 to 100. Default 100.

Example: https://api.dharun.dev/v1/hooks/example?token=example&utm_medium=skill

## Delete a bin now

`POST /v1/hooks/{id}/delete`

Deletes the bin and everything it received. Needs the bin's token.

Send: Nothing: an empty body or {}.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The bin's id.

Example: `POST` https://api.dharun.dev/v1/hooks/example/delete?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Guides

- [See exactly what a webhook sends, with a request bin](https://api.dharun.dev/guides/test-webhooks.md): Point Razorpay, GitHub or your own code at a throwaway URL and read every request it receives as JSON, with no account and nothing to install.
