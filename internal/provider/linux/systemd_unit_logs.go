package linux

import (
	"context"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/journal"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemdUnitLogsTask implements the linux.systemd.unit.logs capability.
type systemdUnitLogsTask struct{ provider *Provider }

const systemdUnitLogsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.systemd.unit.logs Parameters",
  "description": "Parameters for linux.systemd.unit.logs.",
  "properties": {
    "unit": {
      "type": "string",
      "description": "Systemd unit name to query logs for"
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of journal entries to return (default 50, capped at 500)"
    }
  },
  "required": ["unit"]
}`

func (t *systemdUnitLogsTask) Name() string       { return "linux.systemd.unit.logs" }
func (t *systemdUnitLogsTask) JSONSchema() string { return systemdUnitLogsSchema }

func (t *systemdUnitLogsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("systemd.unit.logs starting", "capability", t.Name())

	unit, err := common.RequireString(params, "unit")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := journalEntryLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	cfg := t.provider.CurrentConfig()
	files, err := listJournalFiles(t.provider.CurrentReader(), cfg.JournalDirectory, unit)
	if err != nil {
		slog.Info("systemd.unit.logs failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	reader, err := journal.Open(journalDirs(cfg.JournalDirectory))
	if err != nil {
		slog.Info("systemd.unit.logs failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}
	defer reader.Close()

	entries, err := reader.Query(ctx, journal.Query{
		MatchUnit:   unit,
		PriorityMax: -1,
		Limit:       limit,
	})
	if err != nil {
		slog.Info("systemd.unit.logs failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"unit":          unit,
		"journal_files": files,
		"entries":       journalEntries(entries),
		"count":         len(entries),
	}
	slog.Info("systemd.unit.logs succeeded", "capability", t.Name(), "unit", unit, "count", len(entries))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*systemdUnitLogsTask)(nil)
