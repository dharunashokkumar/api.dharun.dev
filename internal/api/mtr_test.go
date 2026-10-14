package api

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/trace"
)

func init() {
	mcpTestCalls["traceroute_from_india"] = `{"host":"1.1.1.1","rounds":2}`
}

// fakeTrace answers every trace with the same path: the home router, a provider's
// shared address space, a public router in DB-IP's test data, then the target.
func fakeTrace(_ context.Context, opts trace.Options) (trace.Result, error) {
	ms := time.Millisecond
	hop := func(ttl int, address string, received int, average time.Duration) trace.Hop {
		h := trace.Hop{TTL: ttl, Sent: opts.Rounds, Received: received, Best: average - ms, Average: average, Worst: average + ms, StdDev: ms}
		if address != "" {
			h.Address = netip.MustParseAddr(address)
		}
		return h
	}
	result := trace.Result{Reached: true, Hops: []trace.Hop{
		hop(1, "192.168.1.1", opts.Rounds, 1*ms),
		hop(2, "100.72.0.1", opts.Rounds, 5*ms),
		hop(3, "", 0, 0),
		hop(4, "49.204.0.1", opts.Rounds, 12*ms),
		hop(5, opts.Target.String(), opts.Rounds, 30*ms),
	}}
	result.Hops[3].Others = []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.1.1.1")}
	result.Hops[4].Destination = true
	return result, nil
}

func TestMTR(t *testing.T) {
	got := expectStatus(t, "/v1/mtr/1.1.1.1?rounds=3", http.StatusOK)
	data := got.data(t)
	if data["address"] != "1.1.1.1" || data["reached"] != true || data["rounds"].(float64) != 3 {
		t.Fatalf("trace: %v", data)
	}
	hops := data["hops"].([]any)
	if len(hops) != 5 {
		t.Fatalf("%d hops", len(hops))
	}
	home, shared, silent, public, last := hops[0].(map[string]any), hops[1].(map[string]any), hops[2].(map[string]any), hops[3].(map[string]any), hops[4].(map[string]any)
	// The network the server is in must never be published.
	if home["address"] != nil || home["private"] != true || shared["address"] != nil || shared["private"] != true {
		t.Errorf("private hops leak: %v %v", home, shared)
	}
	if strings.Contains(string(got.body), "192.168.") || strings.Contains(string(got.body), "100.72.") || strings.Contains(string(got.body), "10.1.1.1") {
		t.Errorf("a private address is in the response: %s", got.body)
	}
	if silent["lossPercent"].(float64) != 100 || silent["averageMs"] != nil {
		t.Errorf("silent hop: %v", silent)
	}
	if public["address"] != "49.204.0.1" || public["network"].(map[string]any)["number"].(float64) != 55577 || public["country"] != "IN" || public["averageMs"].(float64) != 12 {
		t.Errorf("public hop: %v", public)
	}
	if others := public["otherAddresses"].([]any); len(others) != 1 || others[0] != "8.8.8.8" {
		t.Errorf("other addresses: %v", others)
	}
	if public["hostname"] != nil {
		t.Errorf("hostname without a PTR record: %v", public["hostname"])
	}
	if last["destination"] != true {
		t.Errorf("last hop: %v", last)
	}

	// A name is resolved through the public resolvers.
	if named := expectStatus(t, "/v1/mtr/dharun.dev", http.StatusOK).data(t); named["address"] != "104.21.0.1" {
		t.Errorf("dharun.dev resolved to %v", named["address"])
	}
	// Reverse names come from PTR records.
	google := expectStatus(t, "/v1/mtr/8.8.8.8", http.StatusOK).data(t)
	if hostname := google["hops"].([]any)[4].(map[string]any)["hostname"]; hostname != "dns.google" {
		t.Errorf("PTR name: %v", hostname)
	}

	for _, refused := range []string{"127.0.0.1", "192.168.1.10", "100.64.1.1", "onlyaddress.in", "::1"} {
		expectStatus(t, "/v1/mtr/"+refused, http.StatusUnprocessableEntity)
	}
	expectStatus(t, "/v1/mtr/no-such-name.in", http.StatusNotFound)
	expectStatus(t, "/v1/mtr/1.1.1.1?rounds=11", http.StatusBadRequest)
	expectStatus(t, "/v1/mtr/1.1.1.1?v=5", http.StatusBadRequest)
	expectStatus(t, "/v1/mtr/not%20a%20host", http.StatusBadRequest)
}
