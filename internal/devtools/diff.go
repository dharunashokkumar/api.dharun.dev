package devtools

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// LineChange is one line of a text difference.
type LineChange struct {
	Op   string `json:"op"` // equal, add, remove
	Line string `json:"line"`
	Old  int    `json:"old,omitempty"` // line number in the first text
	New  int    `json:"new,omitempty"` // line number in the second text
}

// TextDiff is a line-by-line difference with a unified patch.
type TextDiff struct {
	Same    bool         `json:"same"`
	Added   int          `json:"added"`
	Removed int          `json:"removed"`
	Patch   string       `json:"patch"`
	Changes []LineChange `json:"changes,omitempty"`
}

// MaxDiffLines bounds each side of a text difference.
const MaxDiffLines = 5000

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// DiffText compares two texts line by line (the longest common subsequence).
func DiffText(a, b string, context int, withChanges bool) (TextDiff, error) {
	x, y := splitLines(a), splitLines(b)
	if len(x) > MaxDiffLines || len(y) > MaxDiffLines {
		return TextDiff{}, fmt.Errorf("each text may have at most %d lines", MaxDiffLines)
	}
	// Trim the common head and tail first: most real differences are small.
	head := 0
	for head < len(x) && head < len(y) && x[head] == y[head] {
		head++
	}
	tail := 0
	for tail < len(x)-head && tail < len(y)-head && x[len(x)-1-tail] == y[len(y)-1-tail] {
		tail++
	}
	mx, my := x[head:len(x)-tail], y[head:len(y)-tail]
	if len(mx)*len(my) > 4_000_000 {
		return TextDiff{}, errors.New("the texts differ in too many places to compare; send smaller parts")
	}
	n, m := len(mx), len(my)
	table := make([][]int32, n+1)
	for i := range table {
		table[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if mx[i] == my[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	var changes []LineChange
	for i := 0; i < head; i++ {
		changes = append(changes, LineChange{Op: "equal", Line: x[i], Old: i + 1, New: i + 1})
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && mx[i] == my[j]:
			changes = append(changes, LineChange{Op: "equal", Line: mx[i], Old: head + i + 1, New: head + j + 1})
			i, j = i+1, j+1
		case j < m && (i == n || table[i][j+1] > table[i+1][j]):
			changes = append(changes, LineChange{Op: "add", Line: my[j], New: head + j + 1})
			j++
		default:
			changes = append(changes, LineChange{Op: "remove", Line: mx[i], Old: head + i + 1})
			i++
		}
	}
	for k := 0; k < tail; k++ {
		changes = append(changes, LineChange{Op: "equal", Line: x[len(x)-tail+k], Old: len(x) - tail + k + 1, New: len(y) - tail + k + 1})
	}
	d := TextDiff{}
	for _, c := range changes {
		switch c.Op {
		case "add":
			d.Added++
		case "remove":
			d.Removed++
		}
	}
	d.Same = d.Added == 0 && d.Removed == 0
	d.Patch = unified(changes, context)
	if withChanges {
		d.Changes = changes
	}
	return d, nil
}

// unified writes changes as a unified diff with the given lines of context.
func unified(changes []LineChange, context int) string {
	var b strings.Builder
	b.WriteString("--- a\n+++ b\n")
	for start := 0; start < len(changes); {
		for start < len(changes) && changes[start].Op == "equal" {
			start++
		}
		if start == len(changes) {
			break
		}
		from := max(0, start-context)
		end := start
		for end < len(changes) {
			if changes[end].Op != "equal" {
				end++
				continue
			}
			run := end
			for run < len(changes) && changes[run].Op == "equal" {
				run++
			}
			if run == len(changes) || run-end > 2*context {
				break
			}
			end = run
		}
		to := min(len(changes), end+context)
		oldStart, newStart, oldCount, newCount := 0, 0, 0, 0
		for _, c := range changes[from:to] {
			if c.Op != "add" {
				if oldStart == 0 {
					oldStart = c.Old
				}
				oldCount++
			}
			if c.Op != "remove" {
				if newStart == 0 {
					newStart = c.New
				}
				newCount++
			}
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, c := range changes[from:to] {
			prefix := " "
			switch c.Op {
			case "add":
				prefix = "+"
			case "remove":
				prefix = "-"
			}
			b.WriteString(prefix + c.Line + "\n")
		}
		start = to
	}
	return b.String()
}

// JSONChange is one difference between two JSON documents, as a JSON Patch (RFC 6902)
// operation with the old value kept for reading.
type JSONChange struct {
	Op    string `json:"op"` // add, remove, replace
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
	Old   any    `json:"old,omitempty"`
}

func pointer(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		b.WriteString("/" + strings.ReplaceAll(strings.ReplaceAll(p, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

// DiffJSON compares two decoded JSON values.
func DiffJSON(a, b any) []JSONChange {
	var out []JSONChange
	diffValue(&out, nil, a, b)
	return out
}

func diffValue(out *[]JSONChange, path []string, a, b any) {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			break
		}
		keys := map[string]bool{}
		for k := range x {
			keys[k] = true
		}
		for k := range y {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			child := append(append([]string(nil), path...), k)
			av, inA := x[k]
			bv, inB := y[k]
			switch {
			case !inB:
				*out = append(*out, JSONChange{Op: "remove", Path: pointer(child), Old: av})
			case !inA:
				*out = append(*out, JSONChange{Op: "add", Path: pointer(child), Value: bv})
			default:
				diffValue(out, child, av, bv)
			}
		}
		return
	case []any:
		y, ok := b.([]any)
		if !ok {
			break
		}
		common := min(len(x), len(y))
		for i := 0; i < common; i++ {
			diffValue(out, append(append([]string(nil), path...), strconv.Itoa(i)), x[i], y[i])
		}
		for i := common; i < len(y); i++ {
			*out = append(*out, JSONChange{Op: "add", Path: pointer(append(append([]string(nil), path...), strconv.Itoa(i))), Value: y[i]})
		}
		for i := len(x) - 1; i >= common; i-- {
			*out = append(*out, JSONChange{Op: "remove", Path: pointer(append(append([]string(nil), path...), strconv.Itoa(i))), Old: x[i]})
		}
		return
	}
	if !reflect.DeepEqual(a, b) {
		*out = append(*out, JSONChange{Op: "replace", Path: pointer(path), Value: b, Old: a})
	}
}
