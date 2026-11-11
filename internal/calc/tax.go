package calc

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"time"
)

// ---- income tax ----

type slab struct {
	upTo float64 // the top of the slab; math.Inf(1) for the last
	rate float64 // percent
}

type band struct {
	above float64 // income above which the surcharge applies
	rate  float64 // percent of the tax
}

type regimeRule struct {
	slabs      map[string][]slab // by age group; "all" when age does not matter
	standard   float64           // standard deduction for salary and pension
	rebateUpTo float64           // section 87A: taxable income up to this
	rebateMax  float64           // and a rebate of at most this
	marginal   bool              // relief just above the rebate limit (new regime)
	surcharge  []band
	deductions bool // chapter VI-A deductions (80C, 80D, ...) are allowed
}

var inf = math.Inf(1)

// newRegime2025 is section 115BAC as amended by the Finance Act 2025, for the
// financial year 2025-26 (assessment year 2026-27).
var newRegime2025 = regimeRule{
	slabs:    map[string][]slab{"all": {{400000, 0}, {800000, 5}, {1200000, 10}, {1600000, 15}, {2000000, 20}, {2400000, 25}, {inf, 30}}},
	standard: 75000, rebateUpTo: 1200000, rebateMax: 60000, marginal: true,
	surcharge: []band{{5000000, 10}, {10000000, 15}, {20000000, 25}},
}

// oldRegime is the regime of section 2 of the Finance Act, unchanged since 2017-18
// apart from the standard deduction (50,000 since 2019-20).
var oldRegime = regimeRule{
	slabs: map[string][]slab{
		"below60": {{250000, 0}, {500000, 5}, {1000000, 20}, {inf, 30}},
		"60to80":  {{300000, 0}, {500000, 5}, {1000000, 20}, {inf, 30}},
		"80plus":  {{500000, 0}, {1000000, 20}, {inf, 30}},
	},
	standard: 50000, rebateUpTo: 500000, rebateMax: 12500, deductions: true,
	surcharge: []band{{5000000, 10}, {10000000, 15}, {20000000, 25}, {50000000, 37}},
}

// taxYears are the years the calculator knows. The Finance Act 2026 kept the 2025-26
// slabs, rebate and standard deduction for 2026-27, the first year under the
// Income-tax Act 2025.
var taxYears = map[string]map[string]regimeRule{
	"2025-26": {"new": newRegime2025, "old": oldRegime},
	"2026-27": {"new": newRegime2025, "old": oldRegime},
}

// TaxYears lists the financial years the income tax calculator knows, newest first.
func TaxYears() []string {
	var years []string
	for year := range taxYears {
		years = append(years, year)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(years)))
	return years
}

// IncomeTaxRequest describes a person's income for one year.
type IncomeTaxRequest struct {
	Income     float64 `json:"income"`     // gross total income, before the standard deduction
	Year       string  `json:"year"`       // financial year, such as 2025-26
	Age        string  `json:"age"`        // below60, 60to80, 80plus
	Salaried   bool    `json:"salaried"`   // salary or pension: the standard deduction applies
	Deductions float64 `json:"deductions"` // chapter VI-A deductions and exemptions, old regime only
}

// IncomeTax is the tax under one regime.
type IncomeTax struct {
	Regime         string    `json:"regime"`
	Taxable        float64   `json:"taxableIncome"`
	Standard       float64   `json:"standardDeduction"`
	Deductions     float64   `json:"deductions"`
	Slabs          []SlabTax `json:"slabs"`
	TaxOnIncome    float64   `json:"taxOnIncome"`
	Rebate         float64   `json:"rebate87A"`
	Surcharge      float64   `json:"surcharge"`
	Cess           float64   `json:"cess"`
	Total          float64   `json:"total"`
	Monthly        float64   `json:"perMonth"`
	EffectiveRate  float64   `json:"effectiveRatePercent"`
	MarginalRelief bool      `json:"marginalRelief"`
}

// SlabTax is the tax on the part of income inside one slab.
type SlabTax struct {
	From   float64 `json:"from"`
	To     float64 `json:"to,omitempty"` // zero for the open top slab
	Rate   float64 `json:"rate"`
	Amount float64 `json:"amount"`
	Tax    float64 `json:"tax"`
}

// IncomeTaxResult compares the regimes.
type IncomeTaxResult struct {
	Year   string           `json:"year"`
	New    *IncomeTax       `json:"new,omitempty"`
	Old    *IncomeTax       `json:"old,omitempty"`
	Better string           `json:"better,omitempty"`
	Saves  float64          `json:"saves,omitempty"`
	Rule   string           `json:"rule"`
	Inputs IncomeTaxRequest `json:"inputs"`
}

// IncomeTaxFor works out the tax under the regimes asked for ("new", "old" or "both").
func IncomeTaxFor(in IncomeTaxRequest, regime string) (IncomeTaxResult, error) {
	if in.Income < 0 || in.Deductions < 0 {
		return IncomeTaxResult{}, errNegative
	}
	if in.Income > 1e12 {
		return IncomeTaxResult{}, errors.New("income is too large")
	}
	if in.Year == "" {
		in.Year = TaxYears()[0]
	}
	rules, ok := taxYears[in.Year]
	if !ok {
		return IncomeTaxResult{}, errors.New("year must be one of: " + joinStrings(TaxYears()))
	}
	if in.Age == "" {
		in.Age = "below60"
	}
	if in.Age != "below60" && in.Age != "60to80" && in.Age != "80plus" {
		return IncomeTaxResult{}, errors.New("age must be below60, 60to80 or 80plus")
	}
	out := IncomeTaxResult{Year: in.Year, Inputs: in,
		Rule: "Section 115BAC (new regime) and the slabs of the Finance Act (old regime) for the financial year " + in.Year +
			"; rebate under section 87A with marginal relief; surcharge with marginal relief; 4% health and education cess. Special-rate income (capital gains, lottery) is not included."}
	switch regime {
	case "", "both":
		n, o := computeTax(rules["new"], in, "new"), computeTax(rules["old"], in, "old")
		out.New, out.Old = &n, &o
		if n.Total <= o.Total {
			out.Better, out.Saves = "new", o.Total-n.Total
		} else {
			out.Better, out.Saves = "old", n.Total-o.Total
		}
	case "new":
		n := computeTax(rules["new"], in, "new")
		out.New = &n
	case "old":
		o := computeTax(rules["old"], in, "old")
		out.Old = &o
	default:
		return IncomeTaxResult{}, errors.New("regime must be new, old or both")
	}
	return out, nil
}

func slabsFor(rule regimeRule, age string) []slab {
	if slabs, ok := rule.slabs["all"]; ok {
		return slabs
	}
	return rule.slabs[age]
}

func onSlabs(income float64, slabs []slab) (float64, []SlabTax) {
	tax, from := 0.0, 0.0
	var rows []SlabTax
	for _, s := range slabs {
		if income <= from {
			break
		}
		part := math.Min(income, s.upTo) - from
		t := part * s.rate / 100
		row := SlabTax{From: from, Rate: s.rate, Amount: round2(part), Tax: round2(t)}
		if !math.IsInf(s.upTo, 1) {
			row.To = s.upTo
		}
		rows = append(rows, row)
		tax += t
		from = s.upTo
	}
	return tax, rows
}

func computeTax(rule regimeRule, in IncomeTaxRequest, name string) IncomeTax {
	out := IncomeTax{Regime: name}
	if in.Salaried {
		out.Standard = math.Min(rule.standard, in.Income)
	}
	if rule.deductions {
		out.Deductions = math.Min(in.Deductions, in.Income-out.Standard)
	}
	taxable := math.Max(0, in.Income-out.Standard-out.Deductions)
	// Total income is rounded to the nearest ten rupees (section 288A).
	taxable = math.Round(taxable/10) * 10
	out.Taxable = taxable
	slabs := slabsFor(rule, in.Age)
	tax, rows := onSlabs(taxable, slabs)
	out.Slabs, out.TaxOnIncome = rows, round0(tax)
	rebate := 0.0
	if taxable <= rule.rebateUpTo {
		rebate = math.Min(tax, rule.rebateMax)
	} else if rule.marginal && tax > taxable-rule.rebateUpTo {
		// Just above the limit, the tax may not be more than the income above it.
		rebate = tax - (taxable - rule.rebateUpTo)
		out.MarginalRelief = true
	}
	out.Rebate = round0(rebate)
	after := tax - rebate
	surcharge := 0.0
	for i := len(rule.surcharge) - 1; i >= 0; i-- {
		b := rule.surcharge[i]
		if taxable > b.above {
			surcharge = after * b.rate / 100
			// Marginal relief: tax and surcharge may rise by no more than the income
			// above the threshold.
			atThreshold, _ := onSlabs(b.above, slabs)
			if i > 0 {
				atThreshold *= 1 + rule.surcharge[i-1].rate/100
			}
			if limit := atThreshold + (taxable - b.above); after+surcharge > limit {
				surcharge = math.Max(0, limit-after)
				out.MarginalRelief = true
			}
			break
		}
	}
	out.Surcharge = round0(surcharge)
	out.Cess = round0((after + surcharge) * 4 / 100)
	out.Total = round0(after + surcharge + (after+surcharge)*4/100)
	out.Monthly = round2(out.Total / 12)
	if in.Income > 0 {
		out.EffectiveRate = round2(out.Total * 100 / in.Income)
	}
	return out
}

func joinStrings(list []string) string {
	out := ""
	for i, item := range list {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
}

// ---- HRA ----

// HRAResult is the exempt part of house rent allowance under section 10(13A).
type HRAResult struct {
	Exempt  float64    `json:"exempt"`
	Taxable float64    `json:"taxable"`
	Limits  [3]float64 `json:"limits"` // HRA received, rent less 10% of salary, 50% or 40% of salary
	Rule    string     `json:"rule"`
}

// HRA: the least of the HRA received, rent paid less 10% of salary, and 50% (metro) or
// 40% of salary. Salary is basic pay plus dearness allowance. Any period works, as long
// as every amount is for the same one.
func HRA(basic, hra, rent float64, metro bool) (HRAResult, error) {
	if basic < 0 || hra < 0 || rent < 0 {
		return HRAResult{}, errNegative
	}
	share := 40.0
	if metro {
		share = 50
	}
	limits := [3]float64{hra, math.Max(0, rent-basic/10), basic * share / 100}
	exempt := math.Min(limits[0], math.Min(limits[1], limits[2]))
	return HRAResult{Exempt: round2(exempt), Taxable: round2(hra - exempt), Limits: [3]float64{round2(limits[0]), round2(limits[1]), round2(limits[2])},
		Rule: "Section 10(13A) and rule 2A: the least of the HRA received, rent paid less 10% of salary, and 50% of salary in Delhi, Mumbai, Kolkata or Chennai (40% elsewhere). Old regime only; the new regime allows no HRA exemption."}, nil
}

// ---- gratuity ----

// GratuityResult is the gratuity due and its tax-free part.
type GratuityResult struct {
	Amount   float64 `json:"amount"`
	Years    int     `json:"yearsCounted"`
	Exempt   float64 `json:"taxFree"`
	Taxable  float64 `json:"taxable"`
	Eligible bool    `json:"eligible"`
	Rule     string  `json:"rule"`
}

// GratuityCap is the most that is tax free for an employee outside government (section
// 10(10)(ii) and (iii), 20 lakh since 29 March 2018).
const GratuityCap = 2000000

// Gratuity: under the Payment of Gratuity Act, 15 days' wages for each completed year,
// a part year over six months counting as a year, a month taken as 26 days; outside the
// Act, half a month's average salary for each completed year.
func Gratuity(monthly, years float64, covered bool) (GratuityResult, error) {
	if monthly < 0 || years < 0 || years > 60 {
		return GratuityResult{}, errors.New("salary must not be negative and years must be 0 to 60")
	}
	out := GratuityResult{Eligible: years >= 5}
	whole := math.Floor(years)
	if covered {
		if years-whole > 0.5 {
			whole++
		}
		out.Amount = round0(monthly * 15 / 26 * whole)
		out.Rule = "Payment of Gratuity Act, section 4: 15 days' wages (basic pay and dearness allowance, the month taken as 26 days) for each completed year of service; more than six months counts as a year. Due after five years of continuous service, except on death or disablement. Tax free up to 20 lakh."
	} else {
		out.Amount = round0(monthly / 2 * whole)
		out.Rule = "Not covered by the Act: half a month's average salary of the last ten months for each completed year (section 10(10)(iii)). Tax free up to 20 lakh."
	}
	out.Years = int(whole)
	out.Exempt = math.Min(out.Amount, GratuityCap)
	out.Taxable = out.Amount - out.Exempt
	return out, nil
}

// ---- PPF ----

// PPFResult is a Public Provident Fund account year by year.
type PPFResult struct {
	Deposited float64   `json:"deposited"`
	Interest  float64   `json:"interest"`
	Maturity  float64   `json:"maturity"`
	Years     []PPFYear `json:"years"`
	Rule      string    `json:"rule"`
}

// PPFYear is one year of an account.
type PPFYear struct {
	Year     int     `json:"year"`
	Interest float64 `json:"interest"`
	Balance  float64 `json:"balance"`
}

// PPFRate is the rate for October to December 2026 (unchanged at 7.1% since April 2020).
const PPFRate = 7.1

// PPF assumes the deposit is made by the fifth of April each year, so it earns the
// whole year's interest, compounded yearly.
func PPF(yearly float64, years int, rate float64) (PPFResult, error) {
	if yearly < 500 || yearly > 150000 {
		return PPFResult{}, errors.New("the yearly deposit must be 500 to 1,50,000 rupees")
	}
	if years < 15 || years > 50 || (years-15)%5 != 0 {
		return PPFResult{}, errors.New("years must be 15, or 15 plus blocks of 5 (20, 25, ...)")
	}
	if rate <= 0 {
		rate = PPFRate
	}
	balance, deposited := 0.0, 0.0
	out := PPFResult{Rule: "Deposits made by 5 April each year earn interest for the whole year, compounded yearly at " + strconv.FormatFloat(rate, 'f', -1, 64) + "%. The rate is set every quarter; " + strconv.FormatFloat(PPFRate, 'f', -1, 64) + "% is the current one."}
	for year := 1; year <= years; year++ {
		balance += yearly
		deposited += yearly
		interest := balance * rate / 100
		balance += interest
		out.Years = append(out.Years, PPFYear{Year: year, Interest: round0(interest), Balance: round0(balance)})
	}
	out.Deposited, out.Maturity = deposited, round0(balance)
	out.Interest = out.Maturity - deposited
	return out, nil
}

// ---- capital gains ----

// costInflationIndex is the index notified under section 48 (base year 2001-02), by the
// year a financial year starts.
var costInflationIndex = map[int]int{
	2001: 100, 2002: 105, 2003: 109, 2004: 113, 2005: 117, 2006: 122, 2007: 129, 2008: 137, 2009: 148, 2010: 167,
	2011: 184, 2012: 200, 2013: 220, 2014: 240, 2015: 254, 2016: 264, 2017: 272, 2018: 280, 2019: 289, 2020: 301,
	2021: 317, 2022: 331, 2023: 348, 2024: 363, 2025: 376,
}

// CapitalGainsRequest describes one sale.
type CapitalGainsRequest struct {
	Asset    string  `json:"asset"` // equity, equity-fund, debt-fund, property, gold, other
	Bought   string  `json:"bought"`
	Sold     string  `json:"sold"`
	Cost     float64 `json:"cost"`
	Price    float64 `json:"price"`
	Expenses float64 `json:"expenses"` // on the sale: brokerage, stamp duty paid by the seller, ...
}

// CapitalGainsResult is the gain and the tax on it.
type CapitalGainsResult struct {
	Term      string   `json:"term"` // short or long
	Gain      float64  `json:"gain"`
	Rate      *float64 `json:"ratePercent"` // nil when taxed at the slab rate
	Exemption float64  `json:"exemption"`
	Tax       float64  `json:"tax"`
	Cess      float64  `json:"cess"`
	Total     float64  `json:"total"`
	Indexed   *Indexed `json:"indexation,omitempty"`
	Rule      string   `json:"rule"`
}

// Indexed is the other way of taxing land and buildings bought before 23 July 2024.
type Indexed struct {
	IndexedCost float64 `json:"indexedCost"`
	Gain        float64 `json:"gain"`
	Tax         float64 `json:"tax"`
	Chosen      bool    `json:"chosen"`
}

var budget2024 = time.Date(2024, 7, 23, 0, 0, 0, 0, time.UTC)

func financialYear(t time.Time) int {
	if t.Month() >= time.April {
		return t.Year()
	}
	return t.Year() - 1
}

// CapitalGains uses the rates of the Finance (No. 2) Act 2024 for sales from 23 July
// 2024: listed shares and equity funds long term after 12 months at 12.5% above the
// yearly 1.25 lakh exemption, short term at 20%; other assets long term after 24 months
// at 12.5% without indexation (land and buildings bought before 23 July 2024 may use
// 20% with indexation instead, whichever is lower); debt funds bought from April 2023
// at the slab rate.
func CapitalGains(in CapitalGainsRequest) (CapitalGainsResult, error) {
	bought, err1 := time.Parse("2006-01-02", in.Bought)
	sold, err2 := time.Parse("2006-01-02", in.Sold)
	if err1 != nil || err2 != nil {
		return CapitalGainsResult{}, errors.New("bought and sold must be dates as YYYY-MM-DD")
	}
	if !sold.After(bought) {
		return CapitalGainsResult{}, errors.New("sold must be after bought")
	}
	if sold.Before(budget2024) {
		return CapitalGainsResult{}, errors.New("sales before 23 July 2024 were taxed at the older rates, which this calculator does not cover")
	}
	if in.Cost < 0 || in.Price < 0 || in.Expenses < 0 {
		return CapitalGainsResult{}, errNegative
	}
	gain := in.Price - in.Expenses - in.Cost
	out := CapitalGainsResult{Gain: round2(gain)}
	rate := func(v float64) *float64 { return &v }
	switch in.Asset {
	case "equity", "equity-fund":
		if sold.After(bought.AddDate(1, 0, 0)) {
			out.Term, out.Rate, out.Exemption = "long", rate(12.5), 125000
			out.Rule = "Section 112A: listed shares and equity funds held over 12 months, with securities transaction tax paid. 12.5% on gains above 1.25 lakh a year (the exemption is shared by all such sales in the year)."
		} else {
			out.Term, out.Rate = "short", rate(20)
			out.Rule = "Section 111A: listed shares and equity funds held 12 months or less, with securities transaction tax paid, at 20%."
		}
	case "debt-fund":
		if !bought.Before(time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC)) {
			out.Term = "short"
			out.Rule = "Section 50AA: units of debt funds bought from 1 April 2023 are taxed as short-term gains at your slab rate, however long they are held."
		} else if sold.After(bought.AddDate(2, 0, 0)) {
			out.Term, out.Rate = "long", rate(12.5)
			out.Rule = "Section 112: debt fund units bought before April 2023 and held over 24 months, at 12.5% without indexation."
		} else {
			out.Term = "short"
			out.Rule = "Held 24 months or less: taxed at your slab rate."
		}
	case "property", "gold", "other":
		if sold.After(bought.AddDate(2, 0, 0)) {
			out.Term, out.Rate = "long", rate(12.5)
			out.Rule = "Section 112: held over 24 months, at 12.5% without indexation."
			if in.Asset == "property" && bought.Before(budget2024) {
				buyIndex, okBuy := costInflationIndex[max(financialYear(bought), 2001)]
				sellIndex, okSell := costInflationIndex[financialYear(sold)]
				if okBuy && okSell {
					indexedCost := in.Cost * float64(sellIndex) / float64(buyIndex)
					indexedGain := in.Price - in.Expenses - indexedCost
					out.Indexed = &Indexed{IndexedCost: round2(indexedCost), Gain: round2(indexedGain), Tax: round0(math.Max(0, indexedGain) * 0.20)}
					out.Rule += " Land and buildings bought before 23 July 2024 by a resident individual or HUF may instead pay 20% on the gain after indexation (cost inflation index), whichever is less."
				} else {
					out.Rule += " The cost inflation index for the year of sale is not in this calculator yet, so the indexed choice is not shown."
				}
			}
		} else {
			out.Term = "short"
			out.Rule = "Held 24 months or less: added to your income and taxed at your slab rate."
		}
	default:
		return CapitalGainsResult{}, errors.New("asset must be equity, equity-fund, debt-fund, property, gold or other")
	}
	if out.Rate != nil {
		taxed := math.Max(0, gain-out.Exemption)
		out.Tax = round0(taxed * *out.Rate / 100)
		if out.Indexed != nil {
			if out.Indexed.Tax < out.Tax {
				out.Indexed.Chosen = true
				out.Tax = out.Indexed.Tax
			}
		}
		out.Cess = round0(out.Tax * 4 / 100)
		out.Total = out.Tax + out.Cess
	}
	return out, nil
}

// ---- trade charges ----

// TradeRequest describes one round trip in an NSE equity share.
type TradeRequest struct {
	Buy       float64 `json:"buy"`
	Sell      float64 `json:"sell"`
	Quantity  int     `json:"quantity"`
	Kind      string  `json:"kind"`      // delivery or intraday
	Brokerage float64 `json:"brokerage"` // per order, in rupees
	DP        float64 `json:"dp"`        // depository charge on a delivery sale
}

// TradeResult breaks down what a trade costs.
type TradeResult struct {
	Turnover  float64            `json:"turnover"`
	Charges   map[string]float64 `json:"charges"`
	Total     float64            `json:"totalCharges"`
	Gross     float64            `json:"grossProfit"`
	Net       float64            `json:"netProfit"`
	Breakeven float64            `json:"breakevenPoints"`
	Rule      string             `json:"rule"`
}

// Trade uses the statutory charges for NSE equity in force from 1 October 2024:
// securities transaction tax (0.1% each side for delivery, 0.025% on the sale for
// intraday), NSE transaction charges (0.00297%), the SEBI fee (10 rupees a crore),
// stamp duty on the purchase (0.015% delivery, 0.003% intraday) and 18% GST on
// brokerage, transaction charges and the SEBI fee.
func Trade(in TradeRequest) (TradeResult, error) {
	if in.Buy <= 0 || in.Sell <= 0 || in.Quantity < 1 || in.Brokerage < 0 || in.DP < 0 {
		return TradeResult{}, errors.New("buy, sell and quantity must be above 0; brokerage and dp must not be negative")
	}
	if in.Kind == "" {
		in.Kind = "delivery"
	}
	q := float64(in.Quantity)
	buy, sell := in.Buy*q, in.Sell*q
	turnover := buy + sell
	charges := map[string]float64{}
	switch in.Kind {
	case "delivery":
		charges["stt"] = math.Round(turnover * 0.001)
		charges["stampDuty"] = math.Round(buy * 0.00015)
		charges["dp"] = in.DP
	case "intraday":
		charges["stt"] = math.Round(sell * 0.00025)
		charges["stampDuty"] = math.Round(buy * 0.00003)
	default:
		return TradeResult{}, errors.New("kind must be delivery or intraday")
	}
	charges["brokerage"] = in.Brokerage * 2
	charges["exchange"] = round2(turnover * 0.0000297)
	charges["sebi"] = round2(turnover * 0.000001)
	charges["gst"] = round2((charges["brokerage"] + charges["exchange"] + charges["sebi"]) * 0.18)
	total := 0.0
	for _, v := range charges {
		total += v
	}
	gross := sell - buy
	return TradeResult{Turnover: round2(turnover), Charges: charges, Total: round2(total), Gross: round2(gross), Net: round2(gross - total),
		Breakeven: round2(total / q),
		Rule:      "NSE equity charges from 1 October 2024: STT, NSE transaction charges 0.00297%, SEBI fee 10 rupees a crore, stamp duty on the buy, and 18% GST on brokerage and those charges. Brokerage and DP charges are your broker's; give them."}, nil
}
