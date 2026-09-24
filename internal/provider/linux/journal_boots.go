package linux

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// journalBootsTask implements the linux.journal.boots capability.
type journalBootsTask struct{ provider *Provider }

const journalBootsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.journal.boots Parameters",
  "description": "Parameters for linux.journal.boots."
}`

func (t *journalBootsTask) Name() string       { return "linux.journal.boots" }
func (t *journalBootsTask) JSONSchema() string { return journalBootsSchema }

func (t *journalBootsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("journal.boots starting", "capability", t.Name())

	cfg := t.provider.CurrentConfig()
	boots, err := listJournalBoots(t.provider.CurrentReader(), cfg.JournalDirectory)
	if err != nil {
		slog.Info("journal.boots failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
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
	ID        string `json:"id"`
	Timestamp string `json:"timestamp,omitempty"`
	Source    string `json:"source,omitempty"`
}

// listJournalBoots lists boot IDs by scanning the journal directory layout.
// Each subdirectory under <journalDir>/<machine-id>/ is a boot ID. It also
// checks /run/log/journal/ for runtime journals.
func listJournalBoots(r *procfs.Reader, journalDir string) ([]journalBoot, error) {
	if journalDir == "" {
		journalDir = "/var/log/journal"
	}

	var boots []journalBoot
	seen := make(map[string]bool)

	collect := func(base string) {
		machineIDs, err := r.ReadDirNames(base)
		if err != nil {
			return
		}
		for _, machineID := range machineIDs {
			bootDir := filepath.Join(base, machineID)
			if !r.IsDir(bootDir) {
				continue
			}
			bootIDs, err := r.ReadDirNames(bootDir)
			if err != nil {
				continue
			}
			for _, bootID := range bootIDs {
				if seen[bootID] {
					continue
				}
				seen[bootID] = true

				boot := journalBoot{ID: bootID, Source: base}
				if info, err := os.Stat(r.Path(bootDir, bootID)); err == nil {
					boot.Timestamp = info.ModTime().UTC().Format(time.RFC3339)
				}
				boots = append(boots, boot)
			}
		}
	}

	collect(journalDir)
	collect("/run/log/journal")

	sort.Slice(boots, func(i, j int) bool {
		return boots[i].ID < boots[j].ID
	})

	return boots, nil
}
