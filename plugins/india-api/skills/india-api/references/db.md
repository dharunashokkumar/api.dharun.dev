# Throwaway databases API

> A private SQLite database for a day, over HTTP.

For tests, demos, teaching SQL and agents that need somewhere to put rows. Up to 10 MB, one statement at a time, three seconds each. No database port: SQL goes over HTTPS. The database demo (secret demo) is read-only and holds the states of India. Each caller may hold 5 bins and databases at a time; each lasts a day, in memory or a temporary file, and is gone after a restart.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Make a database

`POST /v1/db`

Answers with its query address and secret (shown once). Needs a free API key once keys are required (see /keys).

Send: Nothing, or {} .

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/db?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Run SQL

`POST /v1/db/{id}/query`

SQLite SQL, with ? parameters. Answers columns, rows (up to 1,000) and the rows changed. ATTACH, VACUUM and most PRAGMAs are refused. Send the secret as an X-Secret header.

Send: The SQL as text, or JSON {"sql": "...", "params": [...]}.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/db/demo/query?secret=demo&utm_medium=skill

Example body (invoice.json):

```json
{"sql": "SELECT code, name FROM states WHERE name LIKE ? ORDER BY name", "params": ["T%"]}
```

## Delete a database

`POST /v1/db/{id}/delete`

Deletes the database at once. Send the secret as an X-Secret header.

Send: Nothing, or {} .

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/db/demo/delete?secret=demo&utm_medium=skill

Example body (invoice.json):

```json
{}
```
