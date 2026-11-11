// Package indiafmt writes numbers and dates the way they are written in India: lakh
// and crore grouping, short forms, Indian-script digits, the financial year, the
// national (Saka) calendar and the tabular Hijri calendar.
package indiafmt

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Scripts lists the Indian scripts whose digits Digits can write, with the code point of
// each script's zero.
var Scripts = map[string]rune{
	"deva": 0x0966, // Devanagari: Hindi, Marathi, Nepali, Sanskrit
	"beng": 0x09E6, // Bengali and Assamese
	"guru": 0x0A66, // Gurmukhi
	"gujr": 0x0AE6, // Gujarati
	"orya": 0x0B66, // Odia
	"taml": 0x0BE6, // Tamil
	"telu": 0x0C66, // Telugu
	"knda": 0x0CE6, // Kannada
	"mlym": 0x0D66, // Malayalam
}

// ScriptNames returns the script codes in a fixed order.
func ScriptNames() []string {
	return []string{"deva", "beng", "guru", "gujr", "orya", "taml", "telu", "knda", "mlym"}
}

// Digits replaces the ASCII digits in s with the script's digits.
func Digits(s, script string) (string, error) {
	zero, ok := Scripts[script]
	if !ok {
		return "", errors.New("unknown script " + script)
	}
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(zero + r - '0')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String(), nil
}

// ToASCII turns Indian-script digits back into ASCII digits.
func ToASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		converted := r
		for _, zero := range Scripts {
			if r >= zero && r <= zero+9 {
				converted = '0' + r - zero
			}
		}
		b.WriteRune(converted)
	}
	return b.String()
}

// Number is a parsed decimal: its sign, whole digits and fraction digits, kept as text
// so that large values lose nothing.
type Number struct {
	Negative bool
	Whole    string
	Fraction string
}

// Parse reads a decimal number. Commas, spaces, underscores, a rupee sign and
// Indian-script digits are accepted.
func Parse(raw string) (Number, error) {
	s := ToASCII(strings.TrimSpace(raw))
	s = strings.NewReplacer(",", "", " ", "", "_", "", "₹", "", "Rs.", "", "Rs", "", "INR", "").Replace(s)
	var n Number
	if strings.HasPrefix(s, "-") {
		n.Negative, s = true, s[1:]
	} else {
		s = strings.TrimPrefix(s, "+")
	}
	if strings.ContainsAny(s, "eE") {
		value, ok := new(big.Float).SetPrec(200).SetString(s)
		if !ok {
			return n, errors.New("not a number")
		}
		s = value.Text('f', 12)
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		if strings.HasPrefix(s, "-") {
			n.Negative, s = !n.Negative, s[1:]
		}
	}
	whole, fraction, _ := strings.Cut(s, ".")
	if whole == "" && fraction == "" {
		return n, errors.New("not a number")
	}
	if strings.Trim(whole, "0123456789") != "" || strings.Trim(fraction, "0123456789") != "" {
		return n, errors.New("not a number")
	}
	if len(whole) > 40 {
		return n, errors.New("too large: at most 40 digits before the point")
	}
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	n.Whole, n.Fraction = whole, fraction
	if n.Whole == "0" && strings.Trim(n.Fraction, "0") == "" {
		n.Negative = false
	}
	return n, nil
}

// Round returns n rounded half away from zero to the given number of decimals.
func (n Number) Round(decimals int) Number {
	if decimals < 0 || len(n.Fraction) <= decimals {
		out := n
		for len(out.Fraction) < decimals {
			out.Fraction += "0"
		}
		return out
	}
	digits := []byte(n.Whole + n.Fraction[:decimals])
	if n.Fraction[decimals] >= '5' {
		i := len(digits) - 1
		for ; i >= 0; i-- {
			if digits[i] == '9' {
				digits[i] = '0'
				continue
			}
			digits[i]++
			break
		}
		if i < 0 {
			digits = append([]byte{'1'}, digits...)
		}
	}
	whole := string(digits[:len(digits)-decimals])
	out := Number{Negative: n.Negative, Whole: strings.TrimLeft(whole, "0"), Fraction: string(digits[len(digits)-decimals:])}
	if out.Whole == "" {
		out.Whole = "0"
	}
	if out.Whole == "0" && strings.Trim(out.Fraction, "0") == "" {
		out.Negative = false
	}
	return out
}

// Float returns the value as a float64 (approximate for very large numbers).
func (n Number) Float() float64 {
	text := n.Whole
	if n.Fraction != "" {
		text += "." + n.Fraction
	}
	v, _ := strconv.ParseFloat(text, 64)
	if n.Negative {
		v = -v
	}
	return v
}

// GroupIndian groups whole digits in the Indian way: the last three, then twos.
func GroupIndian(whole string) string {
	if len(whole) <= 3 {
		return whole
	}
	head, tail := whole[:len(whole)-3], whole[len(whole)-3:]
	var parts []string
	for len(head) > 2 {
		parts = append([]string{head[len(head)-2:]}, parts...)
		head = head[:len(head)-2]
	}
	if head != "" {
		parts = append([]string{head}, parts...)
	}
	return strings.Join(append(parts, tail), ",")
}

// GroupInternational groups whole digits in threes.
func GroupInternational(whole string) string {
	var parts []string
	for len(whole) > 3 {
		parts = append([]string{whole[len(whole)-3:]}, parts...)
		whole = whole[:len(whole)-3]
	}
	return strings.Join(append([]string{whole}, parts...), ",")
}

func (n Number) join(grouped string) string {
	out := grouped
	if n.Fraction != "" {
		out += "." + n.Fraction
	}
	if n.Negative {
		out = "-" + out
	}
	return out
}

// Indian writes n with lakh and crore grouping.
func (n Number) Indian() string { return n.join(GroupIndian(n.Whole)) }

// International writes n grouped in thousands.
func (n Number) International() string { return n.join(GroupInternational(n.Whole)) }

// Rupees writes n as an amount in rupees with paise.
func (n Number) Rupees() string {
	r := n.Round(2)
	out := "₹" + GroupIndian(r.Whole) + "." + r.Fraction
	if r.Negative {
		out = "-" + out
	}
	return out
}

// Short is a compact form: 1.25 Cr, 3.4 L, 12.5 K, with up to the given decimals.
// Long names it in words: 1.25 crore, 3.4 lakh, 12.5 thousand.
type Short struct {
	Short string `json:"short"`
	Long  string `json:"long"`
	Unit  string `json:"unit"`
}

// ShortForm writes the compact form of v.
func ShortForm(v float64, decimals int) Short {
	units := []struct {
		size        float64
		short, long string
	}{{1e7, "Cr", "crore"}, {1e5, "L", "lakh"}, {1e3, "K", "thousand"}}
	abs := math.Abs(v)
	for _, unit := range units {
		if abs >= unit.size {
			text := trimZeros(strconv.FormatFloat(v/unit.size, 'f', decimals, 64))
			value := v / unit.size
			// 1,00,000 crore and more is still written in crore, grouped.
			if math.Abs(value) >= 1000 && unit.short == "Cr" {
				whole := strconv.FormatFloat(math.Abs(math.Round(value)), 'f', 0, 64)
				text = GroupIndian(whole)
				if value < 0 {
					text = "-" + text
				}
			}
			return Short{Short: text + " " + unit.short, Long: text + " " + unit.long, Unit: unit.long}
		}
	}
	text := trimZeros(strconv.FormatFloat(v, 'f', decimals, 64))
	return Short{Short: text, Long: text}
}

func trimZeros(s string) string {
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" {
		return "0"
	}
	return s
}
