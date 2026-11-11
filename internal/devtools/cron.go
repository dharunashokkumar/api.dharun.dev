package devtools

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cron is a parsed five-field cron expression: minute, hour, day of month, month and
// day of week, as crontab(5) reads them.
type Cron struct {
	Expression string
	fields     [5]cronField
}

type cronField struct {
	set [61]bool
	any bool // written as * (matters for the day fields)
}

var cronLimits = [5]struct {
	name     string
	min, max int
	names    []string
}{
	{"minute", 0, 59, nil},
	{"hour", 0, 23, nil},
	{"day of month", 1, 31, nil},
	{"month", 1, 12, []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}},
	{"day of week", 0, 7, []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}},
}

var cronMacros = map[string]string{
	"@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *", "@monthly": "0 0 1 * *", "@weekly": "0 0 * * 0",
	"@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@hourly": "0 * * * *",
}

// ParseCron reads an expression.
func ParseCron(expression string) (Cron, error) {
	expression = strings.Join(strings.Fields(expression), " ")
	text := expression
	if macro, ok := cronMacros[strings.ToLower(text)]; ok {
		text = macro
	}
	parts := strings.Fields(text)
	if len(parts) == 6 {
		return Cron{}, errors.New("six fields: this reads the standard five (minute hour day month weekday); a seconds field is not supported")
	}
	if len(parts) != 5 {
		return Cron{}, errors.New("a cron expression has five fields: minute hour day-of-month month day-of-week")
	}
	c := Cron{Expression: expression}
	for i, part := range parts {
		f, err := parseCronField(part, i)
		if err != nil {
			return Cron{}, fmt.Errorf("%s field %q: %w", cronLimits[i].name, part, err)
		}
		c.fields[i] = f
	}
	if c.fields[4].set[7] {
		c.fields[4].set[0] = true
	}
	return c, nil
}

func cronValue(s string, i int) (int, error) {
	limit := cronLimits[i]
	for n, name := range limit.names {
		if strings.EqualFold(s, name) {
			if i == 3 {
				return n + 1, nil
			}
			return n, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < limit.min || v > limit.max {
		return 0, fmt.Errorf("%s is not %d to %d", s, limit.min, limit.max)
	}
	return v, nil
}

func parseCronField(s string, i int) (cronField, error) {
	var f cronField
	limit := cronLimits[i]
	for _, item := range strings.Split(s, ",") {
		rangePart, stepPart, hasStep := strings.Cut(item, "/")
		step := 1
		if hasStep {
			var err error
			step, err = strconv.Atoi(stepPart)
			if err != nil || step < 1 || step > limit.max {
				return f, errors.New("bad step " + stepPart)
			}
		}
		lo, hi := limit.min, limit.max
		switch {
		case rangePart == "*" || rangePart == "?":
			if !hasStep {
				f.any = true
			}
			if i == 4 {
				hi = 6
			}
		case strings.Contains(rangePart, "-"):
			a, b, _ := strings.Cut(rangePart, "-")
			var err error
			if lo, err = cronValue(a, i); err != nil {
				return f, err
			}
			if hi, err = cronValue(b, i); err != nil {
				return f, err
			}
			if hi < lo {
				return f, errors.New("the range runs backwards")
			}
		default:
			v, err := cronValue(rangePart, i)
			if err != nil {
				return f, err
			}
			lo = v
			if hasStep {
				hi = limit.max
			} else {
				hi = v
			}
		}
		for v := lo; v <= hi; v += step {
			f.set[v] = true
		}
	}
	return f, nil
}

func (c Cron) matches(t time.Time) bool {
	f := c.fields
	if !f[0].set[t.Minute()] || !f[1].set[t.Hour()] || !f[3].set[int(t.Month())] {
		return false
	}
	dom, dow := f[2].set[t.Day()], f[4].set[int(t.Weekday())]
	// crontab(5): when both day fields are restricted, either one matching is enough.
	if !f[2].any && !f[4].any {
		return dom || dow
	}
	return dom && dow
}

// Next returns the next n run times after from, in from's zone. It looks ahead at most
// five years.
func (c Cron) Next(from time.Time, n int) []time.Time {
	var out []time.Time
	t := from.Truncate(time.Minute).Add(time.Minute)
	limit := from.AddDate(5, 0, 0)
	for len(out) < n && t.Before(limit) {
		if !c.fields[3].set[int(t.Month())] {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !c.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !c.fields[1].set[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		if c.matches(t) {
			out = append(out, t)
		}
		t = t.Add(time.Minute)
	}
	return out
}

func (c Cron) dayMatches(t time.Time) bool {
	f := c.fields
	dom, dow := f[2].set[t.Day()], f[4].set[int(t.Weekday())]
	if !f[2].any && !f[4].any {
		return dom || dow
	}
	return dom && dow
}

// Describe says in English when the expression runs.
func (c Cron) Describe() string {
	f := c.fields
	var b strings.Builder
	minutes, hours := values(f[0], 0, 59), values(f[1], 0, 23)
	switch {
	case f[0].any && f[1].any:
		b.WriteString("Every minute")
	case f[1].any && len(minutes) == 1:
		fmt.Fprintf(&b, "At minute %d of every hour", minutes[0])
	case len(minutes) == 1 && len(hours) == 1:
		fmt.Fprintf(&b, "At %02d:%02d", hours[0], minutes[0])
	case len(minutes) == 1 && len(hours) <= 6:
		times := make([]string, len(hours))
		for i, h := range hours {
			times[i] = fmt.Sprintf("%02d:%02d", h, minutes[0])
		}
		b.WriteString("At " + joinAnd(times))
	default:
		b.WriteString(describeField(f[0], 0, 59, "minute"))
		if !f[1].any {
			b.WriteString(", " + strings.ToLower(describeField(f[1], 0, 23, "hour")))
		}
	}
	monthNames := []string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	var days []string
	if !f[2].any {
		var list []string
		for _, d := range values(f[2], 1, 31) {
			list = append(list, ordinal(d))
		}
		days = append(days, "on the "+compact(list)+" of the month")
	}
	if !f[4].any {
		var list []string
		for _, d := range values(f[4], 0, 6) {
			list = append(list, dayNames[d])
		}
		days = append(days, "on "+compact(list))
	}
	if len(days) > 0 {
		b.WriteString(", " + strings.Join(days, " or "))
	}
	if !f[3].any {
		var list []string
		for _, m := range values(f[3], 1, 12) {
			list = append(list, monthNames[m])
		}
		b.WriteString(", in " + compact(list))
	}
	return b.String() + "."
}

func values(f cronField, lo, hi int) []int {
	var out []int
	for v := lo; v <= hi; v++ {
		if f.set[v] {
			out = append(out, v)
		}
	}
	return out
}

func describeField(f cronField, lo, hi int, unit string) string {
	list := values(f, lo, hi)
	if f.any {
		return "Every " + unit
	}
	if len(list) > 2 {
		step := list[1] - list[0]
		regular := true
		for i := 1; i < len(list); i++ {
			if list[i]-list[i-1] != step {
				regular = false
			}
		}
		if regular && list[0] == lo && list[len(list)-1]+step > hi {
			return fmt.Sprintf("Every %d %ss", step, unit)
		}
	}
	words := make([]string, len(list))
	for i, v := range list {
		words[i] = strconv.Itoa(v)
	}
	return "At " + unit + " " + compact(words)
}

func compact(list []string) string {
	if len(list) > 8 {
		return strings.Join(list[:8], ", ") + " and " + strconv.Itoa(len(list)-8) + " more"
	}
	return joinAnd(list)
}

func joinAnd(list []string) string {
	switch len(list) {
	case 0:
		return ""
	case 1:
		return list[0]
	}
	return strings.Join(list[:len(list)-1], ", ") + " and " + list[len(list)-1]
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}
