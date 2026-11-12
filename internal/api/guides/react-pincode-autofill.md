---
title: Pincode autofill in React, with post office choice and cancellation
description: A React hook that looks up a six-digit pincode as it is typed, cancels stale requests, fills district and state but leaves them editable, and lets the user pick the post office.
apis: pincode
published: 2026-11-12
---

The plain JavaScript version of this is in [Fill city and state from a pincode](https://api.dharun.dev/guides/pincode-autofill). React adds three problems of its own: a fast typist starts several requests and the slowest one must not win, the filled fields must stay editable (the user knows their address better than the post office list), and a pincode with many post offices should let the user choose one.

## What a pincode returns

```sh
curl "https://api.dharun.dev/v1/pincode/637001"
```

```json
{
  "data": {
    "pincode": "637001",
    "district": "Namakkal",
    "stateCode": "TN",
    "state": "Tamil Nadu",
    "offices": [
      {"name": "Namakkal H.O", "type": "Head Office", "delivery": true, "district": "Namakkal", "state": "Tamil Nadu"},
      {"name": "Namakkal Bazaar S.O", "type": "Post Office", "delivery": false, "district": "Namakkal", "state": "Tamil Nadu"}
    ]
  }
}
```

(Shortened: 637001 has seven offices, and each also has its division, region, circle and position.) No key is needed, and CORS is open, so the browser can call it directly.

## The hook

```jsx
import { useEffect, useState } from "react";

const BASE = "https://api.dharun.dev/v1";

export function usePincode(pincode) {
  const [state, setState] = useState({ status: "idle", data: null, error: null });

  useEffect(() => {
    if (!/^[1-9][0-9]{5}$/.test(pincode)) {
      setState({ status: "idle", data: null, error: null });
      return;
    }
    // Each new pincode aborts the request for the previous one, so a slow
    // answer for an earlier pincode can never overwrite the latest one.
    const controller = new AbortController();
    setState((s) => ({ ...s, status: "loading", error: null }));
    fetch(`${BASE}/pincode/${pincode}`, { signal: controller.signal })
      .then(async (r) => {
        if (r.status === 404) throw new Error("We could not find this pincode. Check it, or fill the fields yourself.");
        if (r.status === 429) throw new Error("Too many lookups; fill the fields yourself or try again in a minute.");
        if (!r.ok) throw new Error("Lookup failed; fill the fields yourself.");
        return (await r.json()).data;
      })
      .then((data) => setState({ status: "done", data, error: null }))
      .catch((e) => {
        if (e.name !== "AbortError") setState({ status: "error", data: null, error: e.message });
      });
    return () => controller.abort();
  }, [pincode]);

  return state;
}
```

The pattern check runs before any request: Indian pincodes are six digits and never start with 0.

## The form

```jsx
import { useEffect, useState } from "react";
import { usePincode } from "./usePincode";

export function AddressForm() {
  const [pincode, setPincode] = useState("");
  const [district, setDistrict] = useState("");
  const [state, setStateName] = useState("");
  const [office, setOffice] = useState("");
  const [touched, setTouched] = useState({ district: false, state: false });
  const lookup = usePincode(pincode);

  // Fill only the fields the user has not changed by hand.
  useEffect(() => {
    if (lookup.status !== "done") return;
    if (!touched.district) setDistrict(lookup.data.district);
    if (!touched.state) setStateName(lookup.data.state);
    setOffice(lookup.data.offices.length === 1 ? lookup.data.offices[0].name : "");
  }, [lookup.status, lookup.data]);

  const offices = lookup.data?.offices ?? [];
  return (
    <form>
      <label>
        Pincode
        <input inputMode="numeric" maxLength={6} value={pincode}
          onChange={(e) => setPincode(e.target.value.replace(/\D/g, ""))} />
      </label>
      {lookup.status === "loading" && <small>Looking up…</small>}
      {lookup.error && <small role="alert">{lookup.error}</small>}

      {offices.length > 1 && (
        <label>
          Post office
          <select value={office} onChange={(e) => setOffice(e.target.value)}>
            <option value="">Choose…</option>
            {offices.map((o) => <option key={o.name}>{o.name}</option>)}
          </select>
        </label>
      )}

      <label>
        District
        <input value={district} onChange={(e) => { setDistrict(e.target.value); setTouched((t) => ({ ...t, district: true })); }} />
      </label>
      <label>
        State
        <input value={state} onChange={(e) => { setStateName(e.target.value); setTouched((t) => ({ ...t, state: true })); }} />
      </label>
    </form>
  );
}
```

## Why it is built this way

- **Abort, do not ignore.** Aborting the stale request also saves the user's data plan; the `AbortError` is swallowed because it is not a failure.
- **Editable fields.** A pincode can cover more than one district, and people write their town, not the post office's district. Fill once, then respect what they type.
- **Post office choice.** Many pincodes have several offices; offer them only when there is more than one.
- **Fail open.** Every error leaves the form usable. The lookup is a convenience, never a gate.

In React's development mode the effect runs twice on mount; the abort in the cleanup makes that harmless.
