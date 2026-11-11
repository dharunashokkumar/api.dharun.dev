package indiafmt

import (
	"fmt"
	"strconv"
	"time"
)

// FinancialYear describes the Indian financial year (April to March) a date falls in.
type FinancialYear struct {
	Year           string `json:"year"` // 2026-27
	Starts         string `json:"starts"`
	Ends           string `json:"ends"`
	Quarter        string `json:"quarter"` // Q1 is April to June
	QuarterStarts  string `json:"quarterStarts"`
	QuarterEnds    string `json:"quarterEnds"`
	Half           string `json:"half"`           // H1 is April to September
	AssessmentYear string `json:"assessmentYear"` // the year its income tax return is for
	DayOfYear      int    `json:"dayOfYear"`
	DaysLeft       int    `json:"daysLeft"`
}

func day(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }

// FY returns the financial year of the date.
func FY(t time.Time) FinancialYear {
	t = day(t)
	start := t.Year()
	if t.Month() < time.April {
		start--
	}
	from := time.Date(start, time.April, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(start+1, time.March, 31, 0, 0, 0, 0, time.UTC)
	quarter := (int(t.Month())+8)%12/3 + 1
	qFrom := from.AddDate(0, 3*(quarter-1), 0)
	qTo := qFrom.AddDate(0, 3, -1)
	half := "H1"
	if quarter > 2 {
		half = "H2"
	}
	return FinancialYear{
		Year: fmt.Sprintf("%d-%02d", start, (start+1)%100), Starts: from.Format(time.DateOnly), Ends: to.Format(time.DateOnly),
		Quarter: "Q" + strconv.Itoa(quarter), QuarterStarts: qFrom.Format(time.DateOnly), QuarterEnds: qTo.Format(time.DateOnly), Half: half,
		AssessmentYear: fmt.Sprintf("%d-%02d", start+1, (start+2)%100),
		DayOfYear:      int(t.Sub(from).Hours()/24) + 1, DaysLeft: int(to.Sub(t).Hours() / 24),
	}
}

// CalendarDate is a date in a calendar other than the Gregorian one.
type CalendarDate struct {
	Year      int    `json:"year"`
	Month     int    `json:"month"`
	MonthName string `json:"monthName"`
	Day       int    `json:"day"`
	Text      string `json:"text"`
}

var sakaMonths = []string{"Chaitra", "Vaishakha", "Jyeshtha", "Ashadha", "Shravana", "Bhadra", "Ashwin", "Kartika", "Agrahayana", "Pausha", "Magha", "Phalguna"}

func leap(year int) bool { return year%4 == 0 && (year%100 != 0 || year%400 == 0) }

// chaitra1 is the first day of the national calendar's year that starts in the
// Gregorian year y: 22 March, or 21 March in a Gregorian leap year.
func chaitra1(y int) time.Time {
	if leap(y) {
		return time.Date(y, time.March, 21, 0, 0, 0, 0, time.UTC)
	}
	return time.Date(y, time.March, 22, 0, 0, 0, 0, time.UTC)
}

// Saka returns the date in the Indian national calendar (Saka era), adopted in 1957.
func Saka(t time.Time) CalendarDate {
	t = day(t)
	start := t.Year()
	if t.Before(chaitra1(start)) {
		start--
	}
	days := int(t.Sub(chaitra1(start)).Hours() / 24)
	lengths := []int{30, 31, 31, 31, 31, 31, 30, 30, 30, 30, 30, 30}
	if leap(start) {
		lengths[0] = 31
	}
	month := 0
	for days >= lengths[month] {
		days -= lengths[month]
		month++
	}
	d := CalendarDate{Year: start - 78, Month: month + 1, MonthName: sakaMonths[month], Day: days + 1}
	d.Text = fmt.Sprintf("%d %s %d Saka", d.Day, d.MonthName, d.Year)
	return d
}

var hijriMonths = []string{"Muharram", "Safar", "Rabi al-Awwal", "Rabi al-Thani", "Jumada al-Ula", "Jumada al-Akhirah", "Rajab", "Shaban", "Ramadan", "Shawwal", "Dhu al-Qadah", "Dhu al-Hijjah"}

// rataDie counts days from 1 January of year 1 (day 1), as Calendrical Calculations does.
func rataDie(t time.Time) int { return int(day(t).Unix()/86400) + 719163 }

const islamicEpoch = 227015 // 16 July 622 (Julian)

func fixedFromIslamic(y, m, d int) int {
	return d + 29*(m-1) + (6*m-1)/11 + (y-1)*354 + floorDiv(3+11*y, 30) + islamicEpoch - 1
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// Hijri returns the tabular (arithmetic) Islamic date. Religious dates follow the
// sighting of the moon and can differ from it by a day or two.
func Hijri(t time.Time) CalendarDate {
	fixed := rataDie(t)
	year := floorDiv(30*(fixed-islamicEpoch)+10646, 10631)
	month := 1
	for m := 12; m >= 1; m-- {
		if fixed >= fixedFromIslamic(year, m, 1) {
			month = m
			break
		}
	}
	dayOf := fixed - fixedFromIslamic(year, month, 1) + 1
	d := CalendarDate{Year: year, Month: month, MonthName: hijriMonths[month-1], Day: dayOf}
	d.Text = fmt.Sprintf("%d %s %d AH", d.Day, d.MonthName, d.Year)
	return d
}

// Span is the difference between two dates in calendar terms.
type Span struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Years     int    `json:"years"`
	Months    int    `json:"months"`
	Days      int    `json:"days"`
	TotalDays int    `json:"totalDays"`
	Weeks     int    `json:"weeks"`
	WeekDays  int    `json:"weekDays"` // days beyond the whole weeks
	Text      string `json:"text"`
	Negative  bool   `json:"negative,omitempty"`
}

// Between returns the span from a to b: whole years, months and days, as a person's
// age is counted (a birthday on 29 February is reached on 1 March in other years).
func Between(a, b time.Time) Span {
	a, b = day(a), day(b)
	span := Span{From: a.Format(time.DateOnly), To: b.Format(time.DateOnly)}
	if b.Before(a) {
		a, b = b, a
		span.Negative = true
	}
	years := b.Year() - a.Year()
	if a.AddDate(years, 0, 0).After(b) {
		years--
	}
	cursor := a.AddDate(years, 0, 0)
	months := 0
	for cursor.AddDate(0, months+1, 0).Compare(b) <= 0 {
		months++
	}
	cursor = cursor.AddDate(0, months, 0)
	span.Years, span.Months, span.Days = years, months, int(b.Sub(cursor).Hours()/24)
	span.TotalDays = int(b.Sub(a).Hours() / 24)
	span.Weeks, span.WeekDays = span.TotalDays/7, span.TotalDays%7
	span.Text = plural(span.Years, "year") + ", " + plural(span.Months, "month") + ", " + plural(span.Days, "day")
	return span
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
