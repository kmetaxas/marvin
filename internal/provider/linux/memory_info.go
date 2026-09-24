package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// memoryInfoTask implements the linux.memory.info capability.
type memoryInfoTask struct{ provider *Provider }

const memoryInfoSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.memory.info Parameters",
  "description": "Parameters for linux.memory.info."
}`

func (t *memoryInfoTask) Name() string       { return "linux.memory.info" }
func (t *memoryInfoTask) JSONSchema() string { return memoryInfoSchema }

func (t *memoryInfoTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("memory.info starting", "capability", t.Name())

	data, err := readMemoryInfo(t.provider.CurrentReader())
	if err != nil {
		slog.Info("memory.info failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("memory.info succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*memoryInfoTask)(nil)

func readMemoryInfo(r *procfs.Reader) (map[string]any, error) {
	lines, err := r.ReadFileLines("proc", "meminfo")
	if err != nil {
		return nil, fmt.Errorf("read /proc/meminfo: %w", err)
	}

	meminfo := make(map[string]uint64)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, err := ParseMemInfoKB(line)
		if err != nil {
			continue
		}
		meminfo[key] = val
	}

	result := make(map[string]any)
	for k, v := range meminfo {
		result[k] = v
	}

	if total, ok := meminfo["MemTotal"]; ok {
		result["mem_total_bytes"] = total * 1024
	}
	if free, ok := meminfo["MemFree"]; ok {
		result["mem_free_bytes"] = free * 1024
	}
	if avail, ok := meminfo["MemAvailable"]; ok {
		result["mem_available_bytes"] = avail * 1024
	}
	if swapTotal, ok := meminfo["SwapTotal"]; ok {
		result["swap_total_bytes"] = swapTotal * 1024
	}
	if swapFree, ok := meminfo["SwapFree"]; ok {
		result["swap_free_bytes"] = swapFree * 1024
	}

	return result, nil
}
