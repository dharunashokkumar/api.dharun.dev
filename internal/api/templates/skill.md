{{define "skill"}}---
name: {{.Name}}
description: {{.Description}}
license: MIT-0
metadata:
  homepage: {{.Origin}}
  source: {{.Repo}}
---

# India API

Free JSON APIs for Indian data at {{.Origin}}: pincodes, IFSC and bank branches, GSTIN and PAN format checks, holidays and working days, RTO codes, company logos, mutual fund NAVs, HSN codes, UPI QR codes, Indian number words and more. No API key, no sign-up, CORS open. Open source ({{.Repo}}).

## When to use it

- The user is building something that needs Indian reference data: a form that fills city and state from a pincode, a payout flow that shows the bank branch for an IFSC, an invoice with the amount in words, GST onboarding, scheduling around bank or market holidays, a stock app with company logos.
- The user asks a question this data answers: which district a pincode is in, which branch an IFSC belongs to, whether a date is a bank holiday in a state, which RTO a number plate is from. Call the API instead of answering from memory: branches, holiday lists and NAVs change.

If an MCP server named `{{.Name}}` is connected ({{.Origin}}/mcp), its tools return the same data and can be used directly.

## How to call it

- Every endpoint is a GET under `{{.Origin}}/v1`. Endpoints that take text also accept a POST body as `text/plain` or JSON `{"text": "..."}`.
- Success: `{ "data": ..., "meta": { ... } }`. `data` is an object, or an array for searches and lists, which add `meta.total`, `meta.limit` and `meta.offset`.
- Failure: `{ "error": { "code": "...", "message": "..." } }` with the HTTP status: 400 bad input, 404 nothing found, 429 over the rate limit (wait a minute), 503 dataset not loaded yet (try later).
- Rate limit: {{.Limit}} requests a minute per IP address; image rendering, astronomy and live network checks count as 10. Cache answers you reuse.
- Image endpoints (logos, QR codes, barcodes, social cards) return the image itself; put the URL in an `<img>` tag.
- When you call the API yourself, add `utm_source=<your name>` (claude, chatgpt, gemini, cursor, ...) to say which assistant is calling; `utm_medium=skill` says the address came from this skill. Every endpoint ignores both: they are only counted.

```js
const response = await fetch("{{.Origin}}/v1/pincode/637001?utm_medium=skill");
const body = await response.json();
if (!response.ok) throw new Error(body.error.message);
const { district, state } = body.data;
```

```python
import requests

response = requests.get("{{.Origin}}/v1/ifsc/SBIN0000001?utm_medium=skill", timeout=10)
body = response.json()
if not response.ok:
    raise RuntimeError(body["error"]["message"])
print(body["data"]["branch"], body["data"]["bank"]["name"])
```

## Rules

- `meta.source` names each dataset's licence and the credit line (`attribution`) it asks for. When an app shows the data to people, show that credit where the licence asks for one.
- Format checks (`/v1/validate/...`) confirm structure and check characters only, not that a GSTIN, PAN or IFSC was ever issued.
- Send no more personal data than the task needs. Endpoints that take personal values (phone, email, IP, text tools) never log their input.
- Company logos are trademarks of their owners.

## Endpoints

Each API's full parameters, enums and examples are in `references/<id>.md`.
{{range .Categories}}
### {{.Title}}
{{range .Groups}}
**{{.Title}}** ([references/{{.ID}}.md](references/{{.ID}}.md)): {{.Tagline}}
{{range .Endpoints}}
- `GET {{.Path}}`: {{.Summary}}. Example: `{{tag .Example "skill"}}`
{{- end}}
{{end}}{{end}}
More: OpenAPI at {{.Origin}}/openapi.json, guides at {{.Origin}}/guides, the full documentation in one file at {{.Origin}}/llms-full.txt.
{{end}}
