{{define "about"}}# About

India API puts the reference data that Indian apps need behind one free, consistent API.

It is built and run by [{{.Author}}]({{.AuthorURL}}). Requests for new APIs and reports of wrong data are welcome as [GitHub issues]({{.Repo}}/issues).

## Why it exists

Every Indian app needs the same lookups: a pincode's district, a branch's IFSC, whether a GSTIN is well formed, which days the banks are shut in a state, how much is "1,25,000" in words. The data is public, but it is scattered across government portals, spreadsheets and PDFs, in a different shape each time, and every team ends up scraping and cleaning it again. India API does that once, in the open, and answers in one shape: `{ data, meta }`, with the source and licence of the data in every answer.

## How it works

- **One service, plain addresses.** Every API is a path under `/v1`, answering JSON over HTTPS with open CORS. Most need no key; a [free key]({{.Origin}}/signup) raises the limits, and the model endpoints need one.
- **Data kept fresh by the service itself.** Each dataset is rebuilt from its source on a schedule (IFSC weekly, mutual fund NAVs daily, rates every few hours), checked against the copy in use, and swapped in without a restart. A source that changes shape is refused rather than served half-read.
- **Models run here.** OCR, speech, translation, embeddings and the small language model run on this service; what you send is not passed to another company and not used to train anything.
- **Written for programs and for people.** The same catalogue makes the documentation, the OpenAPI file, `llms.txt`, an [MCP server]({{.Origin}}/ai) for AI assistants, the [playground]({{.Origin}}/play) and the [free tools]({{.Origin}}/tools).

## What it will not do

- Say that an identifier was issued: format checks confirm the shape and the check digit, not that a GSTIN, PAN or Aadhaar number belongs to anyone.
- Keep or log what you send to the private endpoints. The [privacy page]({{.Origin}}/privacy) says exactly what is kept.
- Use data whose terms forbid it. Each dataset's source and licence were checked before it was added; see the table below.

## Contact

Issues and data corrections: [GitHub]({{.Repo}}/issues). Security problems: see the [security page]({{.Origin}}/security).

## Licences

- **Calling the API** puts no licence on your app.
- **The code** is free software under the [GNU AGPL-3.0]({{.Repo}}/blob/main/LICENSE). You may run, study and change it; if you run a changed copy as a service for others, you must offer them your changes.
- **Code examples** in the documentation and the guides, the Agent Skill and the Claude Code plugin are MIT-0: use them anywhere, no credit needed.
- **Data** keeps the licence of its source, listed below and named in every response under `meta.source`.
- **Company logos** are trademarks of their owners and are shown to identify the companies.

## Data sources

| Source | Licence | Retrieved |
|---|---|---|
{{- range .Sources}}
| [{{.Name}}]({{.URL}}) | {{.Licence}} | {{.Retrieved}} |
{{- end}}
{{end}}
