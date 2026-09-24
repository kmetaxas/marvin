package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// processThreadsTask implements the linux.process.threads capability.
type processThreadsTask struct{ provider *Provider }

const processThreadsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.process.threads Parameters",
  "description": "Parameters for linux.process.threads.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID"
    }
  },
  "required": ["pid"]
}`

func (t *processThreadsTask) Name() string       { return "linux.process.threads" }
func (t *processThreadsTask) JSONSchema() string { return processThreadsSchema }

func (t *processThreadsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("process.threads starting", "capability", t.Name())

	pid, err := common.OptionalInt(params, "pid", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	threads, err := readProcessThreads(t.provider.CurrentReader(), pid)
	if err != nil {
		slog.Info("process.threads failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("process.threads succeeded", "capability", t.Name(), "count", len(threads))
	return common.SuccessResult(map[string]any{"threads": threads}), nil
}

var _ task.Task = (*processThreadsTask)(nil)

// ThreadInfo represents a thread belonging to a process.
type ThreadInfo struct {
	TID   int    `json:"tid"`
	PID   int    `json:"pid"`
	State string `json:"state"`
	Name  string `json:"name,omitempty"`
	UTime int64  `json:"utime"`
	STime int64  `json:"stime"`
}

func readProcessThreads(r *procfs.Reader, pid int) ([]ThreadInfo, error) {
	pidStr := strconv.Itoa(pid)
	entries, err := r.ReadDirNames("proc", pidStr, "task")
	if err != nil {
		return nil, fmt.Errorf("read task directory: %w", err)
	}

	var threads []ThreadInfo
	for _, entry := range entries {
		tid, err := strconv.Atoi(entry)
		if err != nil {
			continue
		}

		statLine, err := r.ReadFileString("proc", pidStr, "task", entry, "stat")
		if err != nil {
			continue
		}

		stat, err := ParseProcStat(statLine)
		if err != nil {
			continue
		}

		state := extractState(statLine)
		comm, _ := stat["comm"].(string)
		utime, _ := stat["utime"].(int64)
		stime, _ := stat["stime"].(int64)

		threads = append(threads, ThreadInfo{
			TID:   tid,
			PID:   pid,
			State: state,
			Name:  comm,
			UTime: utime,
			STime: stime,
		})
	}

	return threads, nil
}
