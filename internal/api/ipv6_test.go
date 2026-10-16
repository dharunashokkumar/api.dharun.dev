package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["check_ipv6"] = `{"domain":"dharun.dev"}`
	fakeZone["dharun.dev. NS"] = []string{"dharun.dev. 300 IN NS kate.ns.cloudflare.com.", "dharun.dev. 300 IN NS adam.ns.cloudflare.com."}
	fakeZone["kate.ns.cloudflare.com. AAAA"] = []string{"kate.ns.cloudflare.com. 300 IN AAAA 2803:f800:50::6ca2:c11a"}
	fakeZone["adam.ns.cloudflare.com. AAAA"] = []string{"adam.ns.cloudflare.com. 300 IN AAAA 2a06:98c1:50::ac40:2068"}
	fakeZone["route1.mx.cloudflare.net. AAAA"] = []string{"route1.mx.cloudflare.net. 300 IN AAAA 2606:4700:f5::b"}
	fakeZone["route2.mx.cloudflare.net. AAAA"] = []string{"route2.mx.cloudflare.net. 300 IN AAAA 2606:4700:f5::c"}
	fakeZone["v4only.in. A"] = []string{"v4only.in. 300 IN A 93.184.215.20"}
	fakeZone["v4only.in. NS"] = []string{"v4only.in. 300 IN NS ns1.v4only.in."}
	fakeZone["ns1.v4only.in. A"] = []string{"ns1.v4only.in. 300 IN A 93.184.215.21"}
	fakeZone["tunnel.in. AAAA"] = []string{"tunnel.in. 300 IN AAAA 2002:c000:204::1"}
}

func TestIPv6Check(t *testing.T) {
	ready := expectStatus(t, "/v1/ipv6/dharun.dev", http.StatusOK).data(t)
	website := ready["website"].(map[string]any)
	hosts := website["hosts"].([]any)
	if ready["ready"] != true || website["ok"] != true || hosts[0].(map[string]any)["ipv6"].([]any)[0] != "2606:4700:3030::6815:1" ||
		hosts[1].(map[string]any)["name"] != "www.dharun.dev" || len(ready["problems"].([]any)) != 0 {
		t.Errorf("an IPv6-ready domain: %v", ready)
	}
	if dns := ready["dns"].(map[string]any); dns["ok"] != true || len(dns["hosts"].([]any)) != 2 || dns["hosts"].([]any)[0].(map[string]any)["name"] != "adam.ns.cloudflare.com" {
		t.Errorf("nameservers: %v", dns)
	}
	if mail := ready["mail"].(map[string]any); mail["ok"] != true || mail["hosts"].([]any)[0].(map[string]any)["priority"] != float64(10) {
		t.Errorf("mail: %v", mail)
	}

	old := expectStatus(t, "/v1/ipv6/v4only.in", http.StatusOK).data(t)
	if old["ready"] != false || old["website"].(map[string]any)["ok"] != false || old["dns"].(map[string]any)["ok"] != false ||
		old["mail"].(map[string]any)["ok"] != nil || len(old["problems"].([]any)) != 2 {
		t.Errorf("an IPv4-only domain: %v", old)
	}
	tunnel := expectStatus(t, "/v1/ipv6/tunnel.in", http.StatusOK).data(t)
	if tunnel["website"].(map[string]any)["ok"] != false || len(tunnel["website"].(map[string]any)["hosts"].([]any)[0].(map[string]any)["notes"].([]any)) != 1 {
		t.Errorf("a 6to4 address is not ordinary IPv6: %v", tunnel)
	}
	for _, bad := range []string{"1.1.1.1", "in", "not%20a%20name"} {
		expectStatus(t, "/v1/ipv6/"+bad, http.StatusBadRequest)
	}
}
