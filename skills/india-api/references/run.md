# Run code API

> Run a Python or JavaScript program once in a throwaway microVM and get its output.

Send a program and get back what it printed, its exit code and the time it took. Each run gets a fresh Firecracker microVM with no network, 30 seconds, 256 MB of memory and 64 KB of output, thrown away after. Needs a free API key from the start: 50 runs and 10 CPU minutes a day per key. What ran is recorded as a hash for 30 days, never the code.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Run a program

`POST /v1/run`

JSON {"language": "python" or "javascript", "code": "...", "stdin": "..."}. Returns stdout, stderr, exitCode, durationMs, cpuMs and limitsHit (time, memory, output, processes). 401 without a key; 503 sandbox_off while the sandbox is switched off.

Send: JSON {"language": "python", "code": "print(sum(range(10)))"}, up to 64 KB of code.

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/run?utm_medium=skill

Example body (invoice.json):

```json
{"language": "python", "code": "import sys\nprint('Vanakkam from', sys.version.split()[0])"}
```

## The languages you can run

`GET /v1/run/languages`

Example: https://api.dharun.dev/v1/run/languages?utm_medium=skill
