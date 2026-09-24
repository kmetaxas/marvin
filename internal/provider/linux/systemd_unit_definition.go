package linux

import (
	"context"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemdUnitDefinitionTask implements the linux.systemd.unit.definition capability.
type systemdUnitDefinitionTask struct{ provider *Provider }

const systemdUnitDefinitionSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.systemd.unit.definition Parameters",
  "description": "Parameters for linux.systemd.unit.definition.",
  "properties": {
    "unit": {
      "type": "string",
      "description": "Systemd unit name to query (e.g. sshd.service)"
    }
  },
  "required": ["unit"]
}`

func (t *systemdUnitDefinitionTask) Name() string       { return "linux.systemd.unit.definition" }
func (t *systemdUnitDefinitionTask) JSONSchema() string { return systemdUnitDefinitionSchema }

func (t *systemdUnitDefinitionTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("systemd.unit.definition starting", "capability", t.Name())

	unit, err := common.RequireString(params, "unit")
	if err != nil {
		return common.TaskFailure(err)
	}

	path, content, err := findUnitFile(t.provider.CurrentReader(), unit)
	if err != nil {
		slog.Info("systemd.unit.definition failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	dropIns := listUnitDropIns(t.provider.CurrentReader(), unit)

	result := map[string]any{
		"name":      unit,
		"file_path": path,
		"content":   string(content),
		"drop_ins":  dropIns,
	}
	slog.Info("systemd.unit.definition succeeded", "capability", t.Name(), "unit", unit, "drop_ins", len(dropIns))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*systemdUnitDefinitionTask)(nil)
