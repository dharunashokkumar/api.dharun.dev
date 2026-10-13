package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func init() {
	mcpTestCalls["check_dns"] = `{"name":"dharun.dev","type":"MX"}`
	mcpTestCalls["check_email_address"] = `{"email":"someone@gmail.com"}`
}

// fakeDNS answers from a fixed zone, the same for every resolver unless a record is
// listed for one resolver only.
type fakeDNS struct{}

var fakeZone = map[string][]string{
	"dharun.dev. MX":            {"dharun.dev. 300 IN MX 10 route1.mx.cloudflare.net.", "dharun.dev. 300 IN MX 20 route2.mx.cloudflare.net."},
	"dharun.dev. A":             {"dharun.dev. 300 IN A 104.21.0.1"},
	"gmail.com. MX":             {"gmail.com. 3600 IN MX 5 gmail-smtp-in.l.google.com."},
	"mailinator.com. MX":        {"mailinator.com. 300 IN MX 1 mail.mailinator.com."},
	"onlyaddress.in. A":         {"onlyaddress.in. 300 IN A 203.0.113.10"},
	"nomail.in. MX":             {"nomail.in. 300 IN MX 0 ."},
	"split.in. A":               {"split.in. 60 IN A 198.51.100.1"},
	"split.in. A 1.1.1.1":       {"split.in. 60 IN A 198.51.100.2"},
	"xn--h2brj9c.in. A":         {"xn--h2brj9c.in. 300 IN A 203.0.113.20"},
	"8.8.8.8.in-addr.arpa. PTR": {"8.8.8.8.in-addr.arpa. 300 IN PTR dns.google."},
}

func (fakeDNS) Exchange(ctx context.Context, server string, query *dns.Msg) (*dns.Msg, time.Duration, error) {
	question := query.Question[0]
	key := question.Name + " " + dns.TypeToString[question.Qtype]
	if question.Name == "slow.in." && server == "9.9.9.9" {
		return nil, 3 * time.Second, errors.New("i/o timeout")
	}
	reply := new(dns.Msg)
	reply.SetReply(query)
	records, ok := fakeZone[key+" "+server]
	if !ok {
		records, ok = fakeZone[key]
	}
	known := false
	for zoneKey := range fakeZone {
		known = known || strings.HasPrefix(zoneKey, question.Name+" ")
	}
	if !known && question.Name != "slow.in." {
		reply.Rcode = dns.RcodeNameError
	}
	for _, text := range records {
		record, err := dns.NewRR(text)
		if err != nil {
			panic(err)
		}
		reply.Answer = append(reply.Answer, record)
	}
	return reply, 12 * time.Millisecond, nil
}

func TestDNSCheck(t *testing.T) {
	mx := expectStatus(t, "/v1/dns/dharun.dev?type=MX", http.StatusOK).data(t)
	resolvers := mx["resolvers"].([]any)
	if mx["consistent"] != true || len(resolvers) != 4 {
		t.Fatalf("MX: %v", mx)
	}
	first := resolvers[0].(map[string]any)
	records := first["records"].([]any)
	if first["name"] != "Cloudflare" || first["status"] != "NOERROR" || len(records) != 2 || records[0].(map[string]any)["value"] != "10 route1.mx.cloudflare.net." {
		t.Errorf("Cloudflare's answer: %v", first)
	}

	split := expectStatus(t, "/v1/dns/split.in", http.StatusOK).data(t)
	if split["consistent"] != false {
		t.Errorf("resolvers that disagree must be reported: %v", split)
	}
	slow := expectStatus(t, "/v1/dns/slow.in", http.StatusOK).data(t)
	quad9 := slow["resolvers"].([]any)[2].(map[string]any)
	if quad9["name"] != "Quad9" || quad9["status"] != "ERROR" || slow["consistent"] != false {
		t.Errorf("a resolver that does not answer: %v", quad9)
	}
	missing := expectStatus(t, "/v1/dns/nothing-here.in", http.StatusOK).data(t)
	if missing["resolvers"].([]any)[0].(map[string]any)["status"] != "NXDOMAIN" || missing["consistent"] != true {
		t.Errorf("a name that does not exist: %v", missing)
	}
	// An Indian-script domain name is looked up in its ASCII form.
	hindi := expectStatus(t, "/v1/dns/"+url.PathEscape("भारत.in"), http.StatusOK).data(t)
	if hindi["name"] != "xn--h2brj9c.in" || len(hindi["resolvers"].([]any)[0].(map[string]any)["records"].([]any)) != 1 {
		t.Errorf("IDN: %v", hindi)
	}
	ptr := expectStatus(t, "/v1/dns/8.8.8.8?type=ptr", http.StatusOK).data(t)
	if ptr["name"] != "8.8.8.8.in-addr.arpa" || ptr["resolvers"].([]any)[0].(map[string]any)["records"].([]any)[0].(map[string]any)["value"] != "dns.google." {
		t.Errorf("PTR: %v", ptr)
	}

	for _, bad := range []string{"/v1/dns/dharun.dev?type=AXFR", "/v1/dns/not%20a%20name", "/v1/dns/" + strings.Repeat("a", 64) + ".in", "/v1/dns/a..b", "/v1/dns/dharun.dev?type=PTR"} {
		expectStatus(t, bad, http.StatusBadRequest)
	}
}

func TestEmailCheck(t *testing.T) {
	check := func(address string) map[string]any {
		t.Helper()
		got := expectStatus(t, "/v1/email/"+url.PathEscape(address), http.StatusOK)
		if got.header.Get("Cache-Control") != "private, no-store" {
			t.Errorf("an email check must not be shared-cached")
		}
		return got.data(t)
	}
	gmail := check("Dharun.A+news@Gmail.com")
	if gmail["verdict"] != "ok" || gmail["freeProvider"] != true || gmail["domain"] != "gmail.com" || gmail["disposable"] != false {
		t.Errorf("gmail: %v", gmail)
	}
	if servers := gmail["mailServers"].(map[string]any); servers["hosts"].([]any)[0] != "gmail-smtp-in.l.google.com" {
		t.Errorf("mail servers: %v", servers)
	}
	for field, value := range gmail {
		if s, ok := value.(string); ok && strings.Contains(strings.ToLower(s), "dharun.a") {
			t.Errorf("the mailbox name must not be echoed back (%s)", field)
		}
	}

	if throwaway := check("x@mailinator.com"); throwaway["verdict"] != "disposable" || throwaway["disposable"] != true {
		t.Errorf("mailinator: %v", throwaway)
	}
	if sub := check("x@inbox.mailinator.com"); sub["disposable"] != true {
		t.Errorf("a subdomain of a disposable domain: %v", sub)
	}
	if role := check("support@dharun.dev"); role["roleAccount"] != true || role["verdict"] != "ok" || role["freeProvider"] != false {
		t.Errorf("role account: %v", role)
	}
	if implicit := check("a@onlyaddress.in"); implicit["verdict"] != "ok" || implicit["mailServers"].(map[string]any)["implicit"] != true {
		t.Errorf("a domain with only an address record: %v", implicit)
	}
	if null := check("a@nomail.in"); null["verdict"] != "no_mail_server" || null["mailServers"].(map[string]any)["nullMx"] != true {
		t.Errorf("null MX: %v", null)
	}
	if gone := check("a@nothing-here.in"); gone["verdict"] != "no_mail_server" {
		t.Errorf("a domain that does not exist: %v", gone)
	}
	for _, bad := range []string{"plainaddress", "@gmail.com", "a@", "a..b@gmail.com", ".a@gmail.com", "a b@gmail.com", "a@localhost", "a@gmail..com", strings.Repeat("a", 65) + "@gmail.com"} {
		if result := check(bad); result["validSyntax"] != false || result["verdict"] != "invalid" || result["problem"] == "" {
			t.Errorf("%q should be invalid: %v", bad, result)
		}
	}
}
