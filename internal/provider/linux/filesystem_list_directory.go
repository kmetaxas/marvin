package linux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// filesystemListDirectoryTask implements the linux.filesystem.list_directory capability.
type filesystemListDirectoryTask struct{ provider *Provider }

const filesystemListDirectorySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.filesystem.list_directory Parameters",
  "description": "Parameters for linux.filesystem.list_directory.",
  "properties": {
    "path": {
      "type": "string",
      "description": "Absolute path of the directory to list."
    }
  },
  "required": ["path"]
}`

func (t *filesystemListDirectoryTask) Name() string       { return "linux.filesystem.list_directory" }
func (t *filesystemListDirectoryTask) JSONSchema() string { return filesystemListDirectorySchema }

func (t *filesystemListDirectoryTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	path, err := common.RequireString(params, "path")
	if err != nil {
		return common.TaskFailure(err)
	}

	cfg := t.provider.CurrentConfig()
	if err := ValidateReadPath(path, cfg.AllowedReadPaths); err != nil {
		return common.TaskFailure(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return common.TaskFailure(fmt.Errorf("stat path: %w", err))
	}
	if !info.IsDir() {
		return common.TaskFailure(fmt.Errorf("path %q is not a directory", path))
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return common.TaskFailure(fmt.Errorf("read directory: %w", err))
	}

	maxDepth := cfg.MaxDirectoryDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	maxEntries := cfg.MaxDirectoryEntries
	if maxEntries <= 0 {
		maxEntries = 1000
	}

	items := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if len(items) >= maxEntries {
			break
		}
		item := map[string]any{
			"name": entry.Name(),
			"path": filepath.Join(path, entry.Name()),
		}
		fi, err := entry.Info()
		if err == nil {
			item["size"] = fi.Size()
			item["mode"] = fi.Mode().String()
			item["is_dir"] = fi.IsDir()
			item["modified"] = fi.ModTime().UTC()
		}
		// For directories, include child count if within depth limit.
		if entry.IsDir() && maxDepth > 1 {
			childPath := filepath.Join(path, entry.Name())
			childEntries, err := os.ReadDir(childPath)
			if err == nil {
				childCount := len(childEntries)
				if childCount > maxEntries {
					childCount = maxEntries
					item["children_truncated"] = true
				}
				item["children_count"] = childCount
			}
		}
		items = append(items, item)
	}

	data := map[string]any{
		"path":    path,
		"entries": items,
		"count":   len(items),
	}
	if len(entries) > maxEntries {
		data["truncated"] = true
		data["total"] = len(entries)
	}

	return common.SuccessResult(data), nil
}

var _ task.Task = (*filesystemListDirectoryTask)(nil)
