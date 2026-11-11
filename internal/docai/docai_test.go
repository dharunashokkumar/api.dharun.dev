package docai

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func load(t *testing.T, name string) []Page {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := ParseBBox(data)
	if err != nil {
		t.Fatal(err)
	}
	return pages
}

func TestMarkdownOfSampleDocument(t *testing.T) {
	pages := load(t, "document.html")
	if len(pages) != 2 {
		t.Fatalf("pages = %d", len(pages))
	}
	md := Markdown(pages)
	if !strings.HasPrefix(md, "# India API sample document\n\n") || !strings.Contains(md, "\n# Page two\n") {
		t.Errorf("headings:\n%s", md)
	}
	for _, want := range []string{"This two-page PDF is a sample", "\n\n---\n\n", "வணக்கம்", "नमस्ते"} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown lacks %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "endpoints to\nread") {
		t.Error("lines of a paragraph were not joined")
	}
}

func TestHeadingsListsAndHyphens(t *testing.T) {
	line := func(y, h float64, text string) Line {
		var words []Word
		x := 50.0
		for _, f := range strings.Fields(text) {
			words = append(words, Word{Text: f, X0: x, Y0: y, X1: x + 6*float64(len(f)), Y1: y + h})
			x += 6*float64(len(f)) + 3
		}
		return Line{Words: words, X0: 50, Y0: y, X1: x, Y1: y + h}
	}
	page := Page{Width: 600, Height: 800, Blocks: []Block{
		{Lines: []Line{line(40, 24, "Annual report")}},
		{Lines: []Line{line(80, 10, "The company grew its sales and its profits in every quar-"), line(92, 10, "ter of the year, and opened new shops in four states.")}},
		{Lines: []Line{line(120, 15, "Highlights")}},
		{Lines: []Line{line(140, 10, "• Sales up a fifth"), line(152, 10, "• Profit up a tenth"), line(164, 10, "2. Two new states")}},
		{Lines: []Line{line(785, 10, "3")}},
	}}
	md := Markdown([]Page{page})
	for _, want := range []string{"# Annual report", "every quarter of the year", "## Highlights", "- Sales up a fifth\n- Profit up a tenth\n1. Two new states"} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown lacks %q:\n%s", want, md)
		}
	}
	if strings.HasSuffix(strings.TrimSpace(md), "3") {
		t.Errorf("page number kept:\n%s", md)
	}
}

func TestMarkdownOfInvoice(t *testing.T) {
	md := Markdown(load(t, "invoice.html"))
	for _, want := range []string{"# Sri Murugan Stores · TAX INVOICE\n", "\n| 1 | Basmati rice, 25 kg bag | 1006 | 4 Bag | 2,150.00 | — | 8,600.00 | 5% | 430.00 | 9,030.00 |\n", "Goods once sold will not be taken back."} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown lacks %q:\n%s", want, md)
		}
	}
}

func TestTablesOfInvoice(t *testing.T) {
	tables := Tables(load(t, "invoice.html"))
	var items *Table
	for i, tb := range tables {
		if len(tb.Rows) > 0 && strings.Contains(strings.Join(tb.Rows[0], " "), "HSN") && strings.Contains(strings.Join(tb.Rows[0], " "), "ITEM") {
			items = &tables[i]
		}
	}
	if items == nil {
		raw, _ := json.MarshalIndent(tables, "", " ")
		t.Fatalf("no item table:\n%s", raw)
	}
	if len(items.Rows) != 3 {
		t.Fatalf("item table rows = %d: %q", len(items.Rows), items.Rows)
	}
	row := strings.Join(items.Rows[1], "|")
	for _, want := range []string{"Basmati rice, 25 kg bag", "1006", "8,600.00", "9,030.00"} {
		if !strings.Contains(row, want) {
			t.Errorf("row 1 %q lacks %q", row, want)
		}
	}
	md := MarkdownTable(*items)
	if !strings.HasPrefix(strings.Split(md, "\n")[1], "|---|") {
		t.Errorf("Markdown table:\n%s", md)
	}
}

func TestReadInvoice(t *testing.T) {
	inv := ReadInvoice(load(t, "invoice.html"))
	raw, _ := json.MarshalIndent(inv, "", " ")
	check := func(name, got, want string) {
		if got != want {
			t.Errorf("%s = %q, want %q\n%s", name, got, want, raw)
		}
	}
	check("number", inv.Number, "INV-2026-0042")
	check("date", inv.Date, "2026-10-14")
	check("seller", inv.Seller.GSTIN, "33AABCS1429B1Z1")
	check("seller name", inv.Seller.Name, "Sri Murugan Stores")
	check("buyer", inv.Buyer.GSTIN, "29AAACK6812C1ZS")
	check("buyer name", inv.Buyer.Name, "Kaveri Traders")
	check("place", inv.PlaceOfSupply, "Karnataka (29)")
	check("supply", inv.Supply, "inter_state")
	check("ifsc", inv.IFSC, "SBIN0000001")
	check("upi", inv.UPI, "srimurugan@okaxis")
	check("hsn", strings.Join(inv.HSN, ","), "1006,7323")
	check("words", inv.AmountInWords, "Thirteen thousand thirteen rupees only")
	a := inv.Amounts
	if value(a.Taxable) != 11975 || value(a.IGST) != 1037.5 || value(a.RoundOff) != 0.5 || value(a.Total) != 13013 || a.CGST != nil {
		t.Errorf("amounts wrong:\n%s", raw)
	}
	for _, c := range inv.Checks {
		if !c.Passed {
			t.Errorf("check %s failed: %s", c.Name, c.Detail)
		}
	}
	if len(inv.Checks) < 6 {
		t.Errorf("checks = %d:\n%s", len(inv.Checks), raw)
	}
}

func TestInvoiceChecksCatchMistakes(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	inv := Invoice{Seller: Party{GSTIN: "33AABCS1429B1Z2"}, Buyer: Party{GSTIN: "33AAACK6812C1ZS"}, Number: "INVOICE-NUMBER-TOO-LONG",
		Amounts: Amounts{Taxable: f(100), CGST: f(9), SGST: f(8), IGST: f(18), Total: f(140)}, AmountInWords: "One hundred rupees only"}
	inv.check()
	failed := map[string]bool{}
	for _, c := range inv.Checks {
		if !c.Passed {
			failed[c.Name] = true
		}
	}
	for _, want := range []string{"seller_gstin", "total_adds_up", "cgst_equals_sgst", "one_kind_of_tax", "amount_in_words", "invoice_number_format"} {
		if !failed[want] {
			t.Errorf("%s did not fail: %+v", want, inv.Checks)
		}
	}
}

func TestParseDate(t *testing.T) {
	for in, want := range map[string]string{"14 Oct 2026": "2026-10-14", "14/10/2026": "2026-10-14", "4-Oct-2026": "2026-10-04", "2026-10-14": "2026-10-14", "14.10.2026": "2026-10-14", "October 14, 2026": "2026-10-14"} {
		if got, ok := ParseDate(in); !ok || got != want {
			t.Errorf("ParseDate(%q) = %q, %v", in, got, ok)
		}
	}
	if _, ok := ParseDate("31/02/2026"); ok {
		t.Error("31 February parsed")
	}
}

func TestSameWords(t *testing.T) {
	if !sameWords("thirteen thousand thirteen rupees only", "Rupees Thirteen Thousand Thirteen Only") {
		t.Error("same amount differs")
	}
	if sameWords("thirteen thousand rupees only", "Rupees Thirteen Thousand Thirteen Only") {
		t.Error("different amounts same")
	}
}
