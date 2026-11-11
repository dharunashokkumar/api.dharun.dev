# UPI handles and payment QR codes API

> Which app and bank is behind @okaxis, and what a UPI or Bharat QR code says.

The handle after the @ in a UPI ID names the app and the partner bank that issued it. The QR reader takes the text inside a payment QR code (a upi://pay link or an EMVCo string, which Bharat QR uses), splits it into payee, amount, merchant category, city and reference, and checks its checksum.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Known UPI handles

`GET /v1/upi-handles`

Example: https://api.dharun.dev/v1/upi-handles?utm_medium=skill

## Look up a UPI handle

`GET /v1/upi-handles/{handle}`

Give the handle (okhdfcbank) or a whole UPI ID; only the part after the @ is read.

Parameters:

- `handle` (path, required): The handle, with or without the name and @ before it.

Example: https://api.dharun.dev/v1/upi-handles/okhdfcbank?utm_medium=skill

## Read a UPI or Bharat QR code's text

`GET /v1/qr/parse`, also `POST`

Splits the text of a payment QR code into its fields and checks the EMVCo checksum. To get the text from a picture, POST the picture to /v1/barcode/read first.

POST body: The QR code's text, as text/plain or JSON {"text": "..."}. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): The QR code's text.

Example: https://api.dharun.dev/v1/qr/parse?text=upi%3A%2F%2Fpay%3Fpa%3Dshop%40okaxis%26pn%3DSri%2520Murugan%2520Stores%26am%3D250%26cu%3DINR%26mc%3D5411&utm_medium=skill
