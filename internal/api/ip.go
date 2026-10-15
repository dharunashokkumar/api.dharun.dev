package api

import (
	"math"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// ipReaders are the memory-mapped DB-IP files of the ip dataset in use.
type ipReaders struct {
	city, asn *maxminddb.Reader
}

func (readers *ipReaders) close() {
	readers.city.Close()
	readers.asn.Close()
}

// openIP maps the files of a newly swapped-in ip dataset. The previous files stay mapped
// for a minute so lookups already running can finish.
func (s *Server) openIP() error {
	set := s.data("ip")
	if set == nil {
		return nil
	}
	city, err := maxminddb.Open(set.attachment("city"))
	if err != nil {
		logError("opening the IP city database", err)
		return err
	}
	asn, err := maxminddb.Open(set.attachment("asn"))
	if err != nil {
		city.Close()
		logError("opening the IP network database", err)
		return err
	}
	if old := s.extras.ip.Swap(&ipReaders{city: city, asn: asn}); old != nil {
		time.AfterFunc(time.Minute, old.close)
	}
	return nil
}

type ipPlace struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ipNetwork struct {
	Number       uint   `json:"number"`
	Organization string `json:"organization"`
}

type ipView struct {
	IP        string     `json:"ip"`
	Version   int        `json:"version"`
	Country   *ipPlace   `json:"country"`
	Continent *string    `json:"continent"`
	Region    *string    `json:"region"`
	StateCode *string    `json:"stateCode,omitempty"`
	City      *string    `json:"city"`
	Latitude  *float64   `json:"latitude"`
	Longitude *float64   `json:"longitude"`
	ASN       *ipNetwork `json:"asn"`
}

type dbipCity struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Continent struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"continent"`
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Location struct {
		Latitude  *float64 `maxminddb:"latitude"`
		Longitude *float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
}

type dbipASN struct {
	Number       uint   `maxminddb:"autonomous_system_number"`
	Organization string `maxminddb:"autonomous_system_organization"`
}

// reservedPrefixes are the shared, documentation and benchmarking ranges netip does not
// flag itself.
var reservedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001:db8::/32"),
}

func reserved(addr netip.Addr) bool {
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() || addr.IsLinkLocalMulticast() {
		return true
	}
	for _, prefix := range reservedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (s *Server) lookupIP(w http.ResponseWriter, r *http.Request, raw string) {
	// An address can point at a person; nothing about it is logged or shared-cached.
	info(r).private = true
	if s.need(w, r, "ip") == nil {
		return
	}
	readers := s.extras.ip.Load()
	if readers == nil {
		w.Header().Set("Retry-After", "600")
		s.fail(w, r, http.StatusServiceUnavailable, "dataset_unavailable", "The IP databases could not be opened. Try again later.")
		return
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		s.badRequest(w, r, "That is not an IPv4 or IPv6 address.")
		return
	}
	addr = addr.Unmap().WithZone("")
	if reserved(addr) {
		s.fail(w, r, http.StatusUnprocessableEntity, "reserved_address", addr.String()+" is a private or reserved address: it has no public location.")
		return
	}
	view := ipView{IP: addr.String(), Version: 4}
	if addr.Is6() {
		view.Version = 6
	}
	var city dbipCity
	if err := readers.city.Lookup(addr).Decode(&city); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	if city.Country.ISOCode != "" {
		view.Country = &ipPlace{Code: city.Country.ISOCode, Name: city.Country.Names["en"]}
	}
	view.Continent, view.City = optional(city.Continent.Code), optional(city.City.Names["en"])
	if len(city.Subdivisions) > 0 {
		view.Region = optional(city.Subdivisions[0].Names["en"])
		if state, ok := india.Lookup(city.Subdivisions[0].Names["en"]); ok && city.Country.ISOCode == "IN" {
			view.StateCode = &state.Code
		}
	}
	view.Latitude, view.Longitude = city.Location.Latitude, city.Location.Longitude
	var network dbipASN
	if readers.asn.Lookup(addr).Decode(&network) == nil && network.Number != 0 {
		view.ASN = &ipNetwork{Number: network.Number, Organization: strings.TrimRight(network.Organization, ", ")}
	}
	if view.Country == nil && view.ASN == nil {
		s.notFound(w, r, "No record for "+addr.String()+".")
		return
	}
	s.ok(w, r, view, meta{Source: s.source("ip"),
		Note: "The position is that of a city or district for the address range, not of a device or a person."}, "private, no-store")
}

func (s *Server) handleIP(w http.ResponseWriter, r *http.Request) {
	s.lookupIP(w, r, r.PathValue("address"))
}

// handleMyIP looks up the address the request came from.
func (s *Server) handleMyIP(w http.ResponseWriter, r *http.Request) {
	s.lookupIP(w, r, s.clientKey(r))
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "lookup_ip_address",
		Description: "Find the country, region (Indian state), city and network operator of a public IPv4 or IPv6 address.",
		InputSchema: schema([]string{"ip"}, map[string]any{"ip": text("IPv4 or IPv6 address.")}),
		path: func(args map[string]any) (string, error) {
			ip, err := required(args, "ip")
			return "/v1/ip/" + url.PathEscape(ip), err
		},
	})
}

// clientPlace is roughly where a request came from, as far as its address tells: what
// the access log and the visitor metrics record instead of the address itself.
type clientPlace struct {
	country, region, city, network string
	state                          string // the Indian state's official name, when in India
	asn                            uint
	latitude, longitude            *float64
}

// locate looks up a client address in the IP databases. Without them, or for an address
// they do not know, the country comes from Cloudflare's header when the server sits
// behind it.
func (s *Server) locate(r *http.Request, client string) clientPlace {
	var place clientPlace
	addr, err := netip.ParseAddr(client)
	readers := s.extras.ip.Load()
	if err == nil && readers != nil && !reserved(addr.Unmap()) {
		addr = addr.Unmap().WithZone("")
		var city dbipCity
		if readers.city.Lookup(addr).Decode(&city) == nil {
			place.country, place.city = countryCode(city.Country.ISOCode), city.City.Names["en"]
			if len(city.Subdivisions) > 0 {
				place.region = city.Subdivisions[0].Names["en"]
			}
			place.latitude, place.longitude = city.Location.Latitude, city.Location.Longitude
		}
		var network dbipASN
		if readers.asn.Lookup(addr).Decode(&network) == nil && network.Number != 0 {
			place.asn, place.network = network.Number, strings.TrimRight(network.Organization, ", ")
		}
	}
	if place.country == "" && s.cfg.TrustProxy {
		place.country = countryCode(r.Header.Get("CF-IPCountry"))
	}
	if place.country == "IN" {
		if state, ok := india.Lookup(place.region); ok {
			place.state = state.Name
		}
	}
	return place
}

// countryCode keeps a two-letter country code and drops anything else, such as
// Cloudflare's XX (unknown) and T1 (Tor), so metric labels stay a known small set.
func countryCode(value string) string {
	if len(value) != 2 || value == "XX" || value == "ZZ" {
		return ""
	}
	for _, letter := range []byte(value) {
		if letter < 'A' || letter > 'Z' {
			return ""
		}
	}
	return value
}

// addTo writes the place into a log entry. Coordinates are those of the city the
// address range is in, rounded to about a kilometre.
func (place clientPlace) addTo(entry map[string]any) {
	for field, value := range map[string]string{"country": place.country, "region": place.region, "city": place.city, "network": place.network} {
		if value != "" {
			entry[field] = clip(value, 100)
		}
	}
	if place.asn != 0 {
		entry["asn"] = place.asn
	}
	if place.latitude != nil && place.longitude != nil {
		entry["lat"] = math.Round(*place.latitude*100) / 100
		entry["lon"] = math.Round(*place.longitude*100) / 100
	}
}
