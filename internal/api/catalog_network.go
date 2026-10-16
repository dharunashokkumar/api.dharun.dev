package api

import "strconv"

// The network tools of round 4. They go in the network category after the traceroute.

var networkCatalog = []apiGroup{
	{
		ID:        "whois",
		Category:  "network",
		Title:     "Whois",
		Tagline:   "Who registered a domain, an IP network or an AS number, through RDAP.",
		Intro:     "RDAP is the registries' JSON replacement for the old whois protocol. Give a domain, an IP address or an AS number; IANA's bootstrap registry names the registry that answers for it (NIXI for .in and .co.in, Verisign for .com, APNIC for Indian IP addresses), and the registration is fetched from it on the spot. For a domain: the registrar and its abuse contact, when it was registered and when it expires, its status codes, nameservers and DNSSEC. For an IP network or AS number: the range, its name, the organisation that holds it, country and abuse contact. The contacts of the people behind a domain are never returned, and a network held by a person is not named. Domains under the Indian-script top-level domains (.भारत) cannot be looked up: their registry offers no RDAP. Counts as 10 requests against the rate limit.",
		SourceKey: "rdap",
		Endpoints: []endpoint{
			{
				Path: "/v1/whois/{query}", Summary: "Look up a registration", Live: true,
				Doc:     "A name below a registered domain (www.example.co.in) is looked up as that domain (example.co.in). daysToExpiry counts from now. An IP lookup is treated as private: not logged, not cached.",
				Example: "/v1/whois/google.in",
				Params: []param{
					{Name: "query", In: "path", Type: "string", Required: true, Doc: "A domain name (Indian-script names are accepted), an IPv4 or IPv6 address, or an AS number such as AS55836."},
				},
			},
		},
	},
	{
		ID:       "tls",
		Category: "network",
		Title:    "TLS check",
		Tagline:  "A website's certificate and TLS setup, checked from India.",
		Intro:    "Connects to a host on port 443 and reports what a browser would care about: whether the certificate is valid for the name and leads to a trusted root, how many days it has left, the chain as the server sends it with each certificate's issuer, key and signature, the TLS version, cipher and HTTP/2 negotiated, OCSP stapling and certificate transparency, and which of TLS 1.0, 1.1, 1.2 and 1.3 the server accepts and whether it takes a weak cipher. Problems come as a plain list, errors and warnings, rather than a grade. Only public addresses are checked, and only port 443. Counts as 20 requests against the rate limit.",
		Endpoints: []endpoint{
			{
				Path: "/v1/tls/{host}", Summary: "Check a host's TLS", Live: true,
				Doc:     "valid is false when any problem is an error: expired, not yet valid, name_mismatch, self_signed, incomplete_chain, untrusted, weak_key or sha1_signature. Warnings (expires_soon, old_versions, no_tls13, weak_cipher, chain_order) leave it true. Each version is tried with its own handshake offering every cipher Go supports; SSL 3 is not tested.",
				Example: "/v1/tls/dharun.dev",
				Params: []param{
					{Name: "host", In: "path", Type: "string", Required: true, Doc: "Domain name or public IPv4 address."},
				},
			},
		},
	},
	{
		ID:       "ipv6",
		Category: "network",
		Title:    "IPv6 check",
		Tagline:  "Is a domain ready for IPv6: website, nameservers and mail.",
		Intro:    "Looks up IPv6 (AAAA) addresses for a domain and for www, for each of its nameservers and for each mail server, and says plainly what is missing. Addresses that are not ordinary public IPv6 (6to4, Teredo, NAT64, private) are pointed out. This is a DNS check only: the server has no IPv6 connection, so it does not try to connect over IPv6. Counts as 10 requests against the rate limit.",
		Endpoints: []endpoint{
			{
				Path: "/v1/ipv6/{domain}", Summary: "Check a domain's IPv6", Live: true,
				Doc:     "ready is true when the site (the name, or www when the name has no address) has a public IPv6 address, at least one nameserver does, and every mail server does (a domain without mail servers passes that part). website.network names the network of the first IPv6 address, from DB-IP.",
				Example: "/v1/ipv6/dharun.dev",
				Params: []param{
					{Name: "domain", In: "path", Type: "string", Required: true, Doc: "Domain name."},
				},
			},
		},
	},
	{
		ID:       "probe",
		Category: "network",
		Title:    "HTTP probe from India",
		Tagline:  "How fast a URL answers from India: DNS, connect, TLS, first byte and the CDN edge.",
		Intro:    "Fetches a URL from a server in India and times each step: DNS lookup, TCP connect, TLS handshake, time to first byte and download. It follows up to five redirects, lists them, and returns the status, HTTP version, response headers and which CDN answered and from which city (Cloudflare, CloudFront, Fastly, Vercel and others), so you can see whether Indian visitors are served from Mumbai or Chennai or sent abroad. The body is read, counted and hashed, never returned, and cookie values are hidden. Only http on port 80 and https on 443, only public addresses, GET only. Requests are sent with the user agent " + probeAgent + "; a site that does not want them can refuse that user agent. Each site gets at most " + strconv.Itoa(targetPerMinute) + " checks a minute from all callers together. Counts as 20 requests against the rate limit.",
		Endpoints: []endpoint{
			{
				Path: "/v1/probe", Summary: "Time a URL from India", Live: true, Private: true,
				Doc:     "With rounds above 1 the URL is fetched that many times and summary gives the fastest, median and slowest total; the other fields describe the first round. A redirect into private address space, to another port or to anything other than http or https is refused. The URL may carry a token, so the request is not logged and the answer is not cached.",
				Example: "/v1/probe?url=https://dharun.dev",
				Params: []param{
					{Name: "url", In: "query", Type: "string", Required: true, Doc: "An http or https address. A bare host name is taken as https."},
					{Name: "rounds", In: "query", Type: "integer", Doc: "Times to fetch it, 1 to 5. Default 1."},
				},
			},
		},
	},
	{
		ID:       "icon",
		Category: "network",
		Title:    "Website icons",
		Tagline:  "The icon of any website as a square PNG, for lists of links and bookmarks.",
		Intro:    "Give a domain and get the icon the site names for itself: its Apple touch icon, the icons in its web app manifest, its favicon. The best one is fetched once, drawn again here as a square PNG of the size asked and kept in memory, so the next request is served without asking the site. SVG icons are not used. A site without a usable icon, or one that cannot be reached, gets an initials tile marked with the X-Logo-Fallback: monogram header, so an <img> tag always shows something. For listed Indian companies, the company logos API has better artwork.",
		Endpoints: []endpoint{
			{
				Path: "/v1/icon/{domain}", Summary: "Icon for a website", Image: true, Live: true,
				Doc:     "The X-Icon-Source header names the file the icon was drawn from, the first time it is fetched. An icon already fetched counts as 1 request against the rate limit; one that has to be fetched counts as 20.",
				Example: "/v1/icon/github.com",
				Params: []param{
					{Name: "domain", In: "path", Type: "string", Required: true, Doc: "Domain name; a .png suffix is allowed."},
					{Name: "size", In: "query", Type: "integer", Doc: "Width and height in pixels, " + strconv.Itoa(iconMinSize) + " to " + strconv.Itoa(iconMaxSize) + ". Default " + strconv.Itoa(iconDefaultSize) + "."},
				},
			},
		},
	},
}

// withNetworkTools puts the round 4 groups after the traceroute.
func withNetworkTools(base []apiGroup) []apiGroup {
	out := make([]apiGroup, 0, len(base)+len(networkCatalog))
	for _, group := range base {
		out = append(out, group)
		if group.ID == "mtr" {
			out = append(out, networkCatalog...)
		}
	}
	return out
}
