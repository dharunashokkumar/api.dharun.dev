package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A TLS check of a host on port 443: one ordinary handshake, read as a browser would
// see it, plus one handshake per protocol version and one offering only weak ciphers, to
// tell what else the server accepts. Certificates are verified here rather than by the
// handshake, so a broken chain is reported instead of ending the check.

type tlsProblem struct {
	Level   string `json:"level"` // error or warning
	Code    string `json:"code"`
	Message string `json:"message"`
}

type tlsCertificate struct {
	Subject      string   `json:"subject"`
	Issuer       string   `json:"issuer"`
	Names        []string `json:"names,omitempty"`
	NotBefore    string   `json:"notBefore"`
	NotAfter     string   `json:"notAfter"`
	DaysLeft     *int     `json:"daysLeft,omitempty"`
	Key          string   `json:"key"`
	Signature    string   `json:"signature"`
	Serial       string   `json:"serial"`
	SHA256       string   `json:"sha256"`
	CA           bool     `json:"ca"`
	EmbeddedSCTs *bool    `json:"embeddedScts,omitempty"`
}

type tlsSession struct {
	Version     string `json:"version"`
	Cipher      string `json:"cipher"`
	ALPN        string `json:"alpn"`
	OCSPStapled bool   `json:"ocspStapled"`
	StapledSCTs int    `json:"stapledScts"`
}

type tlsView struct {
	Host        string           `json:"host"`
	Address     string           `json:"address"`
	Port        int              `json:"port"`
	Valid       bool             `json:"valid"`
	Session     tlsSession       `json:"tls"`
	Versions    map[string]bool  `json:"versions"`
	WeakCipher  *string          `json:"weakCipher"`
	Certificate tlsCertificate   `json:"certificate"`
	Chain       []tlsCertificate `json:"chain"`
	Verified    bool             `json:"verified"`
	VerifyError *string          `json:"verifyError"`
	Problems    []tlsProblem     `json:"problems"`
}

var tlsVersions = []struct {
	Name    string
	Version uint16
}{{"TLS 1.0", tls.VersionTLS10}, {"TLS 1.1", tls.VersionTLS11}, {"TLS 1.2", tls.VersionTLS12}, {"TLS 1.3", tls.VersionTLS13}}

// The extension that carries signed certificate timestamps inside a certificate.
var sctExtension = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 4, 2}

const tlsNote = "Checked from India on port 443 with Go's TLS. SSL 3 and export ciphers cannot be offered by it and are not tested. " +
	"Each version is tried with its own handshake, with every cipher Go knows. Trust is decided against the server's own list of root authorities."

// allCipherSuites offers everything Go can speak, weak suites included, so a version
// test does not fail only because a modern client would not offer the server's suites.
func allCipherSuites() []uint16 {
	var ids []uint16
	for _, suite := range append(tls.CipherSuites(), tls.InsecureCipherSuites()...) {
		ids = append(ids, suite.ID)
	}
	return ids
}

func insecureSuites() []uint16 {
	var ids []uint16
	for _, suite := range tls.InsecureCipherSuites() {
		for _, version := range suite.SupportedVersions {
			if version <= tls.VersionTLS12 {
				ids = append(ids, suite.ID)
				break
			}
		}
	}
	return ids
}

func (s *Server) handleTLS(w http.ResponseWriter, r *http.Request) {
	host, err := hostOf(r.PathValue("host"))
	if err != nil {
		s.badRequest(w, r, "Give a domain name or a public IPv4 address.")
		return
	}
	info(r).subject = host
	release, ok := s.startCheck(w, r, host)
	if !ok {
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	guard := s.guard(443)
	address, err := guard.Resolve(ctx, host)
	if err != nil {
		s.failNetwork(w, r, host, err)
		return
	}
	// Every handshake goes to the address just checked.
	target := net.JoinHostPort(address.String(), "443")
	serverName := host
	if _, err := netip.ParseAddr(host); err == nil {
		serverName = ""
	}
	handshake := func(config *tls.Config) (tls.ConnectionState, error) {
		config.ServerName = serverName
		config.InsecureSkipVerify = true // verified below, so problems can be reported
		conn, err := guard.DialContext(ctx, "tcp", target)
		if err != nil {
			return tls.ConnectionState{}, err
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(6 * time.Second))
		client := tls.Client(conn, config)
		if err := client.HandshakeContext(ctx); err != nil {
			return tls.ConnectionState{}, err
		}
		return client.ConnectionState(), nil
	}

	main, err := handshake(&tls.Config{MinVersion: tls.VersionTLS10, NextProtos: []string{"h2", "http/1.1"}})
	if err != nil && isTLSError(err) {
		// A server that only speaks old ciphers still gets a report: offer every one.
		main, err = handshake(&tls.Config{MinVersion: tls.VersionTLS10, CipherSuites: allCipherSuites(), NextProtos: []string{"h2", "http/1.1"}})
	}
	if err != nil {
		if isTLSError(err) {
			s.fail(w, r, http.StatusUnprocessableEntity, "tls_failed", host+" did not complete a TLS handshake on port 443 with any version Go offers.")
			return
		}
		s.failNetwork(w, r, host, err)
		return
	}

	view := tlsView{Host: host, Address: address.String(), Port: 443, Versions: map[string]bool{}, Problems: []tlsProblem{}, Chain: []tlsCertificate{}}
	view.Session = tlsSession{Version: tls.VersionName(main.Version), Cipher: tls.CipherSuiteName(main.CipherSuite), ALPN: main.NegotiatedProtocol,
		OCSPStapled: len(main.OCSPResponse) > 0, StapledSCTs: len(main.SignedCertificateTimestamps)}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, version := range tlsVersions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			config := &tls.Config{MinVersion: version.Version, MaxVersion: version.Version}
			if version.Version < tls.VersionTLS13 {
				config.CipherSuites = allCipherSuites()
			}
			_, err := handshake(config)
			mu.Lock()
			view.Versions[version.Name] = err == nil
			mu.Unlock()
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		state, err := handshake(&tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS12, CipherSuites: insecureSuites()})
		if err == nil {
			name := tls.CipherSuiteName(state.CipherSuite)
			mu.Lock()
			view.WeakCipher = &name
			mu.Unlock()
		}
	}()
	wg.Wait()
	// The ordinary handshake worked, so its version is supported whatever its own test said.
	view.Versions[view.Session.Version] = true

	now := time.Now()
	certs := main.PeerCertificates
	if len(certs) == 0 {
		s.fail(w, r, http.StatusUnprocessableEntity, "tls_failed", host+" sent no certificate.")
		return
	}
	for i, cert := range certs {
		item := describeCertificate(cert, now)
		if i > 0 {
			item.Names, item.DaysLeft, item.EmbeddedSCTs = nil, nil, nil
		}
		view.Chain = append(view.Chain, item)
	}
	view.Certificate = view.Chain[0]
	view.Certificate.Names = certificateNames(certs[0])

	intermediates := x509.NewCertPool()
	for _, cert := range certs[1:] {
		intermediates.AddCert(cert)
	}
	verifyName := host
	_, verifyErr := certs[0].Verify(x509.VerifyOptions{DNSName: verifyName, Roots: s.extras.network.roots, Intermediates: intermediates, CurrentTime: now})
	view.Verified = verifyErr == nil
	problem := func(level, code, message string) {
		view.Problems = append(view.Problems, tlsProblem{level, code, message})
	}
	if verifyErr != nil {
		text := verifyErr.Error()
		view.VerifyError = &text
	}
	leaf := certs[0]
	var hostnameErr x509.HostnameError
	var authorityErr x509.UnknownAuthorityError
	var invalidErr x509.CertificateInvalidError
	switch {
	case now.After(leaf.NotAfter):
		problem("error", "expired", "The certificate expired on "+leaf.NotAfter.UTC().Format("2 January 2006")+".")
	case now.Before(leaf.NotBefore):
		problem("error", "not_yet_valid", "The certificate is not valid until "+leaf.NotBefore.UTC().Format("2 January 2006")+".")
	case leaf.NotAfter.Sub(now) < 14*24*time.Hour:
		problem("warning", "expires_soon", "The certificate expires in under two weeks.")
	}
	switch {
	case errors.As(verifyErr, &hostnameErr):
		problem("error", "name_mismatch", "The certificate is not for "+host+"; it names "+joinNames(certificateNames(leaf))+".")
	case errors.As(verifyErr, &authorityErr):
		if len(certs) == 1 && leaf.Subject.String() == leaf.Issuer.String() {
			problem("error", "self_signed", "The certificate signs itself; browsers do not trust it.")
		} else {
			problem("error", "incomplete_chain", "The chain does not lead to a trusted root authority: an intermediate certificate is probably missing, or the authority is private.")
		}
	case errors.As(verifyErr, &invalidErr) && invalidErr.Reason == x509.Expired:
		// Already reported from the dates, unless it is a certificate further up.
		if !now.After(leaf.NotAfter) && !now.Before(leaf.NotBefore) {
			problem("error", "chain_expired", "A certificate in the chain has expired.")
		}
	case verifyErr != nil:
		problem("error", "untrusted", "The certificate could not be verified.")
	}
	for i := 0; i+1 < len(certs); i++ {
		if certs[i].Issuer.String() != certs[i+1].Subject.String() {
			problem("warning", "chain_order", "The chain is sent out of order or carries a certificate it does not need.")
			break
		}
	}
	for i, cert := range certs {
		if key, ok := cert.PublicKey.(*rsa.PublicKey); ok && key.N.BitLen() < 2048 {
			problem("error", "weak_key", "A certificate in the chain has an RSA key under 2048 bits.")
			break
		}
		if i < len(certs)-1 || cert.Subject.String() != cert.Issuer.String() {
			if cert.SignatureAlgorithm == x509.SHA1WithRSA || cert.SignatureAlgorithm == x509.ECDSAWithSHA1 {
				problem("error", "sha1_signature", "A certificate in the chain is signed with SHA-1.")
				break
			}
		}
	}
	if view.Versions["TLS 1.0"] || view.Versions["TLS 1.1"] {
		problem("warning", "old_versions", "TLS 1.0 or 1.1 is still accepted; both were retired in 2021 (RFC 8996).")
	}
	if !view.Versions["TLS 1.3"] {
		problem("warning", "no_tls13", "TLS 1.3 is not offered.")
	}
	if view.WeakCipher != nil {
		problem("warning", "weak_cipher", "A cipher Go marks as insecure is accepted (RC4, 3DES, CBC with SHA-256, or RSA key exchange without forward secrecy): "+*view.WeakCipher+".")
	}
	view.Valid = true
	for _, p := range view.Problems {
		if p.Level == "error" {
			view.Valid = false
		}
	}
	s.ok(w, r, view, meta{Note: tlsNote}, "public, max-age=60")
}

// isTLSError is true when the connection was made but the TLS handshake failed.
func isTLSError(err error) bool {
	var alert tls.AlertError
	var record tls.RecordHeaderError
	return errors.As(err, &alert) || errors.As(err, &record) || strings.Contains(err.Error(), "tls: ")
}

func joinNames(names []string) string {
	switch {
	case len(names) == 0:
		return "no host name"
	case len(names) > 4:
		return joinWords(names[:4]) + " and " + strconv.Itoa(len(names)-4) + " more"
	}
	return joinWords(names)
}

func certificateNames(cert *x509.Certificate) []string {
	names := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		names = append(names, ip.String())
	}
	return names
}

func describeCertificate(cert *x509.Certificate, now time.Time) tlsCertificate {
	sum := sha256.Sum256(cert.Raw)
	item := tlsCertificate{
		Subject: certName(cert.Subject.CommonName, cert.Subject.Organization), Issuer: certName(cert.Issuer.CommonName, cert.Issuer.Organization),
		NotBefore: cert.NotBefore.UTC().Format(time.RFC3339), NotAfter: cert.NotAfter.UTC().Format(time.RFC3339),
		Key: keyName(cert.PublicKey), Signature: cert.SignatureAlgorithm.String(), Serial: hex.EncodeToString(cert.SerialNumber.Bytes()),
		SHA256: hex.EncodeToString(sum[:]), CA: cert.IsCA,
	}
	days := int(cert.NotAfter.Sub(now).Hours() / 24)
	item.DaysLeft = &days
	embedded := false
	for _, extension := range cert.Extensions {
		if extension.Id.Equal(sctExtension) {
			embedded = true
		}
	}
	item.EmbeddedSCTs = &embedded
	return item
}

func certName(common string, organisation []string) string {
	if common != "" {
		return common
	}
	if len(organisation) > 0 {
		return organisation[0]
	}
	return ""
}

func keyName(key any) string {
	switch k := key.(type) {
	case *rsa.PublicKey:
		return "RSA " + strconv.Itoa(k.N.BitLen())
	case *ecdsa.PublicKey:
		return "ECDSA " + k.Curve.Params().Name
	case ed25519.PublicKey:
		return "Ed25519"
	}
	return "unknown"
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "check_tls",
		Description: "Check a website's TLS certificate and setup on port 443, from India: whether the certificate is valid for the name and trusted, days until it expires, the chain as sent, issuer and key, which TLS versions (1.0 to 1.3) are accepted, the cipher and HTTP/2 offered, weak ciphers, and a plain list of problems.",
		InputSchema: schema([]string{"host"}, map[string]any{
			"host": text("Domain name or public IPv4 address."),
		}),
		path: func(args map[string]any) (string, error) {
			host, err := required(args, "host")
			return "/v1/tls/" + url.PathEscape(host), err
		},
	})
}
