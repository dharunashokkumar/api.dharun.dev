# MCP tools API

> Find the right tool among the MCP server's, and connect to only the sets you need.

The MCP server at /mcp offers every tool by default. Connect to /mcp?tools=money,dates (sets: files, money, network, text, dates, places) to list fewer, so an assistant's context stays small; find_tool finds the rest by task.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Search the MCP tools

`GET /v1/mcp-tools`

Parameters:

- `q` (query): The task in a few words. Empty lists every tool.
- `set` (query): Only one set. One of: files, money, network, text, dates, places.

Example: https://api.dharun.dev/v1/mcp-tools?q=gold%20rate&utm_medium=skill

## Guides

- [Use India API from an MCP client (Claude, Cursor, VS Code)](https://api.dharun.dev/guides/mcp-client.md): Connect an AI assistant to India API's MCP server, load only the tool sets you need, add your key, check the connection, and fix the usual problems.
