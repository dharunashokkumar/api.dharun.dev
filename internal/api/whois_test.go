package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["whois_lookup"] = `{"query":"google.in"}`
}

// Cut down from the answers rdap.nixiregistry.in and rdap.apnic.net gave on 2026-10-16.
const (
	rdapGoogleIn = `{"objectClassName":"domain","handle":"D21089-IN","ldhName":"google.in","unicodeName":"google.in",
"nameservers":[{"objectClassName":"nameserver","ldhName":"ns1.google.com"},{"objectClassName":"nameserver","ldhName":"NS2.GOOGLE.COM."}],
"secureDNS":{"delegationSigned":false},
"entities":[
 {"objectClassName":"entity","roles":["registrant"],"vcardArray":["vcard",[["version",{},"text","4.0"],["org",{},"text","Google LLC"],["fn",{},"text","A Person"],["email",{},"text","someone@example.com"],["adr",{"cc":"US"},"text",["","","1600 Amphitheatre","","","",""]]]]},
 {"objectClassName":"entity","roles":["administrative"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Admin Person"],["tel",{},"uri","tel:+1.6502530000"]]]},
 {"objectClassName":"entity","handle":"292","roles":["registrar"],"publicIds":[{"type":"IANA Registrar ID","identifier":"292"}],
  "vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","MarkMonitor Inc."]]],
  "links":[{"rel":"about","href":"http://www.markmonitor.com","type":"text/html"}],
  "entities":[{"objectClassName":"entity","roles":["abuse"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Markmonitor Inc."],["tel",{"type":"voice"},"uri","tel:+1.2086851750"],["email",{},"text","abusecomplaints@markmonitor.com"]]]}]}
],
"status":["client update prohibited","client transfer prohibited"],
"events":[{"eventAction":"registration","eventDate":"2005-02-14T20:35:14.765Z"},{"eventAction":"expiration","eventDate":"2099-02-14T20:35:14.765Z"},{"eventAction":"last changed","eventDate":"2026-05-26T12:16:48.154Z"}]}`

	rdapJio = `{"objectClassName":"ip network","handle":"49.32.0.0 - 49.47.255.255","startAddress":"49.32.0.0","endAddress":"49.47.255.255",
"ipVersion":"v4","name":"RELIANCEJIO-IN","type":"ALLOCATED PORTABLE","country":"IN","status":["active"],
"cidr0_cidrs":[{"v4prefix":"49.32.0.0","length":12}],
"entities":[
 {"objectClassName":"entity","handle":"IRT-RELIANCEJIO-IN","roles":["abuse"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","IRT-RELIANCEJIO-IN"],["kind",{},"text","group"],["email",{},"text","ip.abuse@ril.com"]]]},
 {"objectClassName":"entity","handle":"ORG-RJIL1-AP","roles":["registrant"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Reliance Jio Infocomm Limited"],["kind",{},"text","org"],["email",{},"text","ip.management@ril.com"]]]}
],
"events":[{"eventAction":"registration","eventDate":"2010-10-27T03:29:49Z"},{"eventAction":"last changed","eventDate":"2020-08-19T13:07:30Z"}]}`

	rdapPersonNetwork = `{"objectClassName":"ip network","handle":"103.1.2.0 - 103.1.2.255","startAddress":"103.1.2.0","endAddress":"103.1.2.255","name":"HOME-NET","country":"IN",
"entities":[{"objectClassName":"entity","roles":["registrant"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Ravi Kumar"],["kind",{},"text","individual"]]]}]}`

	rdapAS55836 = `{"objectClassName":"autnum","handle":"AS55836","startAutnum":55836,"endAutnum":55836,"name":"RELIANCEJIO-IN","country":"IN",
"entities":[{"objectClassName":"entity","roles":["registrant"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Reliance Jio Infocomm Limited"],["kind",{},"text","org"]]]}],
"events":[{"eventAction":"registration","eventDate":"2010-10-27T03:22:42Z"}]}`
)

// serveRegistries sets up the registries the whois tests and the documented example ask.
func serveRegistries(sites *testSites) {
	sites.serve("rdap.nixiregistry.in", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != probeAgent || !strings.Contains(r.Header.Get("Accept"), "application/rdap+json") {
			http.Error(w, "bad request headers", http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case "/rdap/domain/google.in", "/rdap/domain/google.co.in":
			jsonSite(http.StatusOK, rdapGoogleIn).ServeHTTP(w, r)
		default:
			jsonSite(http.StatusNotFound, `{"errorCode":404}`).ServeHTTP(w, r)
		}
	}))
	sites.serve("rdap.apnic.net", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ip/49.36.0.1":
			jsonSite(http.StatusOK, rdapJio).ServeHTTP(w, r)
		case "/ip/103.1.2.3":
			jsonSite(http.StatusOK, rdapPersonNetwork).ServeHTTP(w, r)
		case "/autnum/55836":
			jsonSite(http.StatusOK, rdapAS55836).ServeHTTP(w, r)
		case "/ip/103.9.9.9":
			// Space handed to another registry is sent on with a redirect.
			http.Redirect(w, r, "https://rdap.arin.net/registry/ip/103.9.9.9", http.StatusMovedPermanently)
		default:
			http.NotFound(w, r)
		}
	}))
	sites.serve("rdap.arin.net", jsonSite(http.StatusOK, rdapJio))

}

func TestWhois(t *testing.T) {
	useTestSites(t)
	domain := expectStatus(t, "/v1/whois/www.google.in", http.StatusOK)
	data := domain.data(t)
	registrar, _ := data["registrar"].(map[string]any)
	abuse, _ := registrar["abuse"].(map[string]any)
	if data["domain"] != "google.in" || data["kind"] != "domain" || registrar["name"] != "MarkMonitor Inc." || registrar["ianaId"] != "292" ||
		abuse["email"] != "abusecomplaints@markmonitor.com" || data["registered"] != "2005-02-14T20:35:14Z" || data["dnssec"] != false {
		t.Errorf("domain: %v", data)
	}
	if ns := data["nameservers"].([]any); len(ns) != 2 || ns[1] != "ns2.google.com" {
		t.Errorf("nameservers: %v", ns)
	}
	if days, _ := data["daysToExpiry"].(float64); days < 1000 {
		t.Errorf("days to expiry: %v", data["daysToExpiry"])
	}
	// Nothing about the people behind the domain comes through.
	for _, personal := range []string{"Google LLC", "A Person", "someone@example.com", "Admin Person", "6502530000", "Amphitheatre"} {
		if strings.Contains(string(domain.body), personal) {
			t.Errorf("the answer carries %q", personal)
		}
	}
	if !strings.Contains(domain.header.Get("Cache-Control"), "public") {
		t.Errorf("a domain lookup can be cached: %q", domain.header.Get("Cache-Control"))
	}
	// A subdomain under .co.in is looked up as the registered domain.
	if got := expectStatus(t, "/v1/whois/mail.google.co.in", http.StatusOK).data(t); got["query"] != "google.co.in" {
		t.Errorf("co.in: %v", got["query"])
	}
	expectStatus(t, "/v1/whois/never-registered.in", http.StatusNotFound)
	// The Indian-script top-level domains offer no RDAP.
	if got := expectStatus(t, "/v1/whois/"+url.PathEscape("भारत.भारत"), http.StatusNotFound); !strings.Contains(string(got.body), ".भारत offers no RDAP") {
		t.Errorf("IDN TLD: %s", got.body)
	}

	ip := expectStatus(t, "/v1/whois/49.36.0.1", http.StatusOK)
	network := ip.data(t)
	if network["kind"] != "ip" || network["name"] != "RELIANCEJIO-IN" || network["holder"] != "Reliance Jio Infocomm Limited" ||
		network["country"] != "IN" || network["cidrs"].([]any)[0] != "49.32.0.0/12" || network["abuse"].(map[string]any)["email"] != "ip.abuse@ril.com" {
		t.Errorf("IP: %v", network)
	}
	if strings.Contains(string(ip.body), "ip.management@ril.com") || ip.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("an IP lookup keeps the holder's mailbox out and is private: %s %q", ip.body, ip.header.Get("Cache-Control"))
	}
	// A network assigned to a person does not name them.
	if person := expectStatus(t, "/v1/whois/103.1.2.3", http.StatusOK); person.data(t)["holder"] != nil || strings.Contains(string(person.body), "Ravi") {
		t.Errorf("a person's network: %s", person.body)
	}
	// A registry's redirect to another registry is followed.
	if moved := expectStatus(t, "/v1/whois/103.9.9.9", http.StatusOK).data(t); moved["name"] != "RELIANCEJIO-IN" {
		t.Errorf("redirect: %v", moved)
	}

	for _, query := range []string{"AS55836", "as55836", "55836"} {
		asn := expectStatus(t, "/v1/whois/"+query, http.StatusOK).data(t)
		if asn["kind"] != "asn" || asn["asn"] != float64(55836) || asn["query"] != "AS55836" || asn["holder"] != "Reliance Jio Infocomm Limited" {
			t.Errorf("%s: %v", query, asn)
		}
	}
	expectStatus(t, "/v1/whois/AS64512", http.StatusNotFound) // private AS numbers have no registry
	for _, bad := range []string{"in", "not%20a%20name", "a..b"} {
		expectStatus(t, "/v1/whois/"+bad, http.StatusBadRequest)
	}
}

func TestRegisteredDomain(t *testing.T) {
	for name, want := range map[string]string{
		"google.in": "google.in", "www.google.co.in": "google.co.in", "user.github.io": "github.io",
		"a.b.example.com": "example.com", "example.unknowntld": "example.unknowntld", "x.y.gov.in": "y.gov.in",
	} {
		if got, err := registeredDomain(name); err != nil || got != want {
			t.Errorf("registeredDomain(%s) = %s, %v; want %s", name, got, err, want)
		}
	}
}
