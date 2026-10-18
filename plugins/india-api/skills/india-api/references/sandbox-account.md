# Sandbox account API

> Sign in with GitHub for the sandbox services, and see today's quota.

The sandbox, the terminal and the throwaway databases need an account; nothing else in the API does. Sign-in uses GitHub's device flow, which works where there is no browser to redirect: POST /v1/sandbox/login gives a short code, the person enters it at github.com/login/device, and POST /v1/sandbox/login/poll with the deviceCode then returns a token for 30 days. The API asks GitHub for no permissions and reads only the public profile; it keeps the account's numeric ID, login and creation date, its usage per day and an audit log of what ran for a year, and forgets GitHub's own token at once.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Start a GitHub sign-in

`POST /v1/sandbox/login`

Returns userCode, verificationUri, deviceCode, interval (seconds between polls) and expiresIn.

Send: Nothing: an empty body or {}.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/sandbox/login?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Finish a GitHub sign-in

`POST /v1/sandbox/login/poll`

status is pending until the person approves, slow_down when polled too often, or ok with the token, its expiry and the account's quota. 403 when the sign-in was refused or the code expired.

Send: JSON: {"deviceCode": "..."}.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/sandbox/login/poll?utm_medium=skill

Example body (invoice.json):

```json
{"deviceCode": "3584d83530557fdd1f46af8289938c8ef79f9dc5"}
```

## Today's quota

`GET /v1/sandbox/account`

The login, the daily quota, what has been used today and what is left, and when it resets. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

What you send is never written to the log, and the answer is not cached.

Example: https://api.dharun.dev/v1/sandbox/account?utm_medium=skill

## Revoke a token

`POST /v1/sandbox/logout`

The token stops working at once. Needs a sandbox token (Authorization: Bearer <token>; sign in at POST /v1/sandbox/login).

Send: Nothing: an empty body or {}.

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/sandbox/logout?utm_medium=skill

Example body (invoice.json):

```json
{}
```

## Guides

- [Run code an AI wrote, safely, in a throwaway microVM](https://api.dharun.dev/guides/run-ai-generated-code.md): Give your agent a code interpreter without running its code on your machine: Python, JavaScript, Go, C, Ruby or PHP in a Firecracker microVM that is destroyed afterwards.
