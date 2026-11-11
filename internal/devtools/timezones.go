package devtools

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// zoneNames are the short names people write, mapped to IANA zones. Abbreviations are
// ambiguous (IST is also Irish and Israel time); here they mean the common reading for
// an Indian audience.
var zoneNames = map[string]string{
	"ist": "Asia/Kolkata", "india": "Asia/Kolkata", "utc": "UTC", "gmt": "UTC", "z": "UTC",
	"bst": "Europe/London", "uk": "Europe/London", "london": "Europe/London", "cet": "Europe/Berlin", "cest": "Europe/Berlin", "berlin": "Europe/Berlin",
	"paris": "Europe/Paris", "et": "America/New_York", "est": "America/New_York", "edt": "America/New_York", "new york": "America/New_York", "nyc": "America/New_York",
	"ct": "America/Chicago", "cst": "America/Chicago", "cdt": "America/Chicago", "mt": "America/Denver", "mst": "America/Denver", "mdt": "America/Denver",
	"pt": "America/Los_Angeles", "pst": "America/Los_Angeles", "pdt": "America/Los_Angeles", "san francisco": "America/Los_Angeles", "sf": "America/Los_Angeles",
	"jst": "Asia/Tokyo", "tokyo": "Asia/Tokyo", "sgt": "Asia/Singapore", "singapore": "Asia/Singapore", "gst": "Asia/Dubai", "dubai": "Asia/Dubai",
	"aest": "Australia/Sydney", "aedt": "Australia/Sydney", "sydney": "Australia/Sydney", "npt": "Asia/Kathmandu", "pkt": "Asia/Karachi", "bdt": "Asia/Dhaka",
	"sl": "Asia/Colombo", "hkt": "Asia/Hong_Kong", "kst": "Asia/Seoul", "msk": "Europe/Moscow", "sast": "Africa/Johannesburg", "brt": "America/Sao_Paulo",
	"chennai": "Asia/Kolkata", "mumbai": "Asia/Kolkata", "delhi": "Asia/Kolkata", "bengaluru": "Asia/Kolkata", "bangalore": "Asia/Kolkata", "kolkata": "Asia/Kolkata",
	"toronto": "America/Toronto", "vancouver": "America/Vancouver", "amsterdam": "Europe/Amsterdam", "zurich": "Europe/Zurich", "riyadh": "Asia/Riyadh",
	"doha": "Asia/Qatar", "kuala lumpur": "Asia/Kuala_Lumpur", "jakarta": "Asia/Jakarta", "bangkok": "Asia/Bangkok", "shanghai": "Asia/Shanghai", "beijing": "Asia/Shanghai",
	"auckland": "Pacific/Auckland", "nairobi": "Africa/Nairobi", "lagos": "Africa/Lagos", "cairo": "Africa/Cairo", "istanbul": "Europe/Istanbul",
}

var offsetZone = regexp.MustCompile(`^(?i:utc|gmt)?\s*([+-])(\d{1,2})(?::?(\d{2}))?$`)

// Zone finds a time zone by IANA name, a common short name or city, or an offset.
func Zone(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("no zone given")
	}
	if iana, ok := zoneNames[strings.ToLower(name)]; ok {
		return time.LoadLocation(iana)
	}
	if m := offsetZone.FindStringSubmatch(name); m != nil {
		h, _ := strconv.Atoi(m[2])
		mins, _ := strconv.Atoi(m[3])
		if h > 14 || mins > 59 {
			return nil, errors.New("offset out of range")
		}
		seconds := h*3600 + mins*60
		if m[1] == "-" {
			seconds = -seconds
		}
		return time.FixedZone(name, seconds), nil
	}
	if strings.Contains(name, "/") || name == "UTC" {
		if loc, err := time.LoadLocation(name); err == nil {
			return loc, nil
		}
	}
	return nil, errors.New("unknown time zone " + strconv.Quote(name) + "; use an IANA name such as Europe/Berlin, a short name such as PST, or an offset such as +05:30")
}

var timeLayouts = []string{
	time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02",
	"15:04:05", "15:04", "3:04pm", "3:04 pm", "3pm", "3 pm", "3:04PM", "3:04 PM", "3PM", "3 PM",
	time.RFC1123, time.RFC1123Z, time.RFC822, time.RFC850, time.ANSIC, "02 Jan 2006 15:04", "2 January 2006 15:04", "02/01/2006 15:04", "02-01-2006 15:04", "02/01/2006",
}

// ParseTime reads a moment written many ways. A Unix time in seconds or milliseconds is
// read too. A time without a date is today in the zone; one without a zone is in zone.
func ParseTime(text string, zone *time.Location, now time.Time) (time.Time, error) {
	text = strings.TrimSpace(text)
	if text == "" || strings.EqualFold(text, "now") {
		return now.In(zone), nil
	}
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		if n > 1e11 || n < -1e11 {
			return time.UnixMilli(n).In(zone), nil
		}
		return time.Unix(n, 0).In(zone), nil
	}
	for _, layout := range timeLayouts {
		t, err := time.ParseInLocation(layout, text, zone)
		if err != nil {
			continue
		}
		if !strings.Contains(layout, "2006") && !strings.Contains(layout, "06") {
			today := now.In(zone)
			t = time.Date(today.Year(), today.Month(), today.Day(), t.Hour(), t.Minute(), t.Second(), 0, zone)
		}
		return t, nil
	}
	return time.Time{}, errors.New("the time cannot be read; write it like 2026-11-11 15:00, 3pm, 15:30 or a Unix time")
}

// ZoneTime is a moment shown in one zone.
type ZoneTime struct {
	Zone    string `json:"zone"`
	Time    string `json:"time"`
	Local   string `json:"local"`
	Offset  string `json:"offset"`
	Abbrev  string `json:"abbreviation"`
	DayDiff int    `json:"dayDifference"` // days ahead of (or behind) the first zone's date
}

// In shows t in the zone, with the day difference from the reference date.
func In(t time.Time, loc *time.Location, name string, reference time.Time) ZoneTime {
	local := t.In(loc)
	abbrev, offset := local.Zone()
	sign := "+"
	if offset < 0 {
		sign, offset = "-", -offset
	}
	ref := time.Date(reference.Year(), reference.Month(), reference.Day(), 0, 0, 0, 0, time.UTC)
	here := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return ZoneTime{Zone: name, Time: local.Format(time.RFC3339), Local: local.Format("Mon 2 Jan 2006, 15:04"),
		Offset: sign + strconv.Itoa(offset/3600/10) + strconv.Itoa(offset/3600%10) + ":" + strconv.Itoa(offset%3600/60/10) + strconv.Itoa(offset%3600/60%10),
		Abbrev: abbrev, DayDiff: int(here.Sub(ref).Hours() / 24)}
}
