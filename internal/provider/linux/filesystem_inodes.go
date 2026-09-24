package linux

import (
	"context"
	"fmt"
	"syscall"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// filesystemInodesTask implements the linux.filesystem.inodes capability.
type filesystemInodesTask struct{ provider *Provider }

const filesystemInodesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.filesystem.inodes Parameters",
  "description": "Parameters for linux.filesystem.inodes.",
  "properties": {},
  "required": []
}`

func (t *filesystemInodesTask) Name() string       { return "linux.filesystem.inodes" }
func (t *filesystemInodesTask) JSONSchema() string { return filesystemInodesSchema }

func (t *filesystemInodesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	mounts, err := readMounts()
	if err != nil {
		return common.TaskFailure(fmt.Errorf("read mounts: %w", err))
	}

	var results []map[string]any
	for _, m := range mounts {
		var total, free, used uint64
		var usedPercent float64
		var stat syscall.Statfs_t
		if err := syscall.Statfs(m.Target, &stat); err == nil {
			total = stat.Files
			free = stat.Ffree
			if total > free {
				used = total - free
			}
			if total > 0 {
				usedPercent = float64(used) * 100.0 / float64(total)
			}
		}

		results = append(results, map[string]any{
			"source":          m.Source,
			"target":          m.Target,
			"filesystem_type": m.FilesystemType,
			"total_inodes":    total,
			"free_inodes":     free,
			"used_inodes":     used,
			"used_percent":    usedPercent,
		})
	}

	return common.SuccessResult(map[string]any{
		"filesystems": results,
		"count":       len(results),
	}), nil
}

var _ task.Task = (*filesystemInodesTask)(nil)
