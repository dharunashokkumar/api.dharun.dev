# Indic text tools API

> Letters as readers count them, dictionary order, look-alike fixes, braille and keyboard layouts.

For text in Indian scripts: split into aksharas and count them, sort words in a language's dictionary order, repair sequences that only look like a letter, write Bharati braille, and convert between QWERTY keys and the InScript and Tamil99 layouts.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Split into letters and count them

`GET /v1/indic/aksharas`, also `POST`

POST body: The text, as text/plain or JSON {"text": "..."}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

Parameters:

- `text` (query): The text.

Example: https://api.dharun.dev/v1/indic/aksharas?text=%E0%A4%95%E0%A5%8D%E0%A4%B7%E0%A4%A4%E0%A5%8D%E0%A4%B0%E0%A4%BF%E0%A4%AF&utm_medium=skill

## Dictionary order

`GET /v1/indic/sort`, also `POST`

POST body: JSON {"words": [...], "lang": "ta"}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

Parameters:

- `words` (query): Words separated by commas.
- `lang` (query): hi, ta, bn, te, kn, ml, mr, gu, pa, or or en. Default hi.

Example: https://api.dharun.dev/v1/indic/sort?lang=ta&words=%E0%AE%95%E0%AE%9F%E0%AE%B2%E0%AF%8D,%E0%AE%85%E0%AE%AE%E0%AF%8D%E0%AE%AE%E0%AE%BE,%E0%AE%86%E0%AE%B1%E0%AF%81&utm_medium=skill

## Fix look-alike letters

`GET /v1/indic/normalise`, also `POST`

POST body: The text, as text/plain or JSON {"text": "...", "strip": "true"}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

Parameters:

- `text` (query): The text.
- `strip` (query): Also remove zero-width characters.

Example: https://api.dharun.dev/v1/indic/normalise?text=%E0%A4%85%E0%A4%BE%E0%A4%AE&utm_medium=skill

## Bharati braille

`GET /v1/braille`, also `POST`

Text in any Indian script (and digits) as Unicode braille cells, Grade 1. Check with a braille reader before printing for others.

POST body: The text, as text/plain or JSON {"text": "..."}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

Parameters:

- `text` (query): The text, up to 5,000 characters.

Example: https://api.dharun.dev/v1/braille?text=%E0%A4%A8%E0%A4%AE%E0%A4%B8%E0%A5%8D%E0%A4%A4%E0%A5%87&utm_medium=skill

## QWERTY keys and Indian layouts

`GET /v1/keyboard/convert`, also `POST`

type: what pressing those QWERTY keys gives on the layout (to fix text typed with the wrong keyboard on). keys: the keys to press to type a text.

POST body: The text, as text/plain or JSON {"text": "...", "layout": "tamil99", "direction": "type"}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

Parameters:

- `text` (query): Keys, or text.
- `layout` (query): Default inscript-hindi. One of: inscript-hindi, tamil99.
- `direction` (query): Default type. One of: type, keys.

Example: https://api.dharun.dev/v1/keyboard/convert?layout=tamil99&text=vphhkf&utm_medium=skill
