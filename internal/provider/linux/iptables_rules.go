package linux

import (
	"context"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// iptablesRulesTask implements the linux.firewall.iptables.rules capability.
type iptablesRulesTask struct{ provider *Provider }

const iptablesRulesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.firewall.iptables.rules Parameters",
  "description": "Parameters for linux.firewall.iptables.rules."
}`

func (t *iptablesRulesTask) Name() string       { return "linux.firewall.iptables.rules" }
func (t *iptablesRulesTask) JSONSchema() string { return iptablesRulesSchema }

func (t *iptablesRulesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("firewall.iptables.rules starting", "capability", t.Name())

	data, err := readIptablesRules(t.provider.CurrentReader())
	if err != nil {
		slog.Info("firewall.iptables.rules failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("firewall.iptables.rules succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*iptablesRulesTask)(nil)

func readIptablesRules(r *procfs.Reader) (map[string]any, error) {
	var tables []map[string]any
	var messages []string

	// Read available table names
	if r.Exists("proc", "net", "ip_tables_names") {
		lines, err := r.ReadFileLines("proc", "net", "ip_tables_names")
		if err == nil {
			for _, line := range lines {
				if line == "" {
					continue
				}
				tables = append(tables, map[string]any{
					"name":   line,
					"family": "ip",
					"chains": []string{},
					"rules":  []string{},
				})
			}
		}
	}

	// Read ipv6 table names
	if r.Exists("proc", "net", "ip6_tables_names") {
		lines, err := r.ReadFileLines("proc", "net", "ip6_tables_names")
		if err == nil {
			for _, line := range lines {
				if line == "" {
					continue
				}
				tables = append(tables, map[string]any{
					"name":   line,
					"family": "ip6",
					"chains": []string{},
					"rules":  []string{},
				})
			}
		}
	}

	if len(tables) == 0 {
		messages = append(messages, "iptables rules not readable via procfs; iptables command or netlink required for full rules")
	} else {
		messages = append(messages, "table names available from procfs; actual chains/rules require iptables command or netlink")
	}

	return map[string]any{
		"tables":   tables,
		"messages": messages,
	}, nil
}
