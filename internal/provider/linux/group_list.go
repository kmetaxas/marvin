package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// groupListTask implements the linux.group.list capability.
type groupListTask struct{ provider *Provider }

const groupListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.group.list Parameters",
  "description": "Parameters for linux.group.list."
}`

func (t *groupListTask) Name() string       { return "linux.group.list" }
func (t *groupListTask) JSONSchema() string { return groupListSchema }

func (t *groupListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("group.list starting", "capability", t.Name())

	groups, err := readGroupList()
	if err != nil {
		slog.Info("group.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("group.list succeeded", "capability", t.Name(), "count", len(groups))
	return common.SuccessResult(map[string]any{"groups": groups}), nil
}

var _ task.Task = (*groupListTask)(nil)

type GroupEntry struct {
	Name    string   `json:"name"`
	GID     int      `json:"gid"`
	Members []string `json:"members,omitempty"`
}

func readGroupList() ([]GroupEntry, error) {
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return nil, fmt.Errorf("read /etc/group: %w", err)
	}

	var groups []GroupEntry
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		gid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		members := []string{}
		if parts[3] != "" {
			members = strings.Split(parts[3], ",")
		}
		groups = append(groups, GroupEntry{
			Name:    parts[0],
			GID:     gid,
			Members: members,
		})
	}
	return groups, nil
}
