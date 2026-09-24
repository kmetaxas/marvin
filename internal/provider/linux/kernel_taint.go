package linux

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// kernelTaintTask implements the linux.kernel.taint capability.
type kernelTaintTask struct{ provider *Provider }

const kernelTaintSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.kernel.taint Parameters",
  "description": "Parameters for linux.kernel.taint."
}`

func (t *kernelTaintTask) Name() string       { return "linux.kernel.taint" }
func (t *kernelTaintTask) JSONSchema() string { return kernelTaintSchema }

func (t *kernelTaintTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("kernel.taint starting", "capability", t.Name())

	data, err := readKernelTaint(t.provider.CurrentReader())
	if err != nil {
		slog.Info("kernel.taint failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("kernel.taint succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*kernelTaintTask)(nil)

var taintFlags = []struct {
	Bit  uint
	Name string
}{
	{0, "proprietary_module"},
	{1, "non_gpl_module"},
	{2, "unsafe_smp"},
	{3, "forced_rmmod"},
	{4, "machine_check"},
	{5, "bad_page"},
	{6, "user"},
	{7, "die"},
	{8, "acpi_overridden"},
	{9, "warn"},
	{10, "staging_driver"},
	{11, "firmware_workaround"},
	{12, "out_of_tree_module"},
	{13, "unsigned_module"},
	{14, "softlockup"},
	{15, "livepatch"},
	{16, "aux_taint"},
	{17, "randstruct"},
	{18, "test"},
}

func readKernelTaint(r *procfs.Reader) (map[string]any, error) {
	val, err := r.ReadUint64("proc", "sys", "kernel", "tainted")
	if err != nil {
		return nil, fmt.Errorf("read /proc/sys/kernel/tainted: %w", err)
	}

	var active []string
	for _, flag := range taintFlags {
		if val&(1<<flag.Bit) != 0 {
			active = append(active, flag.Name)
		}
	}

	return map[string]any{
		"tainted":    val,
		"flags":      active,
		"flag_count": len(active),
		"is_tainted": len(active) > 0,
	}, nil
}
