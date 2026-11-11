package devtools

import (
	"regexp"
	"strings"
)

// Agent is what a User-Agent header says about the program that sent it.
type Agent struct {
	Browser        string `json:"browser,omitempty"`
	BrowserVersion string `json:"browserVersion,omitempty"`
	Engine         string `json:"engine,omitempty"`
	OS             string `json:"os,omitempty"`
	OSVersion      string `json:"osVersion,omitempty"`
	Device         string `json:"device"` // desktop, mobile, tablet, tv, bot, program, unknown
	Vendor         string `json:"vendor,omitempty"`
}

type uaRule struct {
	name string
	re   *regexp.Regexp
}

// Order matters: browsers built on Chrome name Chrome too, so they come first.
var browserRules = []uaRule{
	{"Edge", regexp.MustCompile(`Edg(?:e|A|iOS)?/([\d.]+)`)},
	{"Opera", regexp.MustCompile(`(?:OPR|Opera)/([\d.]+)`)},
	{"Samsung Internet", regexp.MustCompile(`SamsungBrowser/([\d.]+)`)},
	{"UC Browser", regexp.MustCompile(`UCBrowser/([\d.]+)`)},
	{"Yandex", regexp.MustCompile(`YaBrowser/([\d.]+)`)},
	{"Vivaldi", regexp.MustCompile(`Vivaldi/([\d.]+)`)},
	{"Brave", regexp.MustCompile(`Brave/([\d.]+)`)},
	{"JioPages", regexp.MustCompile(`JioPages/([\d.]+)`)},
	{"Firefox", regexp.MustCompile(`(?:Firefox|FxiOS)/([\d.]+)`)},
	{"Chrome", regexp.MustCompile(`(?:Chrome|CriOS)/([\d.]+)`)},
	{"Safari", regexp.MustCompile(`Version/([\d.]+).*Safari/`)},
	{"Internet Explorer", regexp.MustCompile(`(?:MSIE |Trident/.*rv:)([\d.]+)`)},
	{"curl", regexp.MustCompile(`^curl/([\d.]+)`)},
	{"Wget", regexp.MustCompile(`^Wget/([\d.]+)`)},
	{"Python requests", regexp.MustCompile(`python-requests/([\d.]+)`)},
	{"Python", regexp.MustCompile(`Python-urllib/([\d.]+)|python-httpx/([\d.]+)|aiohttp/([\d.]+)`)},
	{"Go", regexp.MustCompile(`Go-http-client/([\d.]+)`)},
	{"Node.js", regexp.MustCompile(`node-fetch/([\d.]+)|undici|axios/([\d.]+)|Node\.js/([\d.]+)`)},
	{"Java", regexp.MustCompile(`Java/([\d._]+)|okhttp/([\d.]+)|Apache-HttpClient/([\d.]+)`)},
	{"Postman", regexp.MustCompile(`PostmanRuntime/([\d.]+)`)},
	{"Dart", regexp.MustCompile(`Dart/([\d.]+)`)},
}

var osRules = []uaRule{
	{"Windows", regexp.MustCompile(`Windows NT ([\d.]+)`)},
	{"Android", regexp.MustCompile(`Android ([\d.]+)`)},
	{"iPadOS", regexp.MustCompile(`iPad.*OS ([\d_]+)`)},
	{"iOS", regexp.MustCompile(`(?:iPhone|iPad|iPod).*? OS ([\d_]+)`)},
	{"macOS", regexp.MustCompile(`Mac OS X ([\d_.]+)`)},
	{"Chrome OS", regexp.MustCompile(`CrOS \S+ ([\d.]+)`)},
	{"KaiOS", regexp.MustCompile(`KAIOS/([\d.]+)`)},
	{"Linux", regexp.MustCompile(`Linux`)},
}

var windowsNames = map[string]string{"10.0": "10 or 11", "6.3": "8.1", "6.2": "8", "6.1": "7", "6.0": "Vista", "5.1": "XP"}

var vendors = []struct{ token, name string }{
	{"SM-", "Samsung"}, {"Samsung", "Samsung"}, {"Redmi", "Xiaomi"}, {"Mi ", "Xiaomi"}, {"POCO", "Xiaomi"}, {"vivo", "vivo"}, {"OPPO", "OPPO"}, {"CPH", "OPPO"},
	{"RMX", "realme"}, {"OnePlus", "OnePlus"}, {"Pixel", "Google"}, {"moto", "Motorola"}, {"Nokia", "Nokia"}, {"iPhone", "Apple"}, {"iPad", "Apple"},
	{"Macintosh", "Apple"}, {"LAVA", "Lava"}, {"Micromax", "Micromax"}, {"Infinix", "Infinix"}, {"TECNO", "Tecno"}, {"JioPhone", "Jio"},
}

func firstGroup(m []string) string {
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

// ParseAgent reads a User-Agent header.
func ParseAgent(ua string) Agent {
	a := Agent{Device: "unknown"}
	if strings.TrimSpace(ua) == "" {
		return a
	}
	for _, rule := range browserRules {
		if m := rule.re.FindStringSubmatch(ua); m != nil {
			a.Browser, a.BrowserVersion = rule.name, firstGroup(m)
			break
		}
	}
	for _, rule := range osRules {
		if m := rule.re.FindStringSubmatch(ua); m != nil {
			a.OS = rule.name
			if len(m) > 1 {
				a.OSVersion = strings.ReplaceAll(m[1], "_", ".")
			}
			if a.OS == "Windows" {
				if name, ok := windowsNames[a.OSVersion]; ok {
					a.OSVersion = name
				}
			}
			break
		}
	}
	switch {
	case strings.Contains(ua, "Gecko/") && strings.Contains(ua, "Firefox"):
		a.Engine = "Gecko"
	case strings.Contains(ua, "AppleWebKit") && (strings.Contains(ua, "Chrome") || strings.Contains(ua, "CriOS")):
		a.Engine = "Blink"
	case strings.Contains(ua, "AppleWebKit"):
		a.Engine = "WebKit"
	case strings.Contains(ua, "Trident"):
		a.Engine = "Trident"
	}
	for _, v := range vendors {
		if strings.Contains(ua, v.token) {
			a.Vendor = v.name
			break
		}
	}
	lower := strings.ToLower(ua)
	switch {
	case strings.Contains(lower, "bot") || strings.Contains(lower, "crawler") || strings.Contains(lower, "spider") || strings.Contains(lower, "slurp"):
		a.Device = "bot"
	case strings.Contains(lower, "smart-tv") || strings.Contains(lower, "smarttv") || strings.Contains(lower, "tizen") || strings.Contains(lower, "webos") || strings.Contains(lower, "android tv"):
		a.Device = "tv"
	case strings.Contains(ua, "iPad") || strings.Contains(lower, "tablet") || (strings.Contains(ua, "Android") && !strings.Contains(ua, "Mobile")):
		a.Device = "tablet"
	case strings.Contains(ua, "Mobi") || strings.Contains(ua, "iPhone") || strings.Contains(ua, "KAIOS") || strings.Contains(ua, "JioPhone"):
		a.Device = "mobile"
	case a.OS == "Windows" || a.OS == "macOS" || a.OS == "Linux" || a.OS == "Chrome OS":
		a.Device = "desktop"
	case a.Browser != "" && a.Engine == "":
		a.Device = "program"
	}
	return a
}
