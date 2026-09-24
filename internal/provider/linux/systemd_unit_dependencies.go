package linux

import (
	"context"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemdUnitDependenciesTask implements the linux.systemd.unit.dependencies capability.
type systemdUnitDependenciesTask struct{ provider *Provider }

const systemdUnitDependenciesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.systemd.unit.dependencies Parameters",
  "description": "Parameters for linux.systemd.unit.dependencies.",
  "properties": {
    "unit": {
      "type": "string",
      "description": "Systemd unit name to query (e.g. sshd.service)"
    }
  },
  "required": ["unit"]
}`

func (t *systemdUnitDependenciesTask) Name() string       { return "linux.systemd.unit.dependencies" }
func (t *systemdUnitDependenciesTask) JSONSchema() string { return systemdUnitDependenciesSchema }

func (t *systemdUnitDependenciesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("systemd.unit.dependencies starting", "capability", t.Name())

	unit, err := common.RequireString(params, "unit")
	if err != nil {
		return common.TaskFailure(err)
	}

	_, content, err := findUnitFile(t.provider.CurrentReader(), unit)
	if err != nil {
		slog.Info("systemd.unit.dependencies failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	sections := parseUnitFile(content)
	unitSection := sections["Unit"]

	result := map[string]any{
		"unit":      unit,
		"after":     splitDependencyList(unitSection["After"]),
		"before":    splitDependencyList(unitSection["Before"]),
		"requires":  splitDependencyList(unitSection["Requires"]),
		"wants":     splitDependencyList(unitSection["Wants"]),
		"conflicts": splitDependencyList(unitSection["Conflicts"]),
		"requisite": splitDependencyList(unitSection["Requisite"]),
	}
	slog.Info("systemd.unit.dependencies succeeded", "capability", t.Name(), "unit", unit)
	return common.SuccessResult(result), nil
}

var _ task.Task = (*systemdUnitDependenciesTask)(nil)

// splitDependencyList splits a systemd dependency directive value on commas and
// whitespace, returning a trimmed, non-empty, de-duplicated slice.
func splitDependencyList(v string) []string {
	fields := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	seen := make(map[string]bool)
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}
