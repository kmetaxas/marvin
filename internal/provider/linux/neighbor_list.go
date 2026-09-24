package linux

import (
	"context"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// neighborListTask implements the linux.network.neighbor.list capability.
type neighborListTask struct{ provider *Provider }

const neighborListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Neighbor List Parameters",
  "description": "Parameters for listing ARP/NDP neighbor entries.",
  "properties": {
    "family": {
      "type": "string",
      "description": "Address family filter (arp for IPv4, ndp for IPv6).",
      "enum": ["arp", "ndp"]
    }
  }
}`

func (t *neighborListTask) Name() string       { return "linux.network.neighbor.list" }
func (t *neighborListTask) JSONSchema() string { return neighborListSchema }

func (t *neighborListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("network.neighbor.list starting", "capability", t.Name())

	family, _ := params["family"].(string)

	reader := t.provider.CurrentReader()
	neighbors, err := listNeighbors(reader, family)
	if err != nil {
		slog.Info("network.neighbor.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"neighbors": neighbors,
		"count":     len(neighbors),
	}
	slog.Info("network.neighbor.list succeeded", "capability", t.Name(), "count", len(neighbors))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*neighborListTask)(nil)

func listNeighbors(r *procfs.Reader, family string) ([]NeighborInfo, error) {
	var neighbors []NeighborInfo

	if family == "" || family == "arp" {
		if entries, err := parseArp(r.Path("proc", "net", "arp")); err == nil {
			neighbors = append(neighbors, entries...)
		}
	}

	if family == "" || family == "ndp" {
		for _, file := range []string{"ndisc", "neigh"} {
			if entries, err := parseNeigh(r.Path("proc", "net", file)); err == nil {
				neighbors = append(neighbors, entries...)
			}
		}
	}

	return neighbors, nil
}

func parseArp(path string) ([]NeighborInfo, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	var neighbors []NeighborInfo
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		device := ""
		if len(fields) >= 6 {
			device = fields[5]
		}

		state := "reachable"
		if len(fields) >= 4 && fields[3] != "*" {
			state = "stale"
		}

		neighbors = append(neighbors, NeighborInfo{
			Address: fields[0],
			MAC:     fields[3],
			State:   state,
			Device:  device,
			Type:    "arp",
		})
	}
	return neighbors, nil
}

func parseNeigh(path string) ([]NeighborInfo, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	// /proc/net/neigh columns: IP/address, device, MAC, state, flags, type.
	var neighbors []NeighborInfo
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		neighbors = append(neighbors, NeighborInfo{
			Address: fields[0],
			Device:  fields[1],
			MAC:     fields[2],
			State:   fields[3],
			Type:    "ndp",
		})
	}
	return neighbors, nil
}
