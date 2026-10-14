package regextest

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Token is one piece of a pattern with what it means. Depth is how many groups it is
// inside, so a reader can indent the list into the pattern's shape.
type Token struct {
	Token   string `json:"token"`
	Depth   int    `json:"depth"`
	Meaning string `json:"meaning"`
}

// Explain describes a pattern token by token. It reads the pattern itself rather than
// the engine's parse tree, so the description follows what was written; the engine has
// already accepted the pattern by the time this runs.
func Explain(pattern string, flavor Flavor) []Token {
	e := explainer{pattern: []rune(pattern), flavor: flavor, tokens: []Token{}}
	e.run()
	return e.tokens
}

// captureNames lists the capturing groups in the order they open, "" for an unnamed one.
func captureNames(pattern string) []string {
	e := explainer{pattern: []rune(pattern), flavor: JavaScript}
	e.run()
	return e.names
}

type explainer struct {
	pattern []rune
	at      int
	depth   int
	group   int
	names   []string // each capturing group's name, in order
	flavor  Flavor
	tokens  []Token
	literal []rune // a run of plain characters not yet emitted
}

func (e *explainer) add(token, meaning string) {
	e.flush()
	e.tokens = append(e.tokens, Token{Token: token, Depth: e.depth, Meaning: meaning})
}

func (e *explainer) flush() {
	if len(e.literal) == 0 {
		return
	}
	text := string(e.literal)
	e.literal = nil
	meaning := "The character " + quoted(text) + "."
	if utf8.RuneCountInString(text) > 1 {
		meaning = "The text " + quoted(text) + "."
	}
	e.tokens = append(e.tokens, Token{Token: text, Depth: e.depth, Meaning: meaning})
}

func (e *explainer) peek(offset int) rune {
	if e.at+offset < len(e.pattern) {
		return e.pattern[e.at+offset]
	}
	return 0
}

func (e *explainer) hasPrefix(prefix string) bool {
	return strings.HasPrefix(string(e.pattern[e.at:]), prefix)
}

func (e *explainer) run() {
	for e.at < len(e.pattern) {
		ch := e.pattern[e.at]
		switch ch {
		case '\\':
			token, meaning := e.escape(false)
			if meaning == "" {
				e.literal = append(e.literal, []rune(token[1:])...)
				continue
			}
			e.add(token, meaning)
		case '.':
			e.at++
			e.add(".", "Any one character except a line break (with the s flag, any character at all).")
		case '^':
			e.at++
			e.add("^", "Start of the text (with the m flag, start of any line).")
		case '$':
			e.at++
			e.add("$", "End of the text (with the m flag, end of any line).")
		case '|':
			e.at++
			e.add("|", "Or: the alternative before this, or the one after it.")
		case '[':
			token, meaning := e.class()
			e.add(token, meaning)
		case '(':
			e.open()
		case ')':
			e.at++
			e.flush() // text before the ) belongs inside the group
			if e.depth > 0 {
				e.depth--
			}
			e.add(")", "End of the group.")
		case '*', '+', '?', '{':
			if !e.quantifier() {
				e.at++
				e.literal = append(e.literal, ch)
			}
		default:
			e.at++
			e.literal = append(e.literal, ch)
		}
	}
	e.flush()
}

// quantifier reads *, +, ?, {n}, {n,} or {n,m}, with a lazy ? after it. A quantifier
// applies to the one item before it, so a run of literal text gives up its last
// character first.
func (e *explainer) quantifier() bool {
	start := e.at
	var how string
	switch e.pattern[e.at] {
	case '*':
		e.at++
		how = "zero or more times"
	case '+':
		e.at++
		how = "one or more times"
	case '?':
		e.at++
		how = "optional: zero times or once"
	case '{':
		end := e.at + 1
		for end < len(e.pattern) && (e.pattern[end] >= '0' && e.pattern[end] <= '9' || e.pattern[end] == ',') {
			end++
		}
		if end >= len(e.pattern) || e.pattern[end] != '}' || end == e.at+1 {
			return false // a { that is not a quantifier is a plain character
		}
		body := string(e.pattern[e.at+1 : end])
		low, high, comma := strings.Cut(body, ",")
		if low == "" {
			return false
		}
		switch {
		case !comma:
			how = "exactly " + low + " times"
		case high == "":
			how = low + " or more times"
		default:
			how = "between " + low + " and " + high + " times"
		}
		e.at = end + 1
	}
	greedy := ", as many as possible"
	if e.peek(0) == '?' {
		e.at++
		greedy = ", as few as possible (lazy)"
	}
	if strings.HasPrefix(how, "exactly") {
		greedy = ""
	}
	if len(e.literal) > 1 {
		last := e.literal[len(e.literal)-1]
		e.literal = e.literal[:len(e.literal)-1]
		e.flush()
		e.literal = []rune{last}
	}
	e.flush()
	token := string(e.pattern[start:e.at])
	if len(e.tokens) == 0 {
		e.add(token, "A quantifier with nothing before it to repeat.")
		return true
	}
	e.add(token, "Repeats the item before it "+how+greedy+".")
	return true
}

func (e *explainer) open() {
	e.at++
	if e.peek(0) != '?' {
		e.group++
		e.names = append(e.names, "")
		e.add("(", fmt.Sprintf("Start of capturing group %d.", e.group))
		e.depth++
		return
	}
	kinds := []struct{ prefix, meaning string }{
		{"?:", "Start of a group that does not capture."},
		{"?=", "Start of a lookahead: what follows must match here, without being consumed."},
		{"?!", "Start of a negative lookahead: what follows must not match here."},
		{"?<=", "Start of a lookbehind: the text just before this point must match."},
		{"?<!", "Start of a negative lookbehind: the text just before this point must not match."},
		{"?>", "Start of an atomic group: once matched, it is never backtracked into."},
	}
	for _, kind := range kinds {
		if e.hasPrefix(kind.prefix) {
			e.at += utf8.RuneCountInString(kind.prefix)
			e.add("("+kind.prefix, kind.meaning)
			e.depth++
			return
		}
	}
	for _, opener := range []struct{ prefix, closer string }{{"?P<", ">"}, {"?<", ">"}, {"?'", "'"}} {
		if !e.hasPrefix(opener.prefix) {
			continue
		}
		rest := string(e.pattern[e.at+utf8.RuneCountInString(opener.prefix):])
		if name, _, ok := strings.Cut(rest, opener.closer); ok && name != "" {
			e.group++
			e.names = append(e.names, name)
			token := "(" + opener.prefix + name + opener.closer
			e.at += utf8.RuneCountInString(opener.prefix+name) + 1
			e.add(token, fmt.Sprintf("Start of capturing group %d, named %q.", e.group, name))
			e.depth++
			return
		}
	}
	// Inline flags: (?i) for the rest of the group, or (?i:...) for a group of its own.
	end := e.at + 1
	for end < len(e.pattern) && strings.ContainsRune("imsxuU-", e.pattern[end]) {
		end++
	}
	if end < len(e.pattern) && end > e.at+1 && (e.pattern[end] == ')' || e.pattern[end] == ':') {
		flags := string(e.pattern[e.at+1 : end])
		token := "(?" + flags + string(e.pattern[end])
		e.at = end + 1
		if e.pattern[end] == ')' {
			e.add(token, "Sets the flags "+flags+" from here to the end of the enclosing group ("+flagNames(flags)+").")
			return
		}
		e.add(token, "Start of a group that does not capture, with the flags "+flags+" ("+flagNames(flags)+").")
		e.depth++
		return
	}
	e.add("(?", "Start of a group of a kind this explainer does not know.")
	e.depth++
}

func flagNames(flags string) string {
	names := map[rune]string{'i': "ignore case", 'm': "multi-line", 's': "dot matches line breaks", 'x': "ignore spaces in the pattern", 'U': "ungreedy", 'u': "Unicode", '-': "turn off what follows"}
	var out []string
	for _, flag := range flags {
		if name, ok := names[flag]; ok {
			out = append(out, string(flag)+" "+name)
		}
	}
	return strings.Join(out, ", ")
}

var escapes = map[rune]string{
	'd': "a digit", 'D': "anything but a digit",
	'w': "a word character (letter, digit or underscore)", 'W': "anything but a word character",
	's': "a whitespace character", 'S': "anything but whitespace",
	'n': "a line feed", 'r': "a carriage return", 't': "a tab", 'f': "a form feed", 'v': "a vertical tab",
}

var anchors = map[rune]string{
	'b': "A word boundary: between a word character and a non-word character.",
	'B': "Not a word boundary.",
	'A': "Start of the text, whatever the flags.",
	'z': "End of the text, whatever the flags.",
	'Z': "End of the text, or before a final line break.",
	'G': "Where the previous match ended.",
}

// escape reads a backslash sequence. inClass says it is inside [...], where \b means a
// backspace. A meaning of "" means an escaped literal character.
func (e *explainer) escape(inClass bool) (string, string) {
	start := e.at
	e.at++
	if e.at >= len(e.pattern) {
		return "\\", "A backslash at the end of the pattern."
	}
	ch := e.pattern[e.at]
	e.at++
	token := func() string { return string(e.pattern[start:e.at]) }
	if what, ok := escapes[ch]; ok {
		return token(), capital(what) + "."
	}
	if !inClass {
		if meaning, ok := anchors[ch]; ok {
			return token(), meaning
		}
	} else if ch == 'b' {
		return token(), "A backspace character."
	}
	switch {
	case ch == 'p' || ch == 'P':
		name := ""
		if e.peek(0) == '{' {
			if end := strings.IndexRune(string(e.pattern[e.at:]), '}'); end > 0 {
				name = string(e.pattern[e.at+1 : e.at+end])
				e.at += end + 1
			}
		} else if e.at < len(e.pattern) {
			name = string(e.pattern[e.at])
			e.at++
		}
		if ch == 'P' {
			return token(), "A character outside the Unicode class " + name + "."
		}
		return token(), "A character in the Unicode class " + name + " (" + unicodeClass(name) + ")."
	case ch == 'x':
		if e.peek(0) == '{' {
			if end := strings.IndexRune(string(e.pattern[e.at:]), '}'); end > 0 {
				e.at += end + 1
				return token(), "The character with code " + strings.Trim(token()[2:], "{}") + " (hexadecimal)."
			}
		}
		e.at = min(e.at+2, len(e.pattern))
		return token(), "The character with code " + token()[2:] + " (hexadecimal)."
	case ch == 'u':
		if e.peek(0) == '{' {
			if end := strings.IndexRune(string(e.pattern[e.at:]), '}'); end > 0 {
				e.at += end + 1
				return token(), "The character U+" + strings.ToUpper(strings.Trim(token()[2:], "{}")) + "."
			}
		}
		e.at = min(e.at+4, len(e.pattern))
		return token(), "The character U+" + strings.ToUpper(token()[2:]) + "."
	case ch == 'k' && e.peek(0) == '<':
		if end := strings.IndexRune(string(e.pattern[e.at:]), '>'); end > 0 {
			name := string(e.pattern[e.at+1 : e.at+end])
			e.at += end + 1
			return token(), "The same text group " + quoted(name) + " matched."
		}
	case ch >= '1' && ch <= '9' && !inClass:
		for e.at < len(e.pattern) && e.pattern[e.at] >= '0' && e.pattern[e.at] <= '9' {
			e.at++
		}
		return token(), "The same text group " + token()[1:] + " matched (a backreference)."
	case ch == '0':
		return token(), "The null character."
	case ch == 'Q' && e.flavor == RE2 && !inClass:
		end := strings.Index(string(e.pattern[e.at:]), `\E`)
		if end < 0 {
			end = len(e.pattern) - e.at
			e.at = len(e.pattern)
		} else {
			e.at += end + 2
		}
		return token(), "The text " + quoted(strings.TrimSuffix(token()[2:], `\E`)) + ", taken literally."
	}
	return token(), ""
}

func unicodeClass(name string) string {
	known := map[string]string{
		"L": "letters", "Lu": "upper-case letters", "Ll": "lower-case letters", "N": "numbers", "Nd": "decimal digits",
		"P": "punctuation", "S": "symbols", "Z": "separators", "M": "combining marks",
		"Tamil": "Tamil script", "Devanagari": "Devanagari script", "Bengali": "Bengali script", "Telugu": "Telugu script",
		"Kannada": "Kannada script", "Malayalam": "Malayalam script", "Gujarati": "Gujarati script", "Gurmukhi": "Gurmukhi script",
		"Oriya": "Odia script", "Latin": "Latin script", "Han": "Chinese characters", "Arabic": "Arabic script",
	}
	if meaning, ok := known[strings.TrimPrefix(strings.TrimPrefix(name, "Script="), "sc=")]; ok {
		return meaning
	}
	return "see the Unicode standard"
}

// class reads a character class [...] and lists what it accepts.
func (e *explainer) class() (string, string) {
	start := e.at
	e.at++
	negated := false
	if e.peek(0) == '^' {
		negated = true
		e.at++
	}
	var items []string
	first := true
	for e.at < len(e.pattern) {
		ch := e.pattern[e.at]
		// A ] first in the class is a literal, except in JavaScript, where [] is empty.
		if ch == ']' && !(first && e.flavor == RE2) {
			e.at++
			break
		}
		first = false
		if ch == '[' && e.hasPrefix("[:") {
			if end := strings.Index(string(e.pattern[e.at:]), ":]"); end > 0 {
				items = append(items, "the class "+string(e.pattern[e.at:e.at+end+2]))
				e.at += end + 2
				continue
			}
		}
		item, single := e.classAtom()
		if single != 0 && e.peek(0) == '-' && e.peek(1) != ']' && e.peek(1) != 0 {
			e.at++
			upper, last := e.classAtom()
			if last != 0 {
				items = append(items, quoted(string(single))+" to "+quoted(string(last)))
				continue
			}
			items = append(items, item, quoted("-"), upper)
			continue
		}
		items = append(items, item)
	}
	token := string(e.pattern[start:e.at])
	if len(items) == 0 {
		if negated {
			return token, "Any character at all."
		}
		return token, "Matches nothing (an empty class)."
	}
	if negated {
		return token, "One character that is not " + strings.Join(items, ", ") + "."
	}
	return token, "One character: " + strings.Join(items, ", ") + "."
}

// classAtom reads one member of a class. The rune is set when it is a single character
// that can start or end a range.
func (e *explainer) classAtom() (string, rune) {
	ch := e.pattern[e.at]
	if ch != '\\' {
		e.at++
		return quoted(string(ch)), ch
	}
	token, meaning := e.escape(true)
	if meaning == "" {
		r := []rune(token)[len([]rune(token))-1]
		return quoted(string(r)), r
	}
	switch token {
	case `\n`:
		return "a line feed", '\n'
	case `\t`:
		return "a tab", '\t'
	case `\r`:
		return "a carriage return", '\r'
	}
	return lower(strings.TrimSuffix(meaning, ".")), 0
}

func quoted(text string) string { return "“" + text + "”" }

func capital(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

func lower(text string) string {
	if text == "" {
		return text
	}
	return strings.ToLower(text[:1]) + text[1:]
}
