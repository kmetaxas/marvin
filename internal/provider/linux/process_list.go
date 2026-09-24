package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// processListTask implements the linux.process.list capability.
type processListTask struct{ provider *Provider }

const processListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.process.list Parameters",
  "description": "Parameters for linux.process.list.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Filter by specific PID"
    },
    "user": {
      "type": "string",
      "description": "Filter by username"
    },
    "name": {
      "type": "string",
      "description": "Filter by process name (substring match)"
    },
    "sort": {
      "type": "string",
      "enum": ["pid", "cpu", "mem", "name"],
      "description": "Sort field",
      "default": "pid"
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of results",
      "default": 100
    }
  }
}`

func (t *processListTask) Name() string       { return "linux.process.list" }
func (t *processListTask) JSONSchema() string { return processListSchema }

func (t *processListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("process.list starting", "capability", t.Name())

	data, err := t.runProcessList(params)
	if err != nil {
		slog.Info("process.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("process.list succeeded", "capability", t.Name(), "count", len(data))
	return common.SuccessResult(map[string]any{"processes": data}), nil
}

var _ task.Task = (*processListTask)(nil)

func (t *processListTask) runProcessList(params map[string]any) ([]ProcessSummary, error) {
	r := t.provider.CurrentReader()

	pidFilter, _ := common.OptionalInt(params, "pid", 0)
	userFilter, _ := common.OptionalString(params, "user", "")
	nameFilter, _ := common.OptionalString(params, "name", "")
	sortField, _ := common.OptionalString(params, "sort", "pid")
	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return nil, err
	}

	entries, err := r.ReadDirNames("proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	var processes []ProcessSummary
	nameRe := compileNameFilter(nameFilter)

	for _, entry := range entries {
		pid, ok := parsePIDDir(entry)
		if !ok {
			continue
		}
		if pidFilter > 0 && pid != pidFilter {
			continue
		}

		summary, ok := readProcessSummary(r, pid)
		if !ok {
			continue
		}

		if userFilter != "" && !strings.EqualFold(summary.User, userFilter) {
			continue
		}
		if nameRe != nil && !nameRe.MatchString(summary.Command) {
			continue
		}

		processes = append(processes, summary)
	}

	sortProcesses(processes, sortField)
	if len(processes) > limit {
		processes = processes[:limit]
	}

	return processes, nil
}

func compileNameFilter(name string) *regexp.Regexp {
	if name == "" {
		return nil
	}
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(name))
	if err != nil {
		return nil
	}
	return re
}

func parsePIDDir(name string) (int, bool) {
	pid, err := strconv.Atoi(name)
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func readProcessSummary(r *procfs.Reader, pid int) (ProcessSummary, bool) {
	statLine, err := r.ReadFileString("proc", strconv.Itoa(pid), "stat")
	if err != nil {
		return ProcessSummary{}, false
	}

	stat, err := ParseProcStat(statLine)
	if err != nil {
		return ProcessSummary{}, false
	}

	ppid, _ := stat["ppid"].(int64)
	state, _ := stat["state"].(string)
	if state == "" {
		state, _ = stat["state"].(string)
	}
	if state == "" {
		// state is not numeric; it is a single character parsed as string
		// ParseProcStat stores it as the raw field value. Re-extract.
		fields := strings.Fields(statLine)
		if len(fields) > 2 {
			state = fields[2]
		}
	}

	comm, _ := stat["comm"].(string)

	// Command line is more descriptive than comm
	cmdline, _ := r.ReadFileString("proc", strconv.Itoa(pid), "cmdline")
	command := strings.ReplaceAll(cmdline, "\x00", " ")
	command = strings.TrimSpace(command)
	if command == "" {
		command = comm
	}

	uid := ""
	statusLines, _ := r.ReadFileLines("proc", strconv.Itoa(pid), "status")
	for _, line := range statusLines {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uidVal, _ := strconv.Atoi(fields[1])
				if u, err := user.LookupId(strconv.Itoa(uidVal)); err == nil {
					uid = u.Username
				} else {
					uid = strconv.Itoa(uidVal)
				}
			}
			break
		}
	}

	return ProcessSummary{
		PID:     pid,
		PPID:    int(ppid),
		User:    uid,
		State:   state,
		Command: command,
	}, true
}

func sortProcesses(processes []ProcessSummary, field string) {
	switch field {
	case "cpu":
		// CPU percent not populated in list; fallback to pid
		fallthrough
	case "pid":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].PID < processes[j].PID
		})
	case "mem":
		// Memory percent not populated in list; fallback to name
		fallthrough
	case "name":
		sort.Slice(processes, func(i, j int) bool {
			return strings.ToLower(processes[i].Command) < strings.ToLower(processes[j].Command)
		})
	default:
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].PID < processes[j].PID
		})
	}
}
