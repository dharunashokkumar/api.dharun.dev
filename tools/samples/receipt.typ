// Sample receipt for OCR: English and Tamil. MIT-0.
#set page(width: 9cm, height: auto, margin: 6mm, fill: rgb("#fbfaf6"))
#set text(font: ("Noto Sans", "Noto Sans Tamil"), size: 10pt)
#align(center)[
  #text(13pt, weight: "bold")[SRI MURUGAN STORES] \
  12 Car Street, Namakkal 637001 \
  GSTIN 33AABCS1429B1Z1
]
#line(length: 100%)
#table(columns: (1fr, auto), stroke: none, inset: (x: 0pt, y: 2pt), align: (left, right),
  [Basmati rice 5 kg], [575.00],
  [Toor dal 1 kg], [168.00],
  [Coconut oil 1 L], [245.00],
  [Filter coffee 500 g], [262.00],
)
#line(length: 100%)
#table(columns: (1fr, auto), stroke: none, inset: (x: 0pt, y: 2pt), align: (left, right),
  [*Total*], [*Rs. 1,250.00*],
  [Paid by UPI], [14-10-2026],
)
#v(2mm)
#align(center)[நன்றி, மீண்டும் வருக!]
