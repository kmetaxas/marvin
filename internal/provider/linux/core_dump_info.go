package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// coreDumpInfoTask implements the linux.core_dump.info capability.
type coreDumpInfoTask struct{ provider *Provider }

const coreDumpInfoSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.core_dump.info Parameters",
  "description": "Parameters for linux.core_dump.info.",
  "properties": {
    "file": {
      "type": "string",
      "description": "Coredump file path or identifier"
    }
  },
  "required": ["file"]
}`

func (t *coreDumpInfoTask) Name() string       { return "linux.core_dump.info" }
func (t *coreDumpInfoTask) JSONSchema() string { return coreDumpInfoSchema }

func (t *coreDumpInfoTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("core_dump.info starting", "capability", t.Name())

	fileName, err := common.RequireString(params, "file")
	if err != nil {
		return common.TaskFailure(err)
	}

	info, err := readCoreDumpInfo(fileName)
	if err != nil {
		slog.Info("core_dump.info failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("core_dump.info succeeded", "capability", t.Name())
	return common.SuccessResult(info), nil
}

var _ task.Task = (*coreDumpInfoTask)(nil)

func readCoreDumpInfo(fileName string) (map[string]any, error) {
	// Try known coredump directories
	paths := []string{
		"/var/lib/systemd/coredump",
		"/var/lib/abrt",
	}

	for _, dir := range paths {
		fullPath := filepath.Join(dir, fileName)
		if !strings.HasPrefix(fullPath, dir) {
			continue
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		return map[string]any{
			"file":     fullPath,
			"size":     info.Size(),
			"mod_time": info.ModTime().Format("2006-01-02T15:04:05Z"),
			"exists":   true,
			"message":  "core dump file found; metadata extraction from systemd-coredump requires journal or coredumpctl",
		}, nil
	}

	return nil, fmt.Errorf("core dump %q not found in known paths", fileName)
}
