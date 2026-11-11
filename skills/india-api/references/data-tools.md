# Data format tools API

> JSON, YAML, TOML, CSV and XML to each other; text and JSON differences; spreadsheets to JSON and back.

Everyday conversions for scripts and pipelines. What you send is never logged.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Convert between data formats

`GET /v1/convert`, also `POST`

Reads JSON, YAML, TOML, CSV or XML and writes any of them. The input format is guessed when from is not given.

POST body: The document, as text/plain, or JSON {"text": "...", "from": "yaml", "to": "json"}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The document, for short ones.
- `from` (query): Its format. Default: guessed. One of: json, yaml, toml, csv, xml.
- `to` (query): Default json. One of: json, yaml, toml, csv, xml.
- `raw` (query): true answers with the converted document itself.

Example: https://api.dharun.dev/v1/convert?to=yaml&text=%7B%22name%22%3A%22India%20API%22%2C%22tags%22%3A%5B%22a%22%2C%22b%22%5D%7D&utm_medium=skill

## Compare two texts or JSON documents

`POST /v1/diff`

Text: a unified patch and counts of added and removed lines. JSON: a list of JSON Patch (RFC 6902) changes with the old values.

Send: JSON: {"a": "...", "b": "...", "mode": "text" or "json", "context": 3, "changes": false}. a and b may be JSON values for mode json.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/diff?utm_medium=skill

Example body (invoice.json):

```json
{"a": "rice\ndal\noil\n", "b": "rice\nmoong dal\noil\nsalt\n"}
```

## Spreadsheet to JSON

`POST /v1/sheet/to-json`

Every sheet of an XLSX workbook (or a CSV file) as records named by the first row. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: An XLSX workbook or a CSV file, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/table.csv

What you send is never written to the log, and the answer is not cached.

Parameters:

- `sheet` (query): Only this sheet.
- `header` (query): false gives rows of values instead of records. Default true.

Example: `POST` https://api.dharun.dev/v1/sheet/to-json?utm_medium=skill

## JSON to a spreadsheet

`POST /v1/sheet/from-json`

Records in, an XLSX workbook out, with the header row frozen.

Send: JSON: {"records": [{...}]} for one sheet, or {"sheets": [{"name": "...", "records": [...]}, {"name": "...", "values": [[...]]}]}.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/sheet/from-json?utm_medium=skill (returns an XLSX workbook)

Example body (invoice.json):

```json
{"records": [{"pincode": "637001", "district": "Namakkal"}, {"pincode": "600001", "district": "Chennai"}]}
```
