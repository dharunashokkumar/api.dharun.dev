package api

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
)

// An HTTP probe from India: one GET of a URL, timed step by step (DNS, connect, TLS,
// first byte, download), with the redirects, the response headers and which CDN edge
// answered. The body is read to time it and then thrown away: the probe never returns
// what a site sent, so it cannot be used as a proxy.

const (
	probeRedirects = 5
	probeBodyLimit = 1 << 20
	probeHeaderCap = 16 << 10
)

type probeTimings struct {
	DNS       float64  `json:"dnsMs"`
	Connect   float64  `json:"connectMs"`
	TLS       *float64 `json:"tlsMs"`
	FirstByte float64  `json:"firstByteMs"`
	Download  float64  `json:"downloadMs"`
	Total     float64  `json:"totalMs"`
}

type probeHop struct {
	URL      string       `json:"url"`
	Status   int          `json:"status"`
	Location string       `json:"location,omitempty"`
	Address  string       `json:"address"`
	Timings  probeTimings `json:"timings"`
}

type probeCDN struct {
	Name string  `json:"name"`
	Edge *string `json:"edge"`
	City *string `json:"city"`
}

type probeBody struct {
	Bytes     int     `json:"bytes"`
	Truncated bool    `json:"truncated"`
	SHA256    *string `json:"sha256"`
}

type probeTLS struct {
	Version string `json:"version"`
	Cipher  string `json:"cipher"`
	ALPN    string `json:"alpn"`
}

type probeRound struct {
	Status  int     `json:"status"`
	TotalMs float64 `json:"totalMs"`
}

type probeView struct {
	URL              string             `json:"url"`
	FinalURL         string             `json:"finalUrl"`
	Status           int                `json:"status"`
	HTTPVersion      string             `json:"httpVersion"`
	Address          string             `json:"address"`
	Timings          probeTimings       `json:"timings"`
	Redirects        []probeHop         `json:"redirects"`
	TLS              *probeTLS          `json:"tls"`
	CDN              *probeCDN          `json:"cdn"`
	Headers          map[string]string  `json:"headers"`
	HeadersTruncated bool               `json:"headersTruncated,omitempty"`
	Body             probeBody          `json:"body"`
	Rounds           []probeRound       `json:"rounds,omitempty"`
	Summary          map[string]float64 `json:"summary,omitempty"`
}

const probeNote = "Fetched once per round from a server in India with GET, following up to five redirects. The body is read and counted, never returned. " +
	"Cookie values are not shown. Times are in milliseconds; DNS is the lookup through a public resolver."

var errProbeRedirect = errors.New("redirect to an address that is not http or https")

func millis(d time.Duration) float64 { return math.Round(float64(d)/1e5) / 10 }

// probeURL reads the url parameter: http or https, the usual port, no user name or
// password. A bare host is taken as https.
func probeURL(raw string) (*url.URL, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 {
		return nil, "", errBadName
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Hostname() == "" {
		return nil, "", errBadName
	}
	host, err := hostOf(parsed.Hostname())
	if err != nil {
		return nil, "", err
	}
	if port := parsed.Port(); port != "" && !(parsed.Scheme == "http" && port == "80") && !(parsed.Scheme == "https" && port == "443") {
		return nil, "", netguard.ErrPort
	}
	parsed.Fragment, parsed.RawFragment = "", ""
	if strings.Contains(host, ":") {
		parsed.Host = "[" + host + "]"
	} else {
		parsed.Host = host
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed, host, nil
}

func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	target, host, err := probeURL(query.Get("url"))
	switch {
	case errors.Is(err, netguard.ErrPort):
		s.failNetwork(w, r, "", err)
		return
	case err != nil:
		s.badRequest(w, r, "Give url: an http or https address such as https://example.co.in/, without a user name or password.")
		return
	}
	info(r).subject = host
	rounds := 1
	if text := strings.TrimSpace(query.Get("rounds")); text != "" {
		value, err := strconv.Atoi(text)
		if err != nil || value < 1 || value > 5 {
			s.badRequest(w, r, "rounds is from 1 to 5.")
			return
		}
		rounds = value
	}
	release, ok := s.startCheck(w, r, host)
	if !ok {
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	client := s.guard(80, 443).Client(netguard.ClientOptions{Timeout: 12 * time.Second, Roots: s.extras.network.roots, UserAgent: probeAgent})
	var view probeView
	var totals []float64
	for round := 0; round < rounds; round++ {
		result, err := s.probeOnce(ctx, client, target)
		if err != nil {
			if round == 0 && errors.Is(err, errProbeRedirect) {
				s.fail(w, r, http.StatusUnprocessableEntity, "bad_redirect", "The site redirects to an address that is not http or https.")
				return
			}
			if round == 0 {
				s.failNetwork(w, r, host, err)
				return
			}
			break // later rounds only add timings
		}
		totals = append(totals, result.total)
		if round == 0 {
			view = result.view
		}
		view.Rounds = append(view.Rounds, probeRound{Status: result.view.Status, TotalMs: result.total})
	}
	view.URL = target.String()
	if rounds == 1 {
		view.Rounds = nil
	} else {
		sort.Float64s(totals)
		view.Summary = map[string]float64{"minMs": totals[0], "medianMs": totals[len(totals)/2], "maxMs": totals[len(totals)-1]}
	}
	// The address of the URL may carry a token, so the answer is the caller's alone.
	s.ok(w, r, view, meta{Source: s.source("airports"), Note: probeNote}, "private, no-store")
}

type probeResult struct {
	view  probeView
	total float64
}

// probeOnce follows the URL to its end once, timing every request.
func (s *Server) probeOnce(ctx context.Context, client *http.Client, start *url.URL) (probeResult, error) {
	var result probeResult
	view := &result.view
	view.Redirects = []probeHop{}
	began := time.Now()
	current := start
	for hop := 0; ; hop++ {
		// The callbacks run on the transport's goroutines (HTTP/2 writes and reads on
		// different ones), so everything they set is behind mu.
		var mu sync.Mutex
		var timings probeTimings
		var address netip.Addr
		var sent, tlsStart, firstByteAt time.Time
		var tlsTook time.Duration
		var state *tls.ConnectionState
		hopCtx := netguard.WithTrace(ctx, &netguard.Trace{
			Resolved: func(addr netip.Addr, took time.Duration) {
				mu.Lock()
				address, timings.DNS = addr, millis(took)
				mu.Unlock()
			},
			Connected: func(took time.Duration) {
				mu.Lock()
				timings.Connect = millis(took)
				mu.Unlock()
			},
		})
		hopCtx = httptrace.WithClientTrace(hopCtx, &httptrace.ClientTrace{
			TLSHandshakeStart: func() {
				mu.Lock()
				tlsStart = time.Now()
				mu.Unlock()
			},
			TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
				mu.Lock()
				if err == nil {
					tlsTook, state = time.Since(tlsStart), &cs
				}
				mu.Unlock()
			},
			WroteRequest: func(httptrace.WroteRequestInfo) {
				mu.Lock()
				sent = time.Now()
				mu.Unlock()
			},
			GotFirstResponseByte: func() {
				mu.Lock()
				firstByteAt = time.Now()
				mu.Unlock()
			},
		})
		req, err := http.NewRequestWithContext(hopCtx, http.MethodGet, current.String(), nil)
		if err != nil {
			return result, err
		}
		req.Header.Set("Accept", "*/*")
		// Asked for by hand, so the body is counted as sent rather than unpacked.
		req.Header.Set("Accept-Encoding", "gzip, br")
		requested := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return result, err
		}
		hasher := sha256.New()
		read, _ := io.Copy(hasher, io.LimitReader(resp.Body, probeBodyLimit))
		more, _ := resp.Body.Read(make([]byte, 1))
		resp.Body.Close()
		downloaded := time.Now()
		mu.Lock()
		if sent.IsZero() || sent.After(firstByteAt) {
			sent = firstByteAt // an answer before the request was fully written
		}
		timings.FirstByte = millis(firstByteAt.Sub(sent))
		if state != nil {
			took := millis(tlsTook)
			timings.TLS = &took
		}
		timings.Total = millis(downloaded.Sub(requested))
		timings.Download = math.Max(0, math.Round((timings.Total-timings.DNS-timings.Connect-timingValue(timings.TLS)-timings.FirstByte)*10)/10)
		// Copies taken under the lock: the rest of the hop reads only these.
		hopTimings, hopAddress, hopState := timings, address.String(), state
		mu.Unlock()

		location := resp.Header.Get("Location")
		next, _ := current.Parse(location)
		if resp.StatusCode >= 300 && resp.StatusCode < 400 && location != "" && next != nil && hop < probeRedirects {
			if next.Scheme != "http" && next.Scheme != "https" {
				return result, errProbeRedirect
			}
			view.Redirects = append(view.Redirects, probeHop{URL: current.String(), Status: resp.StatusCode, Location: next.String(), Address: hopAddress, Timings: hopTimings})
			current = next
			continue
		}
		view.FinalURL = current.String()
		view.Status = resp.StatusCode
		view.HTTPVersion = resp.Proto
		view.Address = hopAddress
		view.Timings = hopTimings
		view.Headers, view.HeadersTruncated = probeHeaders(resp.Header)
		view.Body = probeBody{Bytes: int(read), Truncated: more > 0}
		if more == 0 {
			sum := hex.EncodeToString(hasher.Sum(nil))
			view.Body.SHA256 = &sum
		}
		if hopState != nil {
			view.TLS = &probeTLS{Version: tls.VersionName(hopState.Version), Cipher: tls.CipherSuiteName(hopState.CipherSuite), ALPN: hopState.NegotiatedProtocol}
		}
		view.CDN = s.detectCDN(resp.Header)
		result.total = millis(time.Since(began))
		return result, nil
	}
}

func timingValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

// probeHeaders lists the response headers in lower case, cookies without their values,
// up to probeHeaderCap bytes in all.
func probeHeaders(header http.Header) (map[string]string, bool) {
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	sort.Strings(names)
	out, size, truncated := map[string]string{}, 0, false
	for _, name := range names {
		values := header.Values(name)
		if strings.EqualFold(name, "Set-Cookie") {
			for i, cookie := range values {
				first, rest, _ := strings.Cut(cookie, ";")
				cookieName, _, _ := strings.Cut(first, "=")
				values[i] = strings.TrimSpace(cookieName) + "=…"
				if rest != "" {
					values[i] += ";" + rest
				}
			}
		}
		value := strings.Join(values, ", ")
		if len(value) > 1024 {
			value = value[:1024] + "…"
		}
		if size+len(name)+len(value) > probeHeaderCap {
			truncated = true
			continue
		}
		size += len(name) + len(value)
		out[strings.ToLower(name)] = value
	}
	return out, truncated
}

// detectCDN names the network that answered, and its edge, from the headers each one
// adds. Edge codes that are airport codes are named with their city.
func (s *Server) detectCDN(header http.Header) *probeCDN {
	get := func(name string) string { return strings.TrimSpace(header.Get(name)) }
	edge := func(code string) *string {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			return nil
		}
		return &code
	}
	var cdn *probeCDN
	server := strings.ToLower(get("Server"))
	switch {
	case get("Cf-Ray") != "":
		ray := get("Cf-Ray")
		cdn = &probeCDN{Name: "Cloudflare", Edge: edge(ray[strings.LastIndex(ray, "-")+1:])}
	case get("X-Amz-Cf-Pop") != "":
		pop := get("X-Amz-Cf-Pop")
		cdn = &probeCDN{Name: "Amazon CloudFront", Edge: edge(pop[:min(3, len(pop))])}
	case get("X-Vercel-Id") != "":
		id := get("X-Vercel-Id")
		cdn = &probeCDN{Name: "Vercel", Edge: edge(id[:min(3, len(id))])}
	case strings.Contains(get("X-Served-By"), "cache-"):
		served := strings.Split(get("X-Served-By"), ",")
		last := strings.TrimSpace(served[len(served)-1])
		cdn = &probeCDN{Name: "Fastly", Edge: edge(last[strings.LastIndex(last, "-")+1:])}
	case get("X-Nf-Request-Id") != "" || server == "netlify":
		cdn = &probeCDN{Name: "Netlify"}
	case strings.HasPrefix(server, "bunnycdn"):
		cdn = &probeCDN{Name: "Bunny CDN"}
	case get("X-Azure-Ref") != "":
		cdn = &probeCDN{Name: "Azure Front Door"}
	case strings.Contains(server, "akamai") || get("X-Akamai-Transformed") != "":
		cdn = &probeCDN{Name: "Akamai"}
	case server == "cloudflare":
		cdn = &probeCDN{Name: "Cloudflare"}
	case server == "gws" || server == "esf" || strings.Contains(server, "google frontend") || strings.Contains(strings.ToLower(get("Via")), "google"):
		cdn = &probeCDN{Name: "Google"}
	}
	if cdn == nil || cdn.Edge == nil || len(*cdn.Edge) != 3 {
		return cdn
	}
	if set := s.data("airports"); set != nil {
		var city string
		if set.row(`SELECT municipality FROM airports WHERE iata = ? ORDER BY scheduled DESC LIMIT 1`, *cdn.Edge).Scan(&city) == nil && city != "" {
			cdn.City = &city
		}
	}
	return cdn
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "probe_url_from_india",
		Description: "Fetch a URL from a server in India and time it: DNS, connect, TLS, time to first byte and download, the HTTP status and version, every redirect, the response headers (cookie values hidden), the body's size and hash (never the body), and which CDN and edge city answered. Shows how fast a site is for visitors in India.",
		InputSchema: schema([]string{"url"}, map[string]any{
			"url":    text("http or https address. A bare host is taken as https."),
			"rounds": map[string]any{"type": "integer", "description": "Times to fetch it, 1 to 5, for a median. Default 1."},
		}),
		path: func(args map[string]any) (string, error) {
			_, err := required(args, "url")
			return "/v1/probe" + query(args, map[string]string{"url": "url", "rounds": "rounds"}), err
		},
	})
}
