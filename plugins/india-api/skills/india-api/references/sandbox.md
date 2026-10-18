# Code sandbox API

> Run Python, JavaScript, Go, C, C++, Ruby, PHP or Bash in a throwaway microVM.

Code in, output out. Each run gets a Firecracker microVM of its own, with its own Linux kernel, which is destroyed afterwards: nothing from one run reaches the next. A run has 30 seconds, 256 MB of memory, one processor, no network and 64 KB of each of stdout and stderr; files can be sent along and the code can read them. A session keeps a microVM for five minutes (ten for a distribution): run several commands, write and read files, and with network on install packages from each language's own registry (PyPI, npm, the Go module proxy, RubyGems, Packagist), and nothing else. Distributions to try out: Debian, Alpine, Arch and Fedora, in a session or in the terminal page. The sandbox services need a free sign-in with GitHub (POST /v1/sandbox/login, which works from a terminal or an agent), so they cannot be used to mine or spam anonymously. Each account has 200 runs and 30 minutes of processor time a day (50 and 10 for GitHub accounts younger than 30 days), reset at midnight India time. What ran is kept as a hash in an audit log for a year, never the code. The rest of the API stays keyless.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Languages, versions and limits

`GET /v1/sandbox/languages`

The languages the code image runs, with the version of each, the session images, and every limit. Needs no token.

Example: https://api.dharun.dev/v1/sandbox/languages?utm_medium=skill

## Run code once

`POST /v1/sandbox/run`

language (python, javascript, go, c, cpp, ruby, php, bash, or an alias such as py, js, node, golang, c++, sh), code (up to 64 KB), and optionally stdin, args and files ([{path, content, encoding}] with encoding text or base64; up to 20 files of 1 MB each, relative to the working directory /work). Returns stdout, stderr, exitCode, durationMs, cpuMs and limitsHit (time, memory, output). Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: JSON: language, code, and optionally stdin, args and files.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/sandbox/run?utm_medium=skill

Example body (invoice.json):

```json
{"language": "python", "code": "import sys\nprint(sum(range(10)), sys.version.split()[0])"}
```

## Start a session

`POST /v1/sandbox/sessions`

image is code (default) or a distribution: debian, alpine, arch, fedora. network: true lets a code session reach its languages' package registries through a proxy (500 MB of downloads), and nothing else; distributions have no network. Returns the session's id, when it expires and links to exec, run, files, stop and tty. An account may have two sessions at once. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: JSON: image and network, both optional.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/sandbox/sessions?utm_medium=skill

Example body (invoice.json):

```json
{"image": "code", "network": true}
```

## Describe a session

`GET /v1/sandbox/sessions/{id}`

Its image, network and expiry. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The session's id.

Example: https://api.dharun.dev/v1/sandbox/sessions/ses0f1e2d3c4b5a6978?utm_medium=skill

## Run a command in a session

`POST /v1/sandbox/sessions/{id}/exec`

command (a shell line) or argv (a list), and optionally stdin, cwd (default /work), env and timeoutMs (up to 60 seconds). Returns the same as a run. Counts as a run against the daily quota. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: JSON: command or argv, and optionally stdin, cwd, env and timeoutMs.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The session's id.

Example: `POST` https://api.dharun.dev/v1/sandbox/sessions/ses0f1e2d3c4b5a6978/exec?utm_medium=skill

Example body (invoice.json):

```json
{"command": "pip install pyfiglet && python -m pyfiglet hi"}
```

## Run code in a session

`POST /v1/sandbox/sessions/{id}/run`

As /v1/sandbox/run, in the session: files written before and packages installed stay. Code sessions only. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: JSON: language, code, and optionally stdin, args and files.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The session's id.

Example: `POST` https://api.dharun.dev/v1/sandbox/sessions/ses0f1e2d3c4b5a6978/run?utm_medium=skill

Example body (invoice.json):

```json
{"language": "python", "code": "import sys\nprint(sum(range(10)), sys.version.split()[0])"}
```

## Put a file in a session

`POST /v1/sandbox/sessions/{id}/upload`

Writes the body, or the first file of a multipart form, to path: relative to /work, or absolute. Up to 1 MB. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: The file, as the body or as multipart/form-data, up to 1 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/receipt.png

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The session's id.
- `path` (query, required): Where to write it.

Example: `POST` https://api.dharun.dev/v1/sandbox/sessions/ses0f1e2d3c4b5a6978/upload?path=receipt.png&utm_medium=skill

## Read a file from a session

`GET /v1/sandbox/sessions/{id}/download`

Returns the file at path, up to 1 MB. A directory (list=true, a path ending in /, or no path for /work) is listed as JSON: name, size, dir and mode of each entry. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The session's id.
- `path` (query): The file or directory.
- `list` (query): List the directory at path. One of: true.

Example: https://api.dharun.dev/v1/sandbox/sessions/ses0f1e2d3c4b5a6978/download?path=out.png&utm_medium=skill

## Stop a session now

`POST /v1/sandbox/sessions/{id}/stop`

Ends the session and destroys its microVM and everything in it. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: Nothing: an empty body or {}.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The session's id.

Example: `POST` https://api.dharun.dev/v1/sandbox/sessions/ses0f1e2d3c4b5a6978/stop?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Terminal of a session (WebSocket)

`GET /v1/sandbox/sessions/{id}/tty`

A WebSocket: binary messages carry the terminal both ways, and a text message {"cols": 120, "rows": 32} resizes it. Only for sessions started from the terminal page at /terminal, which passes a browser check; programs use exec. The token may be given as ?token=, as browsers cannot set headers on a WebSocket. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

What you send is never written to the log, and the answer is not cached.

Parameters:

- `id` (path, required): The session's id.
- `cols` (query): Columns, default 80.
- `rows` (query): Rows, default 24.
- `token` (query): The sandbox token, for browsers.

Example: https://api.dharun.dev/v1/sandbox/sessions/ses0f1e2d3c4b5a6978/tty?utm_medium=skill

## Guides

- [Run code an AI wrote, safely, in a throwaway microVM](https://api.dharun.dev/guides/run-ai-generated-code.md): Give your agent a code interpreter without running its code on your machine: Python, JavaScript, Go, C, Ruby or PHP in a Firecracker microVM that is destroyed afterwards.
