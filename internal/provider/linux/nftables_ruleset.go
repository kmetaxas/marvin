package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// nftablesRulesetTask implements the linux.firewall.nftables.ruleset capability.
type nftablesRulesetTask struct{ provider *Provider }

const nftablesRulesetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.firewall.nftables.ruleset Parameters",
  "description": "Parameters for linux.firewall.nftables.ruleset."
}`

func (t *nftablesRulesetTask) Name() string       { return "linux.firewall.nftables.ruleset" }
func (t *nftablesRulesetTask) JSONSchema() string { return nftablesRulesetSchema }

func (t *nftablesRulesetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("firewall.nftables.ruleset starting", "capability", t.Name())

	data, err := readNftablesRuleset(t.provider.CurrentReader())
	if err != nil {
		slog.Info("firewall.nftables.ruleset failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("firewall.nftables.ruleset succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*nftablesRulesetTask)(nil)

func readNftablesRuleset(r *procfs.Reader) (map[string]any, error) {
	data, err := readNftablesRulesetFromNetlink()
	if err == nil {
		return data, nil
	}

	procfsData, procfsErr := readNftablesRulesetFromProcfs(r)
	if procfsErr != nil {
		return nil, fmt.Errorf("nftables netlink failed (%w), and procfs fallback failed: %w", err, procfsErr)
	}
	messages, _ := procfsData["messages"].([]string)
	messages = append([]string{fmt.Sprintf("netlink unavailable: %v", err)}, messages...)
	procfsData["messages"] = messages
	procfsData["source"] = "procfs"
	return procfsData, nil
}

func readNftablesRulesetFromProcfs(r *procfs.Reader) (map[string]any, error) {
	var tables []map[string]any
	var messages []string

	// Try /proc/net/nf_tables for basic nftables presence
	if r.Exists("proc", "net", "nf_tables") {
		lines, err := r.ReadFileLines("proc", "net", "nf_tables")
		if err == nil {
			for _, line := range lines {
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					tables = append(tables, map[string]any{
						"name":   fields[0],
						"family": fields[1],
					})
				}
			}
		}
	}

	// Try debugfs path for additional info (often not readable without root)
	if r.Exists("sys", "kernel", "debug", "netfilter", "nf_tables") {
		entries, err := r.ReadDirNames("sys", "kernel", "debug", "netfilter", "nf_tables")
		if err == nil && len(entries) > 0 {
			messages = append(messages, fmt.Sprintf("debugfs entries found: %d", len(entries)))
		}
	}

	if len(tables) == 0 && len(messages) == 0 {
		messages = append(messages, "nftables ruleset not readable via procfs; nft command or netlink required for full ruleset")
	}

	return map[string]any{
		"source":   "procfs",
		"tables":   tables,
		"messages": messages,
	}, nil
}
