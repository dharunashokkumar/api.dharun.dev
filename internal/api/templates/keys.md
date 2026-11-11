{{define "keys"}}# API keys

India API keys are free. Most endpoints keep working without one; a key gives you your own, larger allowance, and the model endpoints will need one.

## What a key gives you

| | Without a key | With a free key |
|---|---|---|
| Counted per | IP address | Key, so apps on a shared address stop sharing one allowance |
| Per minute | {{.Limit}} units | {{.KeyLimit}} units |
| Model endpoints | {{if .KeyDate}}Until {{.KeyDate}} only{{else}}For now, until a date announced here{{end}} | {{.Quota}} requests a day in all, one at a time per key |

A unit is what a request costs: a lookup is 1, real CPU work (pictures, astronomy, live network checks) 10, a file worked on 15, a traceroute 20, a model run 30. Every answer says where you stand in the `RateLimit-Limit`, `RateLimit-Remaining` and `RateLimit-Reset` headers, keyed or not. A logo already in the cache costs nothing.

## Get a key

1. [Sign up]({{.Origin}}/signup) with an email address and a password, or with Google or GitHub.
2. Confirm the address from the mail we send (Google and GitHub accounts arrive confirmed).
3. On [your account page]({{.Origin}}/account), make a key. It is shown once: copy it then. An account holds up to three keys; rotate or revoke them there. A revoked key stops working within a minute.

## Send a key

In a header, never in the address (addresses end up in logs and browser history):

```bash
curl -H "Authorization: Bearer india_live_..." {{.Origin}}/v1/pincode/637001
curl -H "X-API-Key: india_live_..." {{.Origin}}/v1/pincode/637001
```

Keys start with `india_live_`, so secret scanners can find one that leaked; a mistyped key is refused at once with `401 invalid_key`. MCP clients send the same header from their configuration.

A web page cannot keep a key secret, and an `<img>` tag cannot send a header: use logos and lookups in web pages without a key. Cached logos are free.

## The model endpoints

These run a neural network for each request, so they will need a key:

{{range .Keyed}}- `{{.}}`
{{end}}- `/v1/text/redact` with `names=true` (plain redaction stays keyless)

{{if .KeyDate}}From **{{.KeyDate}}** (midnight India time) a keyless call to them answers `401` with the error code `key_required`.{{else}}The date from which they need a key will be announced here at least 30 days ahead.{{end}} Until then they answer as before, with an `X-Key-Notice` header saying so. With a key, `X-Quota-Limit` and `X-Quota-Remaining` show the day's quota; it resets at midnight India time. Over it, the answer is `429 quota_exceeded`.

The free tool pages on this site (such as [image to text]({{.Origin}}/tools/image-to-text)) use a short-lived token of their own, which allows {{.SiteAllowance}} model requests a day per address.

## What we keep

An account holds your email address, a password hash (when you use a password), the Google or GitHub account id (when you use one), the hashes of your keys (never the keys), when they were made, and how many requests each key made per day and API group. The log line of a request to a model endpoint holds the key's id, so abuse can be traced and stopped; other requests log nothing about keys. Deleting your account on the account page removes all of it at once. See [privacy]({{.Origin}}/privacy).
{{end}}
