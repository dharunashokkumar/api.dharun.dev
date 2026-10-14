{{define "about"}}# About

India API puts the reference data that Indian apps need behind one free, consistent API.

It is built and run by [{{.Author}}]({{.AuthorURL}}). Requests for new APIs and reports of wrong data are welcome as [GitHub issues]({{.Repo}}/issues).

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
