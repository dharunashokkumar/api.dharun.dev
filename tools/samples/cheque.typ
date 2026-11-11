// Sample cheque for /v1/doc/cheque: made-up payee and account, the IFSC and MICR code
// of one real branch so the lookup has something to find. Not a real bank's design and
// not valid. The MICR line is set in an ordinary monospaced face, its symbols as quotes
// and a colon, as OCR reads them. MIT-0.
#set page(width: 203mm, height: 92mm, margin: 6mm, fill: rgb("#eef4f1"))
#set text(font: "Noto Sans", size: 10pt)
#grid(columns: (1fr, auto),
  [#text(13pt, weight: "bold")[Sample Bank] \ Nariman Point branch, Mumbai \ IFSC: HDFC0000001],
  [Date: 14102026],
)
#v(4mm)
#text(fill: rgb("#a01818"), weight: "bold")[SAMPLE CHEQUE · NOT VALID]
#v(3mm)
Pay: Kaveri Traders #h(1fr) ₹ 13,013.00 \
Rupees Thirteen thousand thirteen only \
A/c No. 50100123456789
#v(1fr)
#align(center)[#text(font: "Noto Sans Mono", size: 14pt)["000123" 400240003: 123456" 29]]
