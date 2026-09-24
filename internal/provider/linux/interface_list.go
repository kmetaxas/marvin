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

// interfaceListTask implements the linux.network.interface.list capability.
type interfaceListTask struct{ provider *Provider }

const interfaceListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Interface List Parameters",
  "description": "Parameters for listing network interfaces.",
  "properties": {
    "include_down": {
      "type": "boolean",
      "description": "Include interfaces that are administratively down.",
      "default": true
    }
  }
}`

func (t *interfaceListTask) Name() string       { return "linux.network.interface.list" }
func (t *interfaceListTask) JSONSchema() string { return interfaceListSchema }

func (t *interfaceListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("network.interface.list starting", "capability", t.Name())

	includeDown := true
	if v, ok := params["include_down"].(bool); ok {
		includeDown = v
	}

	reader := t.provider.CurrentReader()
	ifaces, err := listInterfaces(reader, includeDown)
	if err != nil {
		slog.Info("network.interface.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"interfaces": ifaces,
		"count":      len(ifaces),
	}
	slog.Info("network.interface.list succeeded", "capability", t.Name(), "count", len(ifaces))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*interfaceListTask)(nil)

func listInterfaces(r *procfs.Reader, includeDown bool) ([]InterfaceInfo, error) {
	// Get counters from /proc/net/dev
	counters, err := readNetDevCounters(r)
	if err != nil {
		counters = make(map[string]netDevCounters)
	}

	netIfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	var result []InterfaceInfo
	for _, iface := range netIfaces {
		if !includeDown && iface.Flags&net.FlagUp == 0 {
			continue
		}

		info := InterfaceInfo{
			Name:  iface.Name,
			Index: iface.Index,
			MTU:   iface.MTU,
			Flags: interfaceFlags(iface.Flags),
		}

		if iface.HardwareAddr != nil {
			info.MACAddress = iface.HardwareAddr.String()
		}

		if iface.Flags&net.FlagUp != 0 {
			info.State = "up"
		} else {
			info.State = "down"
		}

		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				info.Addresses = append(info.Addresses, addr.String())
			}
		}

		if c, ok := counters[iface.Name]; ok {
			info.RxBytes = c.rxBytes
			info.TxBytes = c.txBytes
		}

		result = append(result, info)
	}

	return result, nil
}

type netDevCounters struct {
	rxBytes uint64
	txBytes uint64
}

func readNetDevCounters(r *procfs.Reader) (map[string]netDevCounters, error) {
	lines, err := r.ReadFileLines("proc", "net", "dev")
	if err != nil {
		return nil, err
	}

	result := make(map[string]netDevCounters)
	for i, line := range lines {
		if i < 2 {
			continue // skip header lines
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if iface == "" {
			continue
		}
		// parts[1] = receive counters, parts[2] = transmit counters
		recvFields := strings.Fields(parts[1])
		transFields := strings.Fields(parts[2])
		if len(recvFields) < 1 || len(transFields) < 1 {
			continue
		}
		rxBytes, _ := strconv.ParseUint(recvFields[0], 10, 64)
		txBytes, _ := strconv.ParseUint(transFields[0], 10, 64)
		result[iface] = netDevCounters{rxBytes: rxBytes, txBytes: txBytes}
	}
	return result, nil
}

func interfaceFlags(flags net.Flags) []string {
	var result []string
	if flags&net.FlagUp != 0 {
		result = append(result, "up")
	}
	if flags&net.FlagBroadcast != 0 {
		result = append(result, "broadcast")
	}
	if flags&net.FlagLoopback != 0 {
		result = append(result, "loopback")
	}
	if flags&net.FlagPointToPoint != 0 {
		result = append(result, "pointtopoint")
	}
	if flags&net.FlagMulticast != 0 {
		result = append(result, "multicast")
	}
	return result
}
