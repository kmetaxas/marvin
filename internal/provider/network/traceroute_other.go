//go:build !linux

package network

import (
	"context"
	"errors"
	"net"
)

func runPlatformTraceroute(_ context.Context, _ net.IP, _ tracerouteConfig) ([]tracerouteHop, bool, error) {
	return nil, false, errors.New("UDP traceroute without root privileges requires Linux IP_RECVERR support")
}
