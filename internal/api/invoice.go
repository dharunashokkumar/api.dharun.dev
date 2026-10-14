package api

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
	"github.com/dharunashokkumar/api.dharun.dev/internal/words"
)

// A GST tax invoice from JSON: the server works out each line's taxable value, CGST and
// SGST (or UTGST) for a sale within a state, IGST for one across states, the HSN-wise
// summary, the rounding and the amount in words, then sets it with Typst. format=json
// returns the worked-out invoice without the PDF.

//go:embed templates/invoice.typ
var invoiceTemplate []byte

type invoiceParty struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	GSTIN   string `json:"gstin"`
	State   string `json:"state"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
}

type invoiceBank struct {
	Name    string `json:"name"`
	Account string `json:"account"`
	IFSC    string `json:"ifsc"`
	Branch  string `json:"branch"`
	Holder  string `json:"holder"`
}

type invoiceItem struct {
	Description string   `json:"description"`
	HSN         string   `json:"hsn"`
	Quantity    *float64 `json:"quantity"`
	Unit        string   `json:"unit"`
	Rate        float64  `json:"rate"`
	Discount    float64  `json:"discount"`
	GSTRate     *float64 `json:"gst_rate"`
}

type invoiceRequest struct {
	Title         string        `json:"title"`
	Number        string        `json:"number"`
	Date          string        `json:"date"`
	DueDate       string        `json:"due_date"`
	PlaceOfSupply string        `json:"place_of_supply"`
	ReverseCharge bool          `json:"reverse_charge"`
	Seller        invoiceParty  `json:"seller"`
	Buyer         invoiceParty  `json:"buyer"`
	ShipTo        *invoiceParty `json:"ship_to"`
	Items         []invoiceItem `json:"items"`
	Bank          *invoiceBank  `json:"bank"`
	UPI           string        `json:"upi"`
	Notes         string        `json:"notes"`
	Terms         string        `json:"terms"`
}

// The worked-out invoice. Amounts are rupees rounded to paise.
type invoiceLine struct {
	Number      int     `json:"number"`
	Description string  `json:"description"`
	HSN         string  `json:"hsn"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	Rate        float64 `json:"rate"`
	Discount    float64 `json:"discount"`
	Taxable     float64 `json:"taxable_value"`
	GSTRate     float64 `json:"gst_rate"`
	CGST        float64 `json:"cgst"`
	SGST        float64 `json:"sgst"`
	IGST        float64 `json:"igst"`
	Total       float64 `json:"total"`
}

type invoiceTaxRow struct {
	HSN     string  `json:"hsn"`
	GSTRate float64 `json:"gst_rate"`
	Taxable float64 `json:"taxable_value"`
	CGST    float64 `json:"cgst"`
	SGST    float64 `json:"sgst"`
	IGST    float64 `json:"igst"`
	Tax     float64 `json:"total_tax"`
}

type invoiceStateView struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	GSTCode string `json:"gst_code"`
}

type invoiceTotals struct {
	Taxable  float64 `json:"taxable_value"`
	CGST     float64 `json:"cgst"`
	SGST     float64 `json:"sgst"`
	IGST     float64 `json:"igst"`
	Tax      float64 `json:"total_tax"`
	RoundOff float64 `json:"round_off"`
	Total    float64 `json:"total"`
	Words    string  `json:"in_words"`
}

type invoiceResult struct {
	Title         string           `json:"title"`
	Number        string           `json:"number"`
	Date          string           `json:"date"`
	DueDate       string           `json:"due_date,omitempty"`
	Supply        string           `json:"supply"` // intra_state or inter_state
	StateTax      string           `json:"state_tax"`
	PlaceOfSupply invoiceStateView `json:"place_of_supply"`
	SellerState   invoiceStateView `json:"seller_state"`
	ReverseCharge bool             `json:"reverse_charge"`
	Lines         []invoiceLine    `json:"lines"`
	Summary       []invoiceTaxRow  `json:"hsn_summary"`
	Totals        invoiceTotals    `json:"totals"`
}

// unionTerritoryTax lists the union territories without a legislature, where the
// state part of the tax is UTGST rather than SGST.
var unionTerritoryTax = map[string]bool{"04": true, "26": true, "31": true, "35": true, "38": true}

const invoiceMaxItems = 200

var (
	blackRGBA = color.RGBA{A: 0xff}
	whiteRGBA = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

func paise(value float64) float64 { return math.Round(value*100) / 100 }

func stateView(state india.State) invoiceStateView {
	return invoiceStateView{Code: state.Code, Name: state.Name, GSTCode: state.GSTCode}
}

// partyState finds a party's state from its GSTIN, or from what it names.
func partyState(party invoiceParty, role string) (india.State, bool, error) {
	if party.GSTIN != "" {
		check := validate.GSTIN(party.GSTIN)
		if !check.Valid {
			return india.State{}, false, fmt.Errorf("%s.gstin is not a valid GSTIN: %s", role, check.Reason)
		}
		state, ok := india.ByGSTCode(check.Normalized[:2])
		if !ok {
			return india.State{}, false, fmt.Errorf("%s.gstin starts with %s, which is not a state's code", role, check.Normalized[:2])
		}
		return state, true, nil
	}
	if party.State != "" {
		state, ok := india.Lookup(party.State)
		if !ok {
			state, ok = india.ByGSTCode(party.State)
		}
		if !ok {
			return india.State{}, false, fmt.Errorf("%s.state is not a state or union territory: use a code such as TN, a name, or a GST state code such as 33", role)
		}
		return state, true, nil
	}
	return india.State{}, false, nil
}

func workInvoice(request invoiceRequest, today time.Time) (invoiceResult, error) {
	var result invoiceResult
	if strings.TrimSpace(request.Seller.Name) == "" {
		return result, errors.New("seller.name is required")
	}
	if request.Seller.GSTIN == "" {
		return result, errors.New("seller.gstin is required: a tax invoice is issued by a registered seller")
	}
	if strings.TrimSpace(request.Buyer.Name) == "" {
		return result, errors.New("buyer.name is required")
	}
	if len(request.Items) == 0 || len(request.Items) > invoiceMaxItems {
		return result, fmt.Errorf("items must hold from 1 to %d lines", invoiceMaxItems)
	}
	seller, _, err := partyState(request.Seller, "seller")
	if err != nil {
		return result, err
	}
	buyer, buyerKnown, err := partyState(request.Buyer, "buyer")
	if err != nil {
		return result, err
	}
	place := seller
	switch {
	case request.PlaceOfSupply != "":
		var ok bool
		if place, ok = india.Lookup(request.PlaceOfSupply); !ok {
			if place, ok = india.ByGSTCode(request.PlaceOfSupply); !ok {
				return result, errors.New("place_of_supply is not a state or union territory: use a code such as TN, a name, or a GST state code such as 33")
			}
		}
	case buyerKnown:
		place = buyer
	}
	if request.ShipTo != nil {
		if _, _, err := partyState(*request.ShipTo, "ship_to"); err != nil {
			return result, err
		}
	}
	if request.UPI != "" && !upiAddress.MatchString(request.UPI) {
		return result, errors.New("upi must be a UPI ID, such as shop@okaxis")
	}
	if request.Bank != nil && request.Bank.IFSC != "" && !validate.IFSC(request.Bank.IFSC).Valid {
		return result, errors.New("bank.ifsc is not a valid IFSC")
	}

	date := today
	if request.Date != "" {
		if date, err = time.Parse("2006-01-02", request.Date); err != nil {
			return result, errors.New("date must be YYYY-MM-DD")
		}
	}
	result.Date = date.Format("2006-01-02")
	if request.DueDate != "" {
		due, err := time.Parse("2006-01-02", request.DueDate)
		if err != nil {
			return result, errors.New("due_date must be YYYY-MM-DD")
		}
		result.DueDate = due.Format("2006-01-02")
	}
	result.Title = strings.TrimSpace(request.Title)
	if result.Title == "" {
		result.Title = "Tax Invoice"
	}
	result.Number = strings.TrimSpace(request.Number)
	if result.Number == "" {
		return result, errors.New("number is required: every GST invoice has a serial number, at most 16 characters")
	}
	if len(result.Number) > 16 || strings.IndexFunc(result.Number, func(r rune) bool {
		return !(r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '/'))
	}) >= 0 {
		return result, errors.New("number may hold at most 16 letters, digits, - and /, as the GST rules ask")
	}
	result.ReverseCharge = request.ReverseCharge
	result.PlaceOfSupply, result.SellerState = stateView(place), stateView(seller)
	intra := place.GSTCode == seller.GSTCode
	result.Supply, result.StateTax = "inter_state", ""
	if intra {
		result.Supply, result.StateTax = "intra_state", "SGST"
		if unionTerritoryTax[place.GSTCode] {
			result.StateTax = "UTGST"
		}
	}

	type key struct {
		hsn  string
		rate float64
	}
	summary := map[key]*invoiceTaxRow{}
	for i, item := range request.Items {
		where := fmt.Sprintf("items[%d]", i)
		if strings.TrimSpace(item.Description) == "" {
			return result, errors.New(where + ".description is required")
		}
		hsn := strings.ReplaceAll(item.HSN, " ", "")
		if hsn != "" && (len(hsn) < 4 || len(hsn) > 8 || strings.Trim(hsn, "0123456789") != "") {
			return result, errors.New(where + ".hsn must be 4 to 8 digits")
		}
		quantity := 1.0
		if item.Quantity != nil {
			quantity = *item.Quantity
		}
		if quantity <= 0 || quantity > 1e9 || item.Rate < 0 || item.Rate > 1e12 || item.Discount < 0 {
			return result, errors.New(where + ": quantity must be above zero, and rate and discount zero or more")
		}
		if item.GSTRate == nil {
			return result, errors.New(where + ".gst_rate is required, in percent, such as 18 (0 for exempt goods)")
		}
		rate := *item.GSTRate
		if rate < 0 || rate > 40 {
			return result, errors.New(where + ".gst_rate is a percentage from 0 to 40")
		}
		line := invoiceLine{
			Number: i + 1, Description: strings.TrimSpace(item.Description), HSN: hsn, Quantity: quantity,
			Unit: strings.TrimSpace(item.Unit), Rate: item.Rate, Discount: paise(item.Discount), GSTRate: rate,
		}
		if line.Unit == "" {
			line.Unit = "Nos"
		}
		line.Taxable = paise(paise(quantity*item.Rate) - line.Discount)
		if line.Taxable < 0 {
			return result, errors.New(where + ".discount is more than the line's value")
		}
		if intra {
			line.CGST = paise(line.Taxable * rate / 200)
			line.SGST = line.CGST
		} else {
			line.IGST = paise(line.Taxable * rate / 100)
		}
		line.Total = paise(line.Taxable + line.CGST + line.SGST + line.IGST)
		result.Lines = append(result.Lines, line)

		row := summary[key{hsn, rate}]
		if row == nil {
			row = &invoiceTaxRow{HSN: hsn, GSTRate: rate}
			summary[key{hsn, rate}] = row
		}
		row.Taxable = paise(row.Taxable + line.Taxable)
		row.CGST = paise(row.CGST + line.CGST)
		row.SGST = paise(row.SGST + line.SGST)
		row.IGST = paise(row.IGST + line.IGST)
		row.Tax = paise(row.CGST + row.SGST + row.IGST)

		t := &result.Totals
		t.Taxable, t.CGST, t.SGST, t.IGST = paise(t.Taxable+line.Taxable), paise(t.CGST+line.CGST), paise(t.SGST+line.SGST), paise(t.IGST+line.IGST)
	}
	for _, row := range summary {
		result.Summary = append(result.Summary, *row)
	}
	sort.Slice(result.Summary, func(a, b int) bool {
		if result.Summary[a].HSN != result.Summary[b].HSN {
			return result.Summary[a].HSN < result.Summary[b].HSN
		}
		return result.Summary[a].GSTRate < result.Summary[b].GSTRate
	})
	t := &result.Totals
	t.Tax = paise(t.CGST + t.SGST + t.IGST)
	exact := paise(t.Taxable + t.Tax)
	t.Total = math.Round(exact)
	t.RoundOff = paise(t.Total - exact)
	if t.Total > float64(words.MaxValue) {
		return result, errors.New("the total is larger than this API spells out")
	}
	spelled, err := words.Currency("en", int64(t.Total), 0)
	if err != nil {
		return result, err
	}
	t.Words = upperFirst(spelled)
	return result, nil
}

func upperFirst(text string) string {
	runes := []rune(text)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

// inr writes rupees with Indian digit grouping: 12,34,567.80.
func inr(value float64) string {
	negative := value < 0
	text := strconv.FormatFloat(math.Abs(value), 'f', 2, 64)
	whole, fraction, _ := strings.Cut(text, ".")
	var groups []string
	if len(whole) > 3 {
		groups = append(groups, whole[len(whole)-3:])
		whole = whole[:len(whole)-3]
		for len(whole) > 2 {
			groups = append([]string{whole[len(whole)-2:]}, groups...)
			whole = whole[:len(whole)-2]
		}
	} else {
		groups = []string{whole}
		whole = ""
	}
	if whole != "" {
		groups = append([]string{whole}, groups...)
	}
	out := strings.Join(groups, ",") + "." + fraction
	if negative {
		out = "-" + out
	}
	return out
}

// plainNumber writes a quantity or rate without needless decimals.
func plainNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (s *Server) handleInvoice(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "pdf"
	}
	if format != "pdf" && format != "json" && format != "png" {
		s.badRequest(w, r, "format must be pdf, png (the first page) or json (the worked-out invoice, without a document).")
		return
	}
	var request invoiceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, int64(s.cfg.MaxBodyBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		if isTooLarge(err) {
			s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The body is larger than "+byteSize(s.cfg.MaxBodyBytes)+".")
			return
		}
		message := "The body is not a valid invoice: " + strings.TrimPrefix(err.Error(), "json: ") + "."
		if errors.Is(err, io.EOF) {
			message = "POST the invoice as JSON. The documentation shows every field."
		}
		s.badRequest(w, r, message)
		return
	}
	result, err := workInvoice(request, time.Now().In(indiaTime))
	if err != nil {
		s.badRequest(w, r, upperFirst(err.Error())+".")
		return
	}
	if format == "json" {
		s.ok(w, r, result, meta{Note: "Amounts in rupees. CGST and SGST (or UTGST) for a supply within the seller's state, IGST across states. The total is rounded to the rupee."}, cacheNone)
		return
	}

	dir, done, err := workspace()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	document, qr, err := invoiceDocument(request, result)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	files := map[string][]byte{"main.typ": invoiceTemplate, "invoice.json": document}
	if qr != nil {
		files["upi.svg"] = qr
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
	}
	out := typstOutput{format: format, page: 1, ppi: 144}
	body, err := s.compileTypst(r.Context(), dir, out)
	if err != nil {
		var mistake *typstError
		if errors.As(err, &mistake) {
			logError("invoice template", errors.New("typst could not compile the template"))
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "The invoice could not be set. Please report this.")
			return
		}
		s.typstFailed(w, r, err)
		return
	}
	sendFile(w, r, body, typstFormats[format], "invoice-"+result.Number+"."+format)
}

// invoiceDocument is what the Typst template reads: every value already written out,
// so the template only lays it out.
func invoiceDocument(request invoiceRequest, result invoiceResult) ([]byte, []byte, error) {
	party := func(p invoiceParty) map[string]any {
		state, known, _ := partyState(p, "")
		view := map[string]any{"name": p.Name, "address": p.Address, "gstin": strings.ToUpper(strings.ReplaceAll(p.GSTIN, " ", "")), "phone": p.Phone, "email": p.Email, "state": ""}
		if known {
			view["state"] = state.Name + " (" + state.GSTCode + ")"
		}
		return view
	}
	intra := result.Supply == "intra_state"
	var lines []map[string]any
	for _, line := range result.Lines {
		tax := line.CGST + line.SGST + line.IGST
		lines = append(lines, map[string]any{
			"number": itoa(line.Number), "description": line.Description, "hsn": line.HSN,
			"quantity": plainNumber(line.Quantity) + " " + line.Unit, "rate": inr(line.Rate),
			"discount": inr(line.Discount), "has_discount": line.Discount > 0,
			"taxable": inr(line.Taxable), "gst_rate": plainNumber(line.GSTRate) + "%", "tax": inr(paise(tax)), "total": inr(line.Total),
		})
	}
	var summary []map[string]any
	for _, row := range result.Summary {
		summary = append(summary, map[string]any{
			"hsn": row.HSN, "taxable": inr(row.Taxable), "rate": plainNumber(row.GSTRate) + "%",
			"half_rate": plainNumber(row.GSTRate/2) + "%", "cgst": inr(row.CGST), "sgst": inr(row.SGST), "igst": inr(row.IGST), "tax": inr(row.Tax),
		})
	}
	totals := []map[string]any{{"label": "Taxable value", "amount": inr(result.Totals.Taxable)}}
	if intra {
		totals = append(totals,
			map[string]any{"label": "CGST", "amount": inr(result.Totals.CGST)},
			map[string]any{"label": result.StateTax, "amount": inr(result.Totals.SGST)})
	} else {
		totals = append(totals, map[string]any{"label": "IGST", "amount": inr(result.Totals.IGST)})
	}
	if result.Totals.RoundOff != 0 {
		totals = append(totals, map[string]any{"label": "Round off", "amount": inr(result.Totals.RoundOff)})
	}
	document := map[string]any{
		"title": result.Title, "number": result.Number, "date": indianDate(result.Date), "due_date": indianDate(result.DueDate),
		"place_of_supply": result.PlaceOfSupply.Name + " (" + result.PlaceOfSupply.GSTCode + ")",
		"reverse_charge":  map[bool]string{true: "Yes", false: "No"}[result.ReverseCharge],
		"intra":           intra, "state_tax": result.StateTax,
		"seller": party(request.Seller), "buyer": party(request.Buyer), "ship_to": nil,
		"lines": lines, "summary": summary, "totals": totals,
		"total": inr(result.Totals.Total), "words": result.Totals.Words,
		"tax_total": inr(result.Totals.Tax), "taxable_total": inr(result.Totals.Taxable),
		"cgst_total": inr(result.Totals.CGST), "sgst_total": inr(result.Totals.SGST), "igst_total": inr(result.Totals.IGST),
		"any_discount": false, "bank": nil, "upi": "", "notes": request.Notes, "terms": request.Terms,
	}
	for _, line := range result.Lines {
		if line.Discount > 0 {
			document["any_discount"] = true
		}
	}
	if request.ShipTo != nil {
		document["ship_to"] = party(*request.ShipTo)
	}
	if request.Bank != nil {
		document["bank"] = map[string]any{"name": request.Bank.Name, "account": request.Bank.Account, "ifsc": strings.ToUpper(request.Bank.IFSC), "branch": request.Bank.Branch, "holder": request.Bank.Holder}
	}
	var qr []byte
	if request.UPI != "" && result.Totals.Total > 0 {
		values := map[string]string{"pa": request.UPI, "pn": clip(request.Seller.Name, 80), "am": strconv.FormatFloat(result.Totals.Total, 'f', 2, 64), "tn": "Invoice " + result.Number}
		link, err := buildUPILink(func(name string) string { return values[name] })
		if err != nil {
			return nil, nil, err
		}
		code, err := encodeQR(link.Link, "m")
		if err != nil {
			return nil, nil, err
		}
		qr, _, err = drawing{format: "svg", margin: 2, scale: 256, foreground: blackRGBA, background: whiteRGBA}.render(readModules(code))
		if err != nil {
			return nil, nil, err
		}
		document["upi"] = request.UPI
	}
	encoded, err := json.Marshal(document)
	return encoded, qr, err
}

// indianDate writes 2026-10-14 as 14 Oct 2026.
func indianDate(date string) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	return parsed.Format("02 Jan 2006")
}
