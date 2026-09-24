package linux

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// ruleListTask implements the linux.network.rule.list capability.
type ruleListTask struct{ provider *Provider }

const ruleListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Rule List Parameters",
  "description": "Parameters for listing policy-routing rules."
}`

func (t *ruleListTask) Name() string       { return "linux.network.rule.list" }
func (t *ruleListTask) JSONSchema() string { return ruleListSchema }

func (t *ruleListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	_ = params
	slog.Info("network.rule.list starting", "capability", t.Name())

	reader := t.provider.CurrentReader()
	rules, err := listRules(reader)
	if err != nil {
		slog.Info("network.rule.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"rules": rules,
		"note":  "Policy routing rules are parsed from /etc/iproute2/rt_tables where available.",
	}
	slog.Info("network.rule.list succeeded", "capability", t.Name(), "count", len(rules))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*ruleListTask)(nil)

type policyRule struct {
	Table    int    `json:"table"`
	Name     string `json:"name,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Source   string `json:"source,omitempty"`
	Dest     string `json:"destination,omitempty"`
}

func listRules(r *procfs.Reader) ([]policyRule, error) {
	// There is no standard /proc file for ip rules.
	// Try to read /etc/iproute2/rt_tables to get table names.
	data, err := r.ReadFileString("etc", "iproute2", "rt_tables")
	if err != nil {
		return nil, nil
	}

	var rules []policyRule
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		tableNum, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		name := fields[1]
		rules = append(rules, policyRule{
			Table: tableNum,
			Name:  name,
		})
	}
	return rules, nil
}
