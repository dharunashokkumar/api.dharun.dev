# Email checks API

> Is an email address well formed, throwaway, and able to receive mail.

Checks the form of an address, whether its domain hands out throwaway mailboxes, whether it is a free provider or a role account such as support@, and whether the domain has mail servers. No mail server is contacted and no mailbox is tried. Addresses are never written to the request log. Counts as 10 requests against the rate limit.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Check an email address

`GET /v1/email/{address}`

What you send is never written to the log, and the answer is not cached.

Parameters:

- `address` (path, required): The email address.

Example: https://api.dharun.dev/v1/email/someone@mailinator.com?utm_medium=skill
