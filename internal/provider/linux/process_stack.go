package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// processStackTask implements the linux.process.stack capability.
type processStackTask struct{ provider *Provider }

const processStackSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.process.stack Parameters",
  "description": "Parameters for linux.process.stack.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID"
    }
  },
  "required": ["pid"]
}`

func (t *processStackTask) Name() string       { return "linux.process.stack" }
func (t *processStackTask) JSONSchema() string { return processStackSchema }

func (t *processStackTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("process.stack starting", "capability", t.Name())

	pid, err := common.OptionalInt(params, "pid", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	stack, err := readProcessStack(t.provider.CurrentReader(), pid)
	if err != nil {
		slog.Info("process.stack failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("process.stack succeeded", "capability", t.Name())
	return common.SuccessResult(map[string]any{"stack": stack}), nil
}

var _ task.Task = (*processStackTask)(nil)

// StackEntry represents a single stack frame.
type StackEntry struct {
	Frame    int    `json:"frame"`
	Function string `json:"function"`
	Raw      string `json:"raw"`
}

func readProcessStack(r *procfs.Reader, pid int) ([]StackEntry, error) {
	pidStr := strconv.Itoa(pid)
	data, err := r.ReadFileString("proc", pidStr, "stack")
	if err != nil {
		// Handle permission denied gracefully
		if os.IsPermission(err) {
			return nil, fmt.Errorf("insufficient privileges to read stack for process %d (requires CAP_SYS_PTRACE or root)", pid)
		}
		return nil, fmt.Errorf("read stack: %w", err)
	}

	return parseStack(data), nil
}

func parseStack(data string) []StackEntry {
	lines := strings.Split(data, "\n")
	var entries []StackEntry
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "[<function_name>+0x<offset>/<size>]"
		function := line
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			function = line[1 : len(line)-1]
		}

		entries = append(entries, StackEntry{
			Frame:    i,
			Function: function,
			Raw:      line,
		})
	}
	return entries
}
