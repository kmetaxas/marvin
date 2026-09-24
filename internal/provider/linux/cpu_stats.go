package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// cpuStatsTask implements the linux.cpu.stats capability.
type cpuStatsTask struct{ provider *Provider }

const cpuStatsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.cpu.stats Parameters",
  "description": "Parameters for linux.cpu.stats.",
  "properties": {
    "per_cpu": {
      "type": "boolean",
      "default": false,
      "description": "Include per-CPU statistics."
    }
  }
}`

func (t *cpuStatsTask) Name() string       { return "linux.cpu.stats" }
func (t *cpuStatsTask) JSONSchema() string { return cpuStatsSchema }

func (t *cpuStatsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("cpu.stats starting", "capability", t.Name())

	perCPU, _ := common.OptionalBool(params, "per_cpu", false)
	data, err := readCPUStats(t.provider.CurrentReader(), perCPU)
	if err != nil {
		slog.Info("cpu.stats failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("cpu.stats succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*cpuStatsTask)(nil)

// CPUStatLine represents one cpu line from /proc/stat.
type CPUStatLine struct {
	Name      string `json:"name"`
	User      uint64 `json:"user"`
	Nice      uint64 `json:"nice"`
	System    uint64 `json:"system"`
	Idle      uint64 `json:"idle"`
	IOWait    uint64 `json:"iowait,omitempty"`
	IRQ       uint64 `json:"irq,omitempty"`
	SoftIRQ   uint64 `json:"softirq,omitempty"`
	Steal     uint64 `json:"steal,omitempty"`
	Guest     uint64 `json:"guest,omitempty"`
	GuestNice uint64 `json:"guest_nice,omitempty"`
}

func readCPUStats(r *procfs.Reader, perCPU bool) (map[string]any, error) {
	lines, err := r.ReadFileLines("proc", "stat")
	if err != nil {
		return nil, fmt.Errorf("read /proc/stat: %w", err)
	}

	var total *CPUStatLine
	var perCPUs []CPUStatLine

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		stat := parseCPUStatLine(fields)
		if stat == nil {
			continue
		}
		if stat.Name == "cpu" {
			total = stat
		} else if perCPU {
			perCPUs = append(perCPUs, *stat)
		}
	}

	result := make(map[string]any)
	if total != nil {
		result["total"] = total
	}
	if perCPU {
		result["per_cpu"] = perCPUs
	}

	return result, nil
}

func parseCPUStatLine(fields []string) *CPUStatLine {
	if len(fields) < 5 {
		return nil
	}
	name := fields[0]
	vals := make([]uint64, len(fields)-1)
	for i := 1; i < len(fields); i++ {
		v, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return nil
		}
		vals[i-1] = v
	}
	stat := &CPUStatLine{Name: name}
	if len(vals) > 0 {
		stat.User = vals[0]
	}
	if len(vals) > 1 {
		stat.Nice = vals[1]
	}
	if len(vals) > 2 {
		stat.System = vals[2]
	}
	if len(vals) > 3 {
		stat.Idle = vals[3]
	}
	if len(vals) > 4 {
		stat.IOWait = vals[4]
	}
	if len(vals) > 5 {
		stat.IRQ = vals[5]
	}
	if len(vals) > 6 {
		stat.SoftIRQ = vals[6]
	}
	if len(vals) > 7 {
		stat.Steal = vals[7]
	}
	if len(vals) > 8 {
		stat.Guest = vals[8]
	}
	if len(vals) > 9 {
		stat.GuestNice = vals[9]
	}
	return stat
}
