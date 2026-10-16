---
title: Get warned before a TLS certificate expires
description: Check your sites' certificates from a daily cron job and get an alert when one is close to expiry, invalid or missing an intermediate.
apis: tls, whois
published: 2026-10-16
---

A certificate that renews itself still fails sometimes: the renewal job breaks, a load balancer keeps the old certificate, or a new one is installed without its intermediate. A daily check catches all three before visitors do.

## The request

```sh
curl "https://api.dharun.dev/v1/tls/dharun.dev"
```

```json
{
  "data": {
    "host": "dharun.dev",
    "port": 443,
    "valid": true,
    "tls": { "version": "TLS 1.3", "cipher": "TLS_AES_128_GCM_SHA256", "alpn": "h2" },
    "certificate": {
      "issuer": "WE1",
      "names": ["dharun.dev", "*.dharun.dev"],
      "notAfter": "2026-12-30T10:12:44Z",
      "daysLeft": 75,
      "key": "ECDSA P-256"
    },
    "problems": []
  }
}
```

`valid` is false when any problem is an error: expired, not yet valid, the wrong name, self-signed, or a chain that does not lead to a trusted root (usually a missing intermediate). Warnings, such as TLS 1.0 still being accepted, leave it true. Each problem has a `code` for your program and a `message` for people.

## A daily check in a shell script

The script below checks a list of hosts and prints a line for each one that needs attention. Run it from cron and send its output wherever your alerts go; with no output, all is well.

```sh
#!/bin/sh
# Checks each host's certificate; prints the ones that need attention.
for host in dharun.dev www.example.co.in shop.example.in; do
  answer=$(curl -sS -m 30 "https://api.dharun.dev/v1/tls/${host}") || { echo "$host: the check itself failed"; continue; }
  echo "$answer" | jq -r --arg host "$host" '
    if .error then "\($host): \(.error.message)"
    elif .data.valid == false then "\($host): \([.data.problems[] | select(.level == "error") | .message] | join(" "))"
    elif .data.certificate.daysLeft < 21 then "\($host): expires in \(.data.certificate.daysLeft) days"
    else empty end'
done
```

```sh
# crontab -e: every morning at 7
0 7 * * * /home/me/check-certs.sh | mail -E -s "Certificates" me@example.com
```

A check counts as 20 requests against the rate limit of 600 a minute, so a few dozen hosts a day are no problem. Each site can be checked 10 times a minute in all, by everyone using the API.

## In Node.js

```js
async function checkCertificate(host) {
  const response = await fetch(`https://api.dharun.dev/v1/tls/${encodeURIComponent(host)}`, {
    signal: AbortSignal.timeout(30_000),
  });
  if (response.status === 429 || response.status === 503) {
    // Rate limited, or busy: try again later rather than reporting the site as broken.
    return { host, retryAfter: Number(response.headers.get("Retry-After") ?? 60) };
  }
  const body = await response.json();
  if (!response.ok) {
    // 404: the name has no address; 422: a private address; 502 or 504: the site did not answer.
    return { host, problem: body.error.message };
  }
  const { valid, certificate, problems } = body.data;
  const errors = problems.filter((p) => p.level === "error").map((p) => p.message);
  if (!valid) return { host, problem: errors.join(" ") };
  if (certificate.daysLeft < 21) return { host, problem: `Expires in ${certificate.daysLeft} days.` };
  return { host, ok: true, daysLeft: certificate.daysLeft };
}
```

A timeout or a network error throws, which is right: a check that did not run is not a pass.

## The domain itself can expire too

A valid certificate does not help when the domain lapses. Whois gives the registration's expiry date, for `.in` and `.co.in` as well as `.com`:

```sh
curl "https://api.dharun.dev/v1/whois/google.in"
# "expires": "2027-02-14T20:35:14Z", "daysToExpiry": 121
```

Add `daysToExpiry` to the same daily script: registries renew on payment, not on their own.

## What the check cannot see

The check connects from India on port 443 only, and uses Go's TLS, which cannot offer SSL 3 or export ciphers, so those are never reported. Mail servers on other ports (465, 993) are not checked.
