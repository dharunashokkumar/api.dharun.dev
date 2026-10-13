package textkit

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
)

// Redaction finds personal identifiers by their published structure, and checks the
// ones that carry a check digit (Aadhaar, cards, GSTIN) so a random long number is not
// masked by mistake. It does not find names or addresses: that needs a language model.

// Kind is one sort of identifier the redactor knows.
type Kind struct {
	ID    string
	Label string
	find  *regexp.Regexp
	// accept confirms a match, given its text with spaces and dashes removed.
	accept func(compact string) bool
	// keep is how many trailing characters partial masking leaves visible.
	keep int
}

func digitsOnly(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
}

var panHolders = "PCHFATBLJG"

// Kinds lists the identifiers in the order they are looked for. Longer and more specific
// patterns come first, so a GSTIN is not also reported as the PAN inside it.
var Kinds = []Kind{
	{ID: "email", Label: "EMAIL", find: regexp.MustCompile(`(?i)\b[a-z0-9._%+-]+@[a-z0-9-]+(\.[a-z0-9-]+)*\.[a-z]{2,}\b`), keep: 0},
	{ID: "upi", Label: "UPI_ID", find: regexp.MustCompile(`(?i)\b[a-z0-9._-]{2,256}@[a-z]{2,64}\b`), keep: 0},
	{ID: "gstin", Label: "GSTIN", find: regexp.MustCompile(`\b[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]\b`),
		accept: func(c string) bool { return validate.GSTIN(c).Valid }, keep: 3},
	// Cards: 4-4-4-4 (with an optional 3 more), Amex 4-6-5, or one unbroken run of 13 to 19.
	{ID: "card", Label: "CARD_NUMBER", find: regexp.MustCompile(`\b(?:[0-9]{4}[ -]?){3}[0-9]{4}(?:[ -]?[0-9]{3})?\b|\b[0-9]{4}[ -]?[0-9]{6}[ -]?[0-9]{5}\b|\b[0-9]{13,19}\b`),
		accept: func(c string) bool { d := digitsOnly(c); return len(d) >= 13 && len(d) <= 19 && validate.LuhnValid(d) }, keep: 4},
	{ID: "aadhaar", Label: "AADHAAR", find: regexp.MustCompile(`\b[2-9][0-9]{3}[ -]?[0-9]{4}[ -]?[0-9]{4}\b`),
		accept: func(c string) bool { return validate.VerhoeffValid(digitsOnly(c)) }, keep: 4},
	{ID: "pan", Label: "PAN", find: regexp.MustCompile(`\b[A-Z]{3}[` + panHolders + `][A-Z][0-9]{4}[A-Z]\b`), keep: 2},
	// Indian mobile numbers, with +91, 91 or 0 in front, or nothing.
	{ID: "phone", Label: "PHONE", find: regexp.MustCompile(`(?:(?:\+91|\b91)[ -]?|\b0|\b)[6-9][0-9]{4}[ -]?[0-9]{5}\b`), keep: 2},
	{ID: "vehicle", Label: "VEHICLE_NUMBER", find: regexp.MustCompile(`\b[A-Z]{2}[ -]?[0-9]{1,2}[ -]?[A-Z]{1,3}[ -]?[0-9]{4}\b`), keep: 0},
	{ID: "voter_id", Label: "VOTER_ID", find: regexp.MustCompile(`\b[A-Z]{3}[0-9]{7}\b`), keep: 0},
	{ID: "passport", Label: "PASSPORT", find: regexp.MustCompile(`\b[A-PR-WY][1-9][0-9]{6}\b`), keep: 0},
	{ID: "ip", Label: "IP_ADDRESS", find: regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|1?[0-9]?[0-9])\.){3}(?:25[0-5]|2[0-4][0-9]|1?[0-9]?[0-9])\b`), keep: 0},
}

// KindIDs lists the identifiers by ID.
func KindIDs() []string {
	ids := make([]string, len(Kinds))
	for i, kind := range Kinds {
		ids[i] = kind.ID
	}
	return ids
}

// Finding is one identifier found. Start and End are character offsets in the input.
// The value itself is never returned.
type Finding struct {
	Type  string `json:"type"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// RedactResult is the redacted text and what was found where.
type RedactResult struct {
	Text     string         `json:"text"`
	Findings []Finding      `json:"findings"`
	Counts   map[string]int `json:"counts"`
}

// Redact replaces the identifiers of the given kinds (all when kinds is empty). With
// partial, the last few characters of numbers stay visible, as on a bank statement.
func Redact(text string, kinds []string, partial bool) RedactResult {
	wanted := map[string]bool{}
	for _, kind := range kinds {
		wanted[kind] = true
	}
	type hit struct {
		start, end int
		kind       *Kind
	}
	var hits []hit
	taken := func(start, end int) bool {
		for _, h := range hits {
			if start < h.end && end > h.start {
				return true
			}
		}
		return false
	}
	for i := range Kinds {
		kind := &Kinds[i]
		if len(wanted) > 0 && !wanted[kind.ID] {
			continue
		}
		for _, match := range kind.find.FindAllStringIndex(text, -1) {
			value := text[match[0]:match[1]]
			if kind.accept != nil && !kind.accept(strings.NewReplacer(" ", "", "-", "").Replace(value)) {
				continue
			}
			// An email address is not also a UPI ID, nor a card number also an Aadhaar.
			if taken(match[0], match[1]) {
				continue
			}
			hits = append(hits, hit{match[0], match[1], kind})
		}
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].start < hits[b].start })

	result := RedactResult{Findings: []Finding{}, Counts: map[string]int{}}
	var out strings.Builder
	last := 0
	for _, h := range hits {
		out.WriteString(text[last:h.start])
		out.WriteString(mask(text[h.start:h.end], h.kind, partial))
		last = h.end
		result.Findings = append(result.Findings, Finding{
			Type: h.kind.ID, Start: utf8.RuneCountInString(text[:h.start]), End: utf8.RuneCountInString(text[:h.end]),
		})
		result.Counts[h.kind.ID]++
	}
	out.WriteString(text[last:])
	result.Text = out.String()
	return result
}

func mask(value string, kind *Kind, partial bool) string {
	if !partial || kind.keep == 0 {
		return "[" + kind.Label + "]"
	}
	// Keep the layout: every letter and digit becomes X except the last few.
	runes := []rune(value)
	visible := 0
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]
		if r == ' ' || r == '-' || r == '+' {
			continue
		}
		if visible < kind.keep {
			visible++
			continue
		}
		runes[i] = 'X'
	}
	return string(runes)
}
