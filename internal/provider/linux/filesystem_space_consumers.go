package linux

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// filesystemSpaceConsumersTask implements the linux.filesystem.space_consumers capability.
type filesystemSpaceConsumersTask struct{ provider *Provider }

const filesystemSpaceConsumersSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.filesystem.space_consumers Parameters",
  "description": "Parameters for linux.filesystem.space_consumers.",
  "properties": {
    "path": {
      "type": "string",
      "description": "Absolute path to start scanning."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of results to return."
    }
  },
  "required": ["path"]
}`

func (t *filesystemSpaceConsumersTask) Name() string       { return "linux.filesystem.space_consumers" }
func (t *filesystemSpaceConsumersTask) JSONSchema() string { return filesystemSpaceConsumersSchema }

type consumer struct {
	Path string `json:"path"`
	Size int64  `json:"size_bytes"`
}

func (t *filesystemSpaceConsumersTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	maxDepth := cfg.MaxDirectoryDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	limit, err := common.OptionalInt(params, "limit", 50)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	var consumers []consumer
	baseDepth := len(filepath.SplitList(path))
	if baseDepth == 0 {
		baseDepth = 0
	}
	// Use manual depth counting because filepath.SplitList doesn't split by separator.
	baseDepth = filepathDepth(path)

	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		currentDepth := filepathDepth(p)
		if currentDepth > baseDepth+maxDepth {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		consumers = append(consumers, consumer{Path: p, Size: fi.Size()})
		return nil
	})
	if err != nil {
		return common.TaskFailure(fmt.Errorf("walk directory: %w", err))
	}

	sort.Slice(consumers, func(i, j int) bool {
		return consumers[i].Size > consumers[j].Size
	})

	if len(consumers) > limit {
		consumers = consumers[:limit]
	}

	data := map[string]any{
		"path":      path,
		"consumers": consumers,
		"count":     len(consumers),
	}

	return common.SuccessResult(data), nil
}

func filepathDepth(p string) int {
	clean := filepath.Clean(p)
	parts := 0
	for _, c := range clean {
		if c == filepath.Separator {
			parts++
		}
	}
	return parts
}

var _ task.Task = (*filesystemSpaceConsumersTask)(nil)
