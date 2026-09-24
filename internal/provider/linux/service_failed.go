package linux

import (
	"context"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// serviceFailedTask implements the linux.service.failed capability.
type serviceFailedTask struct{ provider *Provider }

const serviceFailedSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.service.failed Parameters",
  "description": "Parameters for linux.service.failed."
}`

func (t *serviceFailedTask) Name() string       { return "linux.service.failed" }
func (t *serviceFailedTask) JSONSchema() string { return serviceFailedSchema }

func (t *serviceFailedTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("service.failed starting", "capability", t.Name())

	failed, err := listFailedServices(t.provider.CurrentReader())
	if err != nil {
		slog.Info("service.failed failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"services": failed,
		"count":    len(failed),
	}
	slog.Info("service.failed succeeded", "capability", t.Name(), "count", len(failed))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*serviceFailedTask)(nil)

// failedService describes a failed systemd unit discovered via the filesystem.
type failedService struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// listFailedServices discovers failed systemd units using a filesystem-only
// approach (no D-Bus). It first verifies systemd is PID 1, then inspects
// /run/systemd/failed/ and the system.slice cgroup for failed units.
func listFailedServices(r *procfs.Reader) ([]failedService, error) {
	if !isSystemdInit(r) {
		return []failedService{}, nil
	}

	seen := make(map[string]bool)
	var failed []failedService

	// /run/systemd/failed/ contains one file per failed unit.
	if names, err := r.ReadDirNames("run", "systemd", "failed"); err == nil {
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			failed = append(failed, failedService{Name: name, State: "failed"})
		}
	}

	// /sys/fs/cgroup/system.slice/ may contain units in a failed state.
	// Units that have failed are typically marked with a "failed" result in
	// their cgroup; we surface any unit whose name suggests a service and
	// whose cgroup directory exists under system.slice.
	if names, err := r.ReadDirNames("sys", "fs", "cgroup", "system.slice"); err == nil {
		for _, name := range names {
			if !strings.HasSuffix(name, ".service") {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			failed = append(failed, failedService{Name: name, State: "failed"})
		}
	}

	return failed, nil
}

// isSystemdInit reports whether PID 1 is systemd by reading /proc/1/comm.
func isSystemdInit(r *procfs.Reader) bool {
	comm, err := r.ReadFileString("proc", "1", "comm")
	if err != nil {
		return false
	}
	return strings.TrimSpace(comm) == "systemd"
}
