package api

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// An email check looks at the address and its domain only. It never connects to a mail
// server or tries a mailbox: that is how spammers verify lists, and mail servers block it.

var (
	// The dot-atom form of RFC 5322: what real addresses use. Quoted local parts are
	// legal but in practice only appear in tests.
	localPart = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+(\.[A-Za-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+)*$`)

	freeProviders = setOf("gmail.com", "googlemail.com", "yahoo.com", "yahoo.co.in", "yahoo.in", "ymail.com", "rocketmail.com",
		"outlook.com", "outlook.in", "hotmail.com", "live.com", "msn.com", "icloud.com", "me.com", "mac.com", "aol.com",
		"proton.me", "protonmail.com", "pm.me", "zoho.com", "zohomail.com", "zohomail.in", "rediffmail.com", "gmx.com",
		"gmx.net", "mail.com", "yandex.com", "yandex.ru", "tutanota.com", "tuta.io", "fastmail.com", "hey.com", "mail.ru")

	roleAccounts = setOf("admin", "administrator", "info", "support", "sales", "contact", "hello", "help", "noreply", "no-reply",
		"postmaster", "hostmaster", "webmaster", "abuse", "billing", "accounts", "office", "team", "hr", "careers", "jobs",
		"marketing", "enquiry", "enquiries", "feedback", "security", "root", "mail")
)

func setOf(values ...string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

type mailServerView struct {
	// Hosts are the mail servers, most preferred first.
	Hosts []string `json:"hosts"`
	// Implicit is true when the domain has no MX record but has an address record, which
	// mail servers then use instead.
	Implicit bool `json:"implicit"`
	// NullMX is true when the domain publishes that it accepts no mail (RFC 7505).
	NullMX bool `json:"nullMx"`
}

type emailView struct {
	Domain       string          `json:"domain"`
	Syntax       bool            `json:"validSyntax"`
	Problem      string          `json:"problem,omitempty"`
	Disposable   bool            `json:"disposable"`
	FreeProvider bool            `json:"freeProvider"`
	RoleAccount  bool            `json:"roleAccount"`
	MailServers  *mailServerView `json:"mailServers"`
	// Verdict sums it up: invalid, disposable, no_mail_server, unknown (DNS did not
	// answer) or ok. ok means mail can be delivered to the domain, not that the mailbox
	// exists.
	Verdict string `json:"verdict"`
}

func (s *Server) handleEmail(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	address := strings.TrimSpace(r.PathValue("address"))
	view := emailView{Verdict: "invalid"}
	at := strings.LastIndex(address, "@")
	local, domain := "", ""
	if at > 0 {
		local, domain = address[:at], address[at+1:]
	}
	ascii, nameErr := asciiName(domain)
	switch {
	case at <= 0 || at == len(address)-1:
		view.Problem = "There is no @ with text on both sides."
	case len(address) > 254:
		view.Problem = "An address can be at most 254 characters."
	case len(local) > 64:
		view.Problem = "The part before the @ can be at most 64 characters."
	case !localPart.MatchString(local):
		view.Problem = "The part before the @ has characters an address cannot have, or a misplaced dot."
	case nameErr != nil || !strings.Contains(ascii, "."):
		view.Problem = "The part after the @ is not a domain name."
	default:
		view.Syntax = true
	}
	if ascii != "" {
		view.Domain = ascii
	}
	if !view.Syntax {
		s.ok(w, r, view, meta{}, "private, no-store")
		return
	}
	view.FreeProvider = freeProviders[ascii]
	view.RoleAccount = roleAccounts[strings.ToLower(local)]
	if set := s.data("disposable-email"); set != nil {
		// A subdomain of a throwaway domain is throwaway too.
		labels := strings.Split(ascii, ".")
		for i := 0; i < len(labels)-1; i++ {
			var one int
			if set.row(`SELECT 1 FROM domains WHERE domain = ?`, strings.Join(labels[i:], ".")).Scan(&one) == nil {
				view.Disposable = true
				break
			}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	servers, ok := s.mailServers(ctx, ascii)
	view.MailServers = servers
	switch {
	case view.Disposable:
		view.Verdict = "disposable"
	case !ok:
		view.Verdict = "unknown"
	case servers.NullMX || len(servers.Hosts) == 0:
		view.Verdict = "no_mail_server"
	default:
		view.Verdict = "ok"
	}
	m := meta{Source: s.source("disposable-email"),
		Note: "Checks the address's form and its domain. No mail server is contacted, so ok means the domain takes mail, not that the mailbox exists."}
	if s.data("disposable-email") == nil {
		m.Note += " The disposable-domain list is not available on this server right now."
	}
	s.ok(w, r, view, m, "private, no-store")
}

// mailServers finds where a domain's mail goes. The second value is false when no
// resolver gave a usable answer.
func (s *Server) mailServers(ctx context.Context, domain string) (*mailServerView, bool) {
	view := &mailServerView{Hosts: []string{}}
	reply, ok := s.firstAnswer(ctx, domain, dns.TypeMX)
	if !ok {
		return nil, false
	}
	type mx struct {
		pref uint16
		host string
	}
	var found []mx
	for _, record := range reply.Answer {
		if record, isMX := record.(*dns.MX); isMX {
			if record.Mx == "." {
				view.NullMX = true
				continue
			}
			found = append(found, mx{record.Preference, strings.TrimSuffix(record.Mx, ".")})
		}
	}
	sort.SliceStable(found, func(a, b int) bool { return found[a].pref < found[b].pref })
	for _, record := range found {
		view.Hosts = append(view.Hosts, record.host)
	}
	if len(view.Hosts) == 0 && !view.NullMX && reply.Rcode == dns.RcodeSuccess {
		// No MX record: mail goes to the domain's own address, if it has one. A name that
		// only points elsewhere (a CNAME with no address behind it) has none.
		uncertain := false
		for _, kind := range []uint16{dns.TypeA, dns.TypeAAAA} {
			address, ok := s.firstAnswer(ctx, domain, kind)
			if !ok {
				uncertain = true
				continue
			}
			if hasAddress(address.Answer) {
				view.Hosts, view.Implicit = []string{domain}, true
				break
			}
		}
		if len(view.Hosts) == 0 && uncertain {
			return nil, false
		}
	}
	return view, true
}

func hasAddress(records []dns.RR) bool {
	for _, record := range records {
		switch record.(type) {
		case *dns.A, *dns.AAAA:
			return true
		}
	}
	return false
}

// firstAnswer asks the public resolvers in turn until one answers.
func (s *Server) firstAnswer(ctx context.Context, name string, kind uint16) (*dns.Msg, bool) {
	query := new(dns.Msg)
	query.SetQuestion(dns.Fqdn(name), kind)
	query.RecursionDesired = true
	for _, server := range publicResolvers[:2] {
		reply, _, err := s.dnsClient().Exchange(ctx, server.Address, query)
		if err == nil && (reply.Rcode == dns.RcodeSuccess || reply.Rcode == dns.RcodeNameError) {
			return reply, true
		}
	}
	return nil, false
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "check_email_address",
		Description: "Check an email address without contacting its mail server: valid form, disposable (throwaway) domain, free provider, role account, and whether the domain has mail servers.",
		InputSchema: schema([]string{"email"}, map[string]any{"email": text("The email address.")}),
		path: func(args map[string]any) (string, error) {
			email, err := required(args, "email")
			return "/v1/email/" + url.PathEscape(email), err
		},
	})
}
