{{define "ai"}}# Use India API from your AI assistant

> Connect the MCP server and your assistant can look up pincodes, IFSC codes, holidays and more by itself. No key, no sign-up, nothing to install.

MCP server: `{{.Endpoint}}` (Streamable HTTP, stateless, no authentication). Each of the {{len .Tools}} tools returns exactly what the matching REST endpoint returns.
{{range .Clients}}
## {{.Name}}

{{.Note}}

{{fence .Code}}
{{.Code}}
{{fence .Code}}
{{- if .Button}}

{{.Button}}: {{.ButtonURL}}
{{- end}}
{{end}}
## Teach your coding agent the API

The Agent Skill tells Claude Code, Codex, Copilot, Cursor, Gemini CLI and other agents when to use this API and how to call it.

```
npx skills add {{.Repo}}
```

Or download {{.Origin}}/skills/{{.Name}}.zip and upload it where your assistant takes skills. Read it at {{.Origin}}/.well-known/agent-skills/{{.Name}}/SKILL.md. The skill is MIT-0.

## Tools
{{range .Tools}}
- `{{.Name}}`: {{.Description}}
{{- end}}

## For other programs

- OpenAPI 3.1: {{.Origin}}/openapi.json
- llms.txt: {{.Origin}}/llms.txt, and everything in one file: {{.Origin}}/llms-full.txt
- MCP server card: {{.Origin}}/mcp/server-card
- AI catalog: {{.Origin}}/.well-known/ai-catalog.json
- API catalog: {{.Origin}}/.well-known/api-catalog
- Every page is also Markdown: add `.md` to its address or send `Accept: text/markdown`.
{{end}}
