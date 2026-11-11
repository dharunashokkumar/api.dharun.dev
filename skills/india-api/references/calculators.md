# Money calculators API

> SIP, EMI, GST, income tax, HRA, gratuity, PPF, capital gains and trade charges.

Indian money sums with the rules written in: each answer names the rule and the year it used. Income tax knows the financial years 2026-27 and 2025-26, both regimes. For planning: not tax or financial advice. Amounts are in rupees; commas are allowed.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## SIP returns

`GET /v1/calc/sip`

What a monthly investment grows to, year by year, with an optional yearly step-up of the amount.

Parameters:

- `monthly` (query, required): Monthly amount.
- `rate` (query, required): Expected return, percent a year.
- `years` (query, required): 1 to 60.
- `stepup` (query): Yearly increase of the monthly amount, percent.

Example: https://api.dharun.dev/v1/calc/sip?monthly=10000&rate=12&years=10&utm_medium=skill

## Lump sum growth

`GET /v1/calc/lumpsum`

Parameters:

- `amount` (query, required): Amount invested once.
- `rate` (query, required): Expected return, percent a year.
- `years` (query, required): 1 to 60.

Example: https://api.dharun.dev/v1/calc/lumpsum?amount=100000&rate=10&years=10&utm_medium=skill

## Loan EMI and schedule

`GET /v1/calc/emi`

The monthly instalment of a reducing-balance loan, the total interest and, with schedule=true, every month's split of interest and principal.

Parameters:

- `amount` (query, required): Loan amount.
- `rate` (query, required): Interest, percent a year.
- `years` (query): Term in years (or give months).
- `months` (query): Term in months, 1 to 600.
- `schedule` (query): true adds the month-by-month schedule.

Example: https://api.dharun.dev/v1/calc/emi?amount=1000000&rate=8.5&years=20&utm_medium=skill

## GST split

`GET /v1/calc/gst`

Adds GST to an amount, or takes it out of one that includes it, split into CGST and SGST within a state or IGST between states.

Parameters:

- `amount` (query, required): The amount.
- `rate` (query, required): GST rate: 0, 0.1, 0.25, 1.5, 3, 5, 12, 18, 28 or 40.
- `inclusive` (query): true when the amount already includes GST.
- `interstate` (query): true for a supply between states (IGST).

Example: https://api.dharun.dev/v1/calc/gst?amount=1180&rate=18&inclusive=true&utm_medium=skill

## Income tax, old and new regime

`GET /v1/calc/income-tax`

Tax on a year's income under the new regime (section 115BAC) and the old one, with the section 87A rebate, surcharge, marginal relief and 4% cess, slab by slab, and which regime costs less. Special-rate income such as capital gains is not included.

Parameters:

- `income` (query, required): Gross total income for the year, before the standard deduction.
- `year` (query): Financial year. Default the newest. One of: 2026-27, 2025-26.
- `regime` (query): Default both. One of: both, new, old.
- `age` (query): Changes the old regime's exemption limit. Default below60. One of: below60, 60to80, 80plus.
- `salaried` (query): Salary or pension, so the standard deduction applies. Default true.
- `deductions` (query): Old regime deductions and exemptions in all (80C, 80D, HRA, home loan interest...).

Example: https://api.dharun.dev/v1/calc/income-tax?income=1500000&year=2025-26&utm_medium=skill

## HRA exemption

`GET /v1/calc/hra`

The tax-free part of house rent allowance under section 10(13A). Give every amount for the same period (a month or a year).

Parameters:

- `basic` (query, required): Basic pay plus dearness allowance.
- `hra` (query, required): HRA received.
- `rent` (query, required): Rent paid.
- `metro` (query): true in Delhi, Mumbai, Kolkata or Chennai.

Example: https://api.dharun.dev/v1/calc/hra?basic=50000&hra=25000&rent=20000&metro=true&utm_medium=skill

## Gratuity

`GET /v1/calc/gratuity`

Parameters:

- `salary` (query, required): Last monthly basic pay plus dearness allowance.
- `years` (query, required): Years of service; 10.6 is ten years and about seven months.
- `covered` (query): Covered by the Payment of Gratuity Act. Default true.

Example: https://api.dharun.dev/v1/calc/gratuity?salary=52000&years=10.6&utm_medium=skill

## PPF maturity

`GET /v1/calc/ppf`

Parameters:

- `yearly` (query, required): Yearly deposit, 500 to 1,50,000.
- `years` (query): 15, or 15 plus blocks of 5. Default 15.
- `rate` (query): Interest, percent a year. Default the current rate.

Example: https://api.dharun.dev/v1/calc/ppf?yearly=150000&years=15&utm_medium=skill

## Capital gains tax

`GET /v1/calc/capital-gains`

Short or long term, and the tax at the rates for sales from 23 July 2024: shares and equity funds, debt funds, property (with the indexation choice for land and buildings bought before that date), gold and other assets.

Parameters:

- `asset` (query, required): What was sold. One of: equity, equity-fund, debt-fund, property, gold, other.
- `bought` (query, required): Purchase date, YYYY-MM-DD.
- `sold` (query, required): Sale date, from 2024-07-23.
- `cost` (query, required): Purchase cost with improvement costs.
- `price` (query, required): Sale price.
- `expenses` (query): Costs of the sale.

Example: https://api.dharun.dev/v1/calc/capital-gains?asset=equity&bought=2023-01-10&sold=2025-01-10&cost=100000&price=400000&utm_medium=skill

## Share trade charges

`GET /v1/calc/trade`

Every charge on buying and selling an NSE share (STT, exchange charges, SEBI fee, stamp duty, GST), the net profit and the move needed to break even.

Parameters:

- `buy` (query, required): Buy price per share.
- `sell` (query, required): Sell price per share.
- `quantity` (query, required): Shares.
- `kind` (query): Default delivery. One of: delivery, intraday.
- `brokerage` (query): Your broker's charge per order, in rupees.
- `dp` (query): Depository charge on a delivery sale.

Example: https://api.dharun.dev/v1/calc/trade?buy=1000&sell=1100&quantity=100&kind=delivery&brokerage=20&utm_medium=skill
