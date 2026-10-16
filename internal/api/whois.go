package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
)

// Whois through RDAP, the registries' JSON successor to the old whois protocol. IANA's
// bootstrap files (dataset rdap) name the server that answers for a top-level domain,
// an IP network or a range of AS numbers; the registration is fetched from it live.
// Contacts of the people behind a domain are never returned: only the registrar and its
// abuse contact. For an IP network or AS number the holder is named only when the
// registry marks it as an organisation.

var asnShape = regexp.MustCompile(`^(?i:as)?(\d{1,10})$`)

// registryLimit is how many lookups one registry gets from this server in a minute.
const registryLimit = 60

type whoisContact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type whoisRegistrar struct {
	Name   string        `json:"name"`
	IANAID string        `json:"ianaId,omitempty"`
	URL    string        `json:"url,omitempty"`
	Abuse  *whoisContact `json:"abuse"`
}

type whoisDomain struct {
	Query        string          `json:"query"`
	Kind         string          `json:"kind"`
	Domain       string          `json:"domain"`
	UnicodeName  string          `json:"unicodeName"`
	Registrar    *whoisRegistrar `json:"registrar"`
	Registered   *string         `json:"registered"`
	Expires      *string         `json:"expires"`
	DaysToExpiry *int            `json:"daysToExpiry"`
	Updated      *string         `json:"updated"`
	Status       []string        `json:"status"`
	Nameservers  []string        `json:"nameservers"`
	DNSSEC       *bool           `json:"dnssec"`
	Registry     string          `json:"registry"`
}

type whoisNetwork struct {
	Query      string        `json:"query"`
	Kind       string        `json:"kind"`
	Start      string        `json:"start,omitempty"`
	End        string        `json:"end,omitempty"`
	CIDRs      []string      `json:"cidrs,omitempty"`
	ASN        *uint64       `json:"asn,omitempty"`
	Name       string        `json:"name"`
	Handle     string        `json:"handle"`
	Type       string        `json:"type,omitempty"`
	Country    *string       `json:"country"`
	Holder     *string       `json:"holder"`
	Abuse      *whoisContact `json:"abuse"`
	Registered *string       `json:"registered"`
	Updated    *string       `json:"updated"`
	Status     []string      `json:"status"`
	Registry   string        `json:"registry"`
}

type rdapEntity struct {
	Roles     []string `json:"roles"`
	Handle    string   `json:"handle"`
	VCard     []any    `json:"vcardArray"`
	PublicIDs []struct {
		Type       string `json:"type"`
		Identifier string `json:"identifier"`
	} `json:"publicIds"`
	Entities []rdapEntity `json:"entities"`
	Links    []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
		Type string `json:"type"`
	} `json:"links"`
}

type rdapObject struct {
	ObjectClassName string `json:"objectClassName"`
	LDHName         string `json:"ldhName"`
	UnicodeName     string `json:"unicodeName"`
	Handle          string `json:"handle"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Country         string `json:"country"`
	StartAddress    string `json:"startAddress"`
	EndAddress      string `json:"endAddress"`
	StartAutnum     *uint64
	Status          []string `json:"status"`
	Events          []struct {
		Action string `json:"eventAction"`
		Date   string `json:"eventDate"`
	} `json:"events"`
	Entities    []rdapEntity `json:"entities"`
	Nameservers []struct {
		LDHName string `json:"ldhName"`
	} `json:"nameservers"`
	SecureDNS *struct {
		DelegationSigned *bool `json:"delegationSigned"`
	} `json:"secureDNS"`
	CIDRs []struct {
		V4     string `json:"v4prefix"`
		V6     string `json:"v6prefix"`
		Length int    `json:"length"`
	} `json:"cidr0_cidrs"`
}

// vcard returns the values of one property of an entity's vCard (jCard, RFC 7095).
func (e rdapEntity) vcard(property string) []string {
	if len(e.VCard) < 2 {
		return nil
	}
	fields, _ := e.VCard[1].([]any)
	var values []string
	for _, field := range fields {
		parts, _ := field.([]any)
		if len(parts) < 4 || parts[0] != property {
			continue
		}
		switch value := parts[3].(type) {
		case string:
			if value = strings.TrimSpace(value); value != "" {
				values = append(values, value)
			}
		case []any:
			// A structured value (an address, or an org with units): its first part.
			if len(value) > 0 {
				if text, ok := value[0].(string); ok && strings.TrimSpace(text) != "" {
					values = append(values, strings.TrimSpace(text))
				}
			}
		}
	}
	return values
}

func (e rdapEntity) first(property string) string {
	if values := e.vcard(property); len(values) > 0 {
		return values[0]
	}
	return ""
}

func (e rdapEntity) has(role string) bool {
	for _, r := range e.Roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}

// contact is a role contact's name, mailbox and phone. Only used for registrars and
// abuse desks, which publish these to be reached.
func (e rdapEntity) contact() *whoisContact {
	contact := &whoisContact{Name: e.first("fn"), Email: e.first("email"), Phone: strings.TrimPrefix(e.first("tel"), "tel:")}
	if contact.Name == "" && contact.Email == "" && contact.Phone == "" {
		return nil
	}
	return contact
}

// abuse finds the abuse contact among entities, looking one level down too.
func abuseContact(entities []rdapEntity) *whoisContact {
	for _, entity := range entities {
		if entity.has("abuse") {
			if contact := entity.contact(); contact != nil && contact.Email != "" {
				return contact
			}
		}
	}
	for _, entity := range entities {
		if contact := abuseContact(entity.Entities); contact != nil {
			return contact
		}
	}
	return nil
}

// rdapDate puts a registry's timestamp in one form: UTC to the second.
func rdapDate(raw string) *string {
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw)); err == nil {
		text := parsed.UTC().Format(time.RFC3339)
		return &text
	}
	return nil
}

func (o rdapObject) event(action string) *string {
	for _, event := range o.Events {
		if strings.EqualFold(event.Action, action) {
			return rdapDate(event.Date)
		}
	}
	return nil
}

// registeredDomain is the domain a registry holds for a name: google.co.in for
// www.google.co.in, github.io for user.github.io (a suffix only the public suffix list's
// private section knows is still registered as one domain).
func registeredDomain(name string) (string, error) {
	labels := strings.Split(name, ".")
	for i := 1; i < len(labels); i++ {
		candidate := strings.Join(labels[i:], ".")
		if suffix, icann := publicsuffix.PublicSuffix(candidate); icann && suffix == candidate {
			return strings.Join(labels[i-1:], "."), nil
		}
	}
	if len(labels) < 2 {
		return "", errBadName
	}
	// A top-level domain the list does not know: take the last two labels.
	return strings.Join(labels[len(labels)-2:], "."), nil
}

func (s *Server) handleWhois(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "rdap")
	if set == nil {
		return
	}
	raw := strings.TrimSpace(r.PathValue("query"))
	var server, path, kind, query string
	if match := asnShape.FindStringSubmatch(raw); match != nil {
		number, err := strconv.ParseUint(match[1], 10, 32)
		if err != nil {
			s.badRequest(w, r, "An AS number is from 1 to 4294967295.")
			return
		}
		kind, query, path = "asn", "AS"+match[1], "autnum/"+match[1]
		info(r).subject = query
		set.row(`SELECT url FROM asns WHERE first <= ? AND last >= ? ORDER BY last - first LIMIT 1`, number, number).Scan(&server)
	} else if addr, err := netip.ParseAddr(raw); err == nil {
		// An address can point at a person; it is neither logged nor shared-cached.
		info(r).private = true
		addr = addr.Unmap().WithZone("")
		kind, query, path = "ip", addr.String(), "ip/"+addr.String()
		server = rdapNetworkServer(set, addr)
	} else {
		host, err := hostOf(raw)
		if err != nil {
			s.badRequest(w, r, "Give a domain name, an IP address or an AS number such as AS55836.")
			return
		}
		domain, err := registeredDomain(host)
		if err != nil {
			s.badRequest(w, r, "Give a registered domain name such as example.co.in, not a top-level domain.")
			return
		}
		kind, query, path = "domain", domain, "domain/"+domain
		info(r).subject = domain
		tld := domain[strings.LastIndex(domain, ".")+1:]
		set.row(`SELECT url FROM tlds WHERE tld = ? LIMIT 1`, tld).Scan(&server)
		if server == "" {
			unicode, _ := idna.ToUnicode(tld)
			s.notFound(w, r, "The registry for ."+unicode+" offers no RDAP service, so its domains cannot be looked up here.")
			return
		}
	}
	if server == "" {
		s.notFound(w, r, "No registry offers RDAP for "+query+".")
		return
	}
	registry, _ := url.Parse(server)
	if !s.extras.network.registries.Allow(registry.Hostname()) {
		w.Header().Set("Retry-After", "60")
		s.fail(w, r, http.StatusTooManyRequests, "target_limited", "Too many lookups at this registry in the last minute. Try again in a minute.")
		return
	}

	object, status, err := s.fetchRDAP(r.Context(), server+path)
	switch {
	case err != nil:
		s.failNetwork(w, r, registry.Hostname(), err)
		return
	case status == http.StatusNotFound:
		if kind == "domain" {
			s.notFound(w, r, query+" is not registered, or the registry does not say.")
		} else {
			s.notFound(w, r, "The registry has no record of "+query+".")
		}
		return
	case status == http.StatusTooManyRequests:
		w.Header().Set("Retry-After", "60")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The registry is limiting lookups. Try again in a minute.")
		return
	case status != http.StatusOK:
		s.fail(w, r, http.StatusBadGateway, "registry_error", "The registry answered with HTTP status "+strconv.Itoa(status)+".")
		return
	}

	note := "Registration details fetched just now from " + registry.Hostname() + " over RDAP."
	cache := cacheAPI
	if info(r).private {
		cache = "private, no-store"
	}
	if kind == "domain" {
		note += " The registrant's, administrative and technical contacts are not shown."
		s.ok(w, r, domainView(query, server, object), meta{Source: s.source("rdap"), Note: note}, cache)
		return
	}
	s.ok(w, r, networkView(kind, query, server, object), meta{Source: s.source("rdap"), Note: note}, cache)
}

// rdapNetworkServer finds the registry for an address: the most specific network in
// the bootstrap file that holds it.
func rdapNetworkServer(set *handle, addr netip.Addr) string {
	version := 6
	if addr.Is4() {
		version = 4
	}
	rows, err := set.rows(`SELECT prefix, url FROM networks WHERE version = ?`, version)
	if err != nil {
		return ""
	}
	defer rows.Close()
	best, server := -1, ""
	for rows.Next() {
		var text, address string
		if rows.Scan(&text, &address) != nil {
			continue
		}
		if prefix, err := netip.ParsePrefix(text); err == nil && prefix.Contains(addr) && prefix.Bits() > best {
			best, server = prefix.Bits(), address
		}
	}
	return server
}

// fetchRDAP asks a registry. Registries send lookups for space they handed to another
// registry on with a redirect, which is followed (each through the guard).
func (s *Server) fetchRDAP(ctx context.Context, address string) (rdapObject, int, error) {
	var object rdapObject
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	client := s.guard(443).Client(netguard.ClientOptions{Timeout: 12 * time.Second, Roots: s.extras.network.roots, Redirects: 3, UserAgent: probeAgent})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return object, 0, err
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")
	resp, err := client.Do(req)
	if err != nil {
		return object, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return object, resp.StatusCode, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return object, 0, err
	}
	if err := json.Unmarshal(body, &object); err != nil {
		return object, 0, errors.New("the registry's answer is not RDAP JSON")
	}
	return object, http.StatusOK, nil
}

func domainView(query, server string, object rdapObject) whoisDomain {
	view := whoisDomain{Query: query, Kind: "domain", Domain: strings.ToLower(object.LDHName), Registry: server,
		Status: []string{}, Nameservers: []string{}}
	if view.Domain == "" {
		view.Domain = query
	}
	view.UnicodeName, _ = idna.ToUnicode(view.Domain)
	view.Registered, view.Expires, view.Updated = object.event("registration"), object.event("expiration"), object.event("last changed")
	if view.Expires != nil {
		if expires, err := time.Parse(time.RFC3339, *view.Expires); err == nil {
			days := int(time.Until(expires).Hours() / 24)
			view.DaysToExpiry = &days
		}
	}
	view.Status = append(view.Status, object.Status...)
	for _, ns := range object.Nameservers {
		if name := strings.TrimSuffix(strings.ToLower(ns.LDHName), "."); name != "" {
			view.Nameservers = append(view.Nameservers, name)
		}
	}
	if object.SecureDNS != nil {
		view.DNSSEC = object.SecureDNS.DelegationSigned
	}
	for _, entity := range object.Entities {
		if !entity.has("registrar") {
			continue
		}
		registrar := &whoisRegistrar{Name: entity.first("fn"), Abuse: abuseContact(entity.Entities)}
		if registrar.Name == "" {
			registrar.Name = entity.first("org")
		}
		for _, id := range entity.PublicIDs {
			if strings.EqualFold(id.Type, "IANA Registrar ID") {
				registrar.IANAID = id.Identifier
			}
		}
		for _, link := range entity.Links {
			if link.Type == "text/html" && strings.HasPrefix(link.Href, "http") {
				registrar.URL = link.Href
			}
		}
		view.Registrar = registrar
		break
	}
	return view
}

func networkView(kind, query, server string, object rdapObject) whoisNetwork {
	view := whoisNetwork{Query: query, Kind: kind, Name: object.Name, Handle: object.Handle, Type: object.Type,
		Registry: server, Status: []string{}, Abuse: abuseContact(object.Entities)}
	view.Status = append(view.Status, object.Status...)
	if object.Country != "" {
		view.Country = &object.Country
	}
	if kind == "asn" {
		number, _ := strconv.ParseUint(strings.TrimPrefix(query, "AS"), 10, 64)
		view.ASN = &number
	} else {
		view.Start, view.End = object.StartAddress, object.EndAddress
		for _, cidr := range object.CIDRs {
			prefix := cidr.V4
			if prefix == "" {
				prefix = cidr.V6
			}
			if prefix != "" {
				view.CIDRs = append(view.CIDRs, prefix+"/"+strconv.Itoa(cidr.Length))
			}
		}
	}
	view.Registered, view.Updated = object.event("registration"), object.event("last changed")
	// The holder is named only when the registry says it is an organisation; a network
	// assigned to a person stays anonymous here.
	for _, entity := range object.Entities {
		if entity.has("registrant") && strings.EqualFold(entity.first("kind"), "org") {
			if name := entity.first("fn"); name != "" {
				view.Holder = &name
				break
			}
		}
	}
	return view
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "whois_lookup",
		Description: "Who registered a domain, IP network or AS number, through RDAP (the registries' replacement for whois): for a domain the registrar, registration and expiry dates, status, nameservers and DNSSEC; for an IP address or AS number the network, its holding organisation, country and abuse contact. Works for .in and .co.in. Personal contacts are never returned.",
		InputSchema: schema([]string{"query"}, map[string]any{
			"query": text("A domain name (example.co.in), an IP address, or an AS number (AS55836)."),
		}),
		path: func(args map[string]any) (string, error) {
			value, err := required(args, "query")
			return "/v1/whois/" + url.PathEscape(value), err
		},
	})
}
