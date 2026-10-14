# Number to words API

> Numbers and rupee amounts in words, the Indian way.

Spells numbers with lakh and crore in English, Hindi or Tamil. 1,00,000 is "one lakh". Use style=currency for cheque-style amounts.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Spell out a number

`GET /v1/words/{number}`

Parameters:

- `number` (path, required): The number. Commas, spaces and underscores are ignored.
- `lang` (query): Language. Default en. One of: en, hi, ta.
- `style` (query): currency gives rupees and paise. Default number. One of: number, currency.

Example: https://api.dharun.dev/v1/words/1,25,000.50?style=currency

## Guides

- [Write rupee amounts in words for invoices and cheques](https://api.dharun.dev/guides/amount-in-words.md): Spell an amount the Indian way, with lakh and crore, rupees and paise, in English, Hindi or Tamil.
