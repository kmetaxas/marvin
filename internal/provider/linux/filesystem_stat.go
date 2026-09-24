package linux

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// filesystemStatTask implements the linux.filesystem.stat capability.
type filesystemStatTask struct{ provider *Provider }

const filesystemStatSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.filesystem.stat Parameters",
  "description": "Parameters for linux.filesystem.stat.",
  "properties": {
    "path": {
      "type": "string",
      "description": "Absolute path to stat."
    }
  },
  "required": ["path"]
}`

func (t *filesystemStatTask) Name() string       { return "linux.filesystem.stat" }
func (t *filesystemStatTask) JSONSchema() string { return filesystemStatSchema }

func (t *filesystemStatTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	mode := info.Mode()
	data := map[string]any{
		"path":        path,
		"exists":      true,
		"size":        info.Size(),
		"mode":        mode.String(),
		"is_dir":      mode.IsDir(),
		"is_regular":  mode.IsRegular(),
		"is_symlink":  mode.Type()&os.ModeSymlink != 0,
		"modified":    info.ModTime().UTC(),
		"permissions": fmt.Sprintf("0%o", mode.Perm()),
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		data["uid"] = stat.Uid
		data["gid"] = stat.Gid
		data["inode"] = stat.Ino
		data["hard_links"] = stat.Nlink
		data["device"] = stat.Dev
		data["block_size"] = stat.Blksize
		data["blocks"] = stat.Blocks
		data["access_time"] = timespecToTime(stat.Atim)
		data["modify_time"] = timespecToTime(stat.Mtim)
		data["change_time"] = timespecToTime(stat.Ctim)
	}

	return common.SuccessResult(data), nil
}

func timespecToTime(ts syscall.Timespec) interface{} {
	return time.Unix(ts.Sec, ts.Nsec).UTC()
}

var _ task.Task = (*filesystemStatTask)(nil)
