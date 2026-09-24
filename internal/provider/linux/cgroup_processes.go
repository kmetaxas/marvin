package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// cgroupProcessesTask implements the linux.cgroup.processes capability.
type cgroupProcessesTask struct{ provider *Provider }

const cgroupProcessesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.cgroup.processes Parameters",
  "description": "Parameters for linux.cgroup.processes.",
  "properties": {
    "path": {
      "type": "string",
      "description": "Cgroup path (e.g. /system.slice/sshd.service)"
    }
  },
  "required": ["path"]
}`

func (t *cgroupProcessesTask) Name() string       { return "linux.cgroup.processes" }
func (t *cgroupProcessesTask) JSONSchema() string { return cgroupProcessesSchema }

func (t *cgroupProcessesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("cgroup.processes starting", "capability", t.Name())

	path, err := common.RequireString(params, "path")
	if err != nil {
		return common.TaskFailure(err)
	}

	pids, err := readCgroupProcesses(t.provider.CurrentReader(), path)
	if err != nil {
		slog.Info("cgroup.processes failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("cgroup.processes succeeded", "capability", t.Name(), "count", len(pids))
	return common.SuccessResult(map[string]any{"processes": pids}), nil
}

var _ task.Task = (*cgroupProcessesTask)(nil)

func readCgroupProcesses(r *procfs.Reader, cgroupPath string) ([]int, error) {
	mounts, err := r.ReadFileLines("proc", "self", "mountinfo")
	if err != nil {
		return nil, fmt.Errorf("read mountinfo: %w", err)
	}

	var cgroupRoot string
	for _, line := range mounts {
		fields := strings.Fields(line)
		if len(fields) >= 9 {
			if fields[len(fields)-1] == "cgroup" || fields[len(fields)-1] == "cgroup2" {
				cgroupRoot = fields[4]
				if fields[len(fields)-1] == "cgroup2" {
					break
				}
			}
		}
	}
	if cgroupRoot == "" {
		cgroupRoot = "/sys/fs/cgroup"
	}

	fullPath := filepath.Join(cgroupRoot, cgroupPath)
	if !strings.HasPrefix(fullPath, cgroupRoot) {
		return nil, fmt.Errorf("invalid cgroup path")
	}

	procsPath := filepath.Join(fullPath, "cgroup.procs")
	data, err := os.ReadFile(procsPath)
	if err != nil {
		return nil, fmt.Errorf("read cgroup.procs: %w", err)
	}

	var pids []int
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}
