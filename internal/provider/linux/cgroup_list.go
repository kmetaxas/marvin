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

// cgroupListTask implements the linux.cgroup.list capability.
type cgroupListTask struct{ provider *Provider }

const cgroupListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.cgroup.list Parameters",
  "description": "Parameters for linux.cgroup.list.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID to list cgroups for (defaults to self)"
    }
  }
}`

func (t *cgroupListTask) Name() string       { return "linux.cgroup.list" }
func (t *cgroupListTask) JSONSchema() string { return cgroupListSchema }

func (t *cgroupListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("cgroup.list starting", "capability", t.Name())

	pid, _ := common.OptionalInt(params, "pid", 0)
	if pid <= 0 {
		pid = 1
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	cgroups, err := readCgroupList(t.provider.CurrentReader(), pid)
	if err != nil {
		slog.Info("cgroup.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("cgroup.list succeeded", "capability", t.Name(), "count", len(cgroups))
	return common.SuccessResult(map[string]any{"cgroups": cgroups}), nil
}

var _ task.Task = (*cgroupListTask)(nil)

type CgroupEntry struct {
	HierarchyID int      `json:"hierarchy_id"`
	Controllers []string `json:"controllers"`
	Path        string   `json:"path"`
}

func readCgroupList(r *procfs.Reader, pid int) ([]CgroupEntry, error) {
	lines, err := r.ReadFileLines("proc", strconv.Itoa(pid), "cgroup")
	if err != nil {
		return nil, fmt.Errorf("read /proc/%d/cgroup: %w", pid, err)
	}

	var entries []CgroupEntry
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		hid, _ := strconv.Atoi(parts[0])
		controllers := []string{}
		if parts[1] != "" {
			controllers = strings.Split(parts[1], ",")
		}
		entries = append(entries, CgroupEntry{
			HierarchyID: hid,
			Controllers: controllers,
			Path:        parts[2],
		})
	}
	return entries, nil
}
