# Text tools API

> Chunk documents, redact identifiers and names, moderate, count tokens, detect language.

Tools for working with text, especially in AI and retrieval pipelines. Send short text as ?text=, or POST it as text/plain, or as JSON {"text": ...} with the options beside it. Nothing you send is logged or kept. Each request counts as 10 against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Split a document into chunks

`GET /v1/text/chunk`, also `POST`

Splits at headings, paragraphs, sentences and words, in that order of preference, so chunks stay readable. Markdown chunks never cross a heading and carry their heading path. Each chunk is an exact slice of the input with its character offsets.

POST body: The document, plain text or Markdown. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it).
- `size` (query): Largest chunk in characters, overlap included, 100 to 20000. Default 1000.
- `overlap` (query): Characters repeated from the previous chunk, up to half the size. Default a tenth of the size.
- `format` (query): Default auto: Markdown if the text has headings or code fences. One of: auto, markdown, text.

Example: https://api.dharun.dev/v1/text/chunk?size=200&overlap=20&text=%23+Setup%0A%0AInstall+Go.+Clone+the+repository.%0A%0A%23+Deploy%0A%0APush+to+main.

## Redact Indian personal identifiers

`GET /v1/text/redact`, also `POST`

Masks Aadhaar numbers (Verhoeff-checked), PAN, GSTIN (checksum-checked), card numbers (Luhn-checked), Indian mobile numbers, email addresses, UPI IDs, vehicle numbers, voter IDs, passport numbers and IP addresses. Returns where each was found, never the values. With names=true a model also masks people's names: IndicNER for Indian scripts, bert-base-NER for English. Street addresses are not detected.

POST body: The text to redact. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it).
- `types` (query): Comma-separated kinds to redact; default all. Kinds: email, upi, gstin, card, aadhaar, pan, phone, vehicle, voter_id, passport, ip.
- `style` (query): label replaces with [PAN]; partial keeps the last digits, as on a bank statement. Default label. One of: label, partial.
- `names` (query): true also finds people's names with a model (slower; text up to 20,000 characters). Default false. One of: true, false.

Example: https://api.dharun.dev/v1/text/redact?text=Call+98765+43210%2C+PAN+ABCPK1234F

## Count tokens

`GET /v1/text/tokens`, also `POST`

Exact token counts with OpenAI tokenizers, by encoding or model name. Other vendors do not publish their tokenizers, so their counts are not offered.

POST body: The text to count. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it).
- `model` (query): Model name, such as gpt-4o or gpt-4, or an encoding: o200k_base, cl100k_base, p50k_base. Default o200k_base.

Example: https://api.dharun.dev/v1/text/tokens?model=gpt-4o&text=%E0%AE%B5%E0%AE%A3%E0%AE%95%E0%AF%8D%E0%AE%95%E0%AE%AE%E0%AF%8D%2C+how+many+tokens+is+this%3F

## Detect the language

`GET /v1/text/language`, also `POST`

Language and script, with a confidence. Covers about 80 languages including Hindi, Bengali, Marathi, Gujarati, Punjabi, Tamil, Telugu, Kannada, Malayalam, Odia and Urdu. Romanised Indian text (Hinglish, Tanglish) is flagged as such.

POST body: The text. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it).

Example: https://api.dharun.dev/v1/text/language?text=%E0%AE%85%E0%AE%95%E0%AE%B0+%E0%AE%AE%E0%AF%81%E0%AE%A4%E0%AE%B2+%E0%AE%8E%E0%AE%B4%E0%AF%81%E0%AE%A4%E0%AF%8D%E0%AE%A4%E0%AF%86%E0%AE%B2%E0%AF%8D%E0%AE%B2%E0%AE%BE%E0%AE%AE%E0%AF%8D+%E0%AE%86%E0%AE%A4%E0%AE%BF+%E0%AE%AA%E0%AE%95%E0%AE%B5%E0%AE%A9%E0%AF%8D+%E0%AE%AE%E0%AF%81%E0%AE%A4%E0%AE%B1%E0%AF%8D%E0%AE%B1%E0%AF%87+%E0%AE%89%E0%AE%B2%E0%AE%95%E0%AF%81

## Score text for abuse, Hinglish and Tanglish included

`GET /v1/text/moderate`, also `POST`

Three classifiers each score the text from 0 to 1: a general multilingual one (toxic), one for Hindi typed in Roman letters (profanity, harassment, hate_speech, caste, communal, regional, misogyny) and one for Tamil typed in Roman letters (offensive). flagged says whether any score reached the threshold. Up to 4,000 characters; the first 256 tokens are read. Scores are a model's guess: use them to sort or to ask a person, not to decide alone.

POST body: The text. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it).
- `threshold` (query): Score at which flagged becomes true, between 0 and 1. Default 0.5.
- `models` (query): Comma-separated: general, hinglish, tanglish. Default all three.

Example: https://api.dharun.dev/v1/text/moderate?text=tu+bahut+bakwas+aadmi+hai

## Guides

- [Remove Aadhaar, PAN and phone numbers before sending text to an LLM](https://api.dharun.dev/guides/redact-before-llm.md): Mask Indian personal identifiers in text before it reaches a language model or a log, with check-digit validation so ordinary numbers are left alone.
