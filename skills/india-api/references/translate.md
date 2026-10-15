# Translation API

> Translate between English and 22 Indian languages with IndicTrans2.

Machine translation by AI4Bharat's IndicTrans2 (distilled models), between English and Assamese, Bengali, Bodo, Dogri, Gujarati, Hindi, Kannada, Kashmiri, Konkani, Maithili, Malayalam, Manipuri, Marathi, Nepali, Odia, Punjabi, Sanskrit, Santali, Sindhi, Tamil, Telugu and Urdu. Between two Indian languages the text goes through English. Web and e-mail addresses and long numbers are kept as they are. Nothing you send is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Translate text

`GET /v1/translate`, also `POST`

Translates a sentence at a time and returns the whole translation and each sentence. Up to 2,000 characters and 30 sentences. Write Indian languages in their own script.

POST body: The text to translate. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it).
- `from` (query, required): The language of the text: en, or a code from /v1/translate/languages such as hi or ta.
- `to` (query, required): The language to translate into.

Example: https://api.dharun.dev/v1/translate?from=en&to=ta&text=The+train+to+Chennai+leaves+at+6+pm.+Please+reach+the+station+early.

## Languages

`GET /v1/translate/languages`

Every language with its short code and FLORES-200 code; either can be used as from and to.

Example: https://api.dharun.dev/v1/translate/languages
