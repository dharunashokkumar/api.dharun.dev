# Transliteration API

> Rewrite text between Indian scripts and Roman letters, or turn Roman typing into an Indian script.

Converts by rule between Devanagari, Bengali, Gurmukhi, Gujarati, Odia, Tamil, Telugu, Kannada and Malayalam, and to ISO 15919, IAST or plain ASCII. ISO 15919 and IAST can also be turned back into any of the scripts. The spelling changes, not the language: this is not translation. Casual Roman typing (vanakkam) cannot be converted by rule: /v1/transliterate/suggest guesses it with a model, as phone keyboards do.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Transliterate text

`GET /v1/transliterate`, also `POST`

POST body: The text. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it). Letters of other scripts pass through unchanged.
- `to` (query, required): A script (devanagari, bengali, gurmukhi, gujarati, odia, tamil, telugu, kannada, malayalam), a language code such as ta or hi, or iso15919, iast, ascii.
- `from` (query): auto (default: the script of the text), a script, or iso15919 or iast for romanised input.

Example: https://api.dharun.dev/v1/transliterate?to=tamil&text=%E0%A4%A8%E0%A4%AE%E0%A4%B8%E0%A5%8D%E0%A4%A4%E0%A5%87+%E0%A4%AD%E0%A4%BE%E0%A4%B0%E0%A4%A4&utm_medium=skill

## Roman typing to an Indian script (type English, get Tamil)

`GET /v1/transliterate/suggest`, also `POST`

Guesses how words typed in Roman letters are written in an Indian script with IndicXlit (AI4Bharat), giving up to eight suggestions per word, best first, and the text with the best guess for each word. 21 languages. Up to 60 words; anything that is not Roman letters is kept as it is.

POST body: The text in Roman letters. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it).
- `lang` (query, required): as, bn, brx, gom, gu, hi, kn, ks, mai, ml, mni, mr, ne, or, pa, sa, sd, si, ta, te or ur.
- `count` (query): Suggestions per word, 1 to 8. Default 4.

Example: https://api.dharun.dev/v1/transliterate/suggest?lang=ta&text=vanakkam+chennai&utm_medium=skill
