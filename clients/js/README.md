# India API for JavaScript

A small client for the [India API](https://api.dharun.dev/docs): free, open source APIs for Indian developers. Pincodes, IFSC, GSTIN and PAN checks, holidays and working days, RTO codes, company logos, panchang, OCR in Indian languages, PDF and image tools, translation, speech, a request bin and more: 119 methods, one per endpoint. No API key, no dependencies. Works in Node 18 or newer, Deno, Bun, browsers and edge runtimes.

```sh
npm install india-api
```

```js
import { IndiaAPI } from "india-api";

const api = new IndiaAPI();
const { data } = await api.pincodeByPincode("600001");
console.log(data.district, data.state);

const branch = await api.ifscByCode("SBIN0000001");
const words = await api.wordsByNumber("125000.50", { style: "currency" });
```

Every method returns what the API returns: the `{ data, meta }` envelope for JSON, or `{ contentType, bytes, headers }` for an image, a PDF or audio. Anything other than a 2xx answer throws an `IndiaAPIError` with `status`, `code` (such as `not_found` or `rate_limited`) and `message`.

- Methods are named after the API's OpenAPI operations: `/v1/pincode/{pincode}` is `pincodeByPincode(pincode, params)`. The second argument holds the query parameters, as the [documentation](https://api.dharun.dev/docs) names them.
- Text tools take long text as a body: `api.textRedact({ names: "true" }, { body: longText })`.
- File tools take a path (in Node), a `Blob`, a `File`, a `Uint8Array` or an `ArrayBuffer`: `await api.ocr("scan.png", { lang: "tam" })`.
- `api.get(path, query)`, `api.post(path, body, query)` and `api.request(method, path, options)` reach anything else.
- `new IndiaAPI({ token })` sends a sandbox token, which only the sandbox services need.
- `npx india-api db-bridge` (Node.js 22 or newer) lets psql and any Postgres driver reach a throwaway database from `api.db()`: it listens on localhost:5432 and carries each connection over the API's WebSocket. Then connect with the database's `connectionString`, host `localhost`.

The API allows 600 requests a minute per address; see the documentation for what each endpoint counts as.

## Licence

This client is MIT-0: copy it into anything, no credit needed. The API's own code is AGPL-3.0 at https://github.com/dharunashokkumar/api.dharun.dev; the data keeps its sources' licences, credited in each response's `meta.source`.

Generated from the API's catalog 100f2618e8c4.
