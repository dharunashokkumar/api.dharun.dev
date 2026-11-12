---
title: Make a GST tax invoice PDF from JSON
description: Send the seller, buyer and items with HSN codes and GST rates, and get back an A4 tax invoice PDF with CGST, SGST or IGST worked out, the HSN summary and the amount in words.
apis: invoice, hsn, validate
published: 2026-11-12
---

A GST tax invoice has rules that are easy to get wrong by hand: CGST and SGST for a sale inside the state, IGST across states, an HSN-wise tax summary, the total rounded to the rupee and written in words. The invoice endpoint takes the invoice as JSON and returns the finished PDF, with all of that worked out.

## The request

POST the invoice as JSON. The seller's GSTIN is required; the buyer's state (from the buyer's GSTIN, or `place_of_supply`) decides between CGST + SGST and IGST.

```json
{
  "number": "INV-2026-0042",
  "date": "2026-10-14",
  "seller": {"name": "Sri Murugan Stores", "address": "12 Car Street, Namakkal 637001", "gstin": "33AABCS1429B1Z1"},
  "buyer": {"name": "Kaveri Traders", "address": "4 MG Road, Bengaluru 560001", "gstin": "29AAACK6812C1ZS"},
  "items": [
    {"description": "Basmati rice, 25 kg bag", "hsn": "1006", "quantity": 4, "unit": "Bag", "rate": 2150, "gst_rate": 5},
    {"description": "Steel tiffin box", "hsn": "7323", "quantity": 10, "rate": 349.5, "discount": 120, "gst_rate": 18}
  ],
  "bank": {"name": "State Bank of India", "account": "12345678901", "ifsc": "SBIN0000001", "holder": "Sri Murugan Stores"},
  "upi": "srimurugan@okaxis",
  "terms": "Goods once sold will not be taken back."
}
```

`rate` is the price of one unit before tax, and `discount` an amount off the line. With `upi`, the PDF carries a UPI QR code for the exact total.

## In Python

```python
import json
import requests

BASE = "https://api.dharun.dev/v1"

def make_invoice(invoice: dict, path: str) -> None:
    try:
        r = requests.post(f"{BASE}/invoice", json=invoice, timeout=30)
    except requests.RequestException as e:
        raise RuntimeError(f"could not reach the invoice API: {e}") from e
    if r.status_code == 400:
        # A field is missing or misspelt: the message names it.
        raise ValueError(r.json()["error"]["message"])
    if r.status_code == 429:
        raise RuntimeError(f"rate limited; try again in {r.headers.get('Retry-After', '60')} s")
    r.raise_for_status()
    with open(path, "wb") as f:
        f.write(r.content)

make_invoice(json.load(open("invoice.json")), "INV-2026-0042.pdf")
```

## In JavaScript (Node 18 or later)

```js
const BASE = "https://api.dharun.dev/v1";

async function makeInvoice(invoice) {
  const r = await fetch(`${BASE}/invoice`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(invoice),
  });
  if (!r.ok) {
    const { error } = await r.json();
    throw new Error(`${r.status} ${error.code}: ${error.message}`);
  }
  return Buffer.from(await r.arrayBuffer()); // the PDF
}
```

## Check the sums before you print

Add `?format=json` to get the worked-out invoice instead of the PDF. For the request above (Tamil Nadu to Karnataka, so IGST):

```json
{
  "supply": "inter_state",
  "totals": {
    "taxable_value": 11975,
    "cgst": 0,
    "sgst": 0,
    "igst": 1037.5,
    "total_tax": 1037.5,
    "round_off": 0.5,
    "total": 13013,
    "in_words": "Thirteen thousand thirteen rupees only"
  }
}
```

Each line also comes back with its `taxable_value`, tax and total, and `hsn_summary` has one row per HSN code and rate. `?format=png` gives the first page as a picture, for a preview.

## Mistakes the API catches

Unknown fields are refused, so a typo does not silently drop data:

```json
{"error": {"code": "bad_request", "message": "The body is not a valid invoice: unknown field \"seler\"."}}
```

A missing seller GSTIN, a malformed invoice number (up to 16 letters, digits, `-` and `/`) or an impossible date answer 400 with a message that says what to fix.

## Look up HSN codes and check GSTINs first

The invoice uses whatever HSN code you send. To find one, search the code list:

```sh
curl "https://api.dharun.dev/v1/hsn?q=basmati"
```

And check a GSTIN's format and check digit before it goes on an invoice:

```sh
curl "https://api.dharun.dev/v1/validate/gstin/33AABCS1429B1Z1"
```

A GSTIN that passes is well formed; whether it is registered and active is only known to the GST portal.

## Privacy

Nothing you send to the invoice endpoint is logged or kept: the PDF is made in memory and sent back. The PDF is yours; the code samples on this page are MIT-0.
