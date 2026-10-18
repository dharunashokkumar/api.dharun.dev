---
title: Run code an AI wrote, safely, in a throwaway microVM
description: Give your agent a code interpreter without running its code on your machine: Python, JavaScript, Go, C, Ruby or PHP in a Firecracker microVM that is destroyed afterwards.
apis: sandbox, sandbox-account
published: 2026-10-17
---

An agent that writes code wants to run it: to check an answer, crunch a CSV, draw a chart. Running that code on your own server is how secrets leak. The India API sandbox runs it in a microVM of its own, with its own Linux kernel, no network, 256 MB of memory and 30 seconds, and destroys it afterwards.

## Sign in once

The sandbox needs a free account, so it cannot be used to mine or spam anonymously. Sign-in is GitHub's device flow, which works from a terminal or a script:

```sh
API=https://api.dharun.dev
curl -X POST "$API/v1/sandbox/login"
```

Open the `verificationUri` it returns, enter the `userCode`, approve, then poll with the `deviceCode` until it returns your token:

```sh
curl -X POST "$API/v1/sandbox/login/poll" -d '{"deviceCode": "3584d835..."}'
```

The token lasts 30 days. Each account gets 200 runs and 30 CPU-minutes a day.

## Run code

```sh
curl -X POST "$API/v1/sandbox/run" -H "Authorization: Bearer $TOKEN" \
  -d '{"language": "python", "code": "import statistics\nprint(statistics.mean([4, 8, 15, 16, 23, 42]))"}'
```

```json
{
  "data": { "stdout": "18\n", "stderr": "", "exitCode": 0, "durationMs": 182, "cpuMs": 140, "limitsHit": [] }
}
```

`limitsHit` says when the code ran out of time, memory or output, so the agent can tell a crash from a limit. Files go along as `files`, `[{"path": "data.csv", "content": "..."}]`, and the code reads them from its working directory. numpy, pandas and matplotlib are already installed.

The languages and their versions, with every limit:

```sh
curl "https://api.dharun.dev/v1/sandbox/languages?utm_medium=guide"
```

## Keep a machine for a few minutes

When the agent needs to install a package or work in several steps, start a session with network on. It reaches each language's package registry (PyPI, npm, the Go proxy, RubyGems, Packagist) and nothing else:

```sh
curl -X POST "$API/v1/sandbox/sessions" -H "Authorization: Bearer $TOKEN" -d '{"network": true}'
curl -X POST "$API/v1/sandbox/sessions/$ID/exec" -H "Authorization: Bearer $TOKEN" \
  -d '{"command": "pip install pyfiglet && python -m pyfiglet hello"}'
```

Upload a file with `POST .../upload?path=data.csv` and read a result back with `GET .../download?path=chart.png`. The session ends after five minutes, or at `POST .../stop`.

## From an agent, through MCP

Connect the MCP server with your token as a header, and the agent gets `run_code`, `start_sandbox_session` and `run_in_sandbox_session` as tools:

```sh
claude mcp add --transport http india-api https://api.dharun.dev/mcp --header "Authorization: Bearer $TOKEN"
```

## What it will not do

There is no internet in a run, and in a session only the package registries. Nothing is kept: the code itself is never logged, only a hash of it in an audit log for abuse reports. For a shell you can type into, open the [terminal page](https://api.dharun.dev/terminal).
