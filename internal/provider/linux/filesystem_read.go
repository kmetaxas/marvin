package linux

import (
	"context"
	"fmt"
	"os"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// filesystemReadTask implements the linux.filesystem.read capability.
type filesystemReadTask struct{ provider *Provider }

const filesystemReadSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.filesystem.read Parameters",
  "description": "Parameters for linux.filesystem.read.",
  "properties": {
    "path": {
      "type": "string",
      "description": "Absolute path of the file to read."
    },
    "offset": {
      "type": "integer",
      "description": "Optional byte offset to start reading from."
    }
  },
  "required": ["path"]
}`

func (t *filesystemReadTask) Name() string       { return "linux.filesystem.read" }
func (t *filesystemReadTask) JSONSchema() string { return filesystemReadSchema }

func (t *filesystemReadTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	path, err := common.RequireString(params, "path")
	if err != nil {
		return common.TaskFailure(err)
	}

	cfg := t.provider.CurrentConfig()
	if err := ValidateReadPath(path, cfg.AllowedReadPaths); err != nil {
		return common.TaskFailure(err)
	}

	offset, err := common.OptionalInt(params, "offset", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if offset < 0 {
		return common.TaskFailure(fmt.Errorf("offset must be non-negative"))
	}
	if cfg.MaxReadOffset > 0 && offset > cfg.MaxReadOffset {
		return common.TaskFailure(fmt.Errorf("offset %d exceeds max_read_offset %d", offset, cfg.MaxReadOffset))
	}

	f, err := os.Open(path)
	if err != nil {
		return common.TaskFailure(fmt.Errorf("open file: %w", err))
	}
	defer f.Close()

	if offset > 0 {
		_, err = f.Seek(int64(offset), 0)
		if err != nil {
			return common.TaskFailure(fmt.Errorf("seek file: %w", err))
		}
	}

	maxBytes := cfg.MaxReadBytes
	if maxBytes <= 0 {
		maxBytes = 65536
	}

	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return common.TaskFailure(fmt.Errorf("read file: %w", err))
	}

	content := string(buf[:n])
	truncated := false
	if n == maxBytes {
		// Peek to see if there's more data.
		extra := make([]byte, 1)
		m, _ := f.Read(extra)
		if m > 0 {
			truncated = true
		}
	}

	data := map[string]any{
		"path":       path,
		"content":    content,
		"bytes_read": n,
		"truncated":  truncated,
	}
	if offset > 0 {
		data["offset"] = offset
	}

	return common.SuccessResult(data), nil
}

var _ task.Task = (*filesystemReadTask)(nil)
