---
title: Bank holidays, working days and T+1 settlement dates
description: Find whether a date is a bank holiday in a state, the next working day, N working days ahead, and NSE settlement dates, in Python.
apis: working-days, holidays
published: 2026-10-14
---

Bank holidays in India differ by state, and some Saturdays are working days while others are not. The working-days endpoint knows the Reserve Bank's holiday list for each state, the second and fourth Saturday rule, and NSE's trading and clearing holidays.

## Is a date a working day?

```sh
curl "https://api.dharun.dev/v1/working-days/2026-01-26?calendar=bank&state=TN"
```

```json
{
  "data": {
    "date": "2026-01-26",
    "weekday": "Monday",
    "calendar": "bank",
    "state": "TN",
    "working": false,
    "reasons": ["Republic Day"],
    "previousWorkingDay": "2026-01-23",
    "nextWorkingDay": "2026-01-27"
  }
}
```

`calendar` is one of `bank` (needs `state`), `market` (NSE trading days), `settlement` (NSE clearing days) or `central` (central government offices).

## N working days from a date

Add `add=N` (negative to go back) and read `shifted`. A T+1 trade settles one settlement day later:

```sh
curl "https://api.dharun.dev/v1/working-days/2026-01-16?calendar=settlement&add=1"
```

```json
{ "data": { "date": "2026-01-16", "working": true, "add": 1, "shifted": "2026-01-19" } }
```

## In Python

```python
import requests

API = "https://api.dharun.dev/v1"


def working_day(date: str, calendar: str = "bank", state: str | None = None, add: int = 0) -> dict:
    params = {"calendar": calendar}
    if state:
        params["state"] = state
    if add:
        params["add"] = add
    response = requests.get(f"{API}/working-days/{date}", params=params, timeout=10)
    body = response.json()
    if not response.ok:
        raise RuntimeError(body["error"]["message"])
    return body["data"]


day = working_day("2026-01-26", state="TN")
if not day["working"]:
    print(f"Closed for {', '.join(day['reasons'])}; next open {day['nextWorkingDay']}")

due = working_day("2026-01-16", calendar="bank", state="TN", add=3)["shifted"]
print(f"Three bank working days later: {due}")
```

## Count working days in a period

```sh
curl "https://api.dharun.dev/v1/working-days?from=2026-01-01&to=2026-01-31&calendar=market"
```

The answer has the count and each day off with its reason. A period can be up to 366 days long.

## The holiday list itself

```sh
curl "https://api.dharun.dev/v1/holidays/2026?type=bank&state=TN"
```

Only years whose lists have been published are answered, so a request for a year too far ahead returns an error rather than a guess. Festival dates that depend on the moon can move; the lists follow the issuing authority's latest notice.
