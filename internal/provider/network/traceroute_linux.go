//go:build linux

package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

const (
	tracerouteBasePort = 33434
	icmpOrigin         = 2
	icmpTimeExceeded   = 11
	icmpDestUnreach    = 3
	icmpPortUnreach    = 3
)

func runPlatformTraceroute(ctx context.Context, targetIP net.IP, config tracerouteConfig) ([]tracerouteHop, bool, error) {
	if targetIP.To4() == nil {
		return nil, false, fmt.Errorf("UDP traceroute requires an IPv4 target")
	}
	return runUDPTracerouteLinux(ctx, targetIP.To4(), config)
}

func runUDPTracerouteLinux(ctx context.Context, targetIP net.IP, config tracerouteConfig) ([]tracerouteHop, bool, error) {
	hops := make([]tracerouteHop, 0, config.maxHops)

	for ttl := 1; ttl <= config.maxHops; ttl++ {
		if err := ctx.Err(); err != nil {
			hops = append(hops, tracerouteHop{TTL: ttl, Error: err.Error()})
			return hops, false, nil
		}

		hop, reached := probeUDPTracerouteHop(ctx, targetIP, ttl, config.timeout)
		hops = append(hops, hop)
		if reached {
			return hops, true, nil
		}
	}

	return hops, false, nil
}

func probeUDPTracerouteHop(ctx context.Context, targetIP net.IP, ttl int, timeout time.Duration) (tracerouteHop, bool) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return tracerouteHop{TTL: ttl, Error: fmt.Sprintf("failed to open UDP socket: %v", err)}, false
	}
	defer unix.Close(fd)

	probeTimeout := timeoutForContext(ctx, timeout)
	if probeTimeout <= 0 {
		return tracerouteHop{TTL: ttl, Error: ctx.Err().Error()}, false
	}
	timeval := durationToTimeval(probeTimeout)
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeval); err != nil {
		return tracerouteHop{TTL: ttl, Error: fmt.Sprintf("failed to set receive timeout: %v", err)}, false
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_IP, unix.IP_RECVERR, 1); err != nil {
		return tracerouteHop{TTL: ttl, Error: fmt.Sprintf("failed to enable IP_RECVERR: %v", err)}, false
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_IP, unix.IP_TTL, ttl); err != nil {
		return tracerouteHop{TTL: ttl, Error: fmt.Sprintf("failed to set TTL: %v", err)}, false
	}

	addr := unix.SockaddrInet4{Port: tracerouteBasePort + ttl}
	copy(addr.Addr[:], targetIP.To4())

	start := time.Now()
	if err := unix.Sendto(fd, []byte("marvin"), 0, &addr); err != nil {
		return tracerouteHop{TTL: ttl, Error: err.Error()}, false
	}

	deadline := start.Add(probeTimeout)
	for {
		packet := make([]byte, 1500)
		control := make([]byte, 512)
		_, controlLen, _, from, err := unix.Recvmsg(fd, packet, control, unix.MSG_ERRQUEUE)
		if err != nil {
			if ctx.Err() != nil {
				return tracerouteHop{TTL: ttl, Error: ctx.Err().Error()}, false
			}
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return tracerouteHop{TTL: ttl, Error: "request timed out"}, false
			}
			return tracerouteHop{TTL: ttl, Error: err.Error()}, false
		}

		originIP, reached, ok := parseICMPErrorQueue(control[:controlLen])
		if !ok {
			if time.Now().After(deadline) {
				return tracerouteHop{TTL: ttl, Error: "request timed out"}, false
			}
			continue
		}
		if originIP == nil {
			originIP = sockaddrIP(from)
		}

		hop := tracerouteHop{TTL: ttl, RTTMs: durationMs(time.Since(start)), Reached: reached}
		if originIP != nil {
			hop.Address = originIP.String()
		}
		return hop, reached
	}
}

func parseICMPErrorQueue(control []byte) (net.IP, bool, bool) {
	messages, err := unix.ParseSocketControlMessage(control)
	if err != nil {
		return nil, false, false
	}

	for _, message := range messages {
		if message.Header.Level != unix.SOL_IP || message.Header.Type != unix.IP_RECVERR {
			continue
		}
		originIP, reached, ok := parseSockExtendedErr(message.Data)
		if ok {
			return originIP, reached, true
		}
	}

	return nil, false, false
}

func parseSockExtendedErr(data []byte) (net.IP, bool, bool) {
	if len(data) < 16 {
		return nil, false, false
	}
	if data[4] != icmpOrigin {
		return nil, false, false
	}

	icmpType := data[5]
	icmpCode := data[6]
	switch {
	case icmpType == icmpTimeExceeded:
		return parseOffenderIPv4(data[16:]), false, true
	case icmpType == icmpDestUnreach && icmpCode == icmpPortUnreach:
		return parseOffenderIPv4(data[16:]), true, true
	default:
		return nil, false, false
	}
}

func parseOffenderIPv4(data []byte) net.IP {
	if len(data) < 8 || !isAFInet(data[0], data[1]) {
		return nil
	}
	return net.IPv4(data[4], data[5], data[6], data[7])
}

func isAFInet(first, second byte) bool {
	return (first == unix.AF_INET && second == 0) || (first == 0 && second == unix.AF_INET)
}

func sockaddrIP(addr unix.Sockaddr) net.IP {
	inet4, ok := addr.(*unix.SockaddrInet4)
	if !ok {
		return nil
	}
	return net.IPv4(inet4.Addr[0], inet4.Addr[1], inet4.Addr[2], inet4.Addr[3])
}

func timeoutForContext(ctx context.Context, timeout time.Duration) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < timeout {
			return remaining
		}
	}
	return timeout
}

func durationToTimeval(duration time.Duration) unix.Timeval {
	if duration < time.Microsecond {
		duration = time.Microsecond
	}
	return unix.Timeval{
		Sec:  int64(duration / time.Second),
		Usec: int64((duration % time.Second) / time.Microsecond),
	}
}
