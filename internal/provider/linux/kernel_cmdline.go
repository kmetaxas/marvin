package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// kernelCmdlineTask implements the linux.kernel.cmdline capability.
type kernelCmdlineTask struct{ provider *Provider }

const kernelCmdlineSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.kernel.cmdline Parameters",
  "description": "Parameters for linux.kernel.cmdline."
}`

func (t *kernelCmdlineTask) Name() string       { return "linux.kernel.cmdline" }
func (t *kernelCmdlineTask) JSONSchema() string { return kernelCmdlineSchema }

func (t *kernelCmdlineTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("kernel.cmdline starting", "capability", t.Name())

	data, err := readKernelCmdline(t.provider.CurrentReader())
	if err != nil {
		slog.Info("kernel.cmdline failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("kernel.cmdline succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*kernelCmdlineTask)(nil)

func readKernelCmdline(r *procfs.Reader) (map[string]any, error) {
	raw, err := r.ReadFileString("proc", "cmdline")
	if err != nil {
		return nil, fmt.Errorf("read /proc/cmdline: %w", err)
	}
	raw = strings.TrimSpace(raw)

	var args []string
	if strings.Contains(raw, "\x00") {
		for _, p := range strings.Split(raw, "\x00") {
			if p != "" {
				args = append(args, p)
			}
		}
	} else {
		for _, p := range strings.Fields(raw) {
			if p != "" {
				args = append(args, p)
			}
		}
	}

	return map[string]any{
		"raw":   raw,
		"args":  args,
		"count": len(args),
	}, nil
}
