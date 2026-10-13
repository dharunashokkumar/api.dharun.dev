package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/idna"
)

// DNS queries go straight to well-known public resolvers, so answers do not depend on
// how the server's own network is set up, and a name can be compared across them.

type resolver struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

var publicResolvers = []resolver{
	{"Cloudflare", "1.1.1.1"},
	{"Google", "8.8.8.8"},
	{"Quad9", "9.9.9.9"},
	{"OpenDNS", "208.67.222.222"},
}

// dnsExchanger sends one query to one resolver. Tests replace it with canned answers.
type dnsExchanger interface {
	Exchange(ctx context.Context, server string, query *dns.Msg) (*dns.Msg, time.Duration, error)
}

type liveDNS struct{}

func (liveDNS) Exchange(ctx context.Context, server string, query *dns.Msg) (*dns.Msg, time.Duration, error) {
	client := &dns.Client{Timeout: 3 * time.Second}
	answer, took, err := client.ExchangeContext(ctx, query, net.JoinHostPort(server, "53"))
	if err == nil && answer.Truncated {
		client.Net = "tcp"
		answer, took, err = client.ExchangeContext(ctx, query, net.JoinHostPort(server, "53"))
	}
	return answer, took, err
}

func (s *Server) dnsClient() dnsExchanger {
	if client := s.extras.dns; client != nil {
		return client
	}
	return liveDNS{}
}

var dnsTypes = map[string]uint16{
	"A": dns.TypeA, "AAAA": dns.TypeAAAA, "CNAME": dns.TypeCNAME, "MX": dns.TypeMX, "NS": dns.TypeNS,
	"TXT": dns.TypeTXT, "CAA": dns.TypeCAA, "SOA": dns.TypeSOA, "SRV": dns.TypeSRV, "PTR": dns.TypePTR,
}

func dnsTypeNames() []string {
	return []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT", "CAA", "SOA", "SRV", "PTR"}
}

var errBadName = errors.New("not a valid domain name")

// asciiName turns a domain name, possibly in an Indian script, into its ASCII form.
func asciiName(name string) (string, error) {
	name = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	if name == "" || strings.ContainsAny(name, " /@:") {
		return "", errBadName
	}
	ascii, err := idna.Lookup.ToASCII(name)
	if err != nil || len(ascii) > 253 {
		return "", errBadName
	}
	labels := strings.Split(ascii, ".")
	for _, label := range labels {
		// Underscores appear in service names such as _dmarc and _sip._tcp.
		if label == "" || len(label) > 63 || strings.Trim(label, "abcdefghijklmnopqrstuvwxyz0123456789-_") != "" {
			return "", errBadName
		}
	}
	return ascii, nil
}

type dnsRecord struct {
	Type  string `json:"type"`
	TTL   uint32 `json:"ttl"`
	Value string `json:"value"`
}

type dnsResolverAnswer struct {
	resolver
	Status        string      `json:"status"`
	Authenticated bool        `json:"dnssecValidated"`
	Records       []dnsRecord `json:"records"`
	Ms            int64       `json:"ms"`
	Error         string      `json:"error,omitempty"`
}

// recordValue is the data part of a record, as a person would write it.
func recordValue(record dns.RR) string {
	header := record.Header().String()
	return strings.TrimSpace(strings.TrimPrefix(record.String(), header))
}

// queryAll asks every public resolver the same question at once.
func (s *Server) queryAll(ctx context.Context, name string, kind uint16) []dnsResolverAnswer {
	answers := make([]dnsResolverAnswer, len(publicResolvers))
	var wg sync.WaitGroup
	for i, server := range publicResolvers {
		wg.Add(1)
		go func(i int, server resolver) {
			defer wg.Done()
			query := new(dns.Msg)
			query.SetQuestion(dns.Fqdn(name), kind)
			query.RecursionDesired = true
			query.AuthenticatedData = true
			answer := dnsResolverAnswer{resolver: server, Records: []dnsRecord{}}
			reply, took, err := s.dnsClient().Exchange(ctx, server.Address, query)
			answer.Ms = took.Milliseconds()
			if err != nil {
				answer.Status, answer.Error = "ERROR", "No answer from this resolver."
				answers[i] = answer
				return
			}
			answer.Status = dns.RcodeToString[reply.Rcode]
			answer.Authenticated = reply.AuthenticatedData
			for _, record := range reply.Answer {
				answer.Records = append(answer.Records, dnsRecord{
					Type: dns.TypeToString[record.Header().Rrtype], TTL: record.Header().Ttl, Value: recordValue(record),
				})
			}
			answers[i] = answer
		}(i, server)
	}
	wg.Wait()
	return answers
}

// sameAnswer compares what two resolvers returned, ignoring TTLs and order.
func sameAnswer(a, b dnsResolverAnswer) bool {
	if a.Status != b.Status || len(a.Records) != len(b.Records) {
		return false
	}
	seen := map[string]int{}
	for _, record := range a.Records {
		seen[record.Type+" "+record.Value]++
	}
	for _, record := range b.Records {
		seen[record.Type+" "+record.Value]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}

type dnsCheckView struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Consistent bool                `json:"consistent"`
	Resolvers  []dnsResolverAnswer `json:"resolvers"`
}

func (s *Server) handleDNS(w http.ResponseWriter, r *http.Request) {
	kind := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	if kind == "" {
		kind = "A"
	}
	code, ok := dnsTypes[kind]
	if !ok {
		s.badRequest(w, r, "type must be one of: "+strings.Join(dnsTypeNames(), ", ")+".")
		return
	}
	raw := r.PathValue("name")
	info(r).subject = raw
	var name string
	if kind == "PTR" {
		// A reverse lookup takes an address and asks for its in-addr.arpa name.
		reverse, err := dns.ReverseAddr(strings.TrimSpace(raw))
		if err != nil {
			s.badRequest(w, r, "A PTR lookup takes an IP address.")
			return
		}
		name = strings.TrimSuffix(reverse, ".")
	} else {
		var err error
		if name, err = asciiName(raw); err != nil {
			s.badRequest(w, r, "That is not a valid domain name.")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	answers := s.queryAll(ctx, name, code)
	view := dnsCheckView{Name: name, Type: kind, Consistent: true, Resolvers: answers}
	for _, answer := range answers[1:] {
		if !sameAnswer(answers[0], answer) {
			view.Consistent = false
		}
	}
	s.ok(w, r, view, meta{Note: "Asked directly of each public resolver just now. Resolvers that disagree usually mean a change is still spreading, or that answers differ by location."}, "public, max-age=30")
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "check_dns",
		Description: "Look up a domain's DNS records (A, AAAA, MX, TXT, NS, CNAME, CAA, SOA, SRV, or PTR for an IP) at Cloudflare, Google, Quad9 and OpenDNS at once, and say whether they agree.",
		InputSchema: schema([]string{"name"}, map[string]any{
			"name": text("Domain name, or an IP address for PTR."),
			"type": oneOf("Record type. Default A.", dnsTypeNames()...),
		}),
		path: func(args map[string]any) (string, error) {
			name, err := required(args, "name")
			return "/v1/dns/" + url.PathEscape(name) + query(args, map[string]string{"type": "type"}), err
		},
	})
}
