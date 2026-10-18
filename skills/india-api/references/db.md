# Throwaway Postgres API

> A real Postgres database for an hour, for tests and demos.

Ask for a database and get one: its own user and password, 100 MB, five connections, deleted after an hour. It is real Postgres 17, reached over a WebSocket at /v1/db/ws, which serverless drivers such as Neon's speak (no port to open); the India API clients include a small bridge that lets psql and any ordinary driver connect through localhost. A user can reach only its own database. Queries stop after 30 seconds. A database over 100 MB is locked (it can still be dropped). Needs a sandbox token; an account may have three at once.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Make a database

`POST /v1/db`

Returns name, user, password (shown only now), websocket, a connectionString for the bridge, and expiresAt. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: Nothing: an empty body or {}.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/db?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Describe a database

`GET /v1/db/{name}`

Its size, limit and expiry. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

What you send is never written to the log, and the answer is not cached.

Parameters:

- `name` (path, required): The database's name.

Example: https://api.dharun.dev/v1/db/sb_0f1e2d3c4b5a6978?utm_medium=skill

## Delete a database now

`POST /v1/db/{name}/drop`

Cuts off its connections and deletes it. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: Nothing: an empty body or {}.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `name` (path, required): The database's name.

Example: `POST` https://api.dharun.dev/v1/db/sb_0f1e2d3c4b5a6978/drop?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Postgres over WebSocket

`GET /v1/db/ws`

A WebSocket carrying the Postgres protocol to the server, as Neon's serverless driver expects (set its wsProxy to this address). Postgres checks the user and password; no token is needed here.

What you send is never written to the log, and the answer is not cached.

Example: https://api.dharun.dev/v1/db/ws?utm_medium=skill
