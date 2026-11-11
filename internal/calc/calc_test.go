package calc

import (
	"math"
	"testing"
)

func near(got, want, tolerance float64) bool { return math.Abs(got-want) <= tolerance }

func TestSIPAndLumpSum(t *testing.T) {
	// 10,000 a month for 10 years at 12%: the figure every fund house calculator shows.
	sip, err := SIP(SIPRequest{Monthly: 10000, Return: 12, Years: 10})
	if err != nil || !near(sip.Value, 2323391, 2) || sip.Invested != 1200000 {
		t.Errorf("SIP = %v %v", sip.Value, err)
	}
	stepped, _ := SIP(SIPRequest{Monthly: 10000, Return: 12, Years: 2, StepUp: 10})
	if stepped.Years[1].Monthly != 11000 || stepped.Invested != 252000 {
		t.Errorf("step-up %+v", stepped.Years)
	}
	lump, _ := LumpSum(100000, 10, 10)
	if !near(lump.Value, 259374.25, 0.01) {
		t.Errorf("lump sum %v", lump.Value)
	}
	if _, err := SIP(SIPRequest{Monthly: 0, Years: 5}); err == nil {
		t.Error("zero SIP accepted")
	}
}

func TestEMI(t *testing.T) {
	loan, err := EMI(1000000, 8.5, 240, true)
	if err != nil || !near(loan.EMI, 8678.23, 0.01) || len(loan.Schedule) != 240 || loan.Schedule[239].Balance != 0 {
		t.Errorf("EMI = %v %v", loan.EMI, err)
	}
	if !near(loan.Schedule[0].Interest, 7083.33, 0.01) {
		t.Errorf("first month's interest %v", loan.Schedule[0].Interest)
	}
	free, _ := EMI(120000, 0, 12, false)
	if free.EMI != 10000 || free.TotalInterest != 0 {
		t.Errorf("interest-free EMI %+v", free)
	}
}

func TestGST(t *testing.T) {
	within, _ := GST(1180, 18, true, false)
	if within.Taxable != 1000 || within.CGST != 90 || within.SGST != 90 || within.Total != 1180 {
		t.Errorf("inclusive within a state %+v", within)
	}
	between, _ := GST(1000, 5, false, true)
	if between.IGST != 50 || between.Total != 1050 {
		t.Errorf("exclusive between states %+v", between)
	}
	if _, err := GST(100, 17, false, false); err == nil {
		t.Error("17% accepted")
	}
}

func TestIncomeTax(t *testing.T) {
	cases := []struct {
		name     string
		in       IncomeTaxRequest
		regime   string
		want     float64
		marginal bool
	}{
		// 12,75,000 of salary is 12 lakh taxable: fully rebated.
		{"new regime at the rebate limit", IncomeTaxRequest{Income: 1275000, Salaried: true}, "new", 0, false},
		// 12,25,000 taxable: slab tax 63,750, but no more than the 25,000 above 12 lakh.
		{"new regime marginal relief", IncomeTaxRequest{Income: 1300000, Salaried: true}, "new", 26000, true},
		// 20 lakh taxable: 0 + 20,000 + 40,000 + 60,000 + 80,000 = 2,00,000, plus cess.
		{"new regime 20 lakh", IncomeTaxRequest{Income: 2000000}, "new", 208000, false},
		// Old regime: 10 lakh salary, 1.5 lakh of 80C: 8 lakh taxable, 72,500 plus cess.
		{"old regime", IncomeTaxRequest{Income: 1000000, Salaried: true, Deductions: 150000}, "old", 75400, false},
		{"old regime rebate", IncomeTaxRequest{Income: 500000}, "old", 0, false},
		// A senior citizen's exemption limit is 3 lakh: 2 lakh at 5% = 10,000, rebated.
		{"old regime senior", IncomeTaxRequest{Income: 500000, Age: "60to80"}, "old", 0, false},
	}
	for _, c := range cases {
		got, err := IncomeTaxFor(c.in, c.regime)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		result := got.New
		if c.regime == "old" {
			result = got.Old
		}
		if result.Total != c.want || result.MarginalRelief != c.marginal {
			t.Errorf("%s: total %v (relief %v), want %v (%v): %+v", c.name, result.Total, result.MarginalRelief, c.want, c.marginal, result)
		}
	}
	// Just above 50 lakh, the surcharge may not take more than the income above it.
	above, _ := IncomeTaxFor(IncomeTaxRequest{Income: 5010000}, "new")
	at, _ := IncomeTaxFor(IncomeTaxRequest{Income: 5000000}, "new")
	if rise := (above.New.Total - above.New.Cess) - (at.New.Total - at.New.Cess); rise > 10000+1 || !above.New.MarginalRelief {
		t.Errorf("surcharge relief: tax rose by %v for 10,000 more income", rise)
	}
	both, _ := IncomeTaxFor(IncomeTaxRequest{Income: 1500000, Salaried: true}, "both")
	if both.Better == "" || both.New == nil || both.Old == nil {
		t.Errorf("comparison %+v", both)
	}
	if _, err := IncomeTaxFor(IncomeTaxRequest{Income: 1, Year: "1999-00"}, "new"); err == nil {
		t.Error("unknown year accepted")
	}
}

func TestHRAGratuityPPF(t *testing.T) {
	hra, _ := HRA(600000, 300000, 240000, true)
	// Least of 3,00,000, 2,40,000 - 60,000 = 1,80,000, and 3,00,000.
	if hra.Exempt != 180000 || hra.Taxable != 120000 {
		t.Errorf("HRA %+v", hra)
	}
	g, _ := Gratuity(52000, 10.6, true)
	// 52,000 x 15/26 x 11 years (10 years 7 months rounds up).
	if g.Amount != 330000 || g.Years != 11 || !g.Eligible {
		t.Errorf("gratuity %+v", g)
	}
	big, _ := Gratuity(500000, 30, true)
	if big.Exempt != GratuityCap || big.Taxable != big.Amount-GratuityCap {
		t.Errorf("gratuity cap %+v", big)
	}
	ppf, _ := PPF(150000, 15, 7.1)
	if !near(ppf.Maturity, 4068209, 2) || ppf.Deposited != 2250000 {
		t.Errorf("PPF maturity %v", ppf.Maturity)
	}
	if _, err := PPF(150000, 17, 0); err == nil {
		t.Error("17 years accepted")
	}
}

func TestCapitalGains(t *testing.T) {
	long, _ := CapitalGains(CapitalGainsRequest{Asset: "equity", Bought: "2023-01-10", Sold: "2025-01-10", Cost: 100000, Price: 400000})
	// 3 lakh gain, 1.25 lakh exempt: 12.5% of 1.75 lakh = 21,875.
	if long.Term != "long" || long.Tax != 21875 || long.Total != 22750 {
		t.Errorf("equity long term %+v", long)
	}
	short, _ := CapitalGains(CapitalGainsRequest{Asset: "equity", Bought: "2025-01-10", Sold: "2025-06-10", Cost: 100000, Price: 150000})
	if short.Term != "short" || short.Tax != 10000 {
		t.Errorf("equity short term %+v", short)
	}
	house, _ := CapitalGains(CapitalGainsRequest{Asset: "property", Bought: "2010-06-01", Sold: "2025-06-01", Cost: 2000000, Price: 7000000})
	// Indexed cost 20 lakh x 376/167 = 45,02,994; 20% of the 24,97,006 gain = 4,99,401,
	// less than 12.5% of 50 lakh = 6,25,000.
	if house.Indexed == nil || !house.Indexed.Chosen || house.Tax != house.Indexed.Tax || !near(house.Indexed.IndexedCost, 4502994.01, 0.01) {
		t.Errorf("property %+v %+v", house, house.Indexed)
	}
	debt, _ := CapitalGains(CapitalGainsRequest{Asset: "debt-fund", Bought: "2023-05-01", Sold: "2026-05-01", Cost: 1, Price: 2})
	if debt.Rate != nil {
		t.Error("new debt fund units have a special rate")
	}
	if _, err := CapitalGains(CapitalGainsRequest{Asset: "equity", Bought: "2020-01-01", Sold: "2024-01-01", Cost: 1, Price: 2}); err == nil {
		t.Error("a sale before 23 July 2024 was accepted")
	}
}

func TestTrade(t *testing.T) {
	trade, err := Trade(TradeRequest{Buy: 1000, Sell: 1100, Quantity: 100, Kind: "delivery", Brokerage: 20})
	if err != nil {
		t.Fatal(err)
	}
	// Turnover 2,10,000: STT 210, stamp duty 15, exchange 6.24, SEBI 0.21, brokerage 40, GST 18% of 46.45.
	if trade.Charges["stt"] != 210 || trade.Charges["stampDuty"] != 15 || !near(trade.Charges["gst"], 8.36, 0.01) || trade.Gross != 10000 {
		t.Errorf("delivery %+v", trade)
	}
	intraday, _ := Trade(TradeRequest{Buy: 1000, Sell: 1010, Quantity: 10, Kind: "intraday"})
	if intraday.Charges["stt"] != 3 || intraday.Charges["stampDuty"] != 0 {
		t.Errorf("intraday %+v", intraday.Charges)
	}
}
