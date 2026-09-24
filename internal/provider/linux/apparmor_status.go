package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// apparmorStatusTask implements the linux.security.apparmor.status capability.
type apparmorStatusTask struct{ provider *Provider }

const apparmorStatusSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.security.apparmor.status Parameters",
  "description": "Parameters for linux.security.apparmor.status."
}`

func (t *apparmorStatusTask) Name() string       { return "linux.security.apparmor.status" }
func (t *apparmorStatusTask) JSONSchema() string { return apparmorStatusSchema }

func (t *apparmorStatusTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("security.apparmor.status starting", "capability", t.Name())

	data, err := readApparmorStatus()
	if err != nil {
		slog.Info("security.apparmor.status failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("security.apparmor.status succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*apparmorStatusTask)(nil)

func readApparmorStatus() (map[string]any, error) {
	profilesPath := "/sys/kernel/security/apparmor/profiles"
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"enabled":  false,
				"profiles": []string{},
				"message":  "AppArmor not installed or not enabled",
			}, nil
		}
		return nil, fmt.Errorf("read %s: %w", profilesPath, err)
	}

	var profiles []map[string]string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			profiles = append(profiles, map[string]string{
				"name": parts[0],
				"mode": parts[1],
			})
		} else if len(parts) == 1 {
			profiles = append(profiles, map[string]string{
				"name": parts[0],
				"mode": "unknown",
			})
		}
	}

	return map[string]any{
		"enabled":       true,
		"profile_count": len(profiles),
		"profiles":      profiles,
	}, nil
}
