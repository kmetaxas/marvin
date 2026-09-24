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

// selinuxStatusTask implements the linux.security.selinux.status capability.
type selinuxStatusTask struct{ provider *Provider }

const selinuxStatusSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.security.selinux.status Parameters",
  "description": "Parameters for linux.security.selinux.status."
}`

func (t *selinuxStatusTask) Name() string       { return "linux.security.selinux.status" }
func (t *selinuxStatusTask) JSONSchema() string { return selinuxStatusSchema }

func (t *selinuxStatusTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("security.selinux.status starting", "capability", t.Name())

	data, err := readSELinuxStatus()
	if err != nil {
		slog.Info("security.selinux.status failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("security.selinux.status succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*selinuxStatusTask)(nil)

func readSELinuxStatus() (map[string]any, error) {
	enforcePath := "/sys/fs/selinux/enforce"
	data, err := os.ReadFile(enforcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"enabled": false,
				"mode":    "not_installed",
				"message": "SELinux not installed or not enabled",
			}, nil
		}
		return nil, fmt.Errorf("read %s: %w", enforcePath, err)
	}

	mode := strings.TrimSpace(string(data))
	state := "unknown"
	switch mode {
	case "0":
		state = "permissive"
	case "1":
		state = "enforcing"
	}

	policy := "unknown"
	if policyData, err := os.ReadFile("/etc/selinux/config"); err == nil {
		for _, line := range strings.Split(string(policyData), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "SELINUXTYPE=") {
				policy = strings.Trim(strings.TrimPrefix(line, "SELINUXTYPE="), "\"")
				break
			}
		}
	}

	return map[string]any{
		"enabled": true,
		"mode":    state,
		"policy":  policy,
	}, nil
}
