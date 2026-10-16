package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

var publicTest = func(addr netip.Addr) bool {
	return !(addr.IsPrivate() || addr.IsLoopback() || addr.IsUnspecified() || addr.IsLinkLocalUnicast())
}

// testGuard resolves from a fixed table and sends every connection that passes the
// checks to target.
func testGuard(target string, zone map[string][]string) *Guard {
	return &Guard{
		Lookup: func(_ context.Context, host string) ([]netip.Addr, error) {
			var addrs []netip.Addr
			for _, text := range zone[host] {
				addrs = append(addrs, netip.MustParseAddr(text))
			}
			if len(addrs) == 0 {
				return nil, ErrNoAddress
			}
			return addrs, nil
		},
		Public: publicTest,
		Ports:  []int{80, 443},
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", target)
		},
	}
}

func TestResolveRefusesPrivateAddresses(t *testing.T) {
	guard := testGuard("", map[string][]string{
		"public.test":  {"93.184.215.14"},
		"private.test": {"192.168.1.10"},
		"mixed.test":   {"93.184.215.14", "10.0.0.1"},
		"mapped.test":  {"::ffff:127.0.0.1"},
	})
	ctx := context.Background()
	if addr, err := guard.Resolve(ctx, "public.test"); err != nil || addr.String() != "93.184.215.14" {
		t.Errorf("public: %v %v", addr, err)
	}
	for _, host := range []string{"private.test", "mixed.test", "mapped.test", "127.0.0.1", "10.1.2.3", "::ffff:192.168.0.1"} {
		var refused *AddressError
		if _, err := guard.Resolve(ctx, host); !errors.As(err, &refused) || !errors.Is(err, ErrReserved) {
			t.Errorf("%s: %v, want a refusal", host, err)
		}
	}
	if _, err := guard.Resolve(ctx, "2606:4700::1111"); !errors.Is(err, ErrIPv6) {
		t.Errorf("IPv6: %v", err)
	}
	if _, err := guard.Resolve(ctx, "nothing.test"); !errors.Is(err, ErrNoAddress) {
		t.Errorf("unknown name: %v", err)
	}
}

func TestDialChecksPortAndAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/inside":
			http.Redirect(w, r, "http://private.test/", http.StatusFound)
		case "/port":
			http.Redirect(w, r, "http://public.test:8080/", http.StatusFound)
		case "/agent":
			w.Write([]byte(r.Header.Get("User-Agent")))
		default:
			w.Write([]byte("ok"))
		}
	}))
	defer server.Close()
	guard := testGuard(server.Listener.Addr().String(), map[string][]string{"public.test": {"93.184.215.14"}, "private.test": {"10.0.0.5"}})
	client := guard.Client(ClientOptions{Timeout: 5 * time.Second, Redirects: 5, UserAgent: "test-agent"})

	resp, err := client.Get("http://public.test/agent")
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	resp.Body.Close()
	if string(body[:n]) != "test-agent" {
		t.Errorf("user agent %q", body[:n])
	}
	// A redirect into private space or to another port is refused when it connects.
	for path, want := range map[string]error{"/inside": ErrReserved, "/port": ErrPort} {
		if _, err := client.Get("http://public.test" + path); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", path, err, want)
		}
	}
	if _, err := client.Get("http://public.test:22/"); !errors.Is(err, ErrPort) {
		t.Errorf("port 22: %v", err)
	}

	var resolved, connected bool
	ctx := WithTrace(context.Background(), &Trace{
		Resolved:  func(netip.Addr, time.Duration) { resolved = true },
		Connected: func(time.Duration) { connected = true },
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://public.test/", nil)
	if resp, err := client.Do(req); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}
	if !resolved || !connected {
		t.Errorf("trace: resolved %v, connected %v", resolved, connected)
	}
}

func TestRealDialerChecksTheSocketAddress(t *testing.T) {
	// No Dial hook: the address check in the dialer's Control refuses loopback even if
	// the lookup was fooled into returning it as public.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	calls := 0
	guard := &Guard{
		Lookup: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		},
		Public: func(netip.Addr) bool { calls++; return calls == 1 }, // fooled once
		Ports:  []int{atoi(port)},
	}
	if _, err := guard.DialContext(context.Background(), "tcp", "fooled.test:"+port); !errors.Is(err, ErrReserved) {
		t.Errorf("socket check: %v", err)
	}
}

func atoi(text string) int {
	n := 0
	for _, c := range text {
		n = n*10 + int(c-'0')
	}
	return n
}

func TestTargetLimit(t *testing.T) {
	now := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	limit := &TargetLimit{PerMinute: 2, now: func() time.Time { return now }}
	for i, host := range []string{"www.example.co.in", "example.co.in", "mail.example.co.in"} {
		if got := limit.Allow(host); got != (i < 2) {
			t.Errorf("request %d to %s: %v", i, host, got)
		}
	}
	if !limit.Allow("other.in") {
		t.Error("another site has its own count")
	}
	now = now.Add(time.Minute)
	if !limit.Allow("example.co.in") {
		t.Error("the count starts again each minute")
	}
	for host, want := range map[string]string{"1.2.3.4": "1.2.3.0/24", "WWW.Google.COM.": "google.com", "a.b.github.io": "b.github.io"} {
		if got := TargetKey(host); got != want {
			t.Errorf("TargetKey(%s) = %s, want %s", host, got, want)
		}
	}
	if strings.Contains(TargetKey("localhost"), ".") {
		t.Error("a single label stays itself")
	}
}
