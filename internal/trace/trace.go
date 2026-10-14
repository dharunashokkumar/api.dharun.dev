// Package trace runs an MTR-style traceroute: echo requests with rising hop limits, sent
// in several rounds, with loss and round-trip times per hop.
//
// It needs no privileges. On Linux it uses ICMP datagram ("ping") sockets, which any
// user in net.ipv4.ping_group_range may open (systemd and Docker allow all groups), and
// reads the routers' Time Exceeded messages from the socket's error queue (IP_RECVERR),
// the way tracepath does. No raw socket, so no CAP_NET_RAW. Other systems answer
// ErrUnsupported.
package trace

import (
	"errors"
	"math"
	"net/netip"
	"sort"
	"time"
)

// Options says what to trace.
type Options struct {
	Target   netip.Addr
	MaxHops  int           // highest hop limit tried, default 30
	Rounds   int           // probes per hop, default 5
	Interval time.Duration // between rounds, default 1 s
	Wait     time.Duration // for late replies after the last round, default 2 s
}

// ErrUnsupported means this system has no unprivileged way to trace.
var ErrUnsupported = errors.New("traceroute is only available on Linux")

// ErrNotPermitted means the system refuses ICMP datagram sockets to this user: its
// net.ipv4.ping_group_range does not include the process's group.
var ErrNotPermitted = errors.New("the system does not allow unprivileged ICMP sockets (net.ipv4.ping_group_range)")

// reply is one answer to one probe.
type reply struct {
	ttl         int
	from        netip.Addr
	rtt         time.Duration
	destination bool // an echo reply from the target, not a router
	unreachable bool // a router said the target cannot be reached
}

// Hop is what came back for one hop limit over all rounds.
type Hop struct {
	TTL int
	// Address is the address that answered most often; Others any further addresses
	// (routers that balance traffic over several paths answer from several).
	Address     netip.Addr
	Others      []netip.Addr
	Sent        int
	Received    int
	Best, Worst time.Duration
	Average     time.Duration
	StdDev      time.Duration
	Destination bool
	Unreachable bool
}

// Loss is the share of probes at this hop that got no answer, in percent.
func (h Hop) Loss() float64 {
	if h.Sent == 0 {
		return 0
	}
	return math.Round(float64(h.Sent-h.Received)/float64(h.Sent)*1000) / 10
}

// Result is a whole trace.
type Result struct {
	Hops    []Hop
	Reached bool
}

func (o *Options) defaults() {
	if o.MaxHops <= 0 {
		o.MaxHops = 30
	}
	if o.Rounds <= 0 {
		o.Rounds = 5
	}
	if o.Interval <= 0 {
		o.Interval = time.Second
	}
	if o.Wait <= 0 {
		o.Wait = 2 * time.Second
	}
}

// summarise turns the replies into hops, up to the first one where the target answered.
func summarise(replies []reply, sent map[int]int, maxHops int) Result {
	last := maxHops
	reached := false
	for _, r := range replies {
		if (r.destination || r.unreachable) && r.ttl <= last {
			last, reached = r.ttl, r.destination
		}
	}
	// Past the last router that answered, nothing more is known; trailing silent hops
	// are left out unless the target itself was reached.
	if !reached {
		highest := 0
		for _, r := range replies {
			highest = max(highest, r.ttl)
		}
		last = min(last, highest+1, maxHops)
	}
	var result Result
	result.Reached = reached
	for ttl := 1; ttl <= last; ttl++ {
		hop := Hop{TTL: ttl, Sent: sent[ttl]}
		counts := map[netip.Addr]int{}
		var times []float64
		for _, r := range replies {
			if r.ttl != ttl {
				continue
			}
			counts[r.from]++
			hop.Received++
			times = append(times, float64(r.rtt))
			hop.Destination = hop.Destination || r.destination
			hop.Unreachable = hop.Unreachable || r.unreachable
		}
		hop.Received = min(hop.Received, hop.Sent)
		var addresses []netip.Addr
		for address := range counts {
			addresses = append(addresses, address)
		}
		sort.Slice(addresses, func(a, b int) bool {
			if counts[addresses[a]] != counts[addresses[b]] {
				return counts[addresses[a]] > counts[addresses[b]]
			}
			return addresses[a].Less(addresses[b])
		})
		if len(addresses) > 0 {
			hop.Address, hop.Others = addresses[0], addresses[1:]
		}
		if len(times) > 0 {
			sort.Float64s(times)
			sum := 0.0
			for _, t := range times {
				sum += t
			}
			mean := sum / float64(len(times))
			variance := 0.0
			for _, t := range times {
				variance += (t - mean) * (t - mean)
			}
			hop.Best, hop.Worst = time.Duration(times[0]), time.Duration(times[len(times)-1])
			hop.Average, hop.StdDev = time.Duration(mean), time.Duration(math.Sqrt(variance/float64(len(times))))
		}
		result.Hops = append(result.Hops, hop)
	}
	return result
}
