package linux

import (
	"context"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemdUnitStatusTask implements the linux.systemd.unit.status capability.
type systemdUnitStatusTask struct{ provider *Provider }

const systemdUnitStatusSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.systemd.unit.status Parameters",
  "description": "Parameters for linux.systemd.unit.status.",
  "properties": {
    "unit": {
      "type": "string",
      "description": "Systemd unit name to query (e.g. sshd.service)"
    }
  },
  "required": ["unit"]
}`

func (t *systemdUnitStatusTask) Name() string       { return "linux.systemd.unit.status" }
func (t *systemdUnitStatusTask) JSONSchema() string { return systemdUnitStatusSchema }

func (t *systemdUnitStatusTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("systemd.unit.status starting", "capability", t.Name())

	unit, err := common.RequireString(params, "unit")
	if err != nil {
		return common.TaskFailure(err)
	}

	path, content, err := findUnitFile(t.provider.CurrentReader(), unit)
	if err != nil {
		slog.Info("systemd.unit.status failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	sections := parseUnitFile(content)
	r := t.provider.CurrentReader()

	result := map[string]any{
		"name":      unit,
		"file_path": path,
		"active":    unitActive(r, unit, sections),
		"enabled":   unitEnabled(r, unit),
		"main_pid":  unitMainPID(r, sections),
	}
	slog.Info("systemd.unit.status succeeded", "capability", t.Name(), "unit", unit)
	return common.SuccessResult(result), nil
}

var _ task.Task = (*systemdUnitStatusTask)(nil)
