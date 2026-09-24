package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemTimeTask returns current system time, timezone, and NTP status.
type systemTimeTask struct{ provider *Provider }

const systemTimeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "System Time Parameters",
  "description": "Parameters for retrieving system time and timezone."
}`

func (t *systemTimeTask) Name() string       { return "linux.system.time" }
func (t *systemTimeTask) JSONSchema() string { return systemTimeSchema }

func (t *systemTimeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("system.time starting", "capability", t.Name())

	now := time.Now()
	zone, offset := now.Zone()

	tzName := zone
	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		tzName = string(data)
		if len(tzName) > 0 && tzName[len(tzName)-1] == '\n' {
			tzName = tzName[:len(tzName)-1]
		}
	}

	data := map[string]any{
		"current_time": now.Format(time.RFC3339),
		"timezone":     tzName,
		"utc_offset":   fmt.Sprintf("%+03d:%02d", offset/3600, (offset%3600)/60),
		"unix_seconds": now.Unix(),
		"unix_nanos":   now.UnixNano(),
	}

	slog.Info("system.time succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*systemTimeTask)(nil)
