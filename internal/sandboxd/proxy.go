//go:build linux

package sandboxd

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// The microVMs have no network device. A program inside that wants the internet talks to
// the agent's proxy on 127.0.0.1:3128, which carries the connection over vsock to a
// socket of this VM's own here; this side decides where it may go. Code sessions reach
// their languages' package registries and nothing else; the browser reaches any public
// address on ports 80 and 443, a limited number of times a minute per site.

const (
	policyNone       = ""
	policyRegistries = "registries"
	policyBrowser    = "browser"
	// browserCap bounds what one page may download.
	browserCap = 64 << 20
)

// registryHosts are the package registries of the code image's languages and the hosts
// they send downloads to.
var registryHosts = map[string]bool{
	// Python
	"pypi.org": true, "files.pythonhosted.org": true,
	// JavaScript
	"registry.npmjs.org": true, "registry.yarnpkg.com": true,
	// Go
	"proxy.golang.org": true, "sum.golang.org": true, "storage.googleapis.com": true,
	// Ruby
	"rubygems.org": true, "index.rubygems.org": true,
	// PHP: Packagist names the archives on GitHub
	"repo.packagist.org": true, "packagist.org": true, "api.github.com": true, "codeload.github.com": true,
}

// RegistryHosts lists the registry hosts, for the documentation.
func RegistryHosts() []string {
	hosts := make([]string, 0, len(registryHosts))
	for host := range registryHosts {
		hosts = append(hosts, host)
	}
	return hosts
}

type egress struct {
	registries *netguard.Guard
	browser    *netguard.Guard
	targets    *netguard.TargetLimit
	metrics    *metrics
}

func newEgress(m *metrics, perSite int) *egress {
	lookup := publicLookup()
	public := func(addr netip.Addr) bool { return !netguard.Reserved(addr) }
	return &egress{
		registries: &netguard.Guard{Lookup: lookup, Public: public, Ports: []int{443}, ConnectTimeout: 5 * time.Second},
		browser:    &netguard.Guard{Lookup: lookup, Public: public, Ports: []int{80, 443}, ConnectTimeout: 5 * time.Second},
		targets:    &netguard.TargetLimit{PerMinute: perSite},
		metrics:    m,
	}
}

// publicLookup resolves names through public resolvers, never the VM's own: the answer
// must not depend on anything inside the homelab.
func publicLookup() func(ctx context.Context, host string) ([]netip.Addr, error) {
	resolvers := []string{"1.1.1.1:53", "9.9.9.9:53"}
	return func(ctx context.Context, host string) ([]netip.Addr, error) {
		var lastErr error
		for _, server := range resolvers {
			resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, network, server)
			}}
			lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			addrs, err := resolver.LookupNetIP(lookupCtx, "ip4", host)
			cancel()
			if err == nil {
				return addrs, nil
			}
			var dnsErr *net.DNSError
			if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
				return nil, netguard.ErrNoAddress
			}
			lastErr = err
		}
		return nil, errors.Join(netguard.ErrLookup, lastErr)
	}
}

// proxyListener is one VM's end of the proxy.
type proxyListener struct {
	listener net.Listener
	policy   string
	cap      int64
	used     atomic.Int64
	mu       sync.Mutex
	conns    map[net.Conn]bool
	closed   bool
}

func (e *egress) listen(path string, uid int, policy string) (*proxyListener, error) {
	os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	// Firecracker runs as the VM's own user and must be able to connect.
	os.Chown(path, uid, uid)
	os.Chmod(path, 0o600)
	p := &proxyListener{listener: listener, policy: policy, cap: sandbox.DownloadCap, conns: map[net.Conn]bool{}}
	if policy == policyBrowser {
		p.cap = browserCap
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			if !p.track(conn) {
				conn.Close()
				return
			}
			go func() {
				defer p.untrack(conn)
				e.serve(conn, p)
			}()
		}
	}()
	return p, nil
}

func (p *proxyListener) track(conn net.Conn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	p.conns[conn] = true
	return true
}

func (p *proxyListener) untrack(conn net.Conn) {
	p.mu.Lock()
	delete(p.conns, conn)
	p.mu.Unlock()
	conn.Close()
}

func (p *proxyListener) close() {
	p.mu.Lock()
	p.closed = true
	for conn := range p.conns {
		conn.Close()
	}
	p.mu.Unlock()
	p.listener.Close()
}

// allowed reports whether a policy lets a connection go to host:port.
func allowed(policy, host string, port int) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	switch policy {
	case policyRegistries:
		return port == 443 && registryHosts[host]
	case policyBrowser:
		return port == 80 || port == 443
	}
	return false
}

func refuse(conn net.Conn, status int, reason string) {
	body := reason + "\n"
	io.WriteString(conn, "HTTP/1.1 "+strconv.Itoa(status)+" "+http.StatusText(status)+"\r\nContent-Type: text/plain\r\nContent-Length: "+
		strconv.Itoa(len(body))+"\r\nConnection: close\r\n\r\n"+body)
}

// serve handles one proxied connection: a CONNECT tunnel, or one plain HTTP request.
func (e *egress) serve(client net.Conn, p *proxyListener) {
	client.SetReadDeadline(time.Now().Add(30 * time.Second))
	reader := bufio.NewReader(client)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	client.SetReadDeadline(time.Time{})
	var host string
	var port int
	if req.Method == http.MethodConnect {
		h, portText, err := net.SplitHostPort(req.Host)
		if err != nil {
			refuse(client, http.StatusBadRequest, "CONNECT needs host:port")
			return
		}
		host, port = h, atoiOr(portText, 0)
	} else {
		if req.URL == nil || req.URL.Scheme != "http" || req.URL.Host == "" {
			refuse(client, http.StatusBadRequest, "only CONNECT and absolute http:// requests go through this proxy")
			return
		}
		host, port = req.URL.Hostname(), atoiOr(req.URL.Port(), 80)
	}
	if !allowed(p.policy, host, port) {
		e.metrics.proxyRefused.add(p.policy, 1)
		refuse(client, http.StatusForbidden, "this sandbox may not reach "+host+":"+strconv.Itoa(port))
		return
	}
	guard := e.registries
	if p.policy == policyBrowser {
		guard = e.browser
		if !e.targets.Allow(host) {
			e.metrics.proxyRefused.add(p.policy, 1)
			refuse(client, http.StatusTooManyRequests, "too many requests to "+host+" from the sandboxes this minute")
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	upstream, err := guard.DialContext(ctx, "tcp4", net.JoinHostPort(host, strconv.Itoa(port)))
	cancel()
	if err != nil {
		e.metrics.proxyRefused.add(p.policy, 1)
		status := http.StatusBadGateway
		if errors.Is(err, netguard.ErrReserved) || errors.Is(err, netguard.ErrPort) {
			status = http.StatusForbidden
		}
		refuse(client, status, err.Error())
		return
	}
	defer upstream.Close()
	if req.Method == http.MethodConnect {
		io.WriteString(client, "HTTP/1.1 200 Connection established\r\n\r\n")
		// Bytes the client sent after its request line belong to the tunnel.
		if buffered := reader.Buffered(); buffered > 0 {
			pending, _ := reader.Peek(buffered)
			upstream.Write(pending)
		}
	} else {
		// One request per connection: the proxy cannot follow a client that switches
		// hosts on a kept-alive connection.
		req.Header.Set("Connection", "close")
		req.Header.Del("Proxy-Connection")
		req.RequestURI = ""
		if err := req.Write(upstream); err != nil {
			return
		}
	}
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(upstream, reader)
		if tcp, ok := upstream.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		e.copyCapped(client, upstream, p)
		client.Close()
		done <- struct{}{}
	}()
	<-done
	<-done
}

// copyCapped copies the downloaded side, stopping when the VM has used its allowance.
func (e *egress) copyCapped(to io.Writer, from io.Reader, p *proxyListener) {
	buffer := make([]byte, 32<<10)
	for {
		n, err := from.Read(buffer)
		if n > 0 {
			if p.used.Add(int64(n)) > p.cap {
				e.metrics.proxyRefused.add(p.policy, 1)
				return
			}
			e.metrics.proxyBytes.add(p.policy, int64(n))
			if _, werr := to.Write(buffer[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func atoiOr(text string, fallback int) int {
	if text == "" {
		return fallback
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		return -1
	}
	return n
}
