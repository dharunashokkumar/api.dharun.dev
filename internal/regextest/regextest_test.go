package regextest

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func text(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestGroupsAreNumberedLeftToRightInJavaScript(t *testing.T) {
	// regexp2 alone would call the unnamed group 1 and the named one 2.
	result, err := Run(`(?<year>\d{4})-(\d\d)`, "on 2026-10 and 2027-01", Options{Flavor: JavaScript})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 2 || result.Groups[0].Name != "year" || result.Groups[1].Name != "" {
		t.Fatalf("groups: %+v", result.Groups)
	}
	first := result.Matches[0]
	if first.Text != "2026-10" || first.Start != 3 || first.End != 10 || text(first.Groups[0].Text) != "2026" || text(first.Groups[1].Text) != "10" {
		t.Errorf("first match: %+v", first)
	}
	replace := "$2/$1 $<year>"
	result, _ = Run(`(?<year>\d{4})-(\d\d)`, "on 2026-10", Options{Flavor: JavaScript, Replace: &replace})
	if text(result.Replaced) != "on 10/2026 2026" {
		t.Errorf("replaced: %q", text(result.Replaced))
	}
}

func TestOffsetsCountCharacters(t *testing.T) {
	subject := "வணக்கம் 2026 नमस्ते 42"
	for _, flavor := range []Flavor{RE2, JavaScript} {
		result, err := Run(`\d+`, subject, Options{Flavor: flavor})
		if err != nil {
			t.Fatal(err)
		}
		if result.Count != 2 {
			t.Fatalf("%s: %d matches", flavor, result.Count)
		}
		for _, match := range result.Matches {
			if got := string([]rune(subject)[match.Start:match.End]); got != match.Text {
				t.Errorf("%s: characters %d to %d are %q, not %q", flavor, match.Start, match.End, got, match.Text)
			}
		}
	}
	result, _ := Run(`(न)(म)`, subject, Options{Flavor: RE2})
	if group := result.Matches[0].Groups[1]; *group.Start != 14 || *group.End != 15 {
		t.Errorf("RE2 group offsets: %d to %d", *group.Start, *group.End)
	}
}

func TestGroupThatTookNoPart(t *testing.T) {
	for _, flavor := range []Flavor{RE2, JavaScript} {
		result, err := Run(`a(b)?(c*)`, "a", Options{Flavor: flavor})
		if err != nil {
			t.Fatal(err)
		}
		groups := result.Matches[0].Groups
		if groups[0].Text != nil || groups[1].Text == nil || *groups[1].Text != "" {
			t.Errorf("%s: an unmatched group is null, an empty one is \"\": %+v", flavor, groups)
		}
	}
}

func TestFlags(t *testing.T) {
	for _, flavor := range []Flavor{RE2, JavaScript} {
		if result, _ := Run(`^b.c$`, "a\nB\nc", Options{Flavor: flavor, IgnoreCase: true, Multiline: true, DotAll: true}); result.Count != 1 || result.Matches[0].Text != "B\nc" {
			t.Errorf("%s with i, m and s: %+v", flavor, result.Matches)
		}
		if result, _ := Run(`^b.c$`, "a\nB\nc", Options{Flavor: flavor}); result.Count != 0 {
			t.Errorf("%s without flags matched: %+v", flavor, result.Matches)
		}
	}
}

func TestLimitKeepsCounting(t *testing.T) {
	result, _ := Run(`a`, strings.Repeat("a", 50), Options{Flavor: RE2, Limit: 10})
	if result.Count != 50 || len(result.Matches) != 10 || !result.Truncated {
		t.Errorf("count %d, kept %d, truncated %v", result.Count, len(result.Matches), result.Truncated)
	}
}

func TestCatastrophicBacktrackingStops(t *testing.T) {
	start := time.Now()
	_, err := Run(`(a+)+$`, strings.Repeat("a", 40)+"b", Options{Flavor: JavaScript, Timeout: 200 * time.Millisecond})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("took %v against a 200 ms limit", took)
	}
	// The same pattern is linear in RE2.
	if result, err := Run(`(a+)+$`, strings.Repeat("a", 40)+"b", Options{Flavor: RE2}); err != nil || result.Count != 0 {
		t.Errorf("RE2: %v %+v", err, result)
	}
}

func TestFlavourDifferences(t *testing.T) {
	var patternError *PatternError
	if _, err := Run(`foo(?=bar)`, "foobar", Options{Flavor: RE2}); !errors.As(err, &patternError) {
		t.Errorf("RE2 has no lookahead, yet: %v", err)
	}
	if result, err := Run(`foo(?=bar)`, "foobar", Options{Flavor: JavaScript}); err != nil || result.Count != 1 || result.Matches[0].Text != "foo" {
		t.Errorf("JavaScript lookahead: %v %+v", err, result)
	}
	replace := "${2}-${1}"
	if result, _ := Run(`(\w+) (\w+)`, "hello world", Options{Flavor: RE2, Replace: &replace}); text(result.Replaced) != "world-hello" {
		t.Errorf("RE2 replacement: %q", text(result.Replaced))
	}
}

func TestExplain(t *testing.T) {
	tokens := Explain(`^(?:ab|cd)+?\d{2,4}[^a-f\s]$`, JavaScript)
	var got []string
	for _, token := range tokens {
		got = append(got, strings.Repeat(" ", token.Depth)+token.Token)
	}
	want := []string{"^", "(?:", " ab", " |", " cd", ")", "+?", `\d`, "{2,4}", `[^a-f\s]`, "$"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("tokens:\n got %q\nwant %q", got, want)
	}
	if meaning := tokens[9].Meaning; meaning != "One character that is not “a” to “f”, a whitespace character." {
		t.Errorf("class: %q", meaning)
	}
	if meaning := tokens[6].Meaning; !strings.Contains(meaning, "one or more times") || !strings.Contains(meaning, "lazy") {
		t.Errorf("lazy plus: %q", meaning)
	}
	// A quantifier after literal text applies to the last character only.
	tokens = Explain(`abc+`, RE2)
	if len(tokens) != 3 || tokens[0].Token != "ab" || tokens[1].Token != "c" || tokens[2].Token != "+" {
		t.Errorf("abc+: %+v", tokens)
	}
	if names := captureNames(`(a)\((?<x>b)[(](?:c)(d)`); strings.Join(names, ",") != ",x," {
		t.Errorf("capture order: %q", names)
	}
}
