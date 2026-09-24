package linux

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// journalQueryTask implements the linux.journal.query capability.
type journalQueryTask struct{ provider *Provider }

const journalQuerySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.journal.query Parameters",
  "description": "Parameters for linux.journal.query.",
  "properties": {
    "service": {
      "type": "string",
      "description": "Systemd service unit name filter"
    },
    "priority": {
      "type": "string",
      "description": "Priority filter (e.g. err, warning)"
    },
    "since": {
      "type": "string",
      "description": "ISO 8601 timestamp start"
    },
    "until": {
      "type": "string",
      "description": "ISO 8601 timestamp end"
    }
  }
}`

func (t *journalQueryTask) Name() string       { return "linux.journal.query" }
func (t *journalQueryTask) JSONSchema() string { return journalQuerySchema }

func (t *journalQueryTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("journal.query starting", "capability", t.Name())

	service, err := common.OptionalString(params, "service", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	priority, err := common.OptionalString(params, "priority", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	since, err := common.OptionalString(params, "since", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	until, err := common.OptionalString(params, "until", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	cfg := t.provider.CurrentConfig()
	files, err := listJournalFiles(t.provider.CurrentReader(), cfg.JournalDirectory, service)
	if err != nil {
		slog.Info("journal.query failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"files":    files,
		"count":    len(files),
		"service":  service,
		"priority": priority,
		"since":    since,
		"until":    until,
		"note":     "Full journal entry parsing requires a journal reader library; this capability lists available journal files only.",
	}
	slog.Info("journal.query succeeded", "capability", t.Name(), "count", len(files))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*journalQueryTask)(nil)

// journalFile describes a journal file discovered on disk.
type journalFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time,omitempty"`
}

// listJournalFiles scans the journal directory for .journal and .journal~
// files, optionally filtering by a service name. It does not parse the binary
// journal format.
func listJournalFiles(r *procfs.Reader, journalDir, service string) ([]journalFile, error) {
	if journalDir == "" {
		journalDir = "/var/log/journal"
	}

	var files []journalFile

	collect := func(base string) {
		machineIDs, err := r.ReadDirNames(base)
		if err != nil {
			return
		}
		for _, machineID := range machineIDs {
			machineDir := filepath.Join(base, machineID)
			if !r.IsDir(machineDir) {
				continue
			}
			names, err := r.ReadDirNames(machineDir)
			if err != nil {
				continue
			}
			for _, name := range names {
				if !strings.HasSuffix(name, ".journal") && !strings.HasSuffix(name, ".journal~") {
					continue
				}
				if service != "" && !strings.Contains(name, service) {
					continue
				}
				fullPath := r.Path(machineDir, name)
				info, err := os.Stat(fullPath)
				if err != nil {
					continue
				}
				files = append(files, journalFile{
					Name:    name,
					Path:    fullPath,
					Size:    info.Size(),
					ModTime: info.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00"),
				})
			}
		}
	}

	collect(journalDir)
	collect("/run/log/journal")

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	return files, nil
}
