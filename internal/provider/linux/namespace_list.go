package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// namespaceListTask implements the linux.namespace.list capability.
type namespaceListTask struct{ provider *Provider }

const namespaceListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.namespace.list Parameters",
  "description": "Parameters for linux.namespace.list.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID to list namespaces for (defaults to self)"
    }
  }
}`

func (t *namespaceListTask) Name() string       { return "linux.namespace.list" }
func (t *namespaceListTask) JSONSchema() string { return namespaceListSchema }

func (t *namespaceListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("namespace.list starting", "capability", t.Name())

	pid, _ := common.OptionalInt(params, "pid", 0)
	if pid <= 0 {
		pid = 1
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	namespaces, err := readNamespaceList(t.provider.CurrentReader(), pid)
	if err != nil {
		slog.Info("namespace.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("namespace.list succeeded", "capability", t.Name(), "count", len(namespaces))
	return common.SuccessResult(map[string]any{"namespaces": namespaces}), nil
}

var _ task.Task = (*namespaceListTask)(nil)

type NamespaceEntry struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func readNamespaceList(r *procfs.Reader, pid int) ([]NamespaceEntry, error) {
	nsDir := r.Path("proc", strconv.Itoa(pid), "ns")
	entries, err := os.ReadDir(nsDir)
	if err != nil {
		return nil, fmt.Errorf("read /proc/%d/ns: %w", pid, err)
	}

	var result []NamespaceEntry
	for _, entry := range entries {
		name := entry.Name()
		if name == "" {
			continue
		}
		// Resolve symlink to get the NS ID
		link, err := os.Readlink(filepath.Join(nsDir, name))
		if err != nil {
			continue
		}
		// link is like "net:[4026531840]"
		result = append(result, NamespaceEntry{
			Type: name,
			ID:   link,
		})
	}
	return result, nil
}
