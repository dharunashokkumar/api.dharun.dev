// Package regextest runs a regular expression against a text the way the regex tester
// endpoint reports it: every match with its groups, an optional replacement, and a
// token by token explanation of the pattern.
//
// Two flavours are offered. RE2 is Go's regexp package: no backtracking, so its time is
// linear in the input and no pattern can hang it. JavaScript is regexp2 in ECMAScript
// mode, a backtracking engine, so it runs under a time limit against patterns such as
// (a+)+$ that take exponential time.
package regextest

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// Flavor names a regular expression dialect.
type Flavor string

const (
	RE2        Flavor = "re2"
	JavaScript Flavor = "js"
)

// Flavors lists the accepted flavour names.
func Flavors() []string { return []string{string(JavaScript), string(RE2)} }

// Options says how to run a pattern.
type Options struct {
	Flavor     Flavor
	IgnoreCase bool // i
	Multiline  bool // m: ^ and $ match at line breaks
	DotAll     bool // s: . matches a line break too
	// Limit is the most matches reported; the count goes on to the end of the text.
	Limit int
	// Timeout bounds the whole run for the backtracking flavour.
	Timeout time.Duration
	// Replace, when set, is substituted for every match.
	Replace *string
}

// GroupInfo describes one capturing group of the pattern.
type GroupInfo struct {
	Index int    `json:"index"`
	Name  string `json:"name,omitempty"`
}

// Capture is what one group captured in one match. Text is nil when the group took no
// part in the match, which is different from capturing an empty string.
type Capture struct {
	Index int     `json:"index"`
	Name  string  `json:"name,omitempty"`
	Text  *string `json:"text"`
	Start *int    `json:"start"`
	End   *int    `json:"end"`
}

// Match is one match. Offsets count characters (Unicode code points), so Text is
// exactly the subject's characters Start to End.
type Match struct {
	Text   string    `json:"text"`
	Start  int       `json:"start"`
	End    int       `json:"end"`
	Groups []Capture `json:"groups"`
}

// Result is everything a run found.
type Result struct {
	Groups      []GroupInfo `json:"groups"`
	Count       int         `json:"count"`
	Truncated   bool        `json:"truncated"`
	Matches     []Match     `json:"matches"`
	Replaced    *string     `json:"replaced,omitempty"`
	Explanation []Token     `json:"explanation"`
}

// PatternError is a pattern the engine refused, with the engine's own message.
type PatternError struct{ Message string }

func (e *PatternError) Error() string { return e.Message }

// ErrTimeout means the backtracking engine ran out of time, which almost always means
// catastrophic backtracking: nested quantifiers that can split the input in
// exponentially many ways.
var ErrTimeout = errors.New("the pattern took too long against this text")

// Run compiles the pattern in the chosen flavour and applies it to the subject.
func Run(pattern, subject string, opts Options) (Result, error) {
	if opts.Limit <= 0 {
		opts.Limit = 1000
	}
	if opts.Timeout <= 0 {
		opts.Timeout = time.Second
	}
	var result Result
	var err error
	switch opts.Flavor {
	case RE2:
		result, err = runRE2(pattern, subject, opts)
	case JavaScript:
		result, err = runJS(pattern, subject, opts)
	default:
		return Result{}, &PatternError{"unknown flavour " + string(opts.Flavor)}
	}
	if err != nil {
		return Result{}, err
	}
	result.Explanation = Explain(pattern, opts.Flavor)
	return result, nil
}

func runRE2(pattern, subject string, opts Options) (Result, error) {
	flags := ""
	if opts.IgnoreCase {
		flags += "i"
	}
	if opts.Multiline {
		flags += "m"
	}
	if opts.DotAll {
		flags += "s"
	}
	source := pattern
	if flags != "" {
		source = "(?" + flags + ")" + pattern
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return Result{}, &PatternError{strings.TrimPrefix(err.Error(), "error parsing regexp: ")}
	}
	result := Result{Groups: []GroupInfo{}, Matches: []Match{}}
	names := re.SubexpNames()
	for index := 1; index < len(names); index++ {
		result.Groups = append(result.Groups, GroupInfo{Index: index, Name: names[index]})
	}

	// Offsets come back in bytes and ascending, so one pass converts them to characters.
	counter := runeCounter{text: subject}
	for _, found := range re.FindAllStringSubmatchIndex(subject, -1) {
		result.Count++
		if len(result.Matches) == opts.Limit {
			result.Truncated = true
			continue
		}
		start, end := counter.at(found[0]), counter.at(found[1])
		match := Match{Text: subject[found[0]:found[1]], Start: start, End: end, Groups: []Capture{}}
		for index := 1; index < len(names); index++ {
			capture := Capture{Index: index, Name: names[index]}
			if from, to := found[2*index], found[2*index+1]; from >= 0 {
				text := subject[from:to]
				// Groups can end before the match does but cannot start before it, and the
				// counter only moves forwards, so count from the match start.
				first := start + utf8.RuneCountInString(subject[found[0]:from])
				last := first + utf8.RuneCountInString(text)
				capture.Text, capture.Start, capture.End = &text, &first, &last
			}
			match.Groups = append(match.Groups, capture)
		}
		result.Matches = append(result.Matches, match)
	}
	if opts.Replace != nil {
		replaced := re.ReplaceAllString(subject, *opts.Replace)
		result.Replaced = &replaced
	}
	return result, nil
}

// runeCounter turns ascending byte offsets into character offsets in one pass.
type runeCounter struct {
	text        string
	byteAt, out int
}

func (c *runeCounter) at(offset int) int {
	if offset < c.byteAt {
		return utf8.RuneCountInString(c.text[:offset])
	}
	c.out += utf8.RuneCountInString(c.text[c.byteAt:offset])
	c.byteAt = offset
	return c.out
}

func runJS(pattern, subject string, opts Options) (Result, error) {
	options := regexp2.RegexOptions(regexp2.ECMAScript | regexp2.Unicode)
	if opts.IgnoreCase {
		options |= regexp2.IgnoreCase
	}
	if opts.Multiline {
		options |= regexp2.Multiline
	}
	source := pattern
	if opts.DotAll {
		// regexp2 ignores Singleline in ECMAScript mode, so the s flag is applied by
		// rewriting each bare dot into what it means under s.
		source = dotAll(pattern)
	}
	re, err := regexp2.Compile(source, options)
	if err != nil {
		return Result{}, &PatternError{strings.TrimPrefix(err.Error(), "error parsing regexp: ")}
	}
	deadline := time.Now().Add(opts.Timeout)
	remaining := func() time.Duration {
		left := time.Until(deadline)
		if left < time.Millisecond {
			left = time.Millisecond
		}
		return left
	}
	result := Result{Groups: []GroupInfo{}, Matches: []Match{}}
	// regexp2 numbers groups the .NET way, unnamed ones first and named ones after them.
	// JavaScript numbers every group left to right, so the pattern's own order is used,
	// and each group is found in regexp2 by its name or its place among unnamed groups.
	engine := []int{}
	order := captureNames(pattern)
	if len(order) == len(re.GetGroupNumbers())-1 {
		unnamed := 0
		for index, name := range order {
			number := 0
			if name == "" {
				unnamed++
				number = unnamed
			} else {
				number = re.GroupNumberFromName(name)
			}
			result.Groups = append(result.Groups, GroupInfo{Index: index + 1, Name: name})
			engine = append(engine, number)
		}
	} else {
		for _, number := range re.GetGroupNumbers()[1:] {
			name := re.GroupNameFromNumber(number)
			if name == strconv.Itoa(number) {
				name = ""
			}
			result.Groups = append(result.Groups, GroupInfo{Index: number, Name: name})
			engine = append(engine, number)
		}
	}

	runes := []rune(subject)
	re.MatchTimeout = remaining()
	found, err := re.FindRunesMatch(runes)
	for err == nil && found != nil {
		result.Count++
		if len(result.Matches) < opts.Limit {
			match := Match{Text: found.String(), Start: found.Index, End: found.Index + found.Length, Groups: []Capture{}}
			for i, info := range result.Groups {
				capture := Capture{Index: info.Index, Name: info.Name}
				if group := found.GroupByNumber(engine[i]); group != nil && len(group.Captures) > 0 {
					text := group.String()
					first, last := group.Index, group.Index+group.Length
					capture.Text, capture.Start, capture.End = &text, &first, &last
				}
				match.Groups = append(match.Groups, capture)
			}
			result.Matches = append(result.Matches, match)
		} else {
			result.Truncated = true
		}
		if time.Now().After(deadline) {
			return Result{}, ErrTimeout
		}
		re.MatchTimeout = remaining()
		found, err = re.FindNextMatch(found)
	}
	if err != nil {
		return Result{}, ErrTimeout // the only error regexp2 returns while matching
	}
	if opts.Replace != nil {
		re.MatchTimeout = remaining()
		replacement := *opts.Replace
		replaced, err := re.ReplaceFunc(subject, func(m regexp2.Match) string {
			return expandJS(replacement, &m, runes, result.Groups, engine)
		}, -1, -1)
		if err != nil {
			return Result{}, ErrTimeout
		}
		result.Replaced = &replaced
	}
	return result, nil
}

// expandJS fills in a JavaScript replacement string for one match: $$, $&, $`, $',
// $1 to $99 in JavaScript's group numbering, and $<name>. Anything else is literal, as
// in JavaScript.
func expandJS(replacement string, m *regexp2.Match, text []rune, groups []GroupInfo, engine []int) string {
	group := func(index int) string {
		if g := m.GroupByNumber(engine[index-1]); g != nil && len(g.Captures) > 0 {
			return g.String()
		}
		return ""
	}
	var out strings.Builder
	for i := 0; i < len(replacement); i++ {
		if replacement[i] != '$' || i+1 == len(replacement) {
			out.WriteByte(replacement[i])
			continue
		}
		next := replacement[i+1]
		switch {
		case next == '$':
			out.WriteByte('$')
			i++
		case next == '&':
			out.WriteString(m.String())
			i++
		case next == '`':
			out.WriteString(string(text[:m.Index]))
			i++
		case next == '\'':
			out.WriteString(string(text[m.Index+m.Length:]))
			i++
		case next >= '0' && next <= '9':
			// Two digits when they name a group, else one, as JavaScript reads them.
			if i+2 < len(replacement) && replacement[i+2] >= '0' && replacement[i+2] <= '9' {
				if two := int(next-'0')*10 + int(replacement[i+2]-'0'); two >= 1 && two <= len(groups) {
					out.WriteString(group(two))
					i += 2
					continue
				}
			}
			if one := int(next - '0'); one >= 1 && one <= len(groups) {
				out.WriteString(group(one))
				i++
				continue
			}
			out.WriteByte('$')
		case next == '<':
			end := strings.IndexByte(replacement[i+2:], '>')
			if end < 0 {
				out.WriteByte('$')
				continue
			}
			name := replacement[i+2 : i+2+end]
			for _, info := range groups {
				if info.Name == name {
					out.WriteString(group(info.Index))
				}
			}
			i += end + 2
		default:
			out.WriteByte('$')
		}
	}
	return out.String()
}

// dotAll replaces every . that is outside a class and not escaped with [\s\S].
func dotAll(pattern string) string {
	var out strings.Builder
	inClass := false
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch {
		case ch == '\\' && i+1 < len(pattern):
			out.WriteByte(ch)
			out.WriteByte(pattern[i+1])
			i++
			continue
		case ch == '[':
			inClass = true
		case ch == ']':
			inClass = false
		case ch == '.' && !inClass:
			out.WriteString(`[\s\S]`)
			continue
		}
		out.WriteByte(ch)
	}
	return out.String()
}
