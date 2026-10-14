---
title: Make a UPI payment QR code for an amount
description: Generate a QR code that any UPI app can scan to pay a set amount to your UPI ID, or a upi://pay link for a button on mobile.
apis: codes
published: 2026-10-14
---

A UPI QR code is a `upi://pay` link drawn as a QR code. Any UPI app (Google Pay, PhonePe, Paytm, BHIM and the banks' own apps) can scan it and fill in the payee, the amount and a note.

## The QR code

The endpoint returns the image, so it goes straight into an `<img>` tag:

```html
<img
  src="https://api.dharun.dev/v1/upi-qr?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250&tn=Order%2042"
  alt="Scan to pay ₹250 to Sri Murugan Stores" width="256" height="256">
```

| Parameter | Meaning |
|---|---|
| `pa` | Your UPI ID (required) |
| `pn` | The name the payer's app shows |
| `am` | The amount in rupees; leave it out to let the payer type it |
| `tn` | A note, such as an order number |
| `tr` | Your own reference for the payment |

Add `format=png&size=512` for a PNG, which most printing tools prefer.

## A pay button on mobile

On a phone, scanning your own screen is impossible, so offer the link itself. `format=json` returns it:

```sh
curl "https://api.dharun.dev/v1/upi-qr?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250&tn=Order%2042&format=json"
```

```json
{
  "data": {
    "link": "upi://pay?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250.00&cu=INR&tn=Order%2042",
    "payeeAddress": "shop@okaxis",
    "payeeName": "Sri Murugan Stores",
    "amount": "250.00"
  }
}
```

```html
<a href="upi://pay?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250.00&cu=INR&tn=Order%2042">Pay ₹250 with UPI</a>
```

## Good to know

- Nothing is checked with a bank. A mistyped UPI ID still makes a QR code; the payer's app finds out when it tries to pay. Test with a small amount first.
- A QR code does not tell you that someone paid. Confirm payments in your bank or payment gateway, using `tr` or `tn` to match them.
- Payment details you send are never logged, and the images are marked so shared caches do not keep them.
