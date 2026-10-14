{{define "plugin-readme"}}# India API plugin

Free, open source reference data and checks for India, for Claude. The plugin adds two things:

- **The India API skill**, which tells Claude when to use the API and how to call it: pincodes and post offices, IFSC codes and bank branches, GSTIN and PAN format checks, bank, market and government holidays and working days, RTO codes, NSE and BSE company logos, mutual fund NAVs, HSN codes, UPI QR codes, numbers in words in lakh and crore, transliteration, panchang and more. {{.Count}} APIs in all.
- **The India API MCP server** at `{{.Origin}}/mcp`, so Claude can look these up by itself. It takes no key and no sign-up.

## What it sends and where

The MCP server and the skill's examples call `{{.Origin}}` and nothing else. The plugin runs no code on your machine: it has no hooks, scripts or commands. What you look up is sent to the API to get the answer. The API never logs client IP addresses, and endpoints that take personal values (phone numbers, email addresses, text) keep their input out of its logs. See the [privacy page]({{.Origin}}/privacy).

## Install

```
/plugin marketplace add {{.Repo}}
/plugin install {{.Name}}@{{.Market}}
```

## Try

- Which district and state is pincode 637001 in?
- Which bank branch is IFSC SBIN0000001, and does it take RTGS?
- Is 26 January 2026 a bank holiday in Tamil Nadu? What is the next working day?

## Licence and source

The plugin and the skill are MIT-0. The API's source code is AGPL-3.0 at {{.RepoURL}}. Documentation: {{.Origin}}/docs. Report problems at {{.RepoURL}}/issues.
{{end}}
