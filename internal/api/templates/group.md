{{define "group"}}# {{.Title}} API

> {{.Tagline}}

{{.Intro}}

Base URL `{{.Origin}}`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: {{.Limit}} requests a minute per client.
{{range .Endpoints}}
## {{.Summary}}

`{{.Method}} {{.Path}}`{{if .Post}}, also `POST`{{end}}
{{- if .Doc}}

{{.Doc}}
{{- end}}
{{- if .Post}}

POST body: {{.Body}} Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.
{{- end}}
{{- with .Upload}}

Send: {{.Accepts}}{{if not .JSON}} Post the file as the request body, or as `multipart/form-data`{{if gt .Files 1}} with one field per file{{end}}.{{end}}{{if .Sample}} Sample file: {{$.Origin}}/samples/{{.Sample}}{{end}}
{{- end}}
{{- if .Private}}

What you send is never written to the log, and the answer is not cached.
{{- end}}
{{- if or .PathParams .Query}}

Parameters:
{{range .PathParams}}
- `{{.Name}}` (path, required): {{.Doc}}{{if .Enum}} One of: {{join .Enum ", "}}.{{end}}
{{- end}}
{{- range .Query}}
- `{{.Name}}` (query{{if .Required}}, required{{end}}): {{.Doc}}{{if .Enum}} One of: {{join .Enum ", "}}.{{end}}
{{- end}}
{{- end}}

Example: {{if .Upload}}`POST` {{end}}{{.URL}}
{{- if .Returns}} ({{.Returns}}){{end}}
{{- if and .Upload .Upload.JSON}}

Example body (invoice.json):

```json
{{.ExampleBody}}
```
{{- end}}
{{- with .Sample}}

Response{{if .Shortened}}, with lists and long text shortened{{end}}:

{{fence .JSON}}json
{{.JSON}}
{{fence .JSON}}
{{- end}}
{{end}}
{{- if .Sources}}
## Data source
{{range .Sources}}
- [{{.Name}}]({{.URL}}){{if .Version}}, {{.Version}}{{end}}, retrieved {{.Retrieved}}. Licence: {{.Licence}}.{{if .Attribution}} {{.Attribution}}{{end}}
{{- end}}
{{end}}
{{- if .Guides}}
## Guides
{{range .Guides}}
- [{{.Title}}]({{$.Origin}}/guides/{{.Slug}}.md): {{.Description}}
{{- end}}
{{end}}{{end}}
