package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// processGetTask implements the linux.process.get capability.
type processGetTask struct{ provider *Provider }

const processGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.process.get Parameters",
  "description": "Parameters for linux.process.get.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID to query"
    }
  },
  "required": ["pid"]
}`

func (t *processGetTask) Name() string       { return "linux.process.get" }
func (t *processGetTask) JSONSchema() string { return processGetSchema }

func (t *processGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("process.get starting", "capability", t.Name())

	pid, err := common.OptionalInt(params, "pid", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	detail, err := readProcessDetail(t.provider.CurrentReader(), pid)
	if err != nil {
		slog.Info("process.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("process.get succeeded", "capability", t.Name(), "pid", pid)
	return common.SuccessResult(detail), nil
}

var _ task.Task = (*processGetTask)(nil)

func readProcessDetail(r *procfs.Reader, pid int) (ProcessDetail, error) {
	pidStr := strconv.Itoa(pid)
	procPath := filepath.Join("proc", pidStr)

	if !r.Exists(procPath) {
		return ProcessDetail{}, fmt.Errorf("process %d not found", pid)
	}

	// Parse stat
	statLine, err := r.ReadFileString("proc", pidStr, "stat")
	if err != nil {
		return ProcessDetail{}, fmt.Errorf("read stat: %w", err)
	}
	stat, err := ParseProcStat(statLine)
	if err != nil {
		return ProcessDetail{}, fmt.Errorf("parse stat: %w", err)
	}

	ppid, _ := stat["ppid"].(int64)
	state := extractState(statLine)
	comm, _ := stat["comm"].(string)
	threads, _ := stat["num_threads"].(int64)

	// Command line
	cmdline, _ := r.ReadFileString("proc", pidStr, "cmdline")
	command := strings.ReplaceAll(cmdline, "\x00", " ")
	command = strings.TrimSpace(command)
	if command == "" {
		command = comm
	}

	// CWD
	cwd := ""
	if cwdPath, err := filepath.EvalSymlinks(r.Path("proc", pidStr, "cwd")); err == nil {
		cwd = cwdPath
	}

	// Status parsing
	uid := ""
	var memRSS, memVMS uint64
	statusLines, _ := r.ReadFileLines("proc", pidStr, "status")
	for _, line := range statusLines {
		switch {
		case strings.HasPrefix(line, "Uid:"):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uidVal, _ := strconv.Atoi(fields[1])
				if u, err := user.LookupId(strconv.Itoa(uidVal)); err == nil {
					uid = u.Username
				} else {
					uid = strconv.Itoa(uidVal)
				}
			}
		case strings.HasPrefix(line, "VmRSS:"):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseUint(fields[1], 10, 64)
				memRSS = val * 1024 // kB to bytes
			}
		case strings.HasPrefix(line, "VmSize:"):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseUint(fields[1], 10, 64)
				memVMS = val * 1024 // kB to bytes
			}
		}
	}

	// Executable
	exe := ""
	if exePath, err := filepath.EvalSymlinks(r.Path("proc", pidStr, "exe")); err == nil {
		exe = exePath
	}

	return ProcessDetail{
		ProcessSummary: ProcessSummary{
			PID:     pid,
			PPID:    int(ppid),
			User:    uid,
			State:   state,
			Command: command,
		},
		Executable: exe,
		CWD:        cwd,
		Threads:    int(threads),
		Memory: MemoryInfo{
			RSS: memRSS,
			VMS: memVMS,
		},
	}, nil
}

func extractState(statLine string) string {
	// state is the third field after pid and comm
	start := strings.IndexByte(statLine, '(')
	end := strings.LastIndexByte(statLine, ')')
	if start == -1 || end == -1 {
		return ""
	}
	rest := strings.TrimSpace(statLine[end+1:])
	fields := strings.Fields(rest)
	if len(fields) > 0 {
		return fields[0]
	}
	return ""
}
