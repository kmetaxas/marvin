package linux

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// routeLookupTask implements the linux.network.route.lookup capability.
type routeLookupTask struct{ provider *Provider }

const routeLookupSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Route Lookup Parameters",
  "description": "Parameters for performing a kernel route lookup.",
  "properties": {
    "destination": {
      "type": "string",
      "description": "Destination IP address to look up."
    }
  },
  "required": ["destination"]
}`

func (t *routeLookupTask) Name() string       { return "linux.network.route.lookup" }
func (t *routeLookupTask) JSONSchema() string { return routeLookupSchema }

func (t *routeLookupTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("network.route.lookup starting", "capability", t.Name())

	dest, err := common.RequireString(params, "destination")
	if err != nil {
		return common.TaskFailure(err)
	}

	ip := net.ParseIP(dest)
	if ip == nil {
		return common.TaskFailure(fmt.Errorf("invalid destination IP address: %q", dest))
	}

	route, err := lookupRoute(t.provider.CurrentReader(), ip)
	if err != nil {
		slog.Info("network.route.lookup failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}
	if route == nil {
		return common.TaskFailure(fmt.Errorf("no route to destination %s", dest))
	}

	result := map[string]any{
		"destination": dest,
		"gateway":     route.Gateway,
		"interface":   route.Interface,
		"metric":      route.Metric,
		"source":      route.Source,
		"route":       route.Destination,
	}
	slog.Info("network.route.lookup succeeded", "capability", t.Name(), "destination", dest, "interface", route.Interface)
	return common.SuccessResult(result), nil
}

var _ task.Task = (*routeLookupTask)(nil)

// lookupRoute finds the most specific route (longest prefix match) for the
// given destination IP by consulting /proc/net/route (IPv4) and
// /proc/net/ipv6_route (IPv6).
func lookupRoute(r *procfs.Reader, ip net.IP) (*RouteInfo, error) {
	if ip4 := ip.To4(); ip4 != nil {
		return lookupIPv4Route(r, ip4)
	}
	return lookupIPv6Route(r, ip)
}

func lookupIPv4Route(r *procfs.Reader, ip net.IP) (*RouteInfo, error) {
	lines, err := r.ReadFileLines("proc", "net", "route")
	if err != nil {
		return nil, fmt.Errorf("read /proc/net/route: %w", err)
	}

	var best *RouteInfo
	bestBits := -1

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		iface := fields[0]
		destHex := fields[1]
		gatewayHex := fields[2]
		metric, _ := strconv.Atoi(fields[6])
		maskHex := fields[7]

		destIP := parseIPv4Hex(destHex)
		maskIP := parseIPv4Hex(maskHex)
		maskBits := maskBitsFromIPv4(maskIP)

		if !ipv4InNetwork(ip, destIP, maskIP) {
			continue
		}
		if maskBits <= bestBits {
			continue
		}

		bestBits = maskBits
		gatewayIP := parseIPv4Hex(gatewayHex)

		dest := destIP.String()
		if maskBits == 0 {
			dest = "default"
		} else if maskBits < 32 {
			dest = fmt.Sprintf("%s/%d", dest, maskBits)
		}

		route := &RouteInfo{
			Destination: dest,
			Gateway:     gatewayIP.String(),
			Interface:   iface,
			Metric:      metric,
		}
		if route.Gateway == "0.0.0.0" {
			route.Gateway = ""
		}
		best = route
	}

	return best, nil
}

func lookupIPv6Route(r *procfs.Reader, ip net.IP) (*RouteInfo, error) {
	lines, err := r.ReadFileLines("proc", "net", "ipv6_route")
	if err != nil {
		return nil, fmt.Errorf("read /proc/net/ipv6_route: %w", err)
	}

	var best *RouteInfo
	bestBits := -1

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		destHex := fields[0]
		destLen, _ := strconv.Atoi(fields[1])
		srcHex := fields[2]
		srcLen, _ := strconv.Atoi(fields[3])
		gatewayHex := fields[4]
		metric, _ := strconv.Atoi(fields[5])
		iface := fields[9]

		destIP := parseIPv6Hex(destHex)
		if !ipv6InNetwork(ip, destIP, destLen) {
			continue
		}
		if destLen <= bestBits {
			continue
		}

		bestBits = destLen
		gatewayIP := parseIPv6Hex(gatewayHex)
		srcIP := parseIPv6Hex(srcHex)

		dest := fmt.Sprintf("%s/%d", destIP.String(), destLen)
		if destLen == 0 {
			dest = "default"
		}

		route := &RouteInfo{
			Destination: dest,
			Gateway:     gatewayIP.String(),
			Interface:   iface,
			Metric:      metric,
		}
		if route.Gateway == "::" {
			route.Gateway = ""
		}
		if srcLen > 0 {
			route.Source = fmt.Sprintf("%s/%d", srcIP.String(), srcLen)
		}
		best = route
	}

	return best, nil
}

// ipv4InNetwork reports whether ip belongs to the network described by
// destIP and maskIP.
func ipv4InNetwork(ip, destIP, maskIP net.IP) bool {
	ip4 := ip.To4()
	dest4 := destIP.To4()
	mask4 := maskIP.To4()
	if ip4 == nil || dest4 == nil || mask4 == nil {
		return false
	}
	for i := 0; i < 4; i++ {
		if ip4[i]&mask4[i] != dest4[i]&mask4[i] {
			return false
		}
	}
	return true
}

// ipv6InNetwork reports whether ip belongs to the network described by
// destIP and the given prefix length.
func ipv6InNetwork(ip, destIP net.IP, prefixLen int) bool {
	ip16 := ip.To16()
	dest16 := destIP.To16()
	if ip16 == nil || dest16 == nil {
		return false
	}
	if prefixLen < 0 || prefixLen > 128 {
		return false
	}

	fullBytes := prefixLen / 8
	remBits := prefixLen % 8

	for i := 0; i < fullBytes; i++ {
		if ip16[i] != dest16[i] {
			return false
		}
	}
	if remBits > 0 {
		mask := byte(0xff << (8 - remBits))
		if ip16[fullBytes]&mask != dest16[fullBytes]&mask {
			return false
		}
	}
	return true
}
