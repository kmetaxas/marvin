package linux

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// filesystemListTask implements the linux.filesystem.list capability.
type filesystemListTask struct{ provider *Provider }

const filesystemListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.filesystem.list Parameters",
  "description": "Parameters for linux.filesystem.list.",
  "properties": {},
  "required": []
}`

func (t *filesystemListTask) Name() string       { return "linux.filesystem.list" }
func (t *filesystemListTask) JSONSchema() string { return filesystemListSchema }

func (t *filesystemListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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
			bsize := uint64(stat.Bsize)
			total = stat.Blocks * bsize
			free = stat.Bavail * bsize
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
			"options":         m.Options,
			"total_bytes":     total,
			"free_bytes":      free,
			"used_bytes":      used,
			"used_percent":    usedPercent,
		})
	}

	return common.SuccessResult(map[string]any{
		"filesystems": results,
		"count":       len(results),
	}), nil
}

// mountEntry holds a parsed line from /proc/mounts.
type mountEntry struct {
	Source         string
	Target         string
	FilesystemType string
	Options        []string
}

func readMounts() ([]mountEntry, error) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil, err
	}
	return parseMountsContent(string(data)), nil
}

func parseMountsContent(content string) []mountEntry {
	lines := strings.Split(content, "\n")
	var entries []mountEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		entries = append(entries, mountEntry{
			Source:         fields[0],
			Target:         fields[1],
			FilesystemType: fields[2],
			Options:        strings.Split(fields[3], ","),
		})
	}
	return entries
}

var _ task.Task = (*filesystemListTask)(nil)
