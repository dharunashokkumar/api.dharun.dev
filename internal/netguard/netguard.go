// Package netguard opens connections to hosts that strangers name, and only to the
// public internet. A name is looked up once, every address it has must be public, and
// the connection goes to the address that was checked, so a name cannot be pointed at
// a private address between the check and the connection. Only listed ports are
// reached, over IPv4, with short timeouts. Every redirect is a new connection and is
// checked again in the same way.
package netguard

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"syscall"
	"time"
)

var (
	// ErrReserved means the host is, or resolves to, an address that is not public.
	ErrReserved = errors.New("not a public address")
	// ErrPort means the port is not one this guard connects to.
	ErrPort = errors.New("port not allowed")
	// ErrNoAddress means the name has no IPv4 address.
	ErrNoAddress = errors.New("no IPv4 address")
	// ErrIPv6 means an IPv6 address was given; the server has no IPv6 connection.
	ErrIPv6 = errors.New("IPv6 is not available")
	// ErrLookup means no resolver answered.
	ErrLookup = errors.New("the name could not be resolved")
)

// AddressError says which address was refused.
type AddressError struct {
	Host    string
	Address netip.Addr
}

func (e *AddressError) Error() string {
	return e.Host + " is or resolves to " + e.Address.String() + ", which is not a public address"
}

func (e *AddressError) Unwrap() error { return ErrReserved }

// Guard holds the rules for one kind of outgoing connection.
type Guard struct {
	// Lookup returns the IPv4 addresses of a name. ErrNoAddress when it has none.
	Lookup func(ctx context.Context, host string) ([]netip.Addr, error)
	// Public reports whether an address may be connected to.
	Public func(netip.Addr) bool
	// Ports lists the ports that may be reached.
	Ports []int
	// Dial makes the connection to a checked address; nil means an ordinary dialer.
	// Tests send it to servers of their own.
	Dial func(ctx context.Context, network, address string) (net.Conn, error)
	// ConnectTimeout bounds one TCP connection. Default three seconds.
	ConnectTimeout time.Duration
}

// Resolve turns a host (a name or an IPv4 address) into the one address to connect to.
func (g *Guard) Resolve(ctx context.Context, host string) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		addr = addr.Unmap().WithZone("")
		if addr.Is6() {
			return netip.Addr{}, ErrIPv6
		}
		if !g.Public(addr) {
			return netip.Addr{}, &AddressError{Host: host, Address: addr}
		}
		return addr, nil
	}
	addrs, err := g.Lookup(ctx, host)
	if err != nil {
		return netip.Addr{}, err
	}
	if len(addrs) == 0 {
		return netip.Addr{}, ErrNoAddress
	}
	// One private address among public ones is refused as a whole: which one a client
	// would use is not ours to guess.
	for _, addr := range addrs {
		if !g.Public(addr.Unmap()) {
			return netip.Addr{}, &AddressError{Host: host, Address: addr}
		}
	}
	return addrs[0].Unmap(), nil
}

// Trace is told how long each step of a connection took. Attach one with WithTrace.
type Trace struct {
	Resolved  func(addr netip.Addr, took time.Duration)
	Connected func(took time.Duration)
}

type traceKey struct{}

// WithTrace returns a context whose connections report to trace.
func WithTrace(ctx context.Context, trace *Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, trace)
}

// DialContext connects to address ("host:port") after checking it. It has the shape
// http.Transport and tls.Dialer want.
func (g *Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || !slices.Contains(g.Ports, port) {
		return nil, fmt.Errorf("%w: %s", ErrPort, portText)
	}
	trace, _ := ctx.Value(traceKey{}).(*Trace)
	started := time.Now()
	addr, err := g.Resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	if trace != nil && trace.Resolved != nil {
		trace.Resolved(addr, time.Since(started))
	}
	timeout := g.ConnectTimeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	target := netip.AddrPortFrom(addr, uint16(port)).String()
	started = time.Now()
	var conn net.Conn
	if g.Dial != nil {
		conn, err = g.Dial(ctx, "tcp4", target)
	} else {
		// The address is checked once more as the socket connects, in case anything
		// between here and the kernel changed it.
		dialer := net.Dialer{Control: func(_, address string, _ syscall.RawConn) error {
			connecting, err := netip.ParseAddrPort(address)
			if err != nil || !g.Public(connecting.Addr().Unmap()) {
				return &AddressError{Host: host, Address: connecting.Addr()}
			}
			return nil
		}}
		conn, err = dialer.DialContext(ctx, "tcp4", target)
	}
	if err != nil {
		return nil, err
	}
	if trace != nil && trace.Connected != nil {
		trace.Connected(time.Since(started))
	}
	return conn, nil
}

// ClientOptions shape an HTTP client that only makes guarded connections.
type ClientOptions struct {
	// Timeout bounds a whole request, redirects included.
	Timeout time.Duration
	// Roots are the certificate authorities trusted; nil means the system's.
	Roots *x509.CertPool
	// Redirects is how many redirects are followed; zero follows none and returns the
	// redirect itself.
	Redirects int
	// UserAgent is sent with every request.
	UserAgent string
}

// Client returns an HTTP client whose every connection goes through the guard. It
// keeps no connections open between requests and ignores proxy settings.
func (g *Guard) Client(options ClientOptions) *http.Client {
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            g.DialContext,
		TLSClientConfig:        &tls.Config{RootCAs: options.Roots, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  8 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		DisableKeepAlives:      true,
		ForceAttemptHTTP2:      true,
	}
	agent := options.UserAgent
	return &http.Client{
		Timeout:   options.Timeout,
		Transport: userAgent{transport, agent},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > options.Redirects {
				return http.ErrUseLastResponse
			}
			if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
				return fmt.Errorf("redirect to a %s address", req.URL.Scheme)
			}
			return nil
		},
	}
}

type userAgent struct {
	next  http.RoundTripper
	agent string
}

func (u userAgent) RoundTrip(req *http.Request) (*http.Response, error) {
	if u.agent != "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", u.agent)
	}
	return u.next.RoundTrip(req)
}
