---
title: Fill city and state from a pincode in a web form
description: Look up the district and state as soon as someone types a six-digit pincode, with plain JavaScript and no API key.
apis: pincode, states
published: 2026-10-14
---

Indian address forms ask for a pincode, a city and a state. The pincode alone tells you the district and the state, so you can fill those fields for the person typing.

## The request

`GET /v1/pincode/{pincode}` returns the district, the state and every post office that uses the pincode:

```sh
curl https://api.dharun.dev/v1/pincode/637001
```

```json
{
  "data": {
    "pincode": "637001",
    "district": "Namakkal",
    "stateCode": "TN",
    "state": "Tamil Nadu",
    "offices": [
      { "name": "Namakkal H.O", "type": "Head Office", "delivery": true, "district": "Namakkal" },
      { "name": "Namakkal Bazaar S.O", "type": "Post Office", "delivery": false, "district": "Namakkal" }
    ]
  },
  "meta": { "source": { "name": "...", "licence": "..." } }
}
```

The real answer has a few more fields on each office (division, region, latitude and longitude). A pincode that does not exist answers `404` with `{ "error": { "code": "not_found", "message": "..." } }`.

## The form

No key is needed and CORS is open, so the browser can call the API directly:

```html
<label>Pincode <input id="pincode" inputmode="numeric" maxlength="6" autocomplete="postal-code"></label>
<label>District <input id="district" autocomplete="address-level2"></label>
<label>State <input id="state" autocomplete="address-level1"></label>
<label>Post office <select id="office"></select></label>
<p id="pincode-error" hidden>We could not find that pincode.</p>

<script>
  const pincode = document.getElementById("pincode");
  const error = document.getElementById("pincode-error");

  pincode.addEventListener("input", async () => {
    const value = pincode.value.trim();
    error.hidden = true;
    if (!/^[1-9][0-9]{5}$/.test(value)) return; // six digits, not starting with 0

    const response = await fetch(`https://api.dharun.dev/v1/pincode/${value}`);
    if (!response.ok) {
      error.hidden = false;
      return;
    }
    const { data } = await response.json();
    if (pincode.value.trim() !== value) return; // the person kept typing

    document.getElementById("district").value = data.district;
    document.getElementById("state").value = data.state;
    document.getElementById("office").replaceChildren(
      ...data.offices.map((office) => new Option(office.name, office.name))
    );
  });
</script>
```

## Good to know

- Keep the fields editable. A few pincodes reach into two districts; each office in `offices` carries its own `district`, so use the chosen office's district when it must be exact.
- `stateCode` is the two-letter code that every other endpoint accepts in its `state` filter. [`/v1/states/TN`](https://api.dharun.dev/v1/states/TN) gives the state's GST code and vehicle prefixes.
- To find a pincode from a place name instead, search: [`/v1/pincode?q=namakkal&state=TN`](https://api.dharun.dev/v1/pincode?q=namakkal&state=TN).
- Pincodes change rarely. Cache answers for a day or more if you look up the same ones often.
