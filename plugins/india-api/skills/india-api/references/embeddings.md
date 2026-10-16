# Embeddings and reranking API

> Vectors for semantic search in Indian languages and English, and a reranker to order results.

For retrieval (RAG) and search without an API key: turn text into 384 numbers with multilingual-e5-small, so texts that mean the same land close together across about a hundred languages, Hindi, Tamil and the other Indian languages among them; then let bge-reranker-v2-m3 read the query with each candidate and order them. Nothing you send is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Text embeddings

`GET /v1/embed`, also `POST`

Up to 64 texts a request (GET: one or more text parameters; POST: JSON {"texts": [...], "type": ...}). Each comes back as 384 numbers scaled to length 1, so a dot product is the cosine similarity. Embed questions with type=query and the documents searched with type=passage, as the model was trained. Each text is read up to 512 tokens.

POST body: JSON: {"texts": ["..."], "type": "query" or "passage"}, or {"text": "..."}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): A text; repeat for several (or POST them).
- `type` (query): query (default) for questions, passage for the documents searched. One of: query, passage.

Example: https://api.dharun.dev/v1/embed?type=query&text=How+do+I+renew+my+passport%3F&utm_medium=skill

## Rerank documents for a query

`GET /v1/rerank`, also `POST`

Scores how well each document answers the query, from 0 to 1, and returns them best first with their place in the request. Up to 100 documents of up to 8,000 characters; each pair is read up to 512 tokens. GET takes query and repeated document parameters; POST takes JSON.

POST body: JSON: {"query": "...", "documents": ["...", "..."], "top": 3}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `query` (query, required): The question, up to 2,000 characters.
- `document` (query): A candidate; repeat for each (or POST them).
- `top` (query): Return only the best few. Default all.

Example: https://api.dharun.dev/v1/rerank?query=capital+of+Tamil+Nadu&document=Chennai+is+the+capital+of+Tamil+Nadu.&document=Mumbai+is+the+financial+capital+of+India.&utm_medium=skill
