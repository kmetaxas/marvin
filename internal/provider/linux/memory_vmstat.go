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

// memoryVmstatTask implements the linux.memory.vmstat capability.
type memoryVmstatTask struct{ provider *Provider }

const memoryVmstatSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.memory.vmstat Parameters",
  "description": "Parameters for linux.memory.vmstat."
}`

func (t *memoryVmstatTask) Name() string       { return "linux.memory.vmstat" }
func (t *memoryVmstatTask) JSONSchema() string { return memoryVmstatSchema }

func (t *memoryVmstatTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("memory.vmstat starting", "capability", t.Name())

	data, err := readMemoryVmstat(t.provider.CurrentReader())
	if err != nil {
		slog.Info("memory.vmstat failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("memory.vmstat succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*memoryVmstatTask)(nil)

func readMemoryVmstat(r *procfs.Reader) (map[string]any, error) {
	lines, err := r.ReadFileLines("proc", "vmstat")
	if err != nil {
		return nil, fmt.Errorf("read /proc/vmstat: %w", err)
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
		key := fields[0]
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		result[key] = val
	}

	return result, nil
}
