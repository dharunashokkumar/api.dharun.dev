# QR codes and barcodes API

> QR codes, UPI payment QR codes and barcodes as SVG or PNG, and reading them from a photo.

Generates codes, and reads them from pictures. What you encode or send is never logged or kept, and the answers are not cached by shared caches.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## QR code for any text

`GET /v1/qr`, also `POST`

Returns the image itself. Text up to about 2,900 bytes at error correction L.

POST body: The text to encode. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): Text to encode (or POST it).
- `ecc` (query): Error correction: how much damage the code survives. Default M. One of: L, M, Q, H.
- `format` (query): Image format. Default svg. One of: svg, png.
- `size` (query): 2D codes: width in pixels to stay within, 32 to 2048. Default 256.
- `margin` (query): Light border (quiet zone), in modules, 0 to 16. Default 4.
- `fg` (query): Dark colour, hex such as 000000. Default black.
- `bg` (query): Light colour, hex, or transparent. Default white.

Example: https://api.dharun.dev/v1/qr?text=https://api.dharun.dev&format=png&size=256&utm_medium=skill (returns the image itself)

## UPI payment QR code

`GET /v1/upi-qr`

A QR code any UPI app can scan to pay, built to NPCI's upi://pay link format. format=json returns the link instead of the image. Nothing is checked with a bank.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `pa` (query, required): Payee UPI ID, such as shop@okaxis.
- `pn` (query): Payee name shown in the app.
- `am` (query): Amount in rupees, up to two decimals. Leave out to let the payer type it.
- `tn` (query): Note shown with the payment, up to 80 characters.
- `tr` (query): Your reference for the transaction.
- `mc` (query): Merchant category code, four digits.
- `format` (query): Image format. Default svg. One of: svg, png.
- `size` (query): 2D codes: width in pixels to stay within, 32 to 2048. Default 256.
- `margin` (query): Light border (quiet zone), in modules, 0 to 16. Default 4.
- `fg` (query): Dark colour, hex such as 000000. Default black.
- `bg` (query): Light colour, hex, or transparent. Default white.

Example: https://api.dharun.dev/v1/upi-qr?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250&tn=Order%2042&utm_medium=skill (returns the image itself)

## Barcode

`GET /v1/barcode/{type}`, also `POST`

Linear barcodes (code128, code39, code93, ean13, ean8, itf, codabar) and 2D codes (datamatrix, pdf417, aztec). Linear codes are sized with scale and height, 2D codes with size. No human-readable text is printed under the bars.

POST body: The text to encode. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `type` (path, required): Kind of barcode. One of: code128, code39, code93, ean13, ean8, itf, codabar, datamatrix, pdf417, aztec.
- `text` (query): Text to encode (or POST it).
- `scale` (query): Linear codes: pixels per bar module, 1 to 10. Default 2.
- `height` (query): Linear codes: bar height in pixels, 10 to 600. Default 80.
- `format` (query): Image format. Default svg. One of: svg, png.
- `size` (query): 2D codes: width in pixels to stay within, 32 to 2048. Default 256.
- `margin` (query): Light border, in modules, 0 to 16. The default depends on the code.
- `fg` (query): Dark colour, hex such as 000000. Default black.
- `bg` (query): Light colour, hex, or transparent. Default white.

Example: https://api.dharun.dev/v1/barcode/ean13?text=890123456789&utm_medium=skill (returns the image itself)

## Read QR codes and barcodes from a picture

`POST /v1/barcode/read`

Finds every QR code in a photo or screenshot, and Data Matrix, Aztec, EAN, UPC, Code 128, Code 39, Code 93, ITF and Codabar codes. Not PDF417. Returns each code's kind, text and position. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/codes.png

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/barcode/read?utm_medium=skill

## Guides

- [Make a UPI payment QR code for an amount](https://api.dharun.dev/guides/upi-qr-code.md): Generate a QR code that any UPI app can scan to pay a set amount to your UPI ID, or a upi://pay link for a button on mobile.
