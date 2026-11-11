// Sample identity card for /v1/doc/mask-aadhaar: a made-up name and the well-known
// pattern number 9876 5432 1012 (it passes the Verhoeff check), with a QR code. Not a
// real document and not in any official design. MIT-0.
#set page(width: 86mm, height: 54mm, margin: 4mm, fill: rgb("#f4f7fb"))
#set text(font: "Noto Sans", size: 8pt)
#text(7pt, fill: rgb("#a01818"), weight: "bold")[SAMPLE CARD · NOT A REAL DOCUMENT]
#v(2mm)
#grid(columns: (1fr, 22mm), gutter: 3mm,
  [
    #text(10pt, weight: "bold")[Asha Sample] \
    Date of birth: 01/01/1990 \
    Female \
    #v(5mm)
    #text(13pt, weight: "bold")[9876 5432 1012]
  ],
  image("qr.svg", width: 22mm),
)
