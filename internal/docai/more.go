package docai

import (
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ---- receipts ----

// ReceiptItem is one bought line.
type ReceiptItem struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// Receipt is what was read from a shop bill.
type Receipt struct {
	Merchant string        `json:"merchant,omitempty"`
	GSTIN    string        `json:"gstin,omitempty"`
	Date     string        `json:"date,omitempty"`
	Items    []ReceiptItem `json:"items"`
	Tax      *float64      `json:"tax"`
	Total    *float64      `json:"total"`
	Payment  string        `json:"payment,omitempty"` // upi, card or cash, when the bill says
	Checks   []Check       `json:"checks"`
}

var (
	dateFind     = regexp.MustCompile(`\b(\d{1,2}[-/.](\d{1,2}|[A-Za-z]{3})[-/.]\d{2,4}|\d{4}-\d{2}-\d{2}|\d{1,2} [A-Za-z]{3,9},? \d{4})\b`)
	taxLabel     = regexp.MustCompile(`(?i)^(total\s+)?(c?gst|sgst|utgst|igst|tax|vat)\b`)
	receiptTotal = regexp.MustCompile(`(?i)^(grand\s+total|total(\s+amount)?|net\s+(amount|total|payable)|amount\s+(paid|payable|due)|bill\s+amount|to\s+pay)\b`)
	notAnItem    = regexp.MustCompile(`(?i)\b(total|tax|gst|cgst|sgst|igst|round|discount|change|cash|card|upi|paid|balance|tender|saving|qty|amount)\b`)
	moneyCell    = regexp.MustCompile(`^(?:₹|rs\.?|inr)?\s*-?\d{1,3}(?:,\d{2,3})*(?:\.\d{2})$|^(?:₹|rs\.?|inr)?\s*-?\d+\.\d{2}$`)
)

// FindDate returns the first date in a text, as YYYY-MM-DD.
func FindDate(text string) (string, bool) {
	for _, m := range dateFind.FindAllString(text, -1) {
		if d, ok := ParseDate(m); ok {
			return d, true
		}
	}
	return "", false
}

func allRows(pages []Page) [][]string {
	var rows [][]string
	for _, p := range pages {
		rows = append(rows, Rows(p)...)
	}
	return rows
}

func lastMoney(cells []string) (float64, bool) {
	for i := len(cells) - 1; i >= 0; i-- {
		c := strings.TrimSpace(strings.Trim(cells[i], "*"))
		if moneyCell.MatchString(strings.ToLower(c)) {
			return money(strings.TrimLeft(strings.ToLower(c), "₹rsin. "))
		}
	}
	return 0, false
}

// ReadReceipt reads a shop bill: the merchant (its first line), GSTIN, date, the lines
// that end in an amount, tax and total, and checks that the lines add up.
func ReadReceipt(pages []Page) Receipt {
	rc := Receipt{Items: []ReceiptItem{}, Checks: []Check{}}
	rows := allRows(pages)
	var text []string
	totalSeen := false
	var tax float64
	taxFound := false
	for _, r := range rows {
		line := strings.Join(r, "  ")
		text = append(text, line)
		if rc.Merchant == "" && strings.TrimSpace(line) != "" && !dateFind.MatchString(line) {
			rc.Merchant = strings.Trim(strings.TrimSpace(r[0]), "*")
		}
		first := strings.TrimSpace(strings.Trim(r[0], "*"))
		amount, hasAmount := lastMoney(r)
		switch {
		case receiptTotal.MatchString(first) && hasAmount:
			if rc.Total == nil {
				v := amount
				rc.Total = &v
			}
			totalSeen = true
		case taxLabel.MatchString(first) && hasAmount && !totalSeen:
			tax += amount
			taxFound = true
		case hasAmount && !totalSeen && len(r) >= 2 && !notAnItem.MatchString(first) && !dateFind.MatchString(first):
			rc.Items = append(rc.Items, ReceiptItem{Name: first, Amount: amount})
		}
		lower := strings.ToLower(line)
		switch {
		case rc.Payment != "":
		case strings.Contains(lower, "upi") || strings.Contains(lower, "gpay") || strings.Contains(lower, "phonepe") || strings.Contains(lower, "paytm"):
			rc.Payment = "upi"
		case strings.Contains(lower, "card") || strings.Contains(lower, "visa") || strings.Contains(lower, "rupay"):
			rc.Payment = "card"
		case strings.Contains(lower, "cash"):
			rc.Payment = "cash"
		}
	}
	all := strings.Join(text, "\n")
	if g := gstinFind.FindString(all); g != "" {
		rc.GSTIN = g
	}
	if d, ok := FindDate(all); ok {
		rc.Date = d
	}
	if taxFound {
		rc.Tax = &tax
	}
	if rc.Total != nil && len(rc.Items) > 0 {
		sum := 0.0
		for _, it := range rc.Items {
			sum += it.Amount
		}
		withTax := sum + value(rc.Tax)
		ok := near(sum, *rc.Total) || near(withTax, *rc.Total) || math.Abs(withTax-*rc.Total) < 1 // round off
		rc.Checks = append(rc.Checks, Check{Name: "items_add_up", Passed: ok, Detail: "The lines add up to " + rupees(sum) + " (" + rupees(withTax) + " with tax); the total says " + rupees(*rc.Total) + "."})
	}
	if rc.GSTIN != "" {
		inv := Invoice{Seller: Party{GSTIN: rc.GSTIN}}
		inv.check()
		rc.Checks = append(rc.Checks, inv.Checks...)
	}
	return rc
}

// ---- bank statements ----

// Transaction is one line of a bank statement.
type Transaction struct {
	Date        string   `json:"date"`
	Description string   `json:"description"`
	Reference   string   `json:"reference,omitempty"`
	Debit       *float64 `json:"debit"`
	Credit      *float64 `json:"credit"`
	Balance     *float64 `json:"balance"`
	// BalanceOK says whether the balance follows from the previous one; nil when it
	// cannot be checked.
	BalanceOK *bool `json:"balance_ok"`
}

var statementColumns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"date", regexp.MustCompile(`(?i)^(txn\s*|tran(saction)?\s*|post(ing)?\s*)?date$`)},
	{"value_date", regexp.MustCompile(`(?i)^value\s*(date|dt)$`)},
	{"description", regexp.MustCompile(`(?i)^(narration|description|particulars|details|remarks|transaction\s+details)$`)},
	{"reference", regexp.MustCompile(`(?i)^(chq\.?\s*/?\s*ref\.?\s*(no\.?)?|cheque\s*(no\.?)?|ref(erence)?\s*(no\.?)?|chq\s*no\.?)$`)},
	{"debit", regexp.MustCompile(`(?i)^(withdrawal(\s+amt\.?)?s?|debits?(\s+amount)?|dr\.?|withdrawals?\s*\(dr\))$`)},
	{"credit", regexp.MustCompile(`(?i)^(deposits?(\s+amt\.?)?|credits?(\s+amount)?|cr\.?|deposits?\s*\(cr\))$`)},
	{"balance", regexp.MustCompile(`(?i)^(closing\s+)?balance(\s+amt\.?)?$`)},
}

type headerCol struct {
	name   string
	x0, x1 float64
}

// ReadStatement reads a bank statement's transactions: it finds the header row (date,
// withdrawals or debits, deposits or credits, balance), puts each word under the header
// it sits beneath, joins description lines that run on, and checks every balance
// against the one before.
func ReadStatement(pages []Page) []Transaction {
	out := []Transaction{}
	var header []headerCol
	for _, p := range pages {
		for _, r := range pageRows(p) {
			cells := r.cells()
			if h, ok := statementHeader(cells); ok {
				header = h
				continue
			}
			if header == nil {
				continue
			}
			fields := map[string]string{}
			for _, c := range cells {
				name := nearestColumn(header, cellSpan(c))
				text := row{words: c}.text()
				if fields[name] != "" {
					fields[name] += " "
				}
				fields[name] += text
			}
			date, isDate := ParseDate(fields["date"])
			if !isDate {
				// A description that runs on to the next line.
				if n := len(out); n > 0 && fields["description"] != "" && fields["debit"] == "" && fields["credit"] == "" && fields["balance"] == "" && len(fields) == 1 {
					out[n-1].Description += " " + fields["description"]
				}
				continue
			}
			t := Transaction{Date: date, Description: fields["description"], Reference: fields["reference"]}
			amount := func(s string) *float64 {
				s = strings.TrimSpace(s)
				if s == "" {
					return nil
				}
				v, ok := money(s)
				if !ok {
					return nil
				}
				if strings.HasSuffix(strings.ToLower(s), "dr") && v > 0 {
					v = -v
				}
				return &v
			}
			t.Debit, t.Credit, t.Balance = amount(fields["debit"]), amount(fields["credit"]), amount(fields["balance"])
			if t.Debit != nil && *t.Debit == 0 {
				t.Debit = nil
			}
			if t.Credit != nil && *t.Credit == 0 {
				t.Credit = nil
			}
			if n := len(out); n > 0 && out[n-1].Balance != nil && t.Balance != nil {
				want := *out[n-1].Balance - math.Abs(value(t.Debit)) + value(t.Credit)
				ok := near(want, *t.Balance)
				t.BalanceOK = &ok
			}
			out = append(out, t)
		}
	}
	return out
}

func statementHeader(cells [][]Word) ([]headerCol, bool) {
	var cols []headerCol
	found := map[string]bool{}
	for _, c := range cells {
		text := strings.TrimSpace(row{words: c}.text())
		for _, sc := range statementColumns {
			if sc.pattern.MatchString(text) && !found[sc.name] {
				found[sc.name] = true
				cols = append(cols, headerCol{sc.name, c[0].X0, c[len(c)-1].X1})
				break
			}
		}
	}
	ok := found["date"] && found["balance"] && (found["debit"] || found["credit"])
	return cols, ok
}

func nearestColumn(cols []headerCol, s span) string {
	best, bestD := cols[0].name, math.MaxFloat64
	for _, c := range cols {
		// Text sits left-aligned, centred or right-aligned under its header.
		d := math.Min(math.Abs(s.x0-c.x0), math.Min(math.Abs(s.x1-c.x1), math.Abs((s.x0+s.x1)/2-(c.x0+c.x1)/2)))
		if s.overlaps(span{c.x0, c.x1}) {
			d /= 4
		}
		if d < bestD {
			best, bestD = c.name, d
		}
	}
	return best
}

// ---- cheques ----

// Cheque is what was read from a cheque leaf.
type Cheque struct {
	IFSC         string `json:"ifsc,omitempty"`
	MICR         string `json:"micr,omitempty"`
	ChequeNumber string `json:"cheque_number,omitempty"`
	Account      string `json:"account_number,omitempty"`
	Date         string `json:"date,omitempty"`
}

var (
	accountLabel = regexp.MustCompile(`(?i)\b(a/?c|account)\s*(no\.?|number)?\s*[:.]?\s*(\d[\d ]{8,20}\d)`)
	digitRun     = regexp.MustCompile(`\d+`)
)

// ReadCheque finds a cheque's IFSC, its MICR line (cheque number, the nine-digit MICR
// code, and what follows), the account number when it is printed with a label, and a
// date.
func ReadCheque(pages []Page) Cheque {
	var c Cheque
	var lines []string
	for _, r := range allRows(pages) {
		lines = append(lines, strings.Join(r, " "))
	}
	all := strings.Join(lines, "\n")
	for _, f := range ifscFind.FindAllString(strings.ToUpper(all), -1) {
		c.IFSC = f
		break
	}
	if m := accountLabel.FindStringSubmatch(all); m != nil {
		c.Account = strings.ReplaceAll(m[3], " ", "")
	}
	// The MICR line is the lowest line holding a nine-digit group; the six digits
	// before it are the cheque number.
	for i := len(lines) - 1; i >= 0 && c.MICR == ""; i-- {
		runs := digitRun.FindAllString(lines[i], -1)
		for j, run := range runs {
			if len(run) == 9 {
				c.MICR = run
				if j > 0 && len(runs[j-1]) == 6 {
					c.ChequeNumber = runs[j-1]
				}
				break
			}
		}
	}
	// Dates on cheques are often eight boxed digits: 14102026.
	if d, ok := FindDate(all); ok {
		c.Date = d
	} else if m := regexp.MustCompile(`\b(\d)\s?(\d)\s?(\d)\s?(\d)\s?(20\d\d)\b`).FindStringSubmatch(all); m != nil {
		if d, ok := ParseDate(m[1] + m[2] + "/" + m[3] + m[4] + "/" + m[5]); ok {
			c.Date = d
		}
	}
	return c
}

// ---- word offsets, for redaction ----

// PageText is a page's text with where each word sits in it.
type PageText struct {
	Text  string
	Words []Word
	// Start and End of each word, in characters (runes) of Text.
	Start, End []int
}

// TextOf joins a page's lines, one line per row, recording each word's place.
func TextOf(p Page) PageText {
	var b strings.Builder
	var pt PageText
	n := 0
	for i, r := range pageRows(p) {
		if i > 0 {
			b.WriteString("\n")
			n++
		}
		for j, w := range r.words {
			if j > 0 {
				b.WriteString(" ")
				n++
			}
			pt.Words = append(pt.Words, w)
			pt.Start = append(pt.Start, n)
			b.WriteString(w.Text)
			n += utf8.RuneCountInString(w.Text)
			pt.End = append(pt.End, n)
		}
	}
	pt.Text = b.String()
	return pt
}

// Covering returns the words that overlap the characters start to end.
func (pt PageText) Covering(start, end int) []Word {
	var out []Word
	for i, w := range pt.Words {
		if pt.Start[i] < end && start < pt.End[i] {
			out = append(out, w)
		}
	}
	return out
}
