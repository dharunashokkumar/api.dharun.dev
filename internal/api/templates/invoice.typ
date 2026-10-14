// GST tax invoice. The server works out every amount and writes it to invoice.json;
// this file only lays it out. MIT-0, like the guides: copy it freely.
#let d = json("invoice.json")
#let ink = rgb("#1d1d1f")
#let muted = rgb("#6e6e73")
#let rule = rgb("#d2d2d7")
#let shade = rgb("#f5f5f7")

#set document(title: d.title + " " + d.number, author: d.seller.name)
#set page(paper: "a4", margin: (x: 14mm, top: 14mm, bottom: 16mm),
  footer: context [
    #set text(7.5pt, fill: muted)
    #d.title #d.number #h(1fr) Page #counter(page).display() of #counter(page).final().first()
  ])
#set text(font: ("Noto Sans", "Noto Sans Tamil", "Noto Sans Devanagari", "Noto Sans Telugu",
  "Noto Sans Kannada", "Noto Sans Malayalam", "Noto Sans Bengali", "Noto Sans Gujarati",
  "Noto Sans Gurmukhi", "Noto Sans Oriya"), size: 8.5pt, fill: ink, lang: "en", region: "IN")
#set par(leading: 0.5em)

#let label(body) = text(7pt, fill: muted, weight: "bold", upper(body))
#let field(name, value) = if value != none and value != "" [#text(fill: muted)[#name] #value \ ]
#let party(title, p) = block(width: 100%)[
  #label(title) \
  #text(10pt, weight: "bold")[#p.name] \
  #if p.address != "" [#p.address \ ]
  #field("GSTIN", p.gstin)
  #field("State", p.state)
  #field("Phone", p.phone)
  #field("Email", p.email)
]

// Heading: seller on the left, the invoice's own details on the right.
#grid(columns: (1fr, auto), gutter: 8mm,
  [
    #text(15pt, weight: "bold")[#d.seller.name] \
    #if d.seller.address != "" [#d.seller.address \ ]
    #field("GSTIN", d.seller.gstin)
    #field("State", d.seller.state)
    #field("Phone", d.seller.phone)
    #field("Email", d.seller.email)
  ],
  align(right)[
    #text(13pt, weight: "bold", tracking: 0.5pt)[#upper(d.title)] \
    #v(1mm)
    #table(columns: 2, stroke: none, inset: (x: 0pt, y: 1.6pt), column-gutter: 4mm, align: (right, left),
      text(fill: muted)[Invoice no.], strong(d.number),
      text(fill: muted)[Date], d.date,
      ..if d.due_date != "" { (text(fill: muted)[Due date], d.due_date) },
      text(fill: muted)[Place of supply], d.place_of_supply,
      text(fill: muted)[Reverse charge], d.reverse_charge,
    )
  ],
)

#v(3mm)
#line(length: 100%, stroke: 0.6pt + rule)
#v(2mm)
#grid(columns: if d.ship_to != none { (1fr, 1fr) } else { (1fr,) }, gutter: 8mm,
  party("Bill to", d.buyer),
  ..if d.ship_to != none { (party("Ship to", d.ship_to),) },
)
#v(3mm)

// Lines.
#let head(body) = text(7pt, weight: "bold", fill: muted, upper(body))
#let columns = if d.any_discount {
  (auto, 1fr, auto, auto, auto, auto, auto, auto, auto, auto)
} else {
  (auto, 1fr, auto, auto, auto, auto, auto, auto, auto)
}
#table(
  columns: columns,
  stroke: (x, y) => if y == 0 { (bottom: 0.6pt + ink) } else { (bottom: 0.4pt + rule) },
  inset: (x: 3pt, y: 4pt),
  align: (x, y) => if x == 1 { left } else if x == 2 { center } else { right },
  fill: (x, y) => if y == 0 { shade },
  table.header(
    head[\#], head[Item], head[HSN/SAC], head[Qty], head[Rate],
    ..if d.any_discount { (head[Discount],) },
    head[Taxable], head[GST], head[Tax], head[Amount],
  ),
  ..d.lines.map(l => (
    l.number, l.description, l.hsn, l.quantity, l.rate,
    ..if d.any_discount { (if l.has_discount { l.discount } else { [—] },) },
    l.taxable, l.gst_rate, l.tax, strong(l.total),
  )).flatten(),
)

#v(2mm)
// Totals on the right; amount in words on the left.
#grid(columns: (1fr, 62mm), gutter: 8mm,
  [
    #label("Amount in words") \
    #text(9pt)[#d.words]
  ],
  table(columns: (1fr, auto), stroke: none, inset: (x: 0pt, y: 2.2pt), align: (left, right),
    ..d.totals.map(t => (text(fill: muted, t.label), t.amount)).flatten(),
    table.hline(stroke: 0.6pt + ink),
    text(11pt, weight: "bold")[Total (₹)], text(11pt, weight: "bold")[#d.total],
  ),
)

#v(4mm)
// The HSN-wise tax summary the GST rules ask for.
#block(breakable: false)[
#label("Tax summary by HSN/SAC")
#v(1mm)
#if d.intra {
  table(
    columns: (1fr, auto, auto, auto, auto, auto, auto),
    stroke: (x, y) => if y <= 1 { (bottom: 0.4pt + rule) } else { (bottom: 0.4pt + rule) },
    inset: (x: 3pt, y: 3.5pt), align: (x, y) => if x == 0 { left } else { right },
    fill: (x, y) => if y <= 1 { shade },
    table.header(
      table.cell(rowspan: 2, head[HSN/SAC]), table.cell(rowspan: 2, head[Taxable]),
      table.cell(colspan: 2, align: center, head[CGST]), table.cell(colspan: 2, align: center, head[#d.state_tax]),
      table.cell(rowspan: 2, head[Total tax]),
      head[Rate], head[Amount], head[Rate], head[Amount],
    ),
    ..d.summary.map(s => (if s.hsn == "" { [—] } else { s.hsn }, s.taxable, s.half_rate, s.cgst, s.half_rate, s.sgst, s.tax)).flatten(),
    strong[Total], strong(d.taxable_total), [], strong(d.cgst_total), [], strong(d.sgst_total), strong(d.tax_total),
  )
} else {
  table(
    columns: (1fr, auto, auto, auto, auto),
    stroke: (x, y) => (bottom: 0.4pt + rule),
    inset: (x: 3pt, y: 3.5pt), align: (x, y) => if x == 0 { left } else { right },
    fill: (x, y) => if y == 0 { shade },
    table.header(head[HSN/SAC], head[Taxable], head[IGST rate], head[IGST], head[Total tax]),
    ..d.summary.map(s => (if s.hsn == "" { [—] } else { s.hsn }, s.taxable, s.rate, s.igst, s.tax)).flatten(),
    strong[Total], strong(d.taxable_total), [], strong(d.igst_total), strong(d.tax_total),
  )
}
]

#v(5mm)
// Payment, notes and the signature.
#block(breakable: false)[
#grid(columns: (1fr, 1fr), gutter: 10mm,
  [
    #if d.bank != none [
      #label("Bank details") \
      #field("Account name", d.bank.holder)
      #field("Bank", d.bank.name)
      #field("Account no.", d.bank.account)
      #field("IFSC", d.bank.ifsc)
      #field("Branch", d.bank.branch)
      #v(2mm)
    ]
    #if d.upi != "" [
      #grid(columns: (24mm, 1fr), gutter: 3mm, image("upi.svg", width: 24mm),
        align(horizon)[#label("Pay by UPI") \ #d.upi \ #text(fill: muted)[Scan to pay ₹#d.total]])
      #v(2mm)
    ]
    #if d.notes != "" [#label("Notes") \ #d.notes #v(2mm)]
    #if d.terms != "" [#label("Terms") \ #text(fill: muted)[#d.terms]]
  ],
  align(right)[
    #text(fill: muted)[For] #strong(d.seller.name)
    #v(16mm)
    #line(length: 50mm, stroke: 0.4pt + muted)
    #text(fill: muted)[Authorised signatory]
  ],
)
]
