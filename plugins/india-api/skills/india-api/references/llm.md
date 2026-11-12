# Small LLM (English) API

> Summarise, classify, extract to JSON and rewrite English text with a small open model, and an OpenAI-compatible chat.

A small language model (Qwen 2.5 1.5B Instruct, Apache-2.0) that runs on this server, for narrow tasks where its answer is forced into shape: a summary, a label from your list, fields as JSON that match your schema, a rewrite. And /v1/chat/completions in OpenAI's format, so code and tools written for that API work by changing the base URL to https://api.dharun.dev/v1. English only: it was measured and is not good enough in Hindi or Tamil. It can be wrong; check what matters. Nothing you send is logged or kept. Needs a free API key; each key has 100,000 tokens a day (prompt and answer).

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Summarise English text

`POST /v1/llm/summarise`

Send {text, style, words}: style paragraph (default), bullets or headline; words the length, 10 to 300 (default 60). Up to 12,000 characters. Answers {summary, style, truncated, usage}.

Send: A JSON document as below.

Needs a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`), from the start. With a key: 500 model requests and 100,000 tokens a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/llm/summarise?utm_medium=skill

Example body (invoice.json):

```json
{"text": "The Reserve Bank of India kept the repo rate unchanged at 5.5 per cent on Wednesday. The Monetary Policy Committee voted unanimously and said inflation was within the target band, while growth stayed strong.", "style": "headline"}
```

## Pick a label for a text from your list

`POST /v1/llm/classify`

Send {text, labels, multi, instructions}: 2 to 30 labels; the answer is always one of them (label), or with multi true every one that fits (labels). instructions may say what the labels mean.

Send: A JSON document as below.

Needs a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`), from the start. With a key: 500 model requests and 100,000 tokens a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/llm/classify?utm_medium=skill

Example body (invoice.json):

```json
{"text": "The delivery came two days late and the box was torn.", "labels": ["positive", "negative", "neutral"]}
```

## Extract fields from text as JSON

`POST /v1/llm/extract`

Send {text, fields} (up to 40 names, each answered as a string) or {text, schema} (a JSON Schema of type object, up to 8 KB). The answer's data always matches the schema: decoding is constrained to it. Anything the text does not say comes back empty.

Send: A JSON document as below.

Needs a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`), from the start. With a key: 500 model requests and 100,000 tokens a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/llm/extract?utm_medium=skill

Example body (invoice.json):

```json
{"text": "Hi, I am Ravi Kumar from Coimbatore, call me on 98765 43210 about the 2 BHK flat.", "fields": ["name", "city", "phone", "requirement"]}
```

## Rewrite text in another tone

`POST /v1/llm/rewrite`

Send {text, style}: style formal, simple, polite, shorter, friendly or grammar (the default: fix grammar and spelling only). Up to 4,000 characters.

Send: A JSON document as below.

Needs a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`), from the start. With a key: 500 model requests and 100,000 tokens a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/llm/rewrite?utm_medium=skill

Example body (invoice.json):

```json
{"text": "pls send the documents asap we are waiting since monday", "style": "polite"}
```

## Chat, OpenAI-compatible

`POST /v1/chat/completions`

OpenAI's chat completions request and answer (not this API's envelope): messages, max_tokens up to 1,024 (default 512), temperature, top_p, stop, seed, response_format (json_object or json_schema) and stream (server-sent events). The window is 4,096 tokens; messages up to 16,000 characters. No tools, images or audio. Any model name is accepted.

Send: A JSON document as below.

Needs a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`), from the start. With a key: 500 model requests and 100,000 tokens a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/chat/completions?utm_medium=skill (is in OpenAI's own JSON shape, not this API's { data, meta } envelope)

Example body (invoice.json):

```json
{"model": "qwen2.5-1.5b-instruct", "messages": [{"role": "user", "content": "Write a two-line welcome message for a food delivery app."}], "max_tokens": 80}
```

## The model list, for OpenAI clients

`GET /v1/models`

OpenAI's model list with the one model, for clients that ask before they chat.

Example: https://api.dharun.dev/v1/models?utm_medium=skill (is in OpenAI's own JSON shape, not this API's { data, meta } envelope)

## Guides

- [Pull fields out of customer messages with a free small LLM](https://api.dharun.dev/guides/small-llm-openai.md): Turn free-text enquiries into JSON that always matches your schema, classify them, and use the OpenAI SDK against India API's small hosted model by changing the base URL.
