// Package festivals computes the dates of Indian festivals for a year from the
// panchang: lunar festivals by the tithi that prevails at the time the custom names
// (sunrise for most, sunset for Diwali and Holika Dahan, midnight for Janmashtami and
// Maha Shivaratri) in the amanta month; Tamil festivals by the solar month and the
// nakshatra at sunrise; Islamic festivals from the tabular Hijri calendar, marked as
// expected because the moon's sighting decides them; and the fixed and Christian dates.
//
// Lunar festivals are reckoned at New Delhi and Tamil ones at Chennai. Local almanacs
// can differ by a day where a tithi begins or ends near the deciding moment.
package festivals

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/indiafmt"
	"github.com/dharunashokkumar/api.dharun.dev/internal/panchang"
)

// Festival is one festival on one date.
type Festival struct {
	Date     string   `json:"date"`
	Name     string   `json:"name"`
	Tamil    string   `json:"tamil,omitempty"`
	Kind     string   `json:"kind"` // hindu, tamil, sikh, jain, buddhist, islamic, christian, national
	Rule     string   `json:"rule"`
	Expected bool     `json:"expected,omitempty"` // depends on the sighting of the moon
	Regions  []string `json:"regions,omitempty"`  // where it is mostly kept; empty means widely
}

var (
	ist     = time.FixedZone("IST", 5*3600+1800)
	delhi   = panchang.Place{Latitude: 28.6139, Longitude: 77.2090, Zone: ist}
	chennai = panchang.Place{Latitude: 13.0827, Longitude: 80.2707, Zone: ist}
)

// MinYear and MaxYear bound the years computed.
const (
	MinYear = 1950
	MaxYear = 2100
)

type day struct {
	date                          time.Time
	tithiRise, tithiSet, tithiMid int  // at New Delhi
	tithiNoon, tithiAfternoon     int  // midday, and the afternoon (aparahna)
	bhadraOverByMidnight          bool // the purnima's first half (Bhadra) is over at midnight
	tithiNextRise                 int
	tamilTithi, nakshatra         int // at sunrise in Chennai
	nakshatraSet                  int // in the evening (pradosha, two hours after sunset) in Chennai
	amanta                        string
	adhika                        bool
	tamilMonth                    string
	tamilDay                      int
}

func (d day) tithiAt(at string) int {
	switch at {
	case "sunset":
		return d.tithiSet
	case "midnight":
		return d.tithiMid
	case "midday":
		return d.tithiNoon
	case "afternoon":
		return d.tithiAfternoon
	}
	return d.tithiRise
}

type lunarRule struct {
	name, tamil, month string
	tithi              int // 1 to 15 shukla, 16 to 30 krishna (30 is amavasya)
	at                 string
	kind, rule         string
	regions            []string
	offset             int // days after the matching day (Holi is the day after Holika Dahan)
}

var lunarRules = []lunarRule{
	{name: "Vasant Panchami", month: "Magha", tithi: 5, kind: "hindu", rule: "Magha shukla panchami"},
	{name: "Maha Shivaratri", tamil: "மகா சிவராத்திரி", month: "Magha", tithi: 29, at: "midnight", kind: "hindu", rule: "Magha krishna chaturdashi at midnight (amanta; Phalguna in the north's reckoning)"},
	{name: "Holika Dahan", month: "Phalguna", tithi: 15, at: "holika", kind: "hindu", rule: "Phalguna purnima at sunset, after Bhadra (the purnima's first half) has ended; when Bhadra lasts past midnight, the next evening", regions: []string{"north"}},
	{name: "Holi", month: "Phalguna", tithi: 15, at: "holika", kind: "hindu", rule: "The day after Holika Dahan", offset: 1},
	{name: "Ugadi / Gudi Padwa", month: "Chaitra", tithi: 1, kind: "hindu", rule: "Chaitra shukla pratipada, the lunar new year", regions: []string{"AP", "TG", "KA", "MH", "GA"}},
	{name: "Ram Navami", month: "Chaitra", tithi: 9, kind: "hindu", rule: "Chaitra shukla navami"},
	{name: "Mahavir Jayanti", month: "Chaitra", tithi: 13, kind: "jain", rule: "Chaitra shukla trayodashi"},
	{name: "Hanuman Jayanti", month: "Chaitra", tithi: 15, kind: "hindu", rule: "Chaitra purnima (the north's date)", regions: []string{"north"}},
	{name: "Akshaya Tritiya", month: "Vaishakha", tithi: 3, kind: "hindu", rule: "Vaishakha shukla tritiya"},
	{name: "Buddha Purnima", month: "Vaishakha", tithi: 15, kind: "buddhist", rule: "Vaishakha purnima"},
	{name: "Rath Yatra", month: "Ashadha", tithi: 2, kind: "hindu", rule: "Ashadha shukla dvitiya", regions: []string{"OD"}},
	{name: "Guru Purnima", month: "Ashadha", tithi: 15, kind: "hindu", rule: "Ashadha purnima"},
	{name: "Nag Panchami", month: "Shravana", tithi: 5, kind: "hindu", rule: "Shravana shukla panchami"},
	{name: "Varalakshmi Vratam", month: "Shravana", tithi: 0, kind: "hindu", rule: "The Friday before Shravana purnima", regions: []string{"south"}},
	{name: "Raksha Bandhan", month: "Shravana", tithi: 15, kind: "hindu", rule: "Shravana purnima"},
	{name: "Krishna Janmashtami", tamil: "கிருஷ்ண ஜெயந்தி", month: "Shravana", tithi: 23, kind: "hindu", rule: "Krishna ashtami at sunrise in amanta Shravana (Bhadrapada in the north's reckoning), as the public holiday is kept; the smarta reckoning by midnight can be a day earlier"},
	{name: "Ganesh Chaturthi", tamil: "விநாயகர் சதுர்த்தி", month: "Bhadrapada", tithi: 4, at: "midday", kind: "hindu", rule: "Bhadrapada shukla chaturthi at midday (madhyahna)"},
	{name: "Navaratri begins", month: "Ashvina", tithi: 1, kind: "hindu", rule: "Ashvina shukla pratipada"},
	{name: "Durga Ashtami", month: "Ashvina", tithi: 8, kind: "hindu", rule: "Ashvina shukla ashtami"},
	{name: "Maha Navami / Ayudha Puja", tamil: "ஆயுத பூஜை", month: "Ashvina", tithi: 9, kind: "hindu", rule: "Ashvina shukla navami"},
	{name: "Dussehra / Vijayadashami", tamil: "விஜயதசமி", month: "Ashvina", tithi: 10, at: "afternoon", kind: "hindu", rule: "Ashvina shukla dashami in the afternoon (aparahna)"},
	{name: "Karva Chauth", month: "Ashvina", tithi: 19, kind: "hindu", rule: "Krishna chaturthi in amanta Ashvina (Kartika in the north's reckoning)", regions: []string{"north"}},
	{name: "Dhanteras", month: "Ashvina", tithi: 28, at: "sunset", kind: "hindu", rule: "Krishna trayodashi at sunset, amanta Ashvina"},
	{name: "Naraka Chaturdashi", tamil: "தீபாவளி (தமிழ்நாடு)", month: "Ashvina", tithi: 29, kind: "hindu", rule: "Krishna chaturdashi at sunrise, amanta Ashvina: Deepavali in the south"},
	{name: "Diwali (Lakshmi Puja)", tamil: "தீபாவளி", month: "Ashvina", tithi: 30, at: "sunset", kind: "hindu", rule: "Amavasya at sunset, end of amanta Ashvina"},
	{name: "Govardhan Puja", month: "Kartika", tithi: 1, kind: "hindu", rule: "Kartika shukla pratipada", regions: []string{"north"}},
	{name: "Bhai Dooj", month: "Kartika", tithi: 2, kind: "hindu", rule: "Kartika shukla dvitiya", regions: []string{"north"}},
	{name: "Chhath Puja", month: "Kartika", tithi: 6, kind: "hindu", rule: "Kartika shukla shashthi", regions: []string{"BR", "JH", "UP"}},
	{name: "Guru Nanak Jayanti", month: "Kartika", tithi: 15, kind: "sikh", rule: "Kartika purnima"},
	{name: "Vaikunta Ekadashi", tamil: "வைகுண்ட ஏகாதசி", month: "Margashirsha", tithi: 11, kind: "hindu", rule: "Shukla ekadashi of Margashirsha (Margazhi)", regions: []string{"south"}},
}

type tamilRule struct {
	name, tamil string
	month       string // Tamil month
	day         int    // a fixed day of the month, or
	nakshatra   int    // the nakshatra at sunrise (1 to 27)
	offset      int
	kind, rule  string
	regions     []string
	first       bool // take the first match rather than the one nearest the full moon
	sunset      bool // the nakshatra in the evening, when the lamps are lit
}

var tamilRules = []tamilRule{
	{name: "Bhogi", tamil: "போகி", month: "Thai", day: 1, offset: -1, kind: "tamil", rule: "The last day of Margazhi", regions: []string{"TN", "AP", "TG"}},
	{name: "Makar Sankranti / Pongal", tamil: "தைப் பொங்கல்", month: "Thai", day: 1, kind: "hindu", rule: "Thai 1: the sun enters Capricorn (sidereal)"},
	{name: "Mattu Pongal", tamil: "மாட்டுப் பொங்கல்", month: "Thai", day: 2, kind: "tamil", rule: "Thai 2", regions: []string{"TN"}},
	{name: "Kaanum Pongal", tamil: "காணும் பொங்கல்", month: "Thai", day: 3, kind: "tamil", rule: "Thai 3", regions: []string{"TN"}},
	{name: "Thaipusam", tamil: "தைப்பூசம்", month: "Thai", nakshatra: 8, kind: "tamil", rule: "Pusam nakshatra in Thai", regions: []string{"TN"}},
	{name: "Masi Magam", tamil: "மாசி மகம்", month: "Maasi", nakshatra: 10, kind: "tamil", rule: "Magam nakshatra in Maasi", regions: []string{"TN"}},
	{name: "Panguni Uthiram", tamil: "பங்குனி உத்திரம்", month: "Panguni", nakshatra: 12, kind: "tamil", rule: "Uthiram nakshatra in Panguni", regions: []string{"TN"}},
	{name: "Tamil New Year (Puthandu)", tamil: "தமிழ்ப் புத்தாண்டு", month: "Chithirai", day: 1, kind: "tamil", rule: "Chithirai 1: the sun enters Aries (sidereal)", regions: []string{"TN"}},
	{name: "Vaisakhi / Vishu", month: "Chithirai", day: 1, kind: "hindu", rule: "Mesha Sankranti, the solar new year", regions: []string{"PB", "KL", "AS", "OD", "WB"}},
	{name: "Vaikasi Visakam", tamil: "வைகாசி விசாகம்", month: "Vaikasi", nakshatra: 16, kind: "tamil", rule: "Visakam nakshatra in Vaikasi", regions: []string{"TN"}},
	{name: "Aadi Perukku", tamil: "ஆடிப் பெருக்கு", month: "Aadi", day: 18, kind: "tamil", rule: "Aadi 18", regions: []string{"TN"}},
	{name: "Onam (Thiruvonam)", tamil: "ஓணம்", month: "Aavani", nakshatra: 22, kind: "hindu", rule: "Thiruvonam nakshatra in Chingam (Aavani)", regions: []string{"KL"}, first: true},
	{name: "Karthigai Deepam", tamil: "கார்த்திகை தீபம்", month: "Karthigai", nakshatra: 3, sunset: true, kind: "tamil", rule: "Karthigai nakshatra in the evening (pradosha) in Karthigai", regions: []string{"TN"}},
	{name: "Arudra Darshan", tamil: "ஆருத்ரா தரிசனம்", month: "Margazhi", nakshatra: 6, kind: "tamil", rule: "Thiruvathirai nakshatra in Margazhi", regions: []string{"TN"}},
}

type fixedRule struct {
	month time.Month
	day   int
	name  string
	kind  string
}

var fixedRules = []fixedRule{
	{time.January, 1, "New Year's Day", "national"}, {time.January, 26, "Republic Day", "national"},
	{time.April, 14, "Ambedkar Jayanti", "national"}, {time.May, 1, "May Day", "national"},
	{time.August, 15, "Independence Day", "national"}, {time.October, 2, "Gandhi Jayanti", "national"},
	{time.December, 25, "Christmas", "christian"},
}

type islamicRule struct {
	month, day int
	name       string
}

var islamicRules = []islamicRule{
	{1, 1, "Islamic New Year"}, {1, 10, "Muharram (Ashura)"}, {3, 12, "Milad-un-Nabi"},
	{9, 1, "Ramadan begins"}, {10, 1, "Eid al-Fitr"}, {12, 10, "Eid al-Adha (Bakrid)"},
}

// easter is the Gregorian Easter Sunday (the anonymous Gregorian algorithm).
func easter(year int) time.Time {
	a, b, c := year%19, year/100, year%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	dayOf := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), dayOf, 0, 0, 0, 0, time.UTC)
}

var (
	cacheMu sync.Mutex
	cache   = map[int][]Festival{}
)

// Year returns the festivals of a year, by date. It takes a few hundred milliseconds
// the first time and is kept afterwards.
func Year(year int) ([]Festival, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if list, ok := cache[year]; ok {
		return list, nil
	}
	list, err := compute(year)
	if err != nil {
		return nil, err
	}
	if len(cache) > 20 {
		cache = map[int][]Festival{}
	}
	cache[year] = list
	return list, nil
}

func compute(year int) ([]Festival, error) {
	// Days from mid-December before to mid-January after, so that festivals near the
	// year's ends and the day-after rules are found.
	start := time.Date(year-1, time.December, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(year+1, time.January, 15, 0, 0, 0, 0, time.UTC)
	days, err := buildDays(start, end)
	if err != nil {
		return nil, err
	}
	var out []Festival
	add := func(f Festival) {
		if f.Date[:4] == itoa4(year) {
			out = append(out, f)
		}
	}
	for _, rule := range lunarRules {
		for _, f := range lunarDates(days, rule) {
			add(f)
		}
	}
	for _, rule := range tamilRules {
		for _, f := range tamilDates(days, rule) {
			add(f)
		}
	}
	for _, rule := range fixedRules {
		add(Festival{Date: time.Date(year, rule.month, rule.day, 0, 0, 0, 0, time.UTC).Format(time.DateOnly), Name: rule.name, Kind: rule.kind, Rule: "Fixed date"})
	}
	e := easter(year)
	add(Festival{Date: e.AddDate(0, 0, -2).Format(time.DateOnly), Name: "Good Friday", Kind: "christian", Rule: "The Friday before Easter"})
	add(Festival{Date: e.Format(time.DateOnly), Name: "Easter Sunday", Kind: "christian", Rule: "Gregorian Easter"})
	for d := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() == year; d = d.AddDate(0, 0, 1) {
		h := indiafmt.Hijri(d)
		for _, rule := range islamicRules {
			if h.Month == rule.month && h.Day == rule.day {
				add(Festival{Date: d.Format(time.DateOnly), Name: rule.name, Kind: "islamic", Expected: true,
					Rule: itoa4(rule.day) + " " + h.MonthName + " " + itoa4(h.Year) + " AH by the tabular calendar; the sighting of the moon can move it a day or two"})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// buildDays reads a few positions a day: the tithi at Delhi's sunrise, sunset and
// midnight, the tithi and nakshatra at Chennai's sunrise, and the lunar and Tamil months
// from the new moons and the sun's entries into the signs.
func buildDays(start, end time.Time) ([]day, error) {
	from, until := start.AddDate(0, 0, -40), end.AddDate(0, 0, 40)
	moons, err := panchang.NewMoons(from, until)
	if err != nil {
		return nil, err
	}
	type lunarMonth struct {
		begins time.Time
		name   string
		adhika bool
	}
	var months []lunarMonth
	for i := 0; i+1 < len(moons); i++ {
		sunThen, _, err := panchang.Positions(moons[i])
		if err != nil {
			return nil, err
		}
		sunNext, _, err := panchang.Positions(moons[i+1])
		if err != nil {
			return nil, err
		}
		months = append(months, lunarMonth{moons[i], panchang.LunarMonthName(int(sunThen / 30)), int(sunThen/30) == int(sunNext/30)})
	}
	entries, signs, err := panchang.Sankrantis(from, until)
	if err != nil {
		return nil, err
	}
	// A Tamil month begins on the day the sun enters the sign, if before sunset in
	// Chennai, or else on the next day.
	type solarStart struct {
		date time.Time
		sign int
	}
	var solar []solarStart
	for i, entered := range entries {
		local := entered.In(ist)
		date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		_, set, err := panchang.SunTimes(date, chennai)
		if err != nil {
			return nil, err
		}
		if entered.After(set) {
			date = date.AddDate(0, 0, 1)
		}
		solar = append(solar, solarStart{date, signs[i]})
	}
	var days []day
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		rise, set, err := panchang.SunTimes(d, delhi)
		if err != nil {
			return nil, err
		}
		var x day
		x.date = d
		if x.tithiRise, err = panchang.TithiAt(rise); err != nil {
			return nil, err
		}
		if x.tithiSet, err = panchang.TithiAt(set); err != nil {
			return nil, err
		}
		midnight := time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 0, 0, ist)
		if x.tithiMid, err = panchang.TithiAt(midnight); err != nil {
			return nil, err
		}
		daytime := set.Sub(rise)
		if x.tithiNoon, err = panchang.TithiAt(rise.Add(daytime / 2)); err != nil {
			return nil, err
		}
		if x.tithiAfternoon, err = panchang.TithiAt(rise.Add(daytime * 7 / 10)); err != nil {
			return nil, err
		}
		sunMid, moonMid, err := panchang.Positions(midnight)
		if err != nil {
			return nil, err
		}
		x.bhadraOverByMidnight = math.Mod(moonMid-sunMid+720, 360) >= 174
		southRise, southSet, err := panchang.SunTimes(d, chennai)
		if err != nil {
			return nil, err
		}
		sun, moon, err := panchang.Positions(southRise)
		if err != nil {
			return nil, err
		}
		elongation := math.Mod(moon-sun+720, 360)
		x.tamilTithi, x.nakshatra = int(elongation/12)+1, int(moon/(360.0/27))+1
		if x.nakshatraSet, err = panchang.NakshatraAt(southSet.Add(2 * time.Hour)); err != nil {
			return nil, err
		}
		for _, m := range months {
			if !m.begins.After(rise) {
				x.amanta, x.adhika = m.name, m.adhika
			}
		}
		for _, sm := range solar {
			if !sm.date.After(d) {
				x.tamilMonth, _ = panchang.TamilMonthName(sm.sign)
				x.tamilDay = int(d.Sub(sm.date).Hours()/24) + 1
			}
		}
		if len(days) > 0 {
			days[len(days)-1].tithiNextRise = x.tithiRise
		}
		days = append(days, x)
	}
	if len(days) > 0 {
		days[len(days)-1].tithiNextRise = days[len(days)-1].tithiRise
	}
	return days, nil
}

// skipped reports whether tithi begins and ends between two sunrises (a kshaya tithi).
func skipped(from, to, tithi int) bool {
	if from == to {
		return false
	}
	for t := from%30 + 1; t != to && t != from; t = t%30 + 1 {
		if t == tithi {
			return true
		}
	}
	return false
}

func itoa4(n int) string {
	if n < 0 {
		return "-" + itoa4(-n)
	}
	digits := []byte{}
	for n > 0 || len(digits) == 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// lunarDates finds every day a lunar rule falls on in the days given (one a year, two
// when the month comes round twice in the span).
func lunarDates(days []day, rule lunarRule) []Festival {
	var out []Festival
	for i := 0; i < len(days); i++ {
		d := days[i]
		if d.adhika || d.amanta != rule.month {
			continue
		}
		if rule.tithi == 0 { // Varalakshmi: the Friday before Shravana purnima
			if d.tithiRise == 15 {
				for j := i; j >= 0 && j > i-8; j-- {
					if days[j].date.Weekday() == time.Friday && days[j].date.Before(d.date) {
						out = append(out, festival(rule.name, rule.tamil, rule.kind, rule.rule, rule.regions, days[j].date))
						break
					}
				}
				i += 20
			}
			continue
		}
		match := d.tithiAt(rule.at) == rule.tithi
		if rule.at == "holika" {
			match = d.tithiSet == 15 && d.bhadraOverByMidnight
			if d.tithiSet == 15 && !d.bhadraOverByMidnight && i+1 < len(days) {
				d = days[i+1] // Bhadra holds the whole night: the next evening
				match = true
			}
		}
		// A tithi that begins after one sunrise and ends before the next is never the
		// sunrise tithi: its day is the one it falls in.
		if !match && rule.at == "" && skipped(d.tithiRise, d.tithiNextRise, rule.tithi) {
			match = true
		}
		if match {
			out = append(out, festival(rule.name, rule.tamil, rule.kind, rule.rule, rule.regions, d.date.AddDate(0, 0, rule.offset)))
			i += 20 // the next one is a lunar month away
		}
	}
	return out
}

// tamilDates finds a Tamil rule's day: a fixed day of the month, or the nakshatra at
// sunrise. A month can hold a nakshatra twice; the day nearest the full moon is kept,
// as the temples keep it.
func tamilDates(days []day, rule tamilRule) []Festival {
	var out []Festival
	var candidates []day
	flush := func() {
		if len(candidates) == 0 {
			return
		}
		best := candidates[0]
		if !rule.first {
			distance := func(d day) int {
				n := d.tamilTithi
				return min(abs(n-15), abs(n-15+30), abs(n-15-30))
			}
			for _, c := range candidates[1:] {
				if distance(c) < distance(best) {
					best = c
				}
			}
		}
		out = append(out, festival(rule.name, rule.tamil, rule.kind, rule.rule, rule.regions, best.date.AddDate(0, 0, rule.offset)))
		candidates = nil
	}
	previousMonth := ""
	for i, d := range days {
		month := d.tamilMonth
		if month != previousMonth {
			flush()
			previousMonth = month
		}
		if month != rule.month {
			continue
		}
		if rule.day > 0 {
			if d.tamilDay == rule.day {
				candidates = append(candidates, d)
			}
			continue
		}
		star := func(d day) int {
			if rule.sunset {
				return d.nakshatraSet
			}
			return d.nakshatra
		}
		if star(d) == rule.nakshatra {
			// Two days in one nakshatra: the first.
			if i > 0 && days[i-1].tamilMonth == month && star(days[i-1]) == rule.nakshatra {
				continue
			}
			candidates = append(candidates, d)
		}
	}
	flush()
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func festival(name, tamil, kind, rule string, regions []string, date time.Time) Festival {
	return Festival{Date: date.Format(time.DateOnly), Name: name, Tamil: tamil, Kind: kind, Rule: rule, Regions: regions}
}
