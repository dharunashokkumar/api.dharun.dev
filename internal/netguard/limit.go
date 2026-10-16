package netguard

import (
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

// TargetLimit counts requests per target across every caller, so the API cannot be
// used to send one site more than a few checks a minute however many callers ask.
type TargetLimit struct {
	PerMinute int
	mu        sync.Mutex
	window    time.Time
	counts    map[string]int
	now       func() time.Time
}

// Allow counts one request to host and reports whether it is within the limit.
func (l *TargetLimit) Allow(host string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now
	if l.now != nil {
		now = l.now
	}
	minute := now().Truncate(time.Minute)
	if !minute.Equal(l.window) || l.counts == nil {
		l.window, l.counts = minute, map[string]int{}
	}
	key := TargetKey(host)
	if l.counts[key] >= l.PerMinute {
		return false
	}
	l.counts[key]++
	return true
}

// TargetKey is what a host is counted under: the registered domain for a name (every
// subdomain of example.co.in counts as example.co.in), the /24 for an IPv4 address.
func TargetKey(host string) string {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.Is4() {
			prefix, _ := addr.Prefix(24)
			return prefix.String()
		}
		prefix, _ := addr.Prefix(48)
		return prefix.String()
	}
	if domain, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return domain
	}
	return host
}
