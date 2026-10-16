package api

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
)

// What the network tools (whois, TLS check, IPv6 check, HTTP probe, domain icons) share.
// Every connection to a host a caller names goes through netguard: public addresses
// only, the checked address dialled, fixed ports, IPv4. On top of that the server's own
// public address is refused too, so a probe cannot come back in through the router of
// the network the server sits in, and each target domain gets a few checks a minute
// across all callers.

// probeAgent is the User-Agent of every request the tools send, so a site's owner can
// see who is asking and block it.
const probeAgent = "IndiaAPI-check/1.0 (+https://api.dharun.dev/docs/probe)"

const (
	// probeConcurrency is how many TLS checks, probes and icon fetches run at once.
	// They wait on the network, not the processor, so they take no CPU slot.
	probeConcurrency = 4
	// targetPerMinute is how many checks one site may get in a minute from all callers.
	targetPerMinute = 10
)

// networkTools is the state the tools keep. Tests replace dial and roots.
type networkTools struct {
	// dial, when set, makes every checked connection; tests send them to local servers.
	dial func(ctx context.Context, network, address string) (net.Conn, error)
	// roots are the certificate authorities trusted; nil means the system's.
	roots   *x509.CertPool
	probes  chan struct{}
	targets *netguard.TargetLimit
	// registries limits the whois lookups this server sends each registry.
	registries *netguard.TargetLimit

	mu       sync.Mutex
	own      netip.Addr
	ownUntil time.Time
}

func newNetworkTools() *networkTools {
	return &networkTools{
		probes:     make(chan struct{}, probeConcurrency),
		targets:    &netguard.TargetLimit{PerMinute: targetPerMinute},
		registries: &netguard.TargetLimit{PerMinute: registryLimit},
	}
}

// lookupAddrs asks a public resolver for a name's addresses of one kind.
func (s *Server) lookupAddrs(ctx context.Context, name string, kind uint16) ([]netip.Addr, error) {
	ascii, err := asciiName(name)
	if err != nil {
		return nil, err
	}
	for _, server := range []string{"1.1.1.1", "8.8.8.8"} {
		query := new(dns.Msg)
		query.SetQuestion(dns.Fqdn(ascii), kind)
		query.RecursionDesired = true
		reply, _, err := s.dnsClient().Exchange(ctx, server, query)
		if err != nil {
			continue
		}
		var addrs []netip.Addr
		for _, record := range reply.Answer {
			switch r := record.(type) {
			case *dns.A:
				if addr, ok := netip.AddrFromSlice(r.A.To4()); ok {
					addrs = append(addrs, addr)
				}
			case *dns.AAAA:
				if addr, ok := netip.AddrFromSlice(r.AAAA); ok {
					addrs = append(addrs, addr)
				}
			}
		}
		if len(addrs) == 0 {
			return nil, errNoAddress
		}
		return addrs, nil
	}
	return nil, netguard.ErrLookup
}

// ownAddress is the server's own public IPv4 address, as Cloudflare's resolver sees it
// (whoami.cloudflare, class CH), learned again every five minutes since a home line's
// address changes when it reconnects.
func (s *Server) ownAddress(ctx context.Context) (netip.Addr, error) {
	tools := s.extras.network
	tools.mu.Lock()
	defer tools.mu.Unlock()
	if tools.own.IsValid() && time.Now().Before(tools.ownUntil) {
		return tools.own, nil
	}
	for _, server := range []string{"1.1.1.1", "1.0.0.1"} {
		query := new(dns.Msg)
		query.SetQuestion("whoami.cloudflare.", dns.TypeTXT)
		query.Question[0].Qclass = dns.ClassCHAOS
		reply, _, err := s.dnsClient().Exchange(ctx, server, query)
		if err != nil {
			continue
		}
		for _, record := range reply.Answer {
			if txt, ok := record.(*dns.TXT); ok && len(txt.Txt) > 0 {
				if addr, err := netip.ParseAddr(strings.TrimSpace(txt.Txt[0])); err == nil {
					tools.own, tools.ownUntil = addr.Unmap(), time.Now().Add(5*time.Minute)
					return tools.own, nil
				}
			}
		}
	}
	if tools.own.IsValid() && time.Since(tools.ownUntil) < time.Hour {
		return tools.own, nil // a recent answer is better than none
	}
	return netip.Addr{}, errors.New("the server's own address is not known")
}

// guard returns the rules for connections to callers' hosts on the given ports.
func (s *Server) guard(ports ...int) *netguard.Guard {
	tools := s.extras.network
	return &netguard.Guard{
		Lookup: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return s.lookupAddrs(ctx, host, dns.TypeA)
		},
		Public: func(addr netip.Addr) bool {
			tools.mu.Lock()
			own := tools.own
			tools.mu.Unlock()
			return !reserved(addr) && addr != own
		},
		Ports: ports,
		Dial:  tools.dial,
	}
}

// startCheck admits one check of host: within the per-site limit, with the server's own
// address known, and with a free slot. It answers the refusal itself.
func (s *Server) startCheck(w http.ResponseWriter, r *http.Request, host string) (func(), bool) {
	tools := s.extras.network
	if !tools.targets.Allow(host) {
		w.Header().Set("Retry-After", "60")
		s.fail(w, r, http.StatusTooManyRequests, "target_limited",
			"This site has been checked "+itoa(targetPerMinute)+" times in the last minute by callers of this API. Try again in a minute.")
		return nil, false
	}
	if _, err := s.ownAddress(r.Context()); err != nil {
		logError("network check", err)
		w.Header().Set("Retry-After", "60")
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "Network checks are not available right now. Try again in a minute.")
		return nil, false
	}
	return s.takeSlot(w, r, tools.probes, 10*time.Second)
}

// chargeMore takes extra units from the caller's allowance for work the route's own
// cost does not cover, such as an icon that has to be fetched rather than served from
// memory. It answers 429 itself when the allowance is spent.
func (s *Server) chargeMore(w http.ResponseWriter, r *http.Request, units int) bool {
	details := info(r)
	client := details.client
	if !details.internal {
		client = s.clientKey(r)
	}
	if s.limiter.allow(client, units) {
		return true
	}
	s.metrics.rateLimited()
	w.Header().Set("Retry-After", "60")
	s.fail(w, r, http.StatusTooManyRequests, "rate_limited",
		"Rate limit is "+itoa(s.cfg.RateLimitPerMinute)+" requests per minute; fetching an icon that is not cached yet counts as "+itoa(costProbe)+".")
	return false
}

// failNetwork answers for an outgoing connection that did not work.
func (s *Server) failNetwork(w http.ResponseWriter, r *http.Request, host string, err error) {
	var refused *netguard.AddressError
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &refused):
		s.fail(w, r, http.StatusUnprocessableEntity, "reserved_address", host+" is or resolves to an address that is not public ("+refused.Address.String()+"); only public addresses are checked.")
	case errors.Is(err, errBadName):
		s.badRequest(w, r, "Give a domain name or a public IPv4 address.")
	case errors.Is(err, netguard.ErrIPv6):
		s.fail(w, r, http.StatusUnprocessableEntity, "ipv6_unavailable", "The server has no IPv6 connection, so IPv6 addresses cannot be checked.")
	case errors.Is(err, netguard.ErrPort):
		s.fail(w, r, http.StatusUnprocessableEntity, "port_not_allowed", "Only the usual web ports (80 for http, 443 for https) are reached.")
	case errors.Is(err, errNoAddress), errors.Is(err, netguard.ErrNoAddress):
		s.notFound(w, r, host+" has no IPv4 address.")
	case errors.Is(err, netguard.ErrLookup), errors.As(err, &dnsErr):
		s.fail(w, r, http.StatusBadGateway, "dns_failed", "The name could not be resolved: no public resolver answered.")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, os.ErrDeadlineExceeded), isTimeout(err):
		s.fail(w, r, http.StatusGatewayTimeout, "timeout", host+" did not answer in time.")
	default:
		s.fail(w, r, http.StatusBadGateway, "unreachable", host+" could not be reached: "+plainNetError(err))
	}
}

func isTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

// plainNetError says in a few words why a connection failed, without echoing anything
// the far side sent.
func plainNetError(err error) string {
	text := err.Error()
	switch {
	case strings.Contains(text, "connection refused"):
		return "the connection was refused."
	case strings.Contains(text, "connection reset"):
		return "the connection was reset."
	case strings.Contains(text, "no route to host"), strings.Contains(text, "network is unreachable"):
		return "there is no route to it."
	case strings.Contains(text, "tls:"), strings.Contains(text, "x509:"):
		return "the TLS handshake failed."
	case strings.Contains(text, "EOF"):
		return "it closed the connection."
	}
	return "the connection failed."
}

// hostOf normalises a host given by a caller: a domain name in ASCII form (an Indian
// script name is converted) or an IP address.
func hostOf(raw string) (string, error) {
	raw = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if addr, err := netip.ParseAddr(strings.Trim(raw, "[]")); err == nil {
		return addr.Unmap().String(), nil
	}
	name, err := asciiName(raw)
	if err != nil || !strings.Contains(name, ".") {
		return "", errBadName
	}
	return name, nil
}
