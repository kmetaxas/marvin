//go:build linux

package linux

import (
	"context"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

type nftablesTableGetTask struct{ provider *Provider }

const nftablesTableGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.firewall.nftables.table.get Parameters",
  "description": "Identifies one nftables table to retrieve from the kernel netlink interface.",
  "properties": {
    "table_name": {"type": "string", "minLength": 1, "description": "Required nftables table name."},
    "family": {"type": "string", "enum": ["ip", "ip6", "inet", "arp", "bridge", "netdev"], "default": "inet", "description": "nftables address family."}
  },
  "required": ["table_name"],
  "additionalProperties": false
}`

func (t *nftablesTableGetTask) Name() string       { return "linux.firewall.nftables.table.get" }
func (t *nftablesTableGetTask) JSONSchema() string { return nftablesTableGetSchema }

func (t *nftablesTableGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("firewall.nftables.table.get starting", "capability", t.Name())
	tableName, err := common.RequireString(params, "table_name")
	if err != nil {
		return common.TaskFailure(err)
	}
	family, err := common.OptionalString(params, "family", "inet")
	if err != nil {
		return common.TaskFailure(err)
	}

	table, err := getNftablesTableFromKernel(tableName, family)
	if err != nil {
		slog.Info("firewall.nftables.table.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("firewall.nftables.table.get succeeded", "capability", t.Name())
	return common.SuccessResult(map[string]any{"source": "netlink", "table": table}), nil
}

var _ task.Task = (*nftablesTableGetTask)(nil)
