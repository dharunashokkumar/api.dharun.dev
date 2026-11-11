# Address reader API

> A free-text Indian address split into house, street, locality, city, state and pincode, checked.

Splits a written address into its parts and checks the pincode against the post office list: a pincode in another state, or a city that is not near the pincode, is flagged. Rules, not a model. What you send is never logged.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Read an Indian address

`GET /v1/address/parse`, also `POST`

POST body: The address, as text/plain or JSON {"text": "..."}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The address.

Example: https://api.dharun.dev/v1/address/parse?text=12%2C%20Anna%20Salai%2C%20Mount%20Road%2C%20Chennai%2C%20Tamil%20Nadu%20600002&utm_medium=skill
