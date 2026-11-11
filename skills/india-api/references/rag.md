# Search your own documents API

> Upload documents, ask a question, get the best passages back: retrieval for your own AI app.

Make a collection, add documents (plain text or Markdown; take a PDF's text with POST /v1/pdf/text first), and query it: the documents are cut into passages, embedded with the multilingual model on this server, and each query's nearest passages are reranked by the cross-encoder. Collections are private to their secret, kept in memory for a day, up to 1,000 passages. The collection demo (secret demo) holds a few facts about India, for trying. Needs a free API key once keys are required (see /keys).

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Make a collection

`POST /v1/rag`

Send: Nothing, or {} .

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/rag?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Add documents

`POST /v1/rag/{id}/documents`

Send: JSON {"documents": [{"name": "...", "text": "..."}], "size": 800}: up to 2 MB of text.

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/rag/demo/documents?secret=demo&utm_medium=skill

Example body (invoice.json):

```json
{"documents": [{"name": "shop rules", "text": "Returns are accepted within 7 days with the bill."}]}
```

## Ask a question

`POST /v1/rag/{id}/query`

Send: JSON {"query": "...", "top": 5, "rerank": true}.

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/rag/demo/query?secret=demo&utm_medium=skill

Example body (invoice.json):

```json
{"query": "How many digits does a pincode have?", "top": 2}
```

## Delete a collection

`POST /v1/rag/{id}/delete`

Send: Nothing, or {} .

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/rag/demo/delete?secret=demo&utm_medium=skill

Example body (invoice.json):

```json
{}
```
