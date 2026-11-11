package docai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReadStatement(t *testing.T) {
	list := ReadStatement(load(t, "statement.html"))
	raw, _ := json.MarshalIndent(list, "", " ")
	if len(list) != 7 {
		t.Fatalf("transactions = %d:\n%s", len(list), raw)
	}
	first, salary, premium := list[0], list[2], list[4]
	if first.Date != "2026-10-01" || value(first.Balance) != 25000 || first.BalanceOK != nil {
		t.Errorf("first: %+v", first)
	}
	if value(salary.Credit) != 45000 || salary.Debit != nil || salary.Reference != "N278261234567" || !strings.Contains(salary.Description, "SALARY OCT 2026") {
		t.Errorf("salary:\n%s", raw)
	}
	if value(premium.Debit) != 2400.5 || !strings.HasSuffix(premium.Description, "POLICY 12345 OCTOBER") {
		t.Errorf("premium:\n%s", raw)
	}
	for _, tx := range list[1:] {
		if tx.BalanceOK == nil || !*tx.BalanceOK {
			t.Errorf("balance not followed: %+v", tx)
		}
	}
}

func TestTextOfAndCovering(t *testing.T) {
	pages := load(t, "statement.html")
	pt := TextOf(pages[0])
	at := strings.Index(pt.Text, "ABCPS1234D")
	if at < 0 {
		t.Fatalf("PAN not in text:\n%s", pt.Text)
	}
	start := len([]rune(pt.Text[:at]))
	words := pt.Covering(start, start+10)
	if len(words) != 1 || words[0].Text != "ABCPS1234D" {
		t.Errorf("covering = %+v", words)
	}
}

func synthetic(rows ...[]string) []Page {
	var blocks []Block
	for i, r := range rows {
		var line Line
		x := 20.0
		y := float64(20 + i*14)
		for _, cell := range r {
			for _, f := range strings.Fields(cell) {
				line.Words = append(line.Words, Word{Text: f, X0: x, Y0: y, X1: x + 5*float64(len(f)), Y1: y + 10})
				x += 5*float64(len(f)) + 2
			}
			x += 40
		}
		line.X0, line.Y0, line.X1, line.Y1 = 20, y, x, y+10
		blocks = append(blocks, Block{Lines: []Line{line}})
	}
	return []Page{{Width: 400, Height: 600, Blocks: blocks}}
}

func TestReadReceipt(t *testing.T) {
	rc := ReadReceipt(synthetic(
		[]string{"SRI MURUGAN STORES"}, []string{"GSTIN 33AABCS1429B1Z1"}, []string{"Date 14-10-2026"},
		[]string{"Basmati rice 5 kg", "575.00"}, []string{"Toor dal 1 kg", "168.00"}, []string{"Coconut oil 1 L", "245.00"}, []string{"Filter coffee 500 g", "262.00"},
		[]string{"Total", "Rs. 1,250.00"}, []string{"Paid by UPI"},
	))
	raw, _ := json.MarshalIndent(rc, "", " ")
	if rc.Merchant != "SRI MURUGAN STORES" || rc.GSTIN != "33AABCS1429B1Z1" || rc.Date != "2026-10-14" || len(rc.Items) != 4 || value(rc.Total) != 1250 || rc.Payment != "upi" {
		t.Errorf("receipt:\n%s", raw)
	}
	for _, c := range rc.Checks {
		if !c.Passed {
			t.Errorf("check %s: %s", c.Name, c.Detail)
		}
	}
}

func TestReadCheque(t *testing.T) {
	c := ReadCheque(synthetic(
		[]string{"Sample Bank", "Date: 14102026"}, []string{"IFSC: HDFC0000001"}, []string{"Pay Kaveri Traders", "₹ 13,013.00"},
		[]string{"A/c No. 50100123456789"}, []string{`"000123" 400240003: 123456" 29`},
	))
	if c.IFSC != "HDFC0000001" || c.MICR != "400240003" || c.ChequeNumber != "000123" || c.Account != "50100123456789" || c.Date != "2026-10-14" {
		t.Errorf("cheque: %+v", c)
	}
}
