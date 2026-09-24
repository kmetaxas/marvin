package linux

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemdUnitListTask implements the linux.systemd.unit.list capability.
type systemdUnitListTask struct{ provider *Provider }

const systemdUnitListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.systemd.unit.list Parameters",
  "description": "Parameters for linux.systemd.unit.list."
}`

func (t *systemdUnitListTask) Name() string       { return "linux.systemd.unit.list" }
func (t *systemdUnitListTask) JSONSchema() string { return systemdUnitListSchema }

func (t *systemdUnitListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("systemd.unit.list starting", "capability", t.Name())

	units, err := listSystemdUnits(t.provider.CurrentReader())
	if err != nil {
		slog.Info("systemd.unit.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"units": units,
		"count": len(units),
	}
	slog.Info("systemd.unit.list succeeded", "capability", t.Name(), "count", len(units))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*systemdUnitListTask)(nil)

// systemdUnitListItem describes a single systemd unit discovered on disk.
type systemdUnitListItem struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Path    string `json:"path"`
	State   string `json:"state"`
	Enabled bool   `json:"enabled"`
}

// listSystemdUnits scans the standard unit directories and reports each unit
// with its on-disk path, inferred state, and enablement.
func listSystemdUnits(r *procfs.Reader) ([]systemdUnitListItem, error) {
	seen := make(map[string]bool)
	var units []systemdUnitListItem

	for _, dir := range systemdUnitSearchDirs {
		names, err := r.ReadDirNames(dir...)
		if err != nil {
			continue
		}
		for _, name := range names {
			if seen[name] || !hasSystemdUnitSuffix(name) {
				continue
			}
			seen[name] = true

			path := r.Path(append(append([]string{}, dir...), name)...)
			item := systemdUnitListItem{
				Name:    name,
				Type:    systemdUnitType(name),
				Path:    path,
				State:   "inactive",
				Enabled: unitEnabled(r, name),
			}

			if r.Exists("run", "systemd", "units", name) {
				item.State = "active"
			} else if strings.HasSuffix(name, ".service") && r.IsDir("sys", "fs", "cgroup", "system.slice", name) {
				item.State = "active"
			} else {
				content, err := r.ReadFileString(append(append([]string{}, dir...), name)...)
				if err == nil {
					sections := parseSystemdUnit(content)
					if pidFile := unitPIDFile(sections); pidFile != "" && r.Exists(pidFileElements(pidFile)...) {
						item.State = "active"
					}
				}
			}

			units = append(units, item)
		}
	}

	sort.Slice(units, func(i, j int) bool { return units[i].Name < units[j].Name })
	return units, nil
}
