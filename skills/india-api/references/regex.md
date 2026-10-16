# Regex tester API

> Run a regular expression against a text and see every match, explained.

Tests a pattern in JavaScript or RE2 (Go) syntax: every match with its groups and their character offsets, an optional replacement, and the pattern taken apart token by token in plain words. JavaScript patterns run under a one-second limit, since nested quantifiers can take exponentially long; RE2 cannot. Patterns and texts are not logged or kept. Each request counts as 10 against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Test a regular expression

`GET /v1/regex`, also `POST`

An invalid pattern answers 422 with code invalid_pattern and the engine's message; a JavaScript pattern that runs out of time answers 422 with code timeout. A group that took no part in a match has a null text, which is different from an empty one.

POST body: The text to test the pattern on (pattern and options in the query), or JSON {"pattern": ..., "text": ...} with the options beside them. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `pattern` (query, required): The regular expression, without slashes. Up to 4096 bytes.
- `text` (query): The text to test it on (or POST it).
- `flavor` (query): js (default): JavaScript syntax, with lookaround and backreferences. re2: Go's syntax, linear time. One of: js, re2.
- `flags` (query): Any of i (ignore case), m (^ and $ at line breaks), s (. matches line breaks). Every match is always listed, so g is not needed.
- `replace` (query): Replace every match with this: $1, $<name>, $& and $$ in js; ${1}, ${name} and $$ in re2.
- `limit` (query): Matches listed, 1 to 1000. Default 100; count goes on to the end of the text.

Example: https://api.dharun.dev/v1/regex?flags=i&pattern=%28%3F%3Cstate%3E%5BA-Z%5D%7B2%7D%29%5B+-%5D%3F%28%5Cd%7B2%7D%29&text=Cars+TN+37%2C+ka-05+and+MH12&utm_medium=skill
