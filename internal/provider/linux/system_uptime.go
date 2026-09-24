package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemUptimeTask returns uptime, boot time and load averages.
type systemUptimeTask struct{ provider *Provider }

const systemUptimeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "System Uptime Parameters",
  "description": "Parameters for retrieving uptime and load averages."
}`

func (t *systemUptimeTask) Name() string       { return "linux.system.uptime" }
func (t *systemUptimeTask) JSONSchema() string { return systemUptimeSchema }

func (t *systemUptimeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("system.uptime starting", "capability", t.Name())

	data, err := readSystemUptime(t.provider.CurrentReader())
	if err != nil {
		slog.Info("system.uptime failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("system.uptime succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*systemUptimeTask)(nil)

func readSystemUptime(r *procfs.Reader) (map[string]any, error) {
	uptimeData, err := r.ReadFileString("proc", "uptime")
	if err != nil {
		return nil, fmt.Errorf("read /proc/uptime: %w", err)
	}

	fields := strings.Fields(uptimeData)
	if len(fields) < 1 {
		return nil, fmt.Errorf("invalid /proc/uptime format")
	}

	uptimeSeconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return nil, fmt.Errorf("parse uptime: %w", err)
	}

	loadData, err := r.ReadFileString("proc", "loadavg")
	if err != nil {
		return nil, fmt.Errorf("read /proc/loadavg: %w", err)
	}

	loadFields := strings.Fields(loadData)
	if len(loadFields) < 3 {
		return nil, fmt.Errorf("invalid /proc/loadavg format")
	}

	load1, _ := strconv.ParseFloat(loadFields[0], 64)
	load5, _ := strconv.ParseFloat(loadFields[1], 64)
	load15, _ := strconv.ParseFloat(loadFields[2], 64)

	bootTime := time.Now().Add(-time.Duration(uptimeSeconds) * time.Second).UTC()

	return map[string]any{
		"uptime_seconds": uptimeSeconds,
		"uptime_human":   humanDuration(int64(uptimeSeconds)),
		"boot_time":      bootTime.Format(time.RFC3339),
		"load_average": map[string]float64{
			"1min":  load1,
			"5min":  load5,
			"15min": load15,
		},
	}, nil
}

func humanDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%dh%dm", seconds/3600, (seconds%3600)/60)
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	return fmt.Sprintf("%dd%dh", days, hours)
}
