package linux

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// networkStatsTask implements the linux.network.stats capability.
type networkStatsTask struct{ provider *Provider }

const networkStatsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Network Stats Parameters",
  "description": "Parameters for retrieving protocol/network statistics."
}`

func (t *networkStatsTask) Name() string       { return "linux.network.stats" }
func (t *networkStatsTask) JSONSchema() string { return networkStatsSchema }

func (t *networkStatsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	_ = params
	slog.Info("network.stats starting", "capability", t.Name())

	reader := t.provider.CurrentReader()
	stats, err := collectNetworkStats(reader)
	if err != nil {
		slog.Info("network.stats failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("network.stats succeeded", "capability", t.Name())
	return common.SuccessResult(stats), nil
}

var _ task.Task = (*networkStatsTask)(nil)

func collectNetworkStats(r *procfs.Reader) (map[string]any, error) {
	result := make(map[string]any)

	if snmp, err := parseSnmp(r.Path("proc", "net", "snmp")); err == nil {
		result["snmp"] = snmp
	}
	if netstat, err := parseSnmp(r.Path("proc", "net", "netstat")); err == nil {
		result["netstat"] = netstat
	}
	if sockstat, err := parseSockstat(r.Path("proc", "net", "sockstat")); err == nil {
		result["sockstat"] = sockstat
	}

	return result, nil
}

func parseSnmp(path string) (map[string]map[string]int64, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	result := make(map[string]map[string]int64)
	for i := 0; i+1 < len(lines); i += 2 {
		header := strings.Fields(lines[i])
		values := strings.Fields(lines[i+1])
		if len(header) == 0 || len(header) != len(values) {
			continue
		}

		section := strings.TrimSuffix(header[0], ":")
		fields := make(map[string]int64)
		for j := 1; j < len(header); j++ {
			v, err := strconv.ParseInt(values[j], 10, 64)
			if err != nil {
				continue
			}
			fields[header[j]] = v
		}
		result[section] = fields
	}
	return result, nil
}

func parseSockstat(path string) (map[string]any, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	result := make(map[string]any)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		section := strings.TrimSuffix(fields[0], ":")
		values := make(map[string]int64)
		for i := 1; i+1 < len(fields); i += 2 {
			v, err := strconv.ParseInt(fields[i+1], 10, 64)
			if err != nil {
				continue
			}
			values[fields[i]] = v
		}
		result[section] = values
	}
	return result, nil
}
