package docai

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
	"github.com/dharunashokkumar/api.dharun.dev/internal/words"
)

// Rows are a page's lines rebuilt by height on the page (pdftotext can split one visual
// row into several blocks), each cut into cells where the gap between words is wide.
func Rows(p Page) [][]string {
	var lines []Line
	for _, b := range p.Blocks {
		lines = append(lines, b.Lines...)
	}
	var out [][]string
	for _, r := range mergeRows(lines, p.CellGap) {
		var cells []string
		for _, c := range r.cells() {
			parts := make([]string, len(c))
			for i, w := range c {
				parts[i] = w.Text
			}
			cells = append(cells, strings.Join(parts, " "))
		}
		out = append(out, cells)
	}
	return out
}

// Party is the seller or the buyer of an invoice.
type Party struct {
	Name      string `json:"name,omitempty"`
	GSTIN     string `json:"gstin,omitempty"`
	StateCode string `json:"state_code,omitempty"`
	State     string `json:"state,omitempty"`
}

// Amounts are the invoice's totals; a nil field was not found.
type Amounts struct {
	Taxable  *float64 `json:"taxable_value"`
	CGST     *float64 `json:"cgst"`
	SGST     *float64 `json:"sgst"` // SGST or UTGST
	IGST     *float64 `json:"igst"`
	Cess     *float64 `json:"cess"`
	RoundOff *float64 `json:"round_off"`
	Total    *float64 `json:"total"`
}

// Check is one test of the invoice against itself or the GST rules.
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// Invoice is what was read from a GST invoice.
type Invoice struct {
	Title         string   `json:"title,omitempty"`
	Number        string   `json:"number,omitempty"`
	Date          string   `json:"date,omitempty"` // YYYY-MM-DD
	Seller        Party    `json:"seller"`
	Buyer         Party    `json:"buyer"`
	PlaceOfSupply string   `json:"place_of_supply,omitempty"`
	Supply        string   `json:"supply,omitempty"` // intra_state or inter_state, from the tax charged
	HSN           []string `json:"hsn"`
	Amounts       Amounts  `json:"amounts"`
	AmountInWords string   `json:"amount_in_words,omitempty"`
	IFSC          string   `json:"ifsc,omitempty"`
	UPI           string   `json:"upi,omitempty"`
	OtherGSTINs   []string `json:"other_gstins,omitempty"`
	Checks        []Check  `json:"checks"`
}

var (
	gstinFind   = regexp.MustCompile(`\b\d{2}[A-Z]{5}\d{4}[A-Z][0-9A-Z]Z[0-9A-Z]\b`)
	ifscFind    = regexp.MustCompile(`\b[A-Z]{4}0[A-Z0-9]{6}\b`)
	upiFind     = regexp.MustCompile(`\b[a-zA-Z0-9][\w.-]{1,255}@[a-zA-Z][a-zA-Z0-9]{2,63}\b`)
	moneyFind   = regexp.MustCompile(`-?\d{1,3}(?:,\d{2,3})+(?:\.\d{1,2})?|-?\d+\.\d{1,2}|-?\d+`)
	hsnFind     = regexp.MustCompile(`^\d{4}(?:\d{2}){0,2}$`)
	labelClean  = regexp.MustCompile(`(?i)\(\s*(₹|rs\.?|inr)\s*\)|\b(rs\.?|inr)(\s|$)|₹|[:.\-]+\s*$`)
	buyerHeads  = regexp.MustCompile(`(?i)^(bill(ed)? to|buyer|customer|recipient|sold to|consignee|ship(ped)? to|details of (the )?(receiver|recipient|buyer))\b`)
	numberLabel = regexp.MustCompile(`(?i)^(tax\s+)?invoice\s*(no\.?|number|#)\s*:?\s*(.*)$`)
	dateLabel   = regexp.MustCompile(`(?i)^(invoice\s+)?date(\s+of\s+(issue|invoice))?\s*:?\s*(.*)$`)
	placeLabel  = regexp.MustCompile(`(?i)^place\s+of\s+supply\s*:?\s*(.*)$`)
	wordsLabel  = regexp.MustCompile(`(?i)^(total\s+)?(amount|invoice value|rupees)\s+(chargeable\s+)?in\s+words\s*:?\s*(.*)$`)
)

// amountLabels name the totals; a label is matched against the whole cell.
var amountLabels = []struct {
	field   func(*Amounts) **float64
	pattern *regexp.Regexp
}{
	{func(a *Amounts) **float64 { return &a.Taxable }, regexp.MustCompile(`(?i)^(total\s+)?(taxable\s+(value|amount)|sub\s*-?\s*total|assessable\s+value)$`)},
	{func(a *Amounts) **float64 { return &a.CGST }, regexp.MustCompile(`(?i)^(total\s+|add\s*:?\s*)?(cgst|central\s+tax)(\s*@?\s*[\d.]+\s*%)?$`)},
	{func(a *Amounts) **float64 { return &a.SGST }, regexp.MustCompile(`(?i)^(total\s+|add\s*:?\s*)?(sgst|utgst|state\s+tax|ut\s+tax)(\s*@?\s*[\d.]+\s*%)?$`)},
	{func(a *Amounts) **float64 { return &a.IGST }, regexp.MustCompile(`(?i)^(total\s+|add\s*:?\s*)?(igst|integrated\s+tax)(\s*@?\s*[\d.]+\s*%)?$`)},
	{func(a *Amounts) **float64 { return &a.Cess }, regexp.MustCompile(`(?i)^(total\s+)?(gst\s+)?cess$`)},
	{func(a *Amounts) **float64 { return &a.RoundOff }, regexp.MustCompile(`(?i)^round(ed)?\s*(off|ing)$`)},
	{func(a *Amounts) **float64 { return &a.Total }, regexp.MustCompile(`(?i)^(grand\s+total|invoice\s+(total|value|amount)|total(\s+invoice)?(\s+(value|amount))?(\s+payable)?|net\s+(amount|payable)|amount\s+payable|total\s+due|balance\s+due)$`)},
}

func money(s string) (float64, bool) {
	m := moneyFind.FindString(s)
	if m == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	return v, err == nil
}

var dateLayouts = []string{"02 Jan 2006", "2 Jan 2006", "02-Jan-2006", "2-Jan-2006", "02-Jan-06", "02 January 2006", "2 January 2006", "January 2, 2006", "Jan 2, 2006",
	"02/01/2006", "2/1/2006", "02-01-2006", "2-1-2006", "02.01.2006", "2006-01-02", "02/01/06", "02-01-06"}

// ParseDate reads a date in the ways Indian invoices write it, day first.
func ParseDate(s string) (string, bool) {
	s = strings.TrimSpace(strings.Trim(s, ":,"))
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil && t.Year() >= 2000 && t.Year() < 2100 {
			return t.Format(time.DateOnly), true
		}
	}
	return "", false
}

// ReadInvoice reads a GST invoice's fields from its pages and checks them against each
// other: every GSTIN's check character, the taxes adding up to the total, CGST equal to
// SGST, the kind of tax against the place of supply, and the amount in words.
func ReadInvoice(pages []Page) Invoice {
	inv := Invoice{HSN: []string{}, Checks: []Check{}}
	var rows [][]string
	for _, p := range pages {
		rows = append(rows, Rows(p)...)
	}
	var allText []string
	for _, r := range rows {
		allText = append(allText, strings.Join(r, "  "))
	}
	buyerFrom := -1 // the row a buyer heading starts
	nextValue := func(i, j int, rest string) string {
		if rest = strings.TrimSpace(rest); rest != "" {
			return rest
		}
		if j+1 < len(rows[i]) {
			return strings.TrimSpace(rows[i][j+1])
		}
		return ""
	}
	for i, r := range rows {
		for j, cell := range r {
			cell = strings.TrimSpace(cell)
			upper := strings.ToUpper(cell)
			if inv.Title == "" && (strings.Contains(upper, "TAX INVOICE") || upper == "INVOICE" || strings.Contains(upper, "BILL OF SUPPLY")) && len(cell) < 40 {
				inv.Title = cell
			}
			if buyerFrom < 0 && buyerHeads.MatchString(cell) {
				buyerFrom = i
				// The buyer's name is the rest of the cell or the next row's first cell.
				if rest := strings.TrimSpace(buyerHeads.ReplaceAllString(cell, "")); strings.Trim(rest, ": ") != "" {
					inv.Buyer.Name = strings.Trim(rest, ": ")
				} else if i+1 < len(rows) && len(rows[i+1]) > 0 {
					inv.Buyer.Name = rows[i+1][0]
				}
			}
			if m := numberLabel.FindStringSubmatch(cell); m != nil && inv.Number == "" {
				inv.Number = strings.Fields(nextValue(i, j, m[3]) + " ")[0]
			}
			if m := dateLabel.FindStringSubmatch(cell); m != nil && inv.Date == "" {
				if d, ok := ParseDate(nextValue(i, j, m[4])); ok {
					inv.Date = d
				}
			}
			if m := placeLabel.FindStringSubmatch(cell); m != nil && inv.PlaceOfSupply == "" {
				inv.PlaceOfSupply = nextValue(i, j, m[1])
			}
			if m := wordsLabel.FindStringSubmatch(cell); m != nil && inv.AmountInWords == "" {
				if v := strings.TrimSpace(m[4]); v != "" {
					inv.AmountInWords = v
				} else if i+1 < len(rows) && len(rows[i+1]) > 0 {
					inv.AmountInWords = rows[i+1][0]
				}
			}
			label := strings.TrimSpace(labelClean.ReplaceAllString(cell, ""))
			for _, a := range amountLabels {
				field := a.field(&inv.Amounts)
				if *field != nil || !a.pattern.MatchString(label) {
					continue
				}
				// The amount is the next cell on the row holding a number.
				for _, next := range r[j+1:] {
					if v, ok := money(next); ok {
						*field = &v
						break
					}
				}
			}
		}
	}
	if inv.Seller.Name == "" && len(rows) > 0 && len(rows[0]) > 0 && !strings.EqualFold(rows[0][0], inv.Title) {
		inv.Seller.Name = rows[0][0]
	}
	// GSTINs: the first one before the buyer heading is the seller's, the first one after
	// it the buyer's.
	for i, line := range allText {
		for _, g := range gstinFind.FindAllString(line, -1) {
			switch {
			case buyerFrom >= 0 && i >= buyerFrom && inv.Buyer.GSTIN == "":
				inv.Buyer.GSTIN = g
			case (buyerFrom < 0 || i < buyerFrom) && inv.Seller.GSTIN == "":
				inv.Seller.GSTIN = g
			case g != inv.Seller.GSTIN && g != inv.Buyer.GSTIN:
				inv.OtherGSTINs = append(inv.OtherGSTINs, g)
			}
		}
		if inv.IFSC == "" {
			for _, f := range ifscFind.FindAllString(line, -1) {
				if !gstinFind.MatchString(f) {
					inv.IFSC = f
					break
				}
			}
		}
		if inv.UPI == "" {
			for _, u := range upiFind.FindAllString(line, -1) {
				if !strings.Contains(u[strings.Index(u, "@"):], ".") { // a handle, not an email address
					inv.UPI = u
					break
				}
			}
		}
	}
	if buyerFrom < 0 && inv.Buyer.GSTIN == "" && len(inv.OtherGSTINs) > 0 {
		inv.Buyer.GSTIN, inv.OtherGSTINs = inv.OtherGSTINs[0], inv.OtherGSTINs[1:]
	}
	inv.HSN = hsnCodes(pages)
	inv.check()
	return inv
}

// hsnCodes reads the HSN or SAC column of any table.
func hsnCodes(pages []Page) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range Tables(pages) {
		col := -1
		for _, r := range t.Rows {
			for j, c := range r {
				u := strings.ToUpper(c)
				if col < 0 && (strings.Contains(u, "HSN") || u == "SAC") {
					col = j
				}
			}
			if col >= 0 && col < len(r) {
				for _, f := range strings.Fields(r[col]) {
					if hsnFind.MatchString(f) && !seen[f] {
						seen[f] = true
						out = append(out, f)
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.015 }

func value(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func rupees(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func (inv *Invoice) check() {
	add := func(name string, passed bool, detail string) {
		inv.Checks = append(inv.Checks, Check{Name: name, Passed: passed, Detail: detail})
	}
	for _, party := range []*Party{&inv.Seller, &inv.Buyer} {
		if party.GSTIN == "" {
			continue
		}
		result := validate.GSTIN(party.GSTIN)
		role := "seller"
		if party == &inv.Buyer {
			role = "buyer"
		}
		if result.Valid {
			party.StateCode, party.State = result.Parts["stateCode"], result.Parts["state"]
			add(role+"_gstin", true, party.GSTIN+" is a valid GSTIN of "+party.State+".")
		} else {
			add(role+"_gstin", false, party.GSTIN+": "+result.Reason)
		}
	}
	a := inv.Amounts
	if a.Taxable != nil && a.Total != nil {
		sum := value(a.Taxable) + value(a.CGST) + value(a.SGST) + value(a.IGST) + value(a.Cess) + value(a.RoundOff)
		add("total_adds_up", near(sum, *a.Total), "Taxable value, taxes and round off add up to "+rupees(sum)+"; the total says "+rupees(*a.Total)+".")
	}
	if a.CGST != nil || a.SGST != nil {
		add("cgst_equals_sgst", near(value(a.CGST), value(a.SGST)), "CGST "+rupees(value(a.CGST))+", SGST or UTGST "+rupees(value(a.SGST))+": the two halves of the tax must be equal.")
	}
	switch {
	case value(a.IGST) != 0 && (value(a.CGST) != 0 || value(a.SGST) != 0):
		add("one_kind_of_tax", false, "Both IGST and CGST/SGST are charged; a supply is either within a state (CGST and SGST) or between states (IGST).")
	case value(a.IGST) != 0:
		inv.Supply = "inter_state"
	case value(a.CGST) != 0 || value(a.SGST) != 0:
		inv.Supply = "intra_state"
	}
	// The place of supply is the buyer's state unless it says otherwise.
	place := ""
	if m := regexp.MustCompile(`\((\d{2})\)|^(\d{2})\b`).FindStringSubmatch(inv.PlaceOfSupply); m != nil {
		place = m[1] + m[2]
	} else if inv.Buyer.StateCode != "" {
		place = inv.Buyer.StateCode
	}
	if inv.Supply != "" && inv.Seller.StateCode != "" && place != "" {
		want := "inter_state"
		if place == inv.Seller.StateCode {
			want = "intra_state"
		}
		tax := map[string]string{"inter_state": "IGST", "intra_state": "CGST and SGST"}
		add("tax_matches_place_of_supply", want == inv.Supply, "The seller is in state "+inv.Seller.StateCode+" and the place of supply is "+place+", so the tax should be "+tax[want]+"; the invoice charges "+tax[inv.Supply]+".")
	}
	if inv.AmountInWords != "" && a.Total != nil && *a.Total >= 0 {
		whole := int64(*a.Total)
		paise := int(math.Round((*a.Total - float64(whole)) * 100))
		if spelt, err := words.Currency("en", whole, paise); err == nil {
			add("amount_in_words", sameWords(spelt, inv.AmountInWords), "The total "+rupees(*a.Total)+" in words is \""+spelt+"\"; the invoice says \""+inv.AmountInWords+"\".")
		}
	}
	if inv.Date != "" {
		if d, err := time.Parse(time.DateOnly, inv.Date); err == nil && d.After(time.Now().AddDate(0, 0, 1)) {
			add("date_not_in_future", false, "The invoice is dated "+inv.Date+", after today.")
		}
	}
	if inv.Number != "" {
		ok := len(inv.Number) <= 16 && regexp.MustCompile(`^[A-Za-z0-9/-]+$`).MatchString(inv.Number)
		add("invoice_number_format", ok, "Rule 46 of the CGST Rules: at most 16 characters, letters, digits, hyphen and slash. This one is "+strconv.Itoa(len(inv.Number))+" characters.")
	}
}

// sameWords compares two amounts in words, ignoring case, punctuation, word order of
// "rupees", and the filler words.
func sameWords(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.ReplaceAll(s, "-", " "))
		var out []string
		for _, f := range strings.FieldsFunc(s, func(r rune) bool { return !(r >= 'a' && r <= 'z') }) {
			switch f {
			case "rupees", "rupee", "inr", "rs", "only", "and", "indian":
				continue
			}
			out = append(out, f)
		}
		return strings.Join(out, " ")
	}
	return norm(a) == norm(b)
}
