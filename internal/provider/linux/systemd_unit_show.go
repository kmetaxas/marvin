package linux

import (
	"context"
	"log/slog"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemdUnitShowTask implements the linux.systemd.unit.show capability.
type systemdUnitShowTask struct{ provider *Provider }

const systemdUnitShowSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.systemd.unit.show Parameters",
  "description": "Parameters for linux.systemd.unit.show.",
  "properties": {
    "unit": {
      "type": "string",
      "description": "Systemd unit name to query (e.g. sshd.service)"
    }
  },
  "required": ["unit"]
}`

func (t *systemdUnitShowTask) Name() string       { return "linux.systemd.unit.show" }
func (t *systemdUnitShowTask) JSONSchema() string { return systemdUnitShowSchema }

func (t *systemdUnitShowTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("systemd.unit.show starting", "capability", t.Name())

	unit, err := common.RequireString(params, "unit")
	if err != nil {
		return common.TaskFailure(err)
	}

	path, content, err := findUnitFile(t.provider.CurrentReader(), unit)
	if err != nil {
		slog.Info("systemd.unit.show failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	sections := parseUnitFile(content)

	properties := make(map[string]any)
	for section, kv := range sections {
		keys := make([]string, 0, len(kv))
		for k := range kv {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			properties[section+"."+k] = kv[k]
		}
	}

	result := map[string]any{
		"unit":       unit,
		"file_path":  path,
		"properties": properties,
	}
	slog.Info("systemd.unit.show succeeded", "capability", t.Name(), "unit", unit)
	return common.SuccessResult(result), nil
}

var _ task.Task = (*systemdUnitShowTask)(nil)
