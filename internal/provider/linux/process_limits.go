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

// processLimitsTask implements the linux.process.limits capability.
type processLimitsTask struct{ provider *Provider }

const processLimitsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.process.limits Parameters",
  "description": "Parameters for linux.process.limits.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID"
    }
  },
  "required": ["pid"]
}`

func (t *processLimitsTask) Name() string       { return "linux.process.limits" }
func (t *processLimitsTask) JSONSchema() string { return processLimitsSchema }

func (t *processLimitsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("process.limits starting", "capability", t.Name())

	pid, err := common.OptionalInt(params, "pid", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	limits, err := readProcessLimits(t.provider.CurrentReader(), pid)
	if err != nil {
		slog.Info("process.limits failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("process.limits succeeded", "capability", t.Name(), "count", len(limits))
	return common.SuccessResult(map[string]any{"limits": limits}), nil
}

var _ task.Task = (*processLimitsTask)(nil)

// ProcessLimit represents a single resource limit.
type ProcessLimit struct {
	Name      string `json:"name"`
	Soft      string `json:"soft"`
	Hard      string `json:"hard"`
	Unit      string `json:"unit,omitempty"`
	Unlimited bool   `json:"unlimited"`
}

func readProcessLimits(r *procfs.Reader, pid int) ([]ProcessLimit, error) {
	pidStr := strconv.Itoa(pid)
	lines, err := r.ReadFileLines("proc", pidStr, "limits")
	if err != nil {
		return nil, fmt.Errorf("read limits: %w", err)
	}

	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid limits file format")
	}

	var limits []ProcessLimit
	// Skip header line
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		limit := parseLimitLine(line)
		if limit != nil {
			limits = append(limits, *limit)
		}
	}

	return limits, nil
}

func parseLimitLine(line string) *ProcessLimit {
	// Format: "LimitName                            Soft Limit           Hard Limit           Units     "
	// The name is left-aligned, values are right-aligned within fixed-width columns.
	// Limit names may contain spaces (e.g. "Max open files").
	// Strategy: walk from the end since soft/hard are always present and unit is last.
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return nil
	}

	unit := fields[len(fields)-1]
	hard := fields[len(fields)-2]
	soft := fields[len(fields)-3]
	name := strings.Join(fields[:len(fields)-3], " ")

	unlimited := soft == "unlimited" || hard == "unlimited"

	return &ProcessLimit{
		Name:      name,
		Soft:      soft,
		Hard:      hard,
		Unit:      unit,
		Unlimited: unlimited,
	}
}
