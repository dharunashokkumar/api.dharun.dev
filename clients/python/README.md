# India API for Python

A small client for the [India API](https://api.dharun.dev/docs): free, open source APIs for Indian developers. Pincodes, IFSC, GSTIN and PAN checks, holidays and working days, RTO codes, company logos, panchang, OCR in Indian languages, PDF and image tools, translation, speech, a request bin and more: 119 methods, one per endpoint. No API key, no dependencies beyond Python 3.9.

```sh
pip install india-api
```

```python
from india_api import IndiaAPI

api = IndiaAPI()
place = api.pincode_by_pincode("600001")["data"]
print(place["district"], place["state"])

branch = api.ifsc_by_code("SBIN0000001")
words = api.words_by_number("125000.50", style="currency")
```

Every method returns what the API returns: the `{"data", "meta"}` envelope as a dict for JSON, or a `FileAnswer` (`content_type`, `data`, `save(path)`) for an image, a PDF or audio. Anything other than a 2xx answer raises `IndiaAPIError` with `status`, `code` (such as `not_found` or `rate_limited`) and its message.

- Methods are named after the API's OpenAPI operations: `/v1/pincode/{pincode}` is `pincode_by_pincode(pincode, **params)`. Keyword arguments are the query parameters, spelt as the [documentation](https://api.dharun.dev/docs) spells them.
- Text tools take long text as a body: `api.text_redact(body=long_text, names="true")`.
- File tools take a path or bytes: `api.ocr("scan.png", lang="tam").get("data")`, `api.image_convert("photo.heic", format="webp").save("photo.webp")`.
- `api.get(path, **query)`, `api.post(path, body, **query)` and `api.request(method, path, query, body)` reach anything else.
- `IndiaAPI(token=...)` sends a sandbox token, which only the sandbox services need.
- `python -m india_api.bridge` lets psql and any Postgres driver reach a throwaway database from `api.db()`: it listens on localhost:5432 and carries each connection over the API's WebSocket. Then connect with the database's `connectionString`, host `localhost`.

## Licence

This client is MIT-0: copy it into anything, no credit needed. The API's own code is AGPL-3.0 at https://github.com/dharunashokkumar/api.dharun.dev; the data keeps its sources' licences, credited in each response's `meta.source`.

Generated from the API's catalog 100f2618e8c4.
