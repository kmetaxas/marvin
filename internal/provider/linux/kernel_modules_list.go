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

// kernelModulesListTask implements the linux.kernel.modules.list capability.
type kernelModulesListTask struct{ provider *Provider }

const kernelModulesListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.kernel.modules.list Parameters",
  "description": "Parameters for linux.kernel.modules.list."
}`

func (t *kernelModulesListTask) Name() string       { return "linux.kernel.modules.list" }
func (t *kernelModulesListTask) JSONSchema() string { return kernelModulesListSchema }

func (t *kernelModulesListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("kernel.modules.list starting", "capability", t.Name())

	data, err := readKernelModulesList(t.provider.CurrentReader())
	if err != nil {
		slog.Info("kernel.modules.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("kernel.modules.list succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*kernelModulesListTask)(nil)

type moduleInfo struct {
	Name         string   `json:"name"`
	Size         int64    `json:"size"`
	Instances    int      `json:"instances"`
	Dependencies []string `json:"dependencies"`
	State        string   `json:"state"`
	Address      string   `json:"address"`
}

func readKernelModulesList(r *procfs.Reader) (map[string]any, error) {
	lines, err := r.ReadFileLines("proc", "modules")
	if err != nil {
		return nil, fmt.Errorf("read /proc/modules: %w", err)
	}

	var modules []moduleInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		size, _ := strconv.ParseInt(fields[1], 10, 64)
		instances, _ := strconv.Atoi(fields[2])

		var deps []string
		if fields[3] != "-" {
			for _, d := range strings.Split(fields[3], ",") {
				if d != "" {
					deps = append(deps, d)
				}
			}
		}

		modules = append(modules, moduleInfo{
			Name:         fields[0],
			Size:         size,
			Instances:    instances,
			Dependencies: deps,
			State:        fields[4],
			Address:      fields[5],
		})
	}

	return map[string]any{
		"modules": modules,
		"count":   len(modules),
	}, nil
}
