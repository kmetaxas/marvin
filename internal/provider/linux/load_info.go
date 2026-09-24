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

// loadInfoTask implements the linux.load.info capability.
type loadInfoTask struct{ provider *Provider }

const loadInfoSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.load.info Parameters",
  "description": "Parameters for linux.load.info."
}`

func (t *loadInfoTask) Name() string       { return "linux.load.info" }
func (t *loadInfoTask) JSONSchema() string { return loadInfoSchema }

func (t *loadInfoTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("load.info starting", "capability", t.Name())

	data, err := readLoadInfo(t.provider.CurrentReader())
	if err != nil {
		slog.Info("load.info failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("load.info succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*loadInfoTask)(nil)

func readLoadInfo(r *procfs.Reader) (map[string]any, error) {
	data, err := r.ReadFileString("proc", "loadavg")
	if err != nil {
		return nil, fmt.Errorf("read /proc/loadavg: %w", err)
	}

	fields := strings.Fields(data)
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid /proc/loadavg format")
	}

	load1, _ := strconv.ParseFloat(fields[0], 64)
	load5, _ := strconv.ParseFloat(fields[1], 64)
	load15, _ := strconv.ParseFloat(fields[2], 64)

	result := map[string]any{
		"loadavg_1min":  load1,
		"loadavg_5min":  load5,
		"loadavg_15min": load15,
	}

	if len(fields) >= 5 {
		parts := strings.SplitN(fields[3], "/", 2)
		if len(parts) == 2 {
			runnable, _ := strconv.Atoi(parts[0])
			total, _ := strconv.Atoi(parts[1])
			result["runnable_tasks"] = runnable
			result["total_tasks"] = total
		}
		lastPID, _ := strconv.Atoi(fields[4])
		result["last_pid"] = lastPID
	}

	return result, nil
}
