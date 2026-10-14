// Sample for barcode reading: a QR code and an EAN-13 barcode drawn by this API. MIT-0.
#set page(width: 760pt, height: 420pt, margin: 30pt, fill: white)
#set text(font: "Noto Sans", size: 14pt)
#grid(columns: (1fr, 1fr), gutter: 40pt, align: center + horizon,
  [#image("qr.png", width: 300pt) \ QR code],
  [#image("ean.png", width: 300pt) \ EAN-13],
)
