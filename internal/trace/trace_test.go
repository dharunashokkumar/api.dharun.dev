package trace

import (
	"context"
	"errors"
	"net/netip"
	"runtime"
	"testing"
	"time"
)

func addr(s string) netip.Addr { return netip.MustParseAddr(s) }

func TestSummarise(t *testing.T) {
	ms := time.Millisecond
	replies := []reply{
		{ttl: 1, from: addr("10.0.0.1"), rtt: 1 * ms},
		{ttl: 1, from: addr("10.0.0.1"), rtt: 3 * ms},
		{ttl: 2, from: addr("49.44.1.1"), rtt: 10 * ms},
		{ttl: 2, from: addr("49.44.1.2"), rtt: 12 * ms},
		{ttl: 2, from: addr("49.44.1.1"), rtt: 14 * ms},
		// hop 3 never answers
		{ttl: 4, from: addr("1.1.1.1"), rtt: 20 * ms, destination: true},
		{ttl: 4, from: addr("1.1.1.1"), rtt: 22 * ms, destination: true},
		// probes past the target are answered by the target too
		{ttl: 5, from: addr("1.1.1.1"), rtt: 21 * ms, destination: true},
	}
	result := summarise(replies, map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 1}, 30)
	if !result.Reached || len(result.Hops) != 4 {
		t.Fatalf("reached %v with %d hops", result.Reached, len(result.Hops))
	}
	first, second, silent, last := result.Hops[0], result.Hops[1], result.Hops[2], result.Hops[3]
	if first.Loss() != 33.3 || first.Best != 1*ms || first.Worst != 3*ms || first.Average != 2*ms || first.StdDev != 1*ms {
		t.Errorf("hop 1: %+v, loss %v", first, first.Loss())
	}
	if second.Address != addr("49.44.1.1") || len(second.Others) != 1 || second.Others[0] != addr("49.44.1.2") || second.Loss() != 0 {
		t.Errorf("hop 2: %+v", second)
	}
	if silent.Received != 0 || silent.Loss() != 100 || silent.Address.IsValid() {
		t.Errorf("hop 3: %+v", silent)
	}
	if !last.Destination || last.Address != addr("1.1.1.1") {
		t.Errorf("hop 4: %+v", last)
	}

	// Not reached: the hops stop one past the last router that answered.
	lost := summarise(replies[:5], map[int]int{1: 3, 2: 3, 3: 3, 4: 3}, 30)
	if lost.Reached || len(lost.Hops) != 3 {
		t.Errorf("unreached trace: reached %v, %d hops", lost.Reached, len(lost.Hops))
	}
}

// TestTraceLoopback sends real probes to the machine itself. It runs on Linux where the
// system allows ICMP datagram sockets, as systemd and Docker set it up by default.
func TestTraceLoopback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := Run(ctx, Options{Target: addr("127.0.0.1"), Rounds: 3, Interval: 100 * time.Millisecond, Wait: 300 * time.Millisecond})
	if runtime.GOOS != "linux" {
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("err = %v", err)
		}
		return
	}
	if errors.Is(err, ErrNotPermitted) {
		t.Skip("this system does not allow unprivileged ICMP sockets")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reached || len(result.Hops) != 1 || result.Hops[0].Received != 3 || result.Hops[0].Address != addr("127.0.0.1") {
		t.Fatalf("loopback: %+v", result)
	}
}
