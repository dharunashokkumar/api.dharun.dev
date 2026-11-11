// Package calc holds the money calculators behind /v1/calc: SIP and lump sum growth,
// loan EMIs, the GST split, income tax under both regimes, HRA, gratuity, PPF, capital
// gains and the charges on a share trade.
//
// The tax rules are typed from the Finance Acts, the Income-tax Act and the published
// circulars. Each answer names the rule and the year it used. They change every year:
// see Years and the yearly review in ROADMAP.md.
package calc

import (
	"errors"
	"math"
)

// round2 rounds to paise.
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// round0 rounds to whole rupees, as tax is (section 288B: to the nearest ten rupees for
// the total income; here the tax itself to the rupee).
func round0(v float64) float64 { return math.Round(v) }

var errNegative = errors.New("amounts cannot be negative")

// ---- investments ----

// SIPResult is the growth of a monthly investment.
type SIPResult struct {
	Invested float64    `json:"invested"`
	Value    float64    `json:"value"`
	Gain     float64    `json:"gain"`
	Years    []YearSIP  `json:"years"`
	Rule     string     `json:"rule"`
	Inputs   SIPRequest `json:"inputs"`
}

// YearSIP is one year of a SIP.
type YearSIP struct {
	Year     int     `json:"year"`
	Monthly  float64 `json:"monthly"`
	Invested float64 `json:"invested"`
	Value    float64 `json:"value"`
}

// SIPRequest describes a SIP: a monthly amount, an expected yearly return, a number of
// years and an optional yearly step-up of the amount.
type SIPRequest struct {
	Monthly float64 `json:"monthly"`
	Return  float64 `json:"returnPercent"`
	Years   int     `json:"years"`
	StepUp  float64 `json:"stepUpPercent"`
}

// SIP works month by month: each instalment is invested at the start of the month and
// grows at the monthly rate (the yearly rate divided by 12), as fund house calculators do.
func SIP(in SIPRequest) (SIPResult, error) {
	if in.Monthly <= 0 || in.Years < 1 || in.Years > 60 || in.Return < -50 || in.Return > 100 || in.StepUp < 0 || in.StepUp > 100 {
		return SIPResult{}, errors.New("monthly must be above 0, years 1 to 60, return -50 to 100 and step-up 0 to 100 (percent)")
	}
	rate := in.Return / 12 / 100
	value, invested, monthly := 0.0, 0.0, in.Monthly
	out := SIPResult{Inputs: in, Rule: "Each instalment is invested at the start of the month and grows at the yearly return divided by 12, compounded monthly."}
	for year := 1; year <= in.Years; year++ {
		for range 12 {
			value = (value + monthly) * (1 + rate)
			invested += monthly
		}
		out.Years = append(out.Years, YearSIP{Year: year, Monthly: round2(monthly), Invested: round2(invested), Value: round2(value)})
		monthly *= 1 + in.StepUp/100
	}
	out.Invested, out.Value = round2(invested), round2(value)
	out.Gain = round2(value - invested)
	return out, nil
}

// LumpSumResult is the growth of a one-time investment.
type LumpSumResult struct {
	Invested float64 `json:"invested"`
	Value    float64 `json:"value"`
	Gain     float64 `json:"gain"`
	Rule     string  `json:"rule"`
}

// LumpSum compounds once a year.
func LumpSum(amount, returnPercent float64, years int) (LumpSumResult, error) {
	if amount <= 0 || years < 1 || years > 60 || returnPercent < -50 || returnPercent > 100 {
		return LumpSumResult{}, errors.New("amount must be above 0, years 1 to 60 and return -50 to 100 (percent)")
	}
	value := amount * math.Pow(1+returnPercent/100, float64(years))
	return LumpSumResult{Invested: round2(amount), Value: round2(value), Gain: round2(value - amount), Rule: "Compounded once a year."}, nil
}

// ---- loans ----

// EMIResult is a loan's monthly instalment and, if asked, its schedule.
type EMIResult struct {
	EMI           float64  `json:"emi"`
	TotalInterest float64  `json:"totalInterest"`
	TotalPaid     float64  `json:"totalPaid"`
	Schedule      []EMIRow `json:"schedule,omitempty"`
	Rule          string   `json:"rule"`
}

// EMIRow is one month of a loan.
type EMIRow struct {
	Month     int     `json:"month"`
	Interest  float64 `json:"interest"`
	Principal float64 `json:"principal"`
	Balance   float64 `json:"balance"`
}

// EMI is the reducing-balance instalment: P·i·(1+i)^n / ((1+i)^n − 1), i the monthly rate.
func EMI(principal, ratePercent float64, months int, schedule bool) (EMIResult, error) {
	if principal <= 0 || months < 1 || months > 600 || ratePercent < 0 || ratePercent > 60 {
		return EMIResult{}, errors.New("principal must be above 0, months 1 to 600 and rate 0 to 60 (percent a year)")
	}
	i := ratePercent / 12 / 100
	var emi float64
	if i == 0 {
		emi = principal / float64(months)
	} else {
		grow := math.Pow(1+i, float64(months))
		emi = principal * i * grow / (grow - 1)
	}
	out := EMIResult{EMI: round2(emi), TotalPaid: round2(emi * float64(months)), Rule: "Reducing balance, interest charged monthly at the yearly rate divided by 12."}
	out.TotalInterest = round2(emi*float64(months) - principal)
	if schedule {
		balance := principal
		for month := 1; month <= months; month++ {
			interest := balance * i
			part := emi - interest
			balance -= part
			if month == months {
				balance = 0
			}
			out.Schedule = append(out.Schedule, EMIRow{Month: month, Interest: round2(interest), Principal: round2(part), Balance: round2(math.Max(balance, 0))})
		}
	}
	return out, nil
}

// ---- GST ----

// GSTResult splits an amount into its taxable value and tax.
type GSTResult struct {
	Taxable    float64 `json:"taxable"`
	Rate       float64 `json:"rate"`
	Tax        float64 `json:"tax"`
	CGST       float64 `json:"cgst,omitempty"`
	SGST       float64 `json:"sgst,omitempty"`
	IGST       float64 `json:"igst,omitempty"`
	Total      float64 `json:"total"`
	Interstate bool    `json:"interstate"`
	Rule       string  `json:"rule"`
}

// GSTRates are the rates in force from 22 September 2025 (the 56th GST Council meeting):
// the main slabs 5, 18 and 40 percent, with 0, 0.25, 1.5 and 3 for special goods. Older
// invoices may carry 12 or 28, which are still accepted.
var GSTRates = []float64{0, 0.1, 0.25, 1.5, 3, 5, 12, 18, 28, 40}

// GST splits amount at rate. inclusive means the amount already holds the tax. Within
// one state the tax is CGST and SGST in halves; between states it is IGST.
func GST(amount, rate float64, inclusive, interstate bool) (GSTResult, error) {
	if amount < 0 {
		return GSTResult{}, errNegative
	}
	known := false
	for _, r := range GSTRates {
		if r == rate {
			known = true
		}
	}
	if !known {
		return GSTResult{}, errors.New("rate must be a GST rate: 0, 0.1, 0.25, 1.5, 3, 5, 12, 18, 28 or 40")
	}
	taxable := amount
	if inclusive {
		taxable = amount * 100 / (100 + rate)
	}
	tax := taxable * rate / 100
	out := GSTResult{Taxable: round2(taxable), Rate: rate, Tax: round2(tax), Total: round2(taxable + tax), Interstate: interstate,
		Rule: "Within a state the tax is charged as CGST and SGST, half each; between states, or on imports, as IGST."}
	if interstate {
		out.IGST = round2(tax)
	} else {
		out.CGST, out.SGST = round2(tax/2), round2(tax/2)
	}
	return out, nil
}
