---
title: Use India API from an MCP client (Claude, Cursor, VS Code)
description: Connect an AI assistant to India API's MCP server, load only the tool sets you need, add your key, check the connection, and fix the usual problems.
apis: mcp-tools
published: 2026-11-12
---

India API is also an MCP server: an assistant connected to it can look up a pincode, check a GSTIN, read a receipt or find today's gold rate on its own, and every answer names its source. It is one remote server over Streamable HTTP, so there is nothing to install.

## Connect

**Claude Code:**

```sh
claude mcp add --transport http india-api https://api.dharun.dev/mcp
```

**Cursor, VS Code, Claude Desktop and other clients** take a server entry like this in their MCP settings file (`.cursor/mcp.json`, `.vscode/mcp.json`, and so on; the outer key differs by client):

```json
{
  "mcpServers": {
    "india-api": {
      "type": "http",
      "url": "https://api.dharun.dev/mcp"
    }
  }
}
```

No key is needed for most tools.

## Add your key

A [free key](https://api.dharun.dev/signup) raises the limits, and the model tools (OCR, speech, translation and the others) need one from 15 December 2026. Send it as a header from the client, never in the URL:

```sh
claude mcp add --transport http india-api https://api.dharun.dev/mcp \
  --header "Authorization: Bearer $INDIA_API_KEY"
```

```json
{
  "mcpServers": {
    "india-api": {
      "type": "http",
      "url": "https://api.dharun.dev/mcp",
      "headers": { "Authorization": "Bearer ${env:INDIA_API_KEY}" }
    }
  }
}
```

(How a client reads environment variables differs; check yours rather than pasting the key into a shared file.)

## Load fewer tools

The server offers about a hundred tools. Every tool's description takes room in the assistant's context, so connect to only the sets you use:

```text
https://api.dharun.dev/mcp?tools=money,dates
```

Sets: `files`, `money`, `network`, `text`, `dates`, `places`. The `find_tool` tool is always there, so the assistant can still find a tool outside its sets. You can search the same list yourself:

```sh
curl "https://api.dharun.dev/v1/mcp-tools?q=gold%20rate"
```

```json
{"data": [
  {"name": "get_gold_silver_rates", "set": "money", "description": "Today's IBJA benchmark rates for gold (24K 999 to 14K 585), silver and platinum in India, per 10 grams (silver per kg) and per gram, before GST; or the history of one."},
  {"name": "get_rbi_rates", "set": "money", "description": "India's current repo rate, reverse repo, SDF, MSF, bank rate, CRR, SLR, ..."}
]}
```

## Check that it works

Ask the assistant something only the tools can answer: "Which district is pincode 641001 in?" or "Is 33AABCS1429B1Z1 a valid GSTIN format?" It should call `lookup_pincode` or a validation tool and quote the source. To test the server without an assistant:

```sh
curl -X POST https://api.dharun.dev/mcp \
  -H "Content-Type: application/json" -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## When something goes wrong

| What you see | Why, and the fix |
|---|---|
| The client lists no tools | It is set up for a local (stdio) server: choose HTTP or Streamable HTTP, with the URL ending in `/mcp` |
| `405` when opening the URL in a browser | Expected: MCP uses POST. A browser's GET is not a client |
| A tool answers `key_required` | It is a model tool after 15 December 2026: add the `Authorization` header |
| `invalid_key` | The key was copied wrongly or revoked; copy it again from your account page |
| `rate_limited` | Too many calls a minute; a key raises the limit from 600 to 3,000 units |
| The assistant answers from memory | Ask it to use the india-api tools, or name the tool; check the server is enabled in the client |

Files (PDFs, images, audio) go in and out of the file tools as base64, up to 5 MB. Everything a tool returns is exactly what the REST endpoint of the same name returns, so the [documentation](https://api.dharun.dev/docs) describes the tools too.
