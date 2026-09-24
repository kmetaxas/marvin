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

// routeListTask implements the linux.network.route.list capability.
type routeListTask struct{ provider *Provider }

const routeListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Route List Parameters",
  "description": "Parameters for listing routing table entries.",
  "properties": {
    "family": {
      "type": "string",
      "description": "Address family filter.",
      "enum": ["inet", "inet6"]
    }
  }
}`

func (t *routeListTask) Name() string       { return "linux.network.route.list" }
func (t *routeListTask) JSONSchema() string { return routeListSchema }

func (t *routeListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("network.route.list starting", "capability", t.Name())

	family, _ := params["family"].(string)

	reader := t.provider.CurrentReader()
	routes, err := listRoutes(reader, family)
	if err != nil {
		slog.Info("network.route.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"routes": routes,
		"count":  len(routes),
	}
	slog.Info("network.route.list succeeded", "capability", t.Name(), "count", len(routes))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*routeListTask)(nil)

func listRoutes(r *procfs.Reader, family string) ([]RouteInfo, error) {
	var routes []RouteInfo

	if family == "" || family == "inet" {
		ipv4, err := parseProcNetRoute(r.Path("proc", "net", "route"))
		if err == nil {
			routes = append(routes, ipv4...)
		}
	}
	if family == "" || family == "inet6" {
		ipv6, err := parseProcNetIPv6Route(r.Path("proc", "net", "ipv6_route"))
		if err == nil {
			routes = append(routes, ipv6...)
		}
	}

	return routes, nil
}

func parseProcNetRoute(path string) ([]RouteInfo, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	var routes []RouteInfo
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
		flags := fields[3]
		metric, _ := strconv.Atoi(fields[6])
		maskHex := fields[7]

		destIP := parseIPv4Hex(destHex)
		gatewayIP := parseIPv4Hex(gatewayHex)
		maskIP := parseIPv4Hex(maskHex)
		maskBits := maskBitsFromIPv4(maskIP)

		dest := destIP.String()
		if maskBits > 0 && maskBits < 32 {
			dest = fmt.Sprintf("%s/%d", dest, maskBits)
		} else if maskBits == 0 && dest == "0.0.0.0" {
			dest = "default"
		}

		scope := "global"
		if flags == "0001" || flags == "0003" {
			if dest == "default" {
				scope = "global"
			} else if gatewayIP.String() == "0.0.0.0" {
				scope = "link"
			}
		}

		protocol := "static"
		if metric == 0 && gatewayIP.String() == "0.0.0.0" && dest != "default" {
			protocol = "kernel"
		}

		route := RouteInfo{
			Destination: dest,
			Gateway:     gatewayIP.String(),
			Interface:   iface,
			Metric:      metric,
			Protocol:    protocol,
			Scope:       scope,
		}
		if route.Gateway == "0.0.0.0" {
			route.Gateway = ""
		}
		routes = append(routes, route)
	}
	return routes, nil
}

func parseProcNetIPv6Route(path string) ([]RouteInfo, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	var routes []RouteInfo
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
		// ref, use, flags, etc.
		iface := fields[9]
		if len(fields) > 9 {
			iface = fields[9]
		}

		destIP := parseIPv6Hex(destHex)
		dest := fmt.Sprintf("%s/%d", destIP.String(), destLen)
		if destLen == 0 {
			dest = "default"
		}

		gatewayIP := parseIPv6Hex(gatewayHex)
		srcIP := parseIPv6Hex(srcHex)

		route := RouteInfo{
			Destination: dest,
			Gateway:     gatewayIP.String(),
			Interface:   iface,
			Metric:      metric,
			Protocol:    "kernel",
			Scope:       "global",
		}
		if route.Gateway == "::" {
			route.Gateway = ""
		}
		if srcLen > 0 {
			route.Source = fmt.Sprintf("%s/%d", srcIP.String(), srcLen)
		}

		routes = append(routes, route)
	}
	return routes, nil
}

func parseIPv4Hex(hexStr string) net.IP {
	val, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		return nil
	}
	return net.IPv4(byte(val), byte(val>>8), byte(val>>16), byte(val>>24))
}

func maskBitsFromIPv4(ip net.IP) int {
	mask := ip.To4()
	if mask == nil {
		return 0
	}
	var bits int
	for _, b := range mask {
		for b != 0 {
			bits += int(b & 1)
			b >>= 1
		}
	}
	// Actually count leading ones for CIDR
	bits = 0
	for _, b := range mask {
		for i := 7; i >= 0; i-- {
			if b>>i&1 == 1 {
				bits++
			} else {
				return bits
			}
		}
	}
	return bits
}

func parseIPv6Hex(hexStr string) net.IP {
	if len(hexStr) != 32 {
		return net.IP{}
	}
	ip := make(net.IP, 16)
	for i := 0; i < 16; i++ {
		b, err := strconv.ParseUint(hexStr[i*2:(i+1)*2], 16, 8)
		if err != nil {
			return net.IP{}
		}
		ip[i] = byte(b)
	}
	// /proc/net/ipv6_route stores in groups of 4 bytes in reverse order within each group
	for i := 0; i < 4; i++ {
		group := ip[i*4 : (i+1)*4]
		for j := 0; j < 2; j++ {
			group[j], group[3-j] = group[3-j], group[j]
		}
	}
	return ip
}
