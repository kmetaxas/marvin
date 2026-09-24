package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// swapListTask implements the linux.swap.list capability.
type swapListTask struct{ provider *Provider }

const swapListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.swap.list Parameters",
  "description": "Parameters for linux.swap.list."
}`

func (t *swapListTask) Name() string       { return "linux.swap.list" }
func (t *swapListTask) JSONSchema() string { return swapListSchema }

func (t *swapListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("swap.list starting", "capability", t.Name())

	data, err := readSwapList(t.provider.CurrentReader())
	if err != nil {
		slog.Info("swap.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("swap.list succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*swapListTask)(nil)

type SwapEntry struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	Size     uint64 `json:"size"`
	Used     uint64 `json:"used"`
	Priority int    `json:"priority"`
}

func readSwapList(r *procfs.Reader) (map[string]any, error) {
	lines, err := r.ReadFileLines("proc", "swaps")
	if err != nil {
		return nil, fmt.Errorf("read /proc/swaps: %w", err)
	}

	var entries []SwapEntry
	for i, line := range lines {
		if i == 0 {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		entry := SwapEntry{
			Filename: fields[0],
			Type:     fields[1],
		}
		entry.Size, _ = strconv.ParseUint(fields[2], 10, 64)
		entry.Used, _ = strconv.ParseUint(fields[3], 10, 64)
		entry.Priority, _ = strconv.Atoi(fields[4])
		entries = append(entries, entry)
	}

	return map[string]any{
		"swaps": entries,
		"count": len(entries),
	}, nil
}
