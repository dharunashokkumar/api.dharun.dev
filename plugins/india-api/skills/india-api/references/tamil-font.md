# Tamil legacy encodings API

> Old Tamil text that shows as accented Latin letters, to Unicode and back.

Before Unicode, Tamil was stored in 8-bit encodings that only looked like Tamil in one particular font; anywhere else the same text shows as characters like ¾Á¢ú. This converts such text to Unicode and Unicode back to it, moving the vowel signs that those encodings write before the consonant. TSCII 1.7 is offered now; TAB, TAM and Bamini are planned. The tables are written from the published standard.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Convert Tamil text

`GET /v1/tamil-font/convert`, also `POST`

Give the text as it looks without its font (the Windows-1252 characters), or POST the raw bytes as application/octet-stream. Converting to an 8-bit encoding answers with the same kind of text, and with the bytes in base64. Bytes the encoding does not assign are counted in undefined; characters it has no code for are listed in unmapped.

POST body: The text, or with Content-Type application/octet-stream the raw bytes. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The text (or POST it).
- `from` (query, required): What the text is in. One of: unicode, tscii.
- `to` (query): What to convert to. Default unicode; one of from and to must be unicode. One of: unicode, tscii.

Example: https://api.dharun.dev/v1/tamil-font/convert?from=tscii&text=%C2%BE%C3%81%C2%A2%C3%BA+%C3%85%C2%BD%C3%AC%C2%B8%C3%B5

## Encodings offered

`GET /v1/tamil-font/encodings`

The encodings that can be converted, with fonts that use each, to help recognise what a text is in.

Example: https://api.dharun.dev/v1/tamil-font/encodings
