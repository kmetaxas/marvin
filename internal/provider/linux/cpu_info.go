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

// cpuInfoTask implements the linux.cpu.info capability.
type cpuInfoTask struct{ provider *Provider }

const cpuInfoSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.cpu.info Parameters",
  "description": "Parameters for linux.cpu.info."
}`

func (t *cpuInfoTask) Name() string       { return "linux.cpu.info" }
func (t *cpuInfoTask) JSONSchema() string { return cpuInfoSchema }

func (t *cpuInfoTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("cpu.info starting", "capability", t.Name())

	data, err := readCPUInfo(t.provider.CurrentReader())
	if err != nil {
		slog.Info("cpu.info failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("cpu.info succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*cpuInfoTask)(nil)

// CPUInfo represents parsed /proc/cpuinfo for one logical processor.
type CPUInfo struct {
	Processor  int      `json:"processor"`
	ModelName  string   `json:"model_name,omitempty"`
	VendorID   string   `json:"vendor_id,omitempty"`
	CPUCores   int      `json:"cpu_cores,omitempty"`
	Siblings   int      `json:"siblings,omitempty"`
	CPUMHz     float64  `json:"cpu_mhz,omitempty"`
	Flags      []string `json:"flags,omitempty"`
	PhysicalID int      `json:"physical_id,omitempty"`
	CoreID     int      `json:"core_id,omitempty"`
}

func readCPUInfo(r *procfs.Reader) (map[string]any, error) {
	lines, err := r.ReadFileLines("proc", "cpuinfo")
	if err != nil {
		return nil, fmt.Errorf("read /proc/cpuinfo: %w", err)
	}

	var cpus []CPUInfo
	var current *CPUInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				cpus = append(cpus, *current)
				current = nil
			}
			continue
		}
		if current == nil {
			current = &CPUInfo{}
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "processor":
			current.Processor, _ = strconv.Atoi(val)
		case "model name":
			current.ModelName = val
		case "vendor_id":
			current.VendorID = val
		case "cpu cores":
			current.CPUCores, _ = strconv.Atoi(val)
		case "siblings":
			current.Siblings, _ = strconv.Atoi(val)
		case "cpu MHz":
			current.CPUMHz, _ = strconv.ParseFloat(val, 64)
		case "flags":
			current.Flags = strings.Fields(val)
		case "physical id":
			current.PhysicalID, _ = strconv.Atoi(val)
		case "core id":
			current.CoreID, _ = strconv.Atoi(val)
		}
	}
	if current != nil {
		cpus = append(cpus, *current)
	}

	physicalIDs := make(map[int]struct{})
	for _, c := range cpus {
		physicalIDs[c.PhysicalID] = struct{}{}
	}

	return map[string]any{
		"processors":    cpus,
		"count":         len(cpus),
		"physical_cpus": len(physicalIDs),
	}, nil
}
