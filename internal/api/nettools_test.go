package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
)

// The network tools are tested against servers of the tests' own: every checked
// connection to port 443 goes to one TLS listener and every one to port 80 to a plain
// listener, and both answer as whichever site the request names. Certificates are
// issued on the spot by a test authority, which the tools are told to trust. Names
// resolve through fakeDNS to public-looking addresses, so the guards run as they do in
// production.

// The addresses fakeDNS gives the test sites. Public, so the guards let them through.
const (
	testSiteAddress = "93.184.215.14"
	testOwnAddress  = "122.164.10.20" // what whoami.cloudflare says the server is
)

var testSiteNames = []string{
	"rdap.nixiregistry.in", "rdap.verisign.com", "rdap.apnic.net", "rdap.arin.net", "pubapi.registry.google",
	"site.in", "www.site.in", "old.site.in", "expired.site.in", "rc4.site.in", "wrong.site.in", "self.site.in", "noinner.site.in", "legacy.site.in",
	"icons.site.in", "manifest.site.in", "plain.site.in", "noicon.site.in", "long.site.in", "closed.site.in", "big.site.in", "slow.site.in", "github.com",
}

func init() {
	for _, name := range testSiteNames {
		fakeZone[name+". A"] = []string{name + ". 300 IN A " + testSiteAddress}
	}
	fakeZone["whoami.cloudflare. TXT"] = []string{`whoami.cloudflare. 0 CH TXT "` + testOwnAddress + `"`}
	fakeZone["inside.site.in. A"] = []string{"inside.site.in. 300 IN A 192.168.1.20"}
	fakeZone["home.site.in. A"] = []string{"home.site.in. 300 IN A " + testOwnAddress}
	fakeZone["dharun.dev. AAAA"] = []string{"dharun.dev. 300 IN AAAA 2606:4700:3030::6815:1"}
}

// testSites is what the listeners serve: a handler per host name, and per host the
// certificate and TLS settings it presents.
type testSites struct {
	mu       sync.Mutex
	handlers map[string]http.Handler
	configs  map[string]*tls.Config
	ca       *x509.Certificate
	caKey    *ecdsa.PrivateKey
	roots    *x509.CertPool
	tlsLn    net.Listener
	plainLn  net.Listener
	dial     func(ctx context.Context, network, address string) (net.Conn, error)
}

// useFakeNetwork keeps a test server off the internet: DNS from fakeDNS, traceroutes
// from fakeTrace, and every connection the network tools make to the test sites.
func useFakeNetwork(server *Server) {
	server.extras.dns = fakeDNS{}
	server.extras.trace = fakeTrace
	if sites == nil {
		// Before the sites are started a connection has nowhere to go: refuse it.
		server.extras.network.dial = func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("tests never touch the network")
		}
		return
	}
	server.extras.network.roots = sites.roots
	server.extras.network.dial = sites.dial
	// The per-site limit is tested on its own; here it would only get in the way.
	server.extras.network.targets = &netguard.TargetLimit{PerMinute: 10000}
}

var (
	sites     *testSites
	sitesOnce sync.Once
)

// useTestSites returns the test sites; TestMain has started them.
func useTestSites(t *testing.T) *testSites {
	t.Helper()
	startTestSites()
	return sites
}

// startTestSites starts the listeners once and points the server's network tools at them.
func startTestSites() {
	sitesOnce.Do(func() {
		sites = &testSites{handlers: map[string]http.Handler{}, configs: map[string]*tls.Config{}}
		sites.ca, sites.caKey = newTestCA()
		sites.roots = x509.NewCertPool()
		sites.roots.AddCert(sites.ca)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := strings.ToLower(r.Host)
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			sites.mu.Lock()
			site := sites.handlers[host]
			sites.mu.Unlock()
			if site == nil {
				http.NotFound(w, r)
				return
			}
			site.ServeHTTP(w, r)
		})
		secure := httptest.NewUnstartedServer(handler)
		secure.TLS = &tls.Config{GetConfigForClient: sites.configFor, MinVersion: tls.VersionTLS10}
		secure.EnableHTTP2 = true
		secure.Config.ErrorLog = log.New(io.Discard, "", 0) // refused handshakes are the point of some tests
		secure.StartTLS()
		plain := httptest.NewServer(handler)
		sites.tlsLn, sites.plainLn = secure.Listener, plain.Listener
		sites.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			target := plain.Listener.Addr().String()
			if strings.HasSuffix(address, ":443") {
				target = secure.Listener.Addr().String()
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", target)
		}
		serveRegistries(sites)
	})
}

// serve sets the handler for a host.
func (s *testSites) serve(host string, handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[host] = handler
}

// present sets the TLS settings a host's handshakes get; nil goes back to a valid
// certificate for the name.
func (s *testSites) present(host string, config *tls.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[host] = config
}

func (s *testSites) configFor(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	s.mu.Lock()
	config := s.configs[hello.ServerName]
	s.mu.Unlock()
	if config != nil {
		return config, nil
	}
	name := hello.ServerName
	if name == "" {
		name = "unnamed.test"
	}
	leaf := s.issue(name, time.Now().Add(-time.Hour), time.Now().Add(90*24*time.Hour), nil)
	return &tls.Config{Certificates: []tls.Certificate{leaf}, MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}}, nil
}

func newTestCA() (*x509.Certificate, *ecdsa.PrivateKey) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "India API Test Root"},
		NotBefore: time.Now().Add(-24 * time.Hour), NotAfter: time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	ca, _ := x509.ParseCertificate(der)
	return ca, key
}

var testSerial = struct {
	sync.Mutex
	next int64
}{next: 100}

// issue makes a certificate for name, valid between from and until, signed by parent
// (the test root when nil). The result carries the chain up to, not including, the root.
func (s *testSites) issue(name string, from, until time.Time, parent *tls.Certificate) tls.Certificate {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	testSerial.Lock()
	testSerial.next++
	serial := testSerial.next
	testSerial.Unlock()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		NotBefore: from, NotAfter: until, DNSNames: []string{name},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	issuer, issuerKey := s.ca, any(s.caKey)
	var chain [][]byte
	if parent != nil {
		issuer, _ = x509.ParseCertificate(parent.Certificate[0])
		issuerKey = parent.PrivateKey
		chain = parent.Certificate
	}
	der, err := x509.CreateCertificate(rand.Reader, template, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		panic(err)
	}
	return tls.Certificate{Certificate: append([][]byte{der}, chain...), PrivateKey: key}
}

// intermediate makes a CA certificate under the test root.
func (s *testSites) intermediate(name string) tls.Certificate {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(50), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, s.ca, &key.PublicKey, s.caKey)
	if err != nil {
		panic(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// selfSigned makes a certificate that signs itself.
func selfSigned(name string) tls.Certificate {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(30 * 24 * time.Hour),
	}
	der, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// jsonSite answers every request with the same JSON document.
func jsonSite(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	})
}
