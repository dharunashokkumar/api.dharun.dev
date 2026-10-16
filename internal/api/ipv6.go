package api

import (
	"context"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Is a domain reachable over IPv6: does the site have IPv6 addresses (for the name and
// for www), can its nameservers be reached over IPv6, and its mail servers. Answered
// from DNS only: the server's own line has no IPv6, so no IPv6 connection is tried.

type ipv6Host struct {
	Name     string   `json:"name"`
	Priority *uint16  `json:"priority,omitempty"`
	IPv4     []string `json:"ipv4,omitempty"`
	IPv6     []string `json:"ipv6"`
	Notes    []string `json:"notes,omitempty"`
}

type ipv6Part struct {
	OK      *bool       `json:"ok"`
	Hosts   []ipv6Host  `json:"hosts"`
	Network *mtrNetwork `json:"network,omitempty"`
}

type ipv6View struct {
	Domain   string   `json:"domain"`
	Ready    bool     `json:"ready"`
	Website  ipv6Part `json:"website"`
	DNS      ipv6Part `json:"dns"`
	Mail     ipv6Part `json:"mail"`
	Problems []string `json:"problems"`
}

const ipv6Note = "Checked with DNS only, through public resolvers: the server has no IPv6 connection, so no connection over IPv6 was tried. " +
	"An address in DNS does not prove the host answers on it."

// ask sends one question to a public resolver, trying a second if the first is silent.
func (s *Server) ask(ctx context.Context, name string, kind uint16) (*dns.Msg, error) {
	var lastErr error
	for _, server := range []string{"1.1.1.1", "8.8.8.8"} {
		query := new(dns.Msg)
		query.SetQuestion(dns.Fqdn(name), kind)
		query.RecursionDesired = true
		reply, _, err := s.dnsClient().Exchange(ctx, server, query)
		if err == nil {
			return reply, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// addresses looks up a host's IPv4 and IPv6 addresses at once.
func (s *Server) addresses(ctx context.Context, host *ipv6Host, withIPv4 bool) {
	var wg sync.WaitGroup
	var v4 []string
	wg.Add(1)
	go func() {
		defer wg.Done()
		if reply, err := s.ask(ctx, host.Name, dns.TypeAAAA); err == nil {
			for _, record := range reply.Answer {
				if aaaa, ok := record.(*dns.AAAA); ok {
					if addr, ok := netip.AddrFromSlice(aaaa.AAAA); ok && addr.Is6() && !addr.Is4In6() {
						host.IPv6 = append(host.IPv6, addr.String())
						if note := ipv6Kind(addr); note != "" {
							host.Notes = append(host.Notes, note)
						}
					}
				}
			}
		}
	}()
	if withIPv4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if addrs, err := s.lookupAddrs(ctx, host.Name, dns.TypeA); err == nil {
				for _, addr := range addrs {
					v4 = append(v4, addr.String())
				}
			}
		}()
	}
	wg.Wait()
	host.IPv4 = v4
	if host.IPv6 == nil {
		host.IPv6 = []string{}
	}
}

var (
	sixToFour = netip.MustParsePrefix("2002::/16")
	teredo    = netip.MustParsePrefix("2001::/32")
	nat64     = netip.MustParsePrefix("64:ff9b::/96")
)

// ipv6Kind names an address that is not ordinary public IPv6.
func ipv6Kind(addr netip.Addr) string {
	switch {
	case reserved(addr) || addr.IsPrivate():
		return addr.String() + " is not a public address."
	case sixToFour.Contains(addr):
		return addr.String() + " is a 6to4 address, which most networks no longer route."
	case teredo.Contains(addr):
		return addr.String() + " is a Teredo tunnel address."
	case nat64.Contains(addr):
		return addr.String() + " is a NAT64 address, which only works inside networks that translate it."
	}
	return ""
}

func publicV6(host ipv6Host) bool {
	for _, text := range host.IPv6 {
		if addr, err := netip.ParseAddr(text); err == nil && ipv6Kind(addr) == "" {
			return true
		}
	}
	return false
}

func (s *Server) handleIPv6(w http.ResponseWriter, r *http.Request) {
	name, err := hostOf(r.PathValue("domain"))
	if _, notIP := netip.ParseAddr(name); err == nil && notIP == nil {
		err = errBadName // an address, not a domain
	}
	if err != nil {
		s.badRequest(w, r, "Give a domain name, such as example.co.in.")
		return
	}
	info(r).subject = name
	registered, err := registeredDomain(name)
	if err != nil {
		s.badRequest(w, r, "Give a domain name, not a top-level domain.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	view := ipv6View{Domain: name, Problems: []string{}}

	website := []ipv6Host{{Name: name}}
	if !strings.HasPrefix(name, "www.") {
		website = append(website, ipv6Host{Name: "www." + name})
	}
	var nameservers, mail []ipv6Host
	var failed bool
	var wg sync.WaitGroup
	for i := range website {
		wg.Add(1)
		go func() { defer wg.Done(); s.addresses(ctx, &website[i], true) }()
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		reply, err := s.ask(ctx, registered, dns.TypeNS)
		if err != nil {
			failed = true
			return
		}
		for _, record := range reply.Answer {
			if ns, ok := record.(*dns.NS); ok && len(nameservers) < 8 {
				nameservers = append(nameservers, ipv6Host{Name: strings.TrimSuffix(strings.ToLower(ns.Ns), ".")})
			}
		}
		sort.Slice(nameservers, func(a, b int) bool { return nameservers[a].Name < nameservers[b].Name })
		var inner sync.WaitGroup
		for i := range nameservers {
			inner.Add(1)
			go func() { defer inner.Done(); s.addresses(ctx, &nameservers[i], false) }()
		}
		inner.Wait()
	}()
	go func() {
		defer wg.Done()
		reply, err := s.ask(ctx, name, dns.TypeMX)
		if err != nil {
			return
		}
		for _, record := range reply.Answer {
			mx, ok := record.(*dns.MX)
			if !ok {
				continue
			}
			if mx.Mx == "." {
				continue // a null MX: the domain takes no mail
			}
			if len(mail) < 8 {
				priority := mx.Preference
				mail = append(mail, ipv6Host{Name: strings.TrimSuffix(strings.ToLower(mx.Mx), "."), Priority: &priority})
			}
		}
		sort.SliceStable(mail, func(a, b int) bool { return *mail[a].Priority < *mail[b].Priority })
		var inner sync.WaitGroup
		for i := range mail {
			inner.Add(1)
			go func() { defer inner.Done(); s.addresses(ctx, &mail[i], false) }()
		}
		inner.Wait()
	}()
	wg.Wait()
	if failed && len(website[0].IPv4) == 0 && len(website[0].IPv6) == 0 {
		s.fail(w, r, http.StatusBadGateway, "dns_failed", "No public resolver answered for "+name+".")
		return
	}

	yes, no := true, false
	verdict := func(ok bool) *bool {
		if ok {
			return &yes
		}
		return &no
	}
	// The site counts when the name itself, or www when the name has no address at all,
	// has a public IPv6 address.
	siteOK := publicV6(website[0]) || (len(website[0].IPv4) == 0 && len(website) > 1 && publicV6(website[1]))
	view.Website = ipv6Part{OK: verdict(siteOK), Hosts: website}
	if readers := s.extras.ip.Load(); readers != nil {
		for _, host := range website {
			if len(host.IPv6) > 0 {
				var network dbipASN
				if addr, err := netip.ParseAddr(host.IPv6[0]); err == nil && readers.asn.Lookup(addr).Decode(&network) == nil && network.Number != 0 {
					view.Website.Network = &mtrNetwork{Number: network.Number, Organization: strings.TrimRight(network.Organization, ", ")}
				}
				break
			}
		}
	}
	dnsOK := false
	for _, ns := range nameservers {
		dnsOK = dnsOK || publicV6(ns)
	}
	view.DNS = ipv6Part{OK: verdict(dnsOK), Hosts: append([]ipv6Host{}, nameservers...)}
	mailOK := len(mail) > 0
	for _, server := range mail {
		mailOK = mailOK && publicV6(server)
	}
	view.Mail = ipv6Part{Hosts: append([]ipv6Host{}, mail...)}
	if len(mail) > 0 {
		view.Mail.OK = verdict(mailOK)
	}

	if !siteOK {
		none := true
		for _, host := range website {
			none = none && len(host.IPv4) == 0 && len(host.IPv6) == 0
		}
		if none {
			view.Problems = append(view.Problems, name+" has no address at all.")
		} else {
			view.Problems = append(view.Problems, "The website has no public IPv6 address (no AAAA record).")
		}
	}
	if len(nameservers) > 0 && !dnsOK {
		view.Problems = append(view.Problems, "None of the nameservers has an IPv6 address, so resolvers on IPv6-only networks cannot reach them.")
	}
	if len(mail) > 0 && !mailOK {
		view.Problems = append(view.Problems, "Not every mail server has an IPv6 address.")
	}
	view.Ready = siteOK && (dnsOK || len(nameservers) == 0) && (mailOK || len(mail) == 0)
	s.ok(w, r, view, meta{Source: s.source("ip"), Note: ipv6Note}, cacheShort)
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "check_ipv6",
		Description: "Check whether a domain is ready for IPv6, from DNS: IPv6 (AAAA) addresses for the site and www, for each nameserver and each mail server, with a plain list of what is missing. No connection is made over IPv6.",
		InputSchema: schema([]string{"domain"}, map[string]any{
			"domain": text("Domain name, such as example.co.in."),
		}),
		path: func(args map[string]any) (string, error) {
			domain, err := required(args, "domain")
			return "/v1/ipv6/" + url.PathEscape(domain), err
		},
	})
}
