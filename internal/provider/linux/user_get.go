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

// userGetTask implements the linux.user.get capability.
type userGetTask struct{ provider *Provider }

const userGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.user.get Parameters",
  "description": "Parameters for linux.user.get.",
  "properties": {
    "user": {
      "type": "string",
      "description": "Username or UID to query"
    }
  },
  "required": ["user"]
}`

func (t *userGetTask) Name() string       { return "linux.user.get" }
func (t *userGetTask) JSONSchema() string { return userGetSchema }

func (t *userGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("user.get starting", "capability", t.Name())

	userName, err := common.RequireString(params, "user")
	if err != nil {
		return common.TaskFailure(err)
	}

	userEntry, groups, err := readUserGet(userName)
	if err != nil {
		slog.Info("user.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("user.get succeeded", "capability", t.Name(), "user", userEntry.Name)
	return common.SuccessResult(map[string]any{
		"user":   userEntry,
		"groups": groups,
	}), nil
}

var _ task.Task = (*userGetTask)(nil)

func readUserGet(name string) (UserEntry, []string, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return UserEntry{}, nil, fmt.Errorf("read /etc/passwd: %w", err)
	}

	var target UserEntry
	found := false
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		if parts[0] == name || parts[2] == name {
			uid, _ := strconv.Atoi(parts[2])
			gid, _ := strconv.Atoi(parts[3])
			target = UserEntry{
				Name:    parts[0],
				UID:     uid,
				GID:     gid,
				Home:    parts[5],
				Shell:   parts[6],
				Comment: parts[4],
			}
			found = true
			break
		}
	}
	if !found {
		return UserEntry{}, nil, fmt.Errorf("user %q not found", name)
	}

	// Find groups
	groupData, err := os.ReadFile("/etc/group")
	if err != nil {
		return target, nil, nil // return user even if group read fails
	}

	var groups []string
	groupLines := strings.Split(string(groupData), "\n")
	for _, line := range groupLines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		gid, _ := strconv.Atoi(parts[2])
		if gid == target.GID {
			groups = append(groups, parts[0])
		}
		for _, m := range strings.Split(parts[3], ",") {
			if m == target.Name {
				groups = append(groups, parts[0])
				break
			}
		}
	}

	return target, groups, nil
}
