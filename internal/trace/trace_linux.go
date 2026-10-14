//go:build linux

package trace

import (
	"context"
	"encoding/binary"
	"errors"
	"net/netip"
	"time"

	"golang.org/x/sys/unix"
)

// Run traces the route to opts.Target. It returns when the last round's replies are in
// or the context ends, whichever is first.
func Run(ctx context.Context, opts Options) (Result, error) {
	opts.defaults()
	v6 := opts.Target.Is6()
	family, protocol, level, recvErr, hopLimit := unix.AF_INET, unix.IPPROTO_ICMP, unix.SOL_IP, unix.IP_RECVERR, unix.IP_TTL
	echoRequest, echoReply := byte(8), byte(0)
	if v6 {
		family, protocol, level, recvErr, hopLimit = unix.AF_INET6, unix.IPPROTO_ICMPV6, unix.SOL_IPV6, unix.IPV6_RECVERR, unix.IPV6_UNICAST_HOPS
		echoRequest, echoReply = 128, 129
	}
	fd, err := unix.Socket(family, unix.SOCK_DGRAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, protocol)
	if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) || errors.Is(err, unix.EPROTONOSUPPORT) {
		return Result{}, ErrNotPermitted
	}
	if err != nil {
		return Result{}, err
	}
	defer unix.Close(fd)
	if err := unix.SetsockoptInt(fd, level, recvErr, 1); err != nil {
		return Result{}, err
	}
	var to unix.Sockaddr
	if v6 {
		to = &unix.SockaddrInet6{Addr: opts.Target.As16()}
	} else {
		to = &unix.SockaddrInet4{Addr: opts.Target.As4()}
	}

	// A probe's sequence number says which round and hop limit it was sent with. The
	// kernel sets the identifier to the socket's own, and hands this socket only the
	// answers to its own probes, so traces running at once do not mix.
	sentAt := map[uint16]time.Time{}
	ttlOf := map[uint16]int{}
	answered := map[uint16]bool{}
	sent := map[int]int{}
	var replies []reply
	limit := opts.MaxHops

	send := func(round int) {
		for ttl := 1; ttl <= limit; ttl++ {
			seq := uint16(round<<8 | ttl)
			packet := make([]byte, 16)
			packet[0] = echoRequest
			binary.BigEndian.PutUint16(packet[6:], seq)
			if err := unix.SetsockoptInt(fd, level, hopLimit, ttl); err != nil {
				continue
			}
			sentAt[seq], ttlOf[seq] = time.Now(), ttl
			// A send can fail with the error an earlier probe caused (EHOSTUNREACH and
			// the like); that probe is counted as sent and simply not answered.
			unix.Sendto(fd, packet, 0, to)
			sent[ttl]++
		}
	}
	record := func(seq uint16, from netip.Addr, destination, unreachable bool) {
		at, known := sentAt[seq]
		if !known || answered[seq] {
			return
		}
		answered[seq] = true
		replies = append(replies, reply{ttl: ttlOf[seq], from: from, rtt: time.Since(at), destination: destination, unreachable: unreachable})
		if destination && ttlOf[seq] < limit {
			limit = ttlOf[seq] // later rounds stop at the target
		}
	}
	buffer, control := make([]byte, 1500), make([]byte, 512)
	drain := func() {
		// Answers from the target arrive as ordinary datagrams.
		for {
			n, from, err := unix.Recvfrom(fd, buffer, 0)
			if err != nil {
				break
			}
			if n >= 8 && buffer[0] == echoReply {
				record(binary.BigEndian.Uint16(buffer[6:8]), sockaddrAddr(from), true, false)
			}
		}
		// Answers from routers come through the error queue: the probe that caused
		// them, with the router's address and the ICMP type in the control message.
		for {
			n, oobn, _, _, err := unix.Recvmsg(fd, buffer, control, unix.MSG_ERRQUEUE)
			if err != nil {
				break
			}
			if n < 8 {
				continue
			}
			seq := binary.BigEndian.Uint16(buffer[6:8])
			messages, err := unix.ParseSocketControlMessage(control[:oobn])
			if err != nil {
				continue
			}
			for _, message := range messages {
				if message.Header.Level != int32(level) || message.Header.Type != int32(recvErr) || len(message.Data) < 16 {
					continue
				}
				// struct sock_extended_err: errno, origin, type, code, pad, info, data;
				// then the address of whoever sent the ICMP message.
				origin, kind := message.Data[4], message.Data[5]
				if origin != unix.SO_EE_ORIGIN_ICMP && origin != unix.SO_EE_ORIGIN_ICMP6 {
					continue
				}
				from := offender(message.Data[16:], v6)
				timeExceeded := (!v6 && kind == 11) || (v6 && kind == 3)
				unreachable := (!v6 && kind == 3) || (v6 && kind == 1)
				if timeExceeded || unreachable {
					record(seq, from, false, unreachable)
				}
			}
		}
	}
	wait := func(until time.Time) error {
		for {
			left := time.Until(until)
			if left <= 0 {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN | unix.POLLERR}}
			timeout := min(left, 100*time.Millisecond)
			if _, err := unix.Poll(poll, int(timeout.Milliseconds())+1); err != nil && !errors.Is(err, unix.EINTR) {
				return err
			}
			drain()
		}
	}

	for round := 0; round < opts.Rounds; round++ {
		send(round)
		pause := opts.Interval
		if round == opts.Rounds-1 {
			pause = opts.Wait
		}
		if err := wait(time.Now().Add(pause)); err != nil {
			break // the context ended: report what came back so far
		}
	}
	return summarise(replies, sent, opts.MaxHops), nil
}

func sockaddrAddr(sa unix.Sockaddr) netip.Addr {
	switch a := sa.(type) {
	case *unix.SockaddrInet4:
		return netip.AddrFrom4(a.Addr)
	case *unix.SockaddrInet6:
		return netip.AddrFrom16(a.Addr)
	}
	return netip.Addr{}
}

// offender reads the sockaddr_in or sockaddr_in6 after struct sock_extended_err.
func offender(data []byte, v6 bool) netip.Addr {
	if !v6 && len(data) >= 8 {
		return netip.AddrFrom4([4]byte(data[4:8]))
	}
	if v6 && len(data) >= 24 {
		return netip.AddrFrom16([16]byte(data[8:24]))
	}
	return netip.Addr{}
}
