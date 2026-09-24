package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// cgroupGetTask implements the linux.cgroup.get capability.
type cgroupGetTask struct{ provider *Provider }

const cgroupGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.cgroup.get Parameters",
  "description": "Parameters for linux.cgroup.get.",
  "properties": {
    "path": {
      "type": "string",
      "description": "Cgroup path (e.g. /system.slice/sshd.service)"
    }
  },
  "required": ["path"]
}`

func (t *cgroupGetTask) Name() string       { return "linux.cgroup.get" }
func (t *cgroupGetTask) JSONSchema() string { return cgroupGetSchema }

func (t *cgroupGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("cgroup.get starting", "capability", t.Name())

	path, err := common.RequireString(params, "path")
	if err != nil {
		return common.TaskFailure(err)
	}

	data, err := readCgroupGet(t.provider.CurrentReader(), path)
	if err != nil {
		slog.Info("cgroup.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("cgroup.get succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*cgroupGetTask)(nil)

func readCgroupGet(r *procfs.Reader, cgroupPath string) (map[string]any, error) {
	// Try to locate cgroup fs mount point
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

	info := map[string]any{
		"path":        cgroupPath,
		"controllers": []string{},
		"limits":      map[string]string{},
		"usage":       map[string]string{},
	}

	// Try to read cgroup.controllers if cgroup v2
	controllersPath := filepath.Join(fullPath, "cgroup.controllers")
	if ctrlData, err := os.ReadFile(controllersPath); err == nil {
		ctrls := strings.Fields(string(ctrlData))
		info["controllers"] = ctrls
	}

	// Try common cgroup v2 files
	files := map[string]string{
		"memory.current":         "memory_usage",
		"memory.max":             "memory_limit",
		"cpu.weight":             "cpu_weight",
		"cpu.stat":               "cpu_stat",
		"pids.current":           "pids_current",
		"pids.max":               "pids_max",
		"io.stat":                "io_stat",
		"cgroup.freeze":          "frozen",
		"cgroup.procs":           "procs",
		"cgroup.subtree_control": "subtree_control",
	}

	limits := make(map[string]string)
	usage := make(map[string]string)
	for file, key := range files {
		p := filepath.Join(fullPath, file)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(data))
		// Categorize as limit or usage based on name
		if strings.Contains(file, "max") || strings.Contains(file, "limit") || strings.Contains(file, "freeze") {
			limits[key] = val
		} else {
			usage[key] = val
		}
	}
	info["limits"] = limits
	info["usage"] = usage

	return info, nil
}
