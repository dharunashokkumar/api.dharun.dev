---
title: Show the bank and branch for an IFSC code
description: Turn an IFSC into a bank name, branch and address, and check that the branch takes NEFT, RTGS, IMPS or UPI before you send money there.
apis: ifsc, validate
published: 2026-10-14
---

When someone adds a bank account for a payout, showing "State Bank of India, Kolkata Main" under the IFSC they typed catches most mistakes before any money moves.

## Look up the branch

```sh
curl https://api.dharun.dev/v1/ifsc/SBIN0000001
```

```json
{
  "data": {
    "ifsc": "SBIN0000001",
    "bank": { "code": "SBIN", "name": "State Bank of India" },
    "branch": "KOLKATA MAIN",
    "address": "SAMRIDDHI BHAWAN, 1 STRAND ROAD, KOLKATA 700 001",
    "city": "Kolkata",
    "district": "Kolkata",
    "stateCode": "WB",
    "state": "West Bengal",
    "micr": "700002021",
    "imps": true,
    "neft": true,
    "rtgs": true,
    "upi": true
  },
  "meta": { "source": { "name": "IFSC dataset", "licence": "..." } }
}
```

An IFSC that is not in the Reserve Bank's lists answers `404`.

## In Python

```python
from functools import lru_cache

import requests

API = "https://api.dharun.dev/v1"


@lru_cache(maxsize=4096)
def branch(ifsc: str) -> dict | None:
    """The branch for an IFSC, or None if no branch has that code."""
    response = requests.get(f"{API}/ifsc/{ifsc.strip().upper()}", timeout=10)
    if response.status_code == 404:
        return None
    response.raise_for_status()
    return response.json()["data"]


found = branch("SBIN0000001")
if found is None:
    print("No branch has this IFSC. Check it again.")
elif not found["neft"]:
    print(f"{found['bank']['name']}, {found['branch']} does not take NEFT.")
else:
    print(f"{found['bank']['name']}, {found['branch']}, {found['city']}")
```

## Check the format first

If you only want to know whether a code is shaped like an IFSC (four letters, a zero, six letters or digits), use the format check. It answers from the rules alone and never says 404:

```sh
curl https://api.dharun.dev/v1/validate/ifsc/SBIN0000001
```

## Find a bank's code or its branches

- Banks by name: [`/v1/banks?q=gramin`](https://api.dharun.dev/v1/banks?q=gramin)
- One bank's branches in a state, filtered by text: [`/v1/ifsc?bank=SBIN&state=TN&q=namakkal`](https://api.dharun.dev/v1/ifsc?bank=SBIN&state=TN&q=namakkal)

The data comes from the Reserve Bank's NEFT and RTGS lists through the open razorpay/ifsc dataset, and the server refreshes it every week.
