package linux

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// loginHistoryTask implements the linux.login.history capability.
type loginHistoryTask struct{ provider *Provider }

const loginHistorySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.login.history Parameters",
  "description": "Parameters for linux.login.history.",
  "properties": {
    "limit": {
      "type": "integer",
      "description": "Maximum number of entries to return",
      "default": 100
    }
  }
}`

func (t *loginHistoryTask) Name() string       { return "linux.login.history" }
func (t *loginHistoryTask) JSONSchema() string { return loginHistorySchema }

func (t *loginHistoryTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("login.history starting", "capability", t.Name())

	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	entries, err := readLoginHistory(limit)
	if err != nil {
		slog.Info("login.history failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("login.history succeeded", "capability", t.Name(), "count", len(entries))
	return common.SuccessResult(map[string]any{"entries": entries}), nil
}

var _ task.Task = (*loginHistoryTask)(nil)

type LoginHistoryEntry struct {
	User     string `json:"user,omitempty"`
	TTY      string `json:"tty,omitempty"`
	Host     string `json:"host,omitempty"`
	Time     string `json:"time,omitempty"`
	PID      int32  `json:"pid,omitempty"`
	TypeName string `json:"type"`
}

func readLoginHistory(limit int) ([]LoginHistoryEntry, error) {
	data, err := os.ReadFile("/var/log/wtmp")
	if err != nil {
		return nil, fmt.Errorf("read wtmp: %w", err)
	}

	var entries []LoginHistoryEntry
	recordSize := binary.Size(UtmpEntry{})
	for i := 0; i+recordSize <= len(data); i += recordSize {
		var ut UtmpEntry
		buf := data[i : i+recordSize]
		if err := binary.Read(newSliceReader(buf), binary.LittleEndian, &ut); err != nil {
			continue
		}
		// Types: 7 = USER_PROCESS (login), 8 = DEAD_PROCESS (logout)
		var typeName string
		switch ut.Type {
		case 7:
			typeName = "login"
		case 8:
			typeName = "logout"
		case 1:
			typeName = "run_level"
		case 2:
			typeName = "boot_time"
		case 5:
			typeName = "init_process"
		case 6:
			typeName = "login_process"
		default:
			continue
		}
		entries = append(entries, LoginHistoryEntry{
			User:     stringFromBytes(ut.User[:]),
			TTY:      stringFromBytes(ut.Line[:]),
			Host:     stringFromBytes(ut.Host[:]),
			Time:     time.Unix(int64(ut.Tv.Sec), 0).UTC().Format(time.RFC3339),
			PID:      ut.Pid,
			TypeName: typeName,
		})
		if len(entries) >= limit {
			break
		}
	}

	// Reverse to show most recent first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}
