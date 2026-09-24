package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// kernelDmesgTask implements the linux.kernel.dmesg capability.
type kernelDmesgTask struct{ provider *Provider }

const kernelDmesgSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.kernel.dmesg Parameters",
  "description": "Parameters for linux.kernel.dmesg.",
  "properties": {
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 100,
      "description": "Maximum number of messages to return."
    },
    "filter": {
      "type": "string",
      "description": "Optional substring filter for messages."
    }
  }
}`

func (t *kernelDmesgTask) Name() string       { return "linux.kernel.dmesg" }
func (t *kernelDmesgTask) JSONSchema() string { return kernelDmesgSchema }

func (t *kernelDmesgTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("kernel.dmesg starting", "capability", t.Name())

	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	filter, _ := common.OptionalString(params, "filter", "")

	data, err := readKernelDmesg(t.provider.CurrentReader(), limit, filter)
	if err != nil {
		slog.Info("kernel.dmesg failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("kernel.dmesg succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*kernelDmesgTask)(nil)

func readKernelDmesg(r *procfs.Reader, limit int, filter string) (map[string]any, error) {
	raw, err := r.ReadFileString("dev", "kmsg")
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("unable to read /dev/kmsg: %w (requires CAP_SYSLOG or root privileges)", err)
		}
		return nil, fmt.Errorf("read /dev/kmsg: %w", err)
	}

	var messages []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if filter != "" && !strings.Contains(line, filter) {
			continue
		}
		messages = append(messages, line)
		if len(messages) >= limit {
			break
		}
	}

	return map[string]any{
		"messages": messages,
		"count":    len(messages),
		"source":   "/dev/kmsg",
	}, nil
}
