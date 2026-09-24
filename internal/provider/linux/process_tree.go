package linux

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// processTreeTask implements the linux.process.tree capability.
type processTreeTask struct{ provider *Provider }

const processTreeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.process.tree Parameters",
  "description": "Parameters for linux.process.tree.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Root PID for tree (default: 1)"
    },
    "user": {
      "type": "string",
      "description": "Filter by username"
    }
  }
}`

func (t *processTreeTask) Name() string       { return "linux.process.tree" }
func (t *processTreeTask) JSONSchema() string { return processTreeSchema }

func (t *processTreeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("process.tree starting", "capability", t.Name())

	rootPID, _ := common.OptionalInt(params, "pid", 1)
	if rootPID <= 0 {
		rootPID = 1
	}
	userFilter, _ := common.OptionalString(params, "user", "")

	tree, err := buildProcessTree(t.provider.CurrentReader(), rootPID, userFilter)
	if err != nil {
		slog.Info("process.tree failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("process.tree succeeded", "capability", t.Name())
	return common.SuccessResult(tree), nil
}

var _ task.Task = (*processTreeTask)(nil)

// ProcessTreeNode represents a node in the process tree.
type ProcessTreeNode struct {
	PID      int               `json:"pid"`
	PPID     int               `json:"ppid"`
	User     string            `json:"user,omitempty"`
	State    string            `json:"state"`
	Command  string            `json:"command"`
	Children []ProcessTreeNode `json:"children,omitempty"`
}

func buildProcessTree(r *procfs.Reader, rootPID int, userFilter string) (map[string]any, error) {
	entries, err := r.ReadDirNames("proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	type procInfo struct {
		pid     int
		ppid    int
		user    string
		state   string
		command string
	}

	processes := make(map[int]procInfo)
	children := make(map[int][]int)

	for _, entry := range entries {
		pid, ok := parsePIDDir(entry)
		if !ok {
			continue
		}

		summary, ok := readProcessSummary(r, pid)
		if !ok {
			continue
		}

		if userFilter != "" && !strings.EqualFold(summary.User, userFilter) {
			continue
		}

		info := procInfo{
			pid:     summary.PID,
			ppid:    summary.PPID,
			user:    summary.User,
			state:   summary.State,
			command: summary.Command,
		}
		processes[pid] = info
		children[info.ppid] = append(children[info.ppid], pid)
	}

	// Sort children by PID for stable output
	for ppid := range children {
		sort.Ints(children[ppid])
	}

	// Build tree recursively
	var buildNode func(pid int) *ProcessTreeNode
	buildNode = func(pid int) *ProcessTreeNode {
		info, ok := processes[pid]
		if !ok {
			return nil
		}
		node := &ProcessTreeNode{
			PID:     info.pid,
			PPID:    info.ppid,
			User:    info.user,
			State:   info.state,
			Command: info.command,
		}
		for _, childPID := range children[pid] {
			child := buildNode(childPID)
			if child != nil {
				node.Children = append(node.Children, *child)
			}
		}
		return node
	}

	root := buildNode(rootPID)
	if root == nil {
		return nil, fmt.Errorf("root process %d not found", rootPID)
	}

	return map[string]any{
		"root": *root,
	}, nil
}
