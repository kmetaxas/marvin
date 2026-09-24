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

// userListTask implements the linux.user.list capability.
type userListTask struct{ provider *Provider }

const userListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.user.list Parameters",
  "description": "Parameters for linux.user.list."
}`

func (t *userListTask) Name() string       { return "linux.user.list" }
func (t *userListTask) JSONSchema() string { return userListSchema }

func (t *userListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("user.list starting", "capability", t.Name())

	users, err := readUserList()
	if err != nil {
		slog.Info("user.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("user.list succeeded", "capability", t.Name(), "count", len(users))
	return common.SuccessResult(map[string]any{"users": users}), nil
}

var _ task.Task = (*userListTask)(nil)

type UserEntry struct {
	Name    string `json:"name"`
	UID     int    `json:"uid"`
	GID     int    `json:"gid"`
	Home    string `json:"home"`
	Shell   string `json:"shell"`
	Comment string `json:"comment,omitempty"`
}

func readUserList() ([]UserEntry, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, fmt.Errorf("read /etc/passwd: %w", err)
	}

	var users []UserEntry
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		uid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		gid, err := strconv.Atoi(parts[3])
		if err != nil {
			continue
		}
		users = append(users, UserEntry{
			Name:    parts[0],
			UID:     uid,
			GID:     gid,
			Home:    parts[5],
			Shell:   parts[6],
			Comment: parts[4],
		})
	}
	return users, nil
}
