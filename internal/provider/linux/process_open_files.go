package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// processOpenFilesTask implements the linux.process.open_files capability.
type processOpenFilesTask struct{ provider *Provider }

const processOpenFilesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.process.open_files Parameters",
  "description": "Parameters for linux.process.open_files.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID"
    }
  },
  "required": ["pid"]
}`

func (t *processOpenFilesTask) Name() string       { return "linux.process.open_files" }
func (t *processOpenFilesTask) JSONSchema() string { return processOpenFilesSchema }

func (t *processOpenFilesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("process.open_files starting", "capability", t.Name())

	pid, err := common.OptionalInt(params, "pid", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	files, err := readOpenFiles(t.provider.CurrentReader(), pid)
	if err != nil {
		slog.Info("process.open_files failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("process.open_files succeeded", "capability", t.Name(), "count", len(files))
	return common.SuccessResult(map[string]any{"files": files}), nil
}

var _ task.Task = (*processOpenFilesTask)(nil)

// OpenFileInfo represents an open file descriptor.
type OpenFileInfo struct {
	FD       int    `json:"fd"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	RealPath string `json:"real_path,omitempty"`
}

func readOpenFiles(r *procfs.Reader, pid int) ([]OpenFileInfo, error) {
	pidStr := strconv.Itoa(pid)
	fdDir := filepath.Join("proc", pidStr, "fd")

	if !r.Exists(fdDir) {
		return nil, fmt.Errorf("process %d not found or no fd directory", pid)
	}

	entries, err := r.ReadDirNames(fdDir)
	if err != nil {
		return nil, fmt.Errorf("read fd directory: %w", err)
	}

	var files []OpenFileInfo
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry)
		if err != nil {
			continue
		}

		linkTarget, err := os.Readlink(r.Path(fdDir, entry))
		if err != nil {
			linkTarget = ""
		}

		fileType := "unknown"
		switch {
		case strings.HasPrefix(linkTarget, "pipe:"):
			fileType = "pipe"
		case strings.HasPrefix(linkTarget, "socket:"):
			fileType = "socket"
		case strings.HasPrefix(linkTarget, "anon_inode:"):
			fileType = "anon_inode"
		case linkTarget == "":
			fileType = "unknown"
		default:
			fileType = "file"
		}

		files = append(files, OpenFileInfo{
			FD:       fd,
			Path:     linkTarget,
			Type:     fileType,
			RealPath: linkTarget,
		})
	}

	return files, nil
}
