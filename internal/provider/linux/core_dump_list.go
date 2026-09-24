package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// coreDumpListTask implements the linux.core_dump.list capability.
type coreDumpListTask struct{ provider *Provider }

const coreDumpListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.core_dump.list Parameters",
  "description": "Parameters for linux.core_dump.list."
}`

func (t *coreDumpListTask) Name() string       { return "linux.core_dump.list" }
func (t *coreDumpListTask) JSONSchema() string { return coreDumpListSchema }

func (t *coreDumpListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("core_dump.list starting", "capability", t.Name())

	entries, err := readCoreDumpList()
	if err != nil {
		slog.Info("core_dump.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("core_dump.list succeeded", "capability", t.Name(), "count", len(entries))
	return common.SuccessResult(map[string]any{"dumps": entries}), nil
}

var _ task.Task = (*coreDumpListTask)(nil)

type CoreDumpEntry struct {
	PID       int    `json:"pid"`
	UID       int    `json:"uid"`
	GID       int    `json:"gid"`
	Signal    int    `json:"signal"`
	Timestamp string `json:"timestamp"`
	Exe       string `json:"exe,omitempty"`
	Size      int64  `json:"size,omitempty"`
}

func readCoreDumpList() ([]CoreDumpEntry, error) {
	var entries []CoreDumpEntry
	var messages []string

	// Try systemd-coredump storage paths
	paths := []string{
		"/var/lib/systemd/coredump",
		"/var/lib/abrt",
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			continue
		}
		files, err := os.ReadDir(p)
		if err != nil {
			messages = append(messages, fmt.Sprintf("cannot read %s: %v", p, err))
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			fi, err := f.Info()
			if err != nil {
				continue
			}
			entries = append(entries, CoreDumpEntry{
				Timestamp: fi.ModTime().Format("2006-01-02T15:04:05Z"),
				Size:      fi.Size(),
			})
		}
	}

	if len(entries) == 0 {
		messages = append(messages, "no coredump entries found in known paths")
	}

	return entries, nil
}
