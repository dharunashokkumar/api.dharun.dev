# Phone numbers API

> Check and format an Indian phone number, and see whose series it is.

Validates an Indian number, tells mobile from toll-free and fixed lines, formats it (E.164 and national), and names the operator and telecom circle a mobile number's series was first given to. With number portability the current operator can differ. Numbers are never written to the request log.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Check a phone number

`GET /v1/phone/{number}`

What you send is never written to the log, and the answer is not cached.

Parameters:

- `number` (path, required): The number, with or without +91, 91 or 0; spaces and dashes are ignored.

Example: https://api.dharun.dev/v1/phone/+91%2095001%2012345?utm_medium=skill
