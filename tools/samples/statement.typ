// Sample bank statement for /v1/doc/statement and /v1/doc/redact: a made-up account
// holder, account, PAN, phone and email. Not a real bank's design. MIT-0.
#set page(paper: "a4", margin: 18mm)
#set text(font: "Noto Sans", size: 9pt)
#text(13pt, weight: "bold")[Sample Bank] #h(1fr) #text(fill: rgb("#a01818"), weight: "bold")[SAMPLE STATEMENT · NOT A REAL DOCUMENT]

#grid(columns: (1fr, 1fr), gutter: 4pt,
  [Account holder: Asha Sample \ Email: asha.sample\@example.com \ Mobile: 9876543210 \ PAN: ABCPS1234D],
  [Account number: 50100123456789 \ IFSC: HDFC0000001 \ Statement from 01/10/2026 to 31/10/2026],
)
#v(6pt)
#table(columns: (auto, 1fr, auto, auto, auto, auto), stroke: none, inset: (x: 4pt, y: 3pt),
  align: (left, left, left, right, right, right),
  table.header([*Date*], [*Narration*], [*Chq./Ref.No.*], [*Withdrawal Amt.*], [*Deposit Amt.*], [*Closing Balance*]),
  table.hline(),
  [01/10/26], [Opening balance], [], [], [], [25,000.00],
  [02/10/26], [UPI-SRI MURUGAN STORES-srimurugan\@okaxis], [627512345678], [1,250.00], [], [23,750.00],
  [05/10/26], [NEFT CR-KAVERI TRADERS-SALARY OCT 2026], [N278261234567], [], [45,000.00], [68,750.00],
  [07/10/26], [ATM WDL-MG ROAD BENGALURU], [000781], [5,000.00], [], [63,750.00],
  [12/10/26], [ACH D-SAMPLE INSURANCE PREMIUM \ POLICY 12345 OCTOBER], [ACH00123], [2,400.50], [], [61,349.50],
  [20/10/26], [INTEREST PAID], [], [], [312.00], [61,661.50],
  [28/10/26], [CHQ PAID-000123-RENT], [000123], [18,000.00], [], [43,661.50],
)
