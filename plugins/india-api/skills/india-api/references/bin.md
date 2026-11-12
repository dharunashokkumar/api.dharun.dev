# Card BIN lookup API

> Which network, card type and bank a card's first six digits belong to: Visa, Mastercard, RuPay, debit or credit.

The first six to eight digits of a card (the BIN or IIN) name its network, whether it is debit, credit or prepaid, its product and the bank that issued it. About 374,000 prefixes worldwide, about 12,000 of them Indian, from an open BIN list (CC BY 4.0). Use it to show the right card logo or to route a payment; never send a whole card number.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Network, type and bank of a card prefix

`GET /v1/bin/{prefix}`

The longest known prefix of the digits given. More than eight digits is refused, so a card number is never looked up. Quality varies: the list is not the networks' own.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `prefix` (path, required): 6 to 8 digits.

Example: https://api.dharun.dev/v1/bin/605067?utm_medium=skill
