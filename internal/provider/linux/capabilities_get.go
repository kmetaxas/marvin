package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// capabilitiesGetTask implements the linux.security.capabilities.get capability.
type capabilitiesGetTask struct{ provider *Provider }

const capabilitiesGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.security.capabilities.get Parameters",
  "description": "Parameters for linux.security.capabilities.get.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID to query capabilities for (defaults to self)"
    }
  }
}`

func (t *capabilitiesGetTask) Name() string       { return "linux.security.capabilities.get" }
func (t *capabilitiesGetTask) JSONSchema() string { return capabilitiesGetSchema }

func (t *capabilitiesGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("security.capabilities.get starting", "capability", t.Name())

	pid, _ := common.OptionalInt(params, "pid", 0)
	if pid <= 0 {
		pid = 1 // default to init for listing; caller can pass specific pid
		// Actually self is better, but we can let the proc read handle it.
		pid = os.Getpid()
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	data, err := readCapabilities(t.provider.CurrentReader(), pid)
	if err != nil {
		slog.Info("security.capabilities.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("security.capabilities.get succeeded", "capability", t.Name(), "pid", pid)
	return common.SuccessResult(data), nil
}

var _ task.Task = (*capabilitiesGetTask)(nil)

func readCapabilities(r *procfs.Reader, pid int) (map[string]any, error) {
	statusPath := "proc"
	if pid > 0 {
		statusPath = strconv.Itoa(pid)
	}

	lines, err := r.ReadFileLines("proc", statusPath, "status")
	if err != nil {
		return nil, fmt.Errorf("read /proc/%d/status: %w", pid, err)
	}

	result := map[string]any{
		"pid":         pid,
		"effective":   []string{},
		"permitted":   []string{},
		"inheritable": []string{},
		"ambient":     []string{},
		"bounding":    []string{},
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "CapEff:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				caps := parseCapHex(fields[1])
				result["effective"] = caps
			}
		} else if strings.HasPrefix(line, "CapPrm:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				result["permitted"] = parseCapHex(fields[1])
			}
		} else if strings.HasPrefix(line, "CapInh:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				result["inheritable"] = parseCapHex(fields[1])
			}
		} else if strings.HasPrefix(line, "CapAmb:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				result["ambient"] = parseCapHex(fields[1])
			}
		} else if strings.HasPrefix(line, "CapBnd:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				result["bounding"] = parseCapHex(fields[1])
			}
		}
	}

	return result, nil
}

// parseCapHex parses a hex capability bitmask and returns a slice of capability names.
// For now returns raw hex string; full capability name mapping is large.
func parseCapHex(hexStr string) []string {
	val, err := strconv.ParseUint(hexStr, 0, 64)
	if err != nil {
		return []string{hexStr}
	}
	if val == 0 {
		return []string{}
	}
	// Return hex representation as a single entry for now.
	return []string{fmt.Sprintf("0x%x", val)}
}
