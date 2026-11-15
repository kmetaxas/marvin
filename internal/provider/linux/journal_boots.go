package linux

import (
	"context"
	"log/slog"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/journal"
	"github.com/marvin-agent/marvin/internal/task"
)

// journalBootsTask implements the linux.journal.boots capability.
type journalBootsTask struct{ provider *Provider }

const journalBootsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.journal.boots Parameters",
  "description": "Parameters for linux.journal.boots. Takes no parameters; returns boots known to the systemd journal."
}`

func (t *journalBootsTask) Name() string       { return "linux.journal.boots" }
func (t *journalBootsTask) JSONSchema() string { return journalBootsSchema }

func (t *journalBootsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = params
	slog.Info("journal.boots starting", "capability", t.Name())

	cfg := t.provider.CurrentConfig()
	reader, err := journal.Open(journalDirs(cfg.JournalDirectory))
	if err != nil {
		slog.Info("journal.boots failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}
	defer reader.Close()

	found, err := reader.Boots(ctx)
	if err != nil {
		slog.Info("journal.boots failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	boots := make([]journalBoot, 0, len(found))
	for _, b := range found {
		boot := journalBoot{ID: b.ID}
		if !b.FirstEntry.IsZero() {
			boot.FirstEntry = b.FirstEntry.UTC().Format(time.RFC3339)
		}
		if !b.LastEntry.IsZero() {
			boot.LastEntry = b.LastEntry.UTC().Format(time.RFC3339)
		}
		boots = append(boots, boot)
	}

	result := map[string]any{
		"boots": boots,
		"count": len(boots),
	}
	slog.Info("journal.boots succeeded", "capability", t.Name(), "count", len(boots))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*journalBootsTask)(nil)

// journalBoot describes a single boot known to journald.
type journalBoot struct {
	ID         string `json:"id"`
	FirstEntry string `json:"first_entry,omitempty"`
	LastEntry  string `json:"last_entry,omitempty"`
	Source     string `json:"source,omitempty"`
}
