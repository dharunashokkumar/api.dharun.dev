package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/dharunashokkumar/api.dharun.dev/internal/trace"
)

// Traceroute from the server's own connection in India, MTR style. Only public
// addresses can be traced. Hops inside private or shared address space (the server's
// own network, a provider's carrier-grade NAT) are reported without their address, so
// the API does not publish the layout of the network it runs in.

// tracer runs one trace; tests replace it.
type tracer func(context.Context, trace.Options) (trace.Result, error)

func (s *Server) tracer() tracer {
	if s.extras.trace != nil {
		return s.extras.trace
	}
	return trace.Run
}

// mtrConcurrency is how many traces run at once. A trace mostly waits on the network,
// so it takes no CPU slot; this keeps the probes leaving the server few.
const mtrConcurrency = 2

type mtrNetwork struct {
	Number       uint   `json:"number"`
	Organization string `json:"organization"`
}

type mtrHop struct {
	Hop         int         `json:"hop"`
	Address     *string     `json:"address"`
	Private     bool        `json:"private,omitempty"`
	Hostname    *string     `json:"hostname"`
	Network     *mtrNetwork `json:"network"`
	Country     *string     `json:"country"`
	Others      []string    `json:"otherAddresses,omitempty"`
	Sent        int         `json:"sent"`
	Received    int         `json:"received"`
	Loss        float64     `json:"lossPercent"`
	Best        *float64    `json:"bestMs"`
	Average     *float64    `json:"averageMs"`
	Worst       *float64    `json:"worstMs"`
	StdDev      *float64    `json:"stdDevMs"`
	Destination bool        `json:"destination,omitempty"`
	Unreachable bool        `json:"unreachable,omitempty"`
}

type mtrView struct {
	Host    string   `json:"host"`
	Address string   `json:"address"`
	Version int      `json:"version"`
	Rounds  int      `json:"rounds"`
	Reached bool     `json:"reached"`
	Hops    []mtrHop `json:"hops"`
}

// resolveTarget turns the host into one public address, asking a public resolver so the
// answer does not depend on the server's own network.
func (s *Server) resolveTarget(ctx context.Context, host string, v6 bool) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.Unmap().WithZone(""), nil
	}
	kind := dns.TypeA
	if v6 {
		kind = dns.TypeAAAA
	}
	addrs, err := s.lookupAddrs(ctx, host, kind)
	if err != nil {
		return netip.Addr{}, err
	}
	return addrs[0], nil
}

var errNoAddress = errors.New("the name has no address of that kind")

func (s *Server) handleMTR(w http.ResponseWriter, r *http.Request) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(r.PathValue("host"))), ".")
	info(r).subject = host
	query := r.URL.Query()
	rounds := 5
	if raw := strings.TrimSpace(query.Get("rounds")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 10 {
			s.badRequest(w, r, "rounds is from 1 to 10.")
			return
		}
		rounds = value
	}
	version := strings.TrimSpace(query.Get("v"))
	if version != "" && version != "4" && version != "6" {
		s.badRequest(w, r, "v is 4 or 6.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	target, err := s.resolveTarget(ctx, host, version == "6")
	switch {
	case errors.Is(err, errBadName):
		s.badRequest(w, r, "Give a domain name or an IP address.")
		return
	case errors.Is(err, errNoAddress):
		s.notFound(w, r, host+" has no IPv"+map[bool]string{true: "6", false: "4"}[version == "6"]+" address.")
		return
	case err != nil:
		s.fail(w, r, http.StatusBadGateway, "dns_failed", "The name could not be resolved: no public resolver answered.")
		return
	}
	if reserved(target) {
		s.fail(w, r, http.StatusUnprocessableEntity, "reserved_address", host+" is or resolves to a private or reserved address ("+target.String()+"); only public addresses can be traced.")
		return
	}

	select {
	case s.extras.traces <- struct{}{}:
		defer func() { <-s.extras.traces }()
	case <-time.After(10 * time.Second):
		s.metrics.busy()
		w.Header().Set("Retry-After", "10")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "Other traces are running. Try again in a few seconds.")
		return
	case <-r.Context().Done():
		return
	}
	result, err := s.tracer()(ctx, trace.Options{Target: target, Rounds: rounds})
	switch {
	case errors.Is(err, trace.ErrUnsupported):
		s.fail(w, r, http.StatusNotImplemented, "not_available", "Traceroute is not available on this server.")
		return
	case err != nil:
		logError("traceroute", err)
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "Traceroute is not available on this server right now.")
		return
	}

	view := mtrView{Host: host, Address: target.String(), Version: 4, Rounds: rounds, Reached: result.Reached, Hops: []mtrHop{}}
	if target.Is6() {
		view.Version = 6
	}
	readers := s.extras.ip.Load()
	names := s.reverseNames(ctx, result.Hops)
	ms := func(d time.Duration, received int) *float64 {
		if received == 0 {
			return nil
		}
		value := math.Round(float64(d)/1e5) / 10
		return &value
	}
	for _, hop := range result.Hops {
		item := mtrHop{Hop: hop.TTL, Sent: hop.Sent, Received: hop.Received, Loss: hop.Loss(),
			Best: ms(hop.Best, hop.Received), Average: ms(hop.Average, hop.Received), Worst: ms(hop.Worst, hop.Received), StdDev: ms(hop.StdDev, hop.Received),
			Destination: hop.Destination, Unreachable: hop.Unreachable}
		if hop.Address.IsValid() {
			if reserved(hop.Address) {
				item.Private = true
			} else {
				address := hop.Address.String()
				item.Address = &address
				if name, ok := names[hop.Address]; ok {
					item.Hostname = &name
				}
				if readers != nil {
					var network dbipASN
					if readers.asn.Lookup(hop.Address).Decode(&network) == nil && network.Number != 0 {
						item.Network = &mtrNetwork{Number: network.Number, Organization: strings.TrimRight(network.Organization, ", ")}
					}
					var city dbipCity
					if readers.city.Lookup(hop.Address).Decode(&city) == nil && city.Country.ISOCode != "" {
						item.Country = &city.Country.ISOCode
					}
				}
			}
		}
		for _, other := range hop.Others {
			if !reserved(other) {
				item.Others = append(item.Others, other.String())
			}
		}
		view.Hops = append(view.Hops, item)
	}
	s.ok(w, r, view, meta{Source: s.source("ip"), Note: mtrNote}, "public, max-age=60")
}

// reverseNames looks up the names of the public hop addresses at once, giving up on any
// that take longer than a second and a half.
func (s *Server) reverseNames(ctx context.Context, hops []trace.Hop) map[netip.Addr]string {
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	names := map[netip.Addr]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, hop := range hops {
		if !hop.Address.IsValid() || reserved(hop.Address) {
			continue
		}
		wg.Add(1)
		go func(addr netip.Addr) {
			defer wg.Done()
			reverse, err := dns.ReverseAddr(addr.String())
			if err != nil {
				return
			}
			query := new(dns.Msg)
			query.SetQuestion(reverse, dns.TypePTR)
			query.RecursionDesired = true
			reply, _, err := s.dnsClient().Exchange(ctx, "1.1.1.1", query)
			if err != nil {
				return
			}
			for _, record := range reply.Answer {
				if ptr, ok := record.(*dns.PTR); ok {
					mu.Lock()
					names[addr] = strings.TrimSuffix(ptr.Ptr, ".")
					mu.Unlock()
					return
				}
			}
		}(hop.Address)
	}
	wg.Wait()
	return names
}

const mtrNote = "Traced from the server in India with ICMP echo requests, MTR style: each hop is probed once per round. A hop with 100% loss " +
	"may simply not answer probes while forwarding traffic normally. Hops in private or shared address space are shown without their address. " +
	"Networks and countries are from DB-IP."

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "traceroute_from_india",
		Description: "Trace the network path from a server in India to a host, MTR style: every hop with its address, name, network (AS) and country, packet loss and round-trip times. Shows how Indian networks reach a site.",
		InputSchema: schema([]string{"host"}, map[string]any{
			"host":   text("Domain name or public IP address."),
			"rounds": map[string]any{"type": "integer", "description": "Probes per hop, 1 to 10. Default 5."},
			"v":      oneOf("IP version. Default 4.", "4", "6"),
		}),
		path: func(args map[string]any) (string, error) {
			host, err := required(args, "host")
			return "/v1/mtr/" + url.PathEscape(host) + query(args, map[string]string{"rounds": "rounds", "v": "v"}), err
		},
	})
}
