# GST invoice API

> A GST tax invoice as a PDF from JSON, with the tax worked out.

Send the seller, the buyer and the items with their HSN codes and GST rates. The API works out each line's taxable value, CGST and SGST (UTGST in union territories without a legislature) for a sale within the state, or IGST across states, from the GSTINs or the place of supply; adds the HSN-wise tax summary, rounding and the amount in words; and sets it on an A4 page with an optional UPI QR code for the exact amount. Nothing you send is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Make a GST tax invoice

`POST /v1/invoice`

Fields: number (required, up to 16 letters, digits, - and /), date and due_date (YYYY-MM-DD, date defaults to today), title (default Tax Invoice), place_of_supply (state code, name or GST code; defaults to the buyer's state from the GSTIN or state), reverse_charge, seller and buyer (name, address, gstin, state, phone, email; the seller's GSTIN is required), ship_to, items (description, hsn, quantity (default 1), unit (default Nos), rate before tax, discount as an amount, gst_rate in percent), bank (name, account, ifsc, branch, holder), upi, notes and terms. Unknown fields are refused, so a misspelt field is caught. format=json returns the worked-out invoice instead of a document; format=png the first page as a picture.

Send: The invoice as JSON, up to 256 KB.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `format` (query): pdf (default), png (the first page) or json (the amounts only). One of: pdf, png, json.

Example: `POST` https://api.dharun.dev/v1/invoice?utm_medium=skill (returns the PDF itself)

Example body (invoice.json):

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
