// Package payqr reads the text inside Indian payment QR codes: UPI links
// (upi://pay?pa=...) and EMVCo merchant-presented QR strings, which Bharat QR uses.
// It reads what the code says; it cannot tell whether the payee is genuine.
package payqr

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Result is what a payment code holds.
type Result struct {
	Kind         string            `json:"kind"` // upi or emvco
	Payee        string            `json:"payee,omitempty"`
	PayeeName    string            `json:"payeeName,omitempty"`
	Amount       string            `json:"amount,omitempty"`
	Currency     string            `json:"currency,omitempty"`
	Note         string            `json:"note,omitempty"`
	MerchantCode string            `json:"merchantCategoryCode,omitempty"`
	MerchantKind string            `json:"merchantCategory,omitempty"`
	Reference    string            `json:"reference,omitempty"`
	City         string            `json:"city,omitempty"`
	PostalCode   string            `json:"postalCode,omitempty"`
	Country      string            `json:"country,omitempty"`
	Dynamic      *bool             `json:"dynamic,omitempty"` // EMVCo: made for one payment
	Networks     []string          `json:"networks,omitempty"`
	CRCValid     *bool             `json:"crcValid,omitempty"`
	Fields       map[string]string `json:"fields"`
	Tags         []Tag             `json:"tags,omitempty"`
	Warnings     []string          `json:"warnings"`
}

// Tag is one EMVCo field.
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
	Tags  []Tag  `json:"tags,omitempty"`
}

// Parse reads a UPI link or an EMVCo string.
func Parse(text string) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, errors.New("give the text of the QR code")
	}
	if len(text) > 2048 {
		return Result{}, errors.New("the text is longer than any payment code (2,048 characters at most)")
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "upi:") {
		return parseUPI(text)
	}
	if strings.HasPrefix(text, "000201") {
		return parseEMVCo(text)
	}
	return Result{}, errors.New("this is neither a UPI link (upi://pay?...) nor an EMVCo payment string (starting 000201)")
}

var upiNames = map[string]string{
	"pa": "payee address", "pn": "payee name", "am": "amount", "cu": "currency", "tn": "note", "mc": "merchant category code",
	"tr": "transaction reference", "tid": "transaction id", "url": "link", "mode": "mode", "purpose": "purpose", "mam": "minimum amount",
	"orgid": "organisation id", "sign": "signature", "qrmedium": "QR medium", "invoiceno": "invoice number", "invoicedate": "invoice date",
	"gstin": "GSTIN", "gstbrkup": "GST breakup", "category": "category",
}

func parseUPI(text string) (Result, error) {
	link, err := url.Parse(text)
	if err != nil {
		return Result{}, errors.New("the UPI link cannot be read: " + err.Error())
	}
	query, err := url.ParseQuery(link.RawQuery)
	if err != nil {
		return Result{}, errors.New("the UPI link's parameters cannot be read")
	}
	r := Result{Kind: "upi", Fields: map[string]string{}, Warnings: []string{}}
	for key, values := range query {
		r.Fields[strings.ToLower(key)] = values[0]
	}
	action := strings.ToLower(strings.Trim(link.Host+link.Path, "/"))
	if action != "pay" && action != "mandate" && action != "collect" {
		r.Warnings = append(r.Warnings, "The link's action is "+strconv.Quote(action)+", not pay.")
	}
	r.Payee, r.PayeeName, r.Amount, r.Currency, r.Note = r.Fields["pa"], r.Fields["pn"], r.Fields["am"], r.Fields["cu"], r.Fields["tn"]
	r.MerchantCode, r.Reference = r.Fields["mc"], r.Fields["tr"]
	if r.MerchantCode != "" {
		r.MerchantKind = MerchantCategory(r.MerchantCode)
	}
	if r.Payee == "" {
		r.Warnings = append(r.Warnings, "There is no payee address (pa).")
	} else if !validVPA(r.Payee) {
		r.Warnings = append(r.Warnings, "The payee address does not look like a UPI ID (name@handle).")
	}
	if r.Amount != "" {
		if v, err := strconv.ParseFloat(r.Amount, 64); err != nil || v < 0 {
			r.Warnings = append(r.Warnings, "The amount is not a number.")
		}
	}
	if r.Currency != "" && r.Currency != "INR" {
		r.Warnings = append(r.Warnings, "The currency is "+r.Currency+"; UPI pays in INR.")
	}
	return r, nil
}

func validVPA(vpa string) bool {
	name, handle, ok := strings.Cut(vpa, "@")
	if !ok || name == "" || handle == "" || len(vpa) > 255 {
		return false
	}
	for _, c := range name + handle {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

var emvNames = map[string]string{
	"00": "payload format", "01": "point of initiation", "52": "merchant category code", "53": "currency", "54": "amount",
	"55": "tip or convenience indicator", "56": "convenience fee (fixed)", "57": "convenience fee (percent)", "58": "country",
	"59": "merchant name", "60": "merchant city", "61": "postal code", "62": "additional data", "63": "CRC", "64": "merchant name in another language",
}

var additionalNames = map[string]string{"01": "bill number", "02": "mobile number", "03": "store label", "04": "loyalty number", "05": "reference label",
	"06": "customer label", "07": "terminal label", "08": "purpose", "09": "additional consumer data request"}

func accountName(id string) string {
	n, _ := strconv.Atoi(id)
	switch {
	case n >= 2 && n <= 3:
		return "Visa"
	case n >= 4 && n <= 5:
		return "Mastercard"
	case n >= 6 && n <= 8:
		return "EMVCo"
	case n >= 9 && n <= 10:
		return "Discover"
	case n >= 11 && n <= 12:
		return "Amex"
	case n >= 13 && n <= 14:
		return "JCB"
	case n >= 15 && n <= 16:
		return "UnionPay"
	case n >= 17 && n <= 25:
		return "EMVCo (reserved)"
	case n >= 26 && n <= 51:
		return "merchant account"
	}
	return ""
}

func tlv(s string) ([]Tag, error) {
	var tags []Tag
	for len(s) > 0 {
		if len(s) < 4 {
			return tags, errors.New("the string ends in the middle of a field")
		}
		id := s[:2]
		length, err := strconv.Atoi(s[2:4])
		if err != nil || length < 0 || len(s) < 4+length {
			return tags, fmt.Errorf("field %s has a bad length", id)
		}
		tags = append(tags, Tag{ID: id, Value: s[4 : 4+length]})
		s = s[4+length:]
	}
	return tags, nil
}

// CRC16 is CRC-16/CCITT-FALSE, the checksum EMVCo QR codes end with.
func CRC16(data string) uint16 {
	crc := uint16(0xFFFF)
	for i := 0; i < len(data); i++ {
		crc ^= uint16(data[i]) << 8
		for b := 0; b < 8; b++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func parseEMVCo(text string) (Result, error) {
	tags, err := tlv(text)
	if err != nil {
		return Result{}, errors.New("the EMVCo string cannot be read: " + err.Error())
	}
	r := Result{Kind: "emvco", Fields: map[string]string{}, Warnings: []string{}}
	seen := map[string]bool{}
	for i := range tags {
		tag := &tags[i]
		tag.Name = emvNames[tag.ID]
		r.Fields[tag.ID] = tag.Value
		if account := accountName(tag.ID); account != "" {
			tag.Name = account
			network := account
			if sub, err := tlv(tag.Value); err == nil && account == "merchant account" {
				for j := range sub {
					switch sub[j].ID {
					case "00":
						sub[j].Name = "globally unique id"
					case "01":
						sub[j].Name = "payee address"
					default:
						sub[j].Name = "field " + sub[j].ID
					}
				}
				tag.Tags = sub
				network = "merchant account " + tag.ID
				for _, s := range sub {
					if s.ID == "00" && strings.Contains(strings.ToLower(s.Value), "a000000677") {
						network = "UPI"
					}
					if s.ID == "01" && validVPA(s.Value) && r.Payee == "" {
						r.Payee = s.Value
					}
				}
			}
			if !seen[network] {
				r.Networks = append(r.Networks, network)
				seen[network] = true
			}
		}
		if tag.ID == "62" {
			if sub, err := tlv(tag.Value); err == nil {
				for j := range sub {
					sub[j].Name = additionalNames[sub[j].ID]
				}
				tag.Tags = sub
				for _, s := range sub {
					if s.ID == "05" || (s.ID == "01" && r.Reference == "") {
						r.Reference = s.Value
					}
				}
			}
		}
	}
	r.Tags = tags
	f := r.Fields
	if f["00"] != "01" {
		r.Warnings = append(r.Warnings, "The payload format is not 01.")
	}
	switch f["01"] {
	case "11":
		r.Dynamic = new(bool)
	case "12":
		r.Dynamic = new(bool)
		*r.Dynamic = true
	}
	r.PayeeName, r.City, r.PostalCode, r.Country, r.Amount = f["59"], f["60"], f["61"], f["58"], f["54"]
	r.MerchantCode = f["52"]
	if r.MerchantCode != "" {
		r.MerchantKind = MerchantCategory(r.MerchantCode)
	}
	switch f["53"] {
	case "356":
		r.Currency = "INR"
	case "":
	default:
		r.Currency = f["53"]
	}
	if crc, ok := f["63"]; ok {
		at := strings.LastIndex(text, "6304")
		valid := at >= 0 && at+8 == len(text) && strings.EqualFold(fmt.Sprintf("%04X", CRC16(text[:at+4])), crc)
		r.CRCValid = &valid
		if !valid {
			r.Warnings = append(r.Warnings, "The checksum (CRC) does not match: the text was changed or misread.")
		}
	} else {
		r.Warnings = append(r.Warnings, "There is no checksum (field 63).")
	}
	return r, nil
}

// MerchantCategory names the common merchant category codes (ISO 18245).
func MerchantCategory(code string) string {
	return merchantCategories[code]
}

var merchantCategories = map[string]string{
	"0000": "person to person", "4111": "local commuter transport", "4121": "taxis", "4131": "bus lines", "4511": "airlines", "4722": "travel agencies",
	"4812": "telecom equipment", "4814": "telecom services", "4900": "utilities", "5045": "computers", "5111": "stationery", "5137": "uniforms",
	"5172": "petroleum products", "5200": "home supply stores", "5251": "hardware stores", "5261": "nurseries and garden", "5311": "department stores",
	"5331": "variety stores", "5399": "general merchandise", "5411": "grocery stores and supermarkets", "5422": "meat and fish", "5441": "sweets and confectionery",
	"5451": "dairy products", "5462": "bakeries", "5499": "food stores", "5541": "service stations", "5542": "fuel dispensers", "5651": "clothing",
	"5661": "shoe stores", "5691": "clothing stores", "5712": "furniture", "5722": "household appliances", "5732": "electronics", "5812": "restaurants",
	"5813": "bars", "5814": "fast food", "5912": "pharmacies", "5942": "book stores", "5944": "jewellery", "5945": "toys", "5977": "cosmetics",
	"5999": "retail stores", "6011": "cash withdrawal", "6012": "financial institutions", "6211": "securities brokers", "6300": "insurance",
	"7011": "hotels", "7230": "beauty and barber shops", "7299": "personal services", "7399": "business services", "7512": "car rental",
	"7523": "parking", "7538": "car service", "7832": "cinemas", "7997": "clubs and gyms", "8011": "doctors", "8021": "dentists", "8062": "hospitals",
	"8071": "medical laboratories", "8099": "health services", "8211": "schools", "8220": "colleges and universities", "8299": "education services",
	"8398": "charities", "8661": "religious organisations", "9311": "tax payments", "9399": "government services",
}
