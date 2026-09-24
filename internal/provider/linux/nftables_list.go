//go:build linux

package linux

import (
	"context"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

type nftablesListTask struct{ provider *Provider }

const nftablesListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.firewall.nftables.list Parameters",
  "description": "Optional filters for listing nftables tables, chains, and rules.",
  "properties": {
    "table_name": {"type": "string", "description": "Optional nftables table name filter."},
    "family": {"type": "string", "enum": ["ip", "ip6", "inet", "arp", "bridge", "netdev"], "description": "Optional nftables address family filter."}
  },
  "additionalProperties": false
}`

func (t *nftablesListTask) Name() string       { return "linux.firewall.nftables.list" }
func (t *nftablesListTask) JSONSchema() string { return nftablesListSchema }

func (t *nftablesListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("firewall.nftables.list starting", "capability", t.Name())
	tableName, err := common.OptionalString(params, "table_name", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	family, err := common.OptionalString(params, "family", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	tables, err := listNftablesTablesFromKernel(tableName, family)
	if err != nil {
		slog.Info("firewall.nftables.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("firewall.nftables.list succeeded", "capability", t.Name())
	return common.SuccessResult(map[string]any{"source": "netlink", "tables": tables}), nil
}

var _ task.Task = (*nftablesListTask)(nil)
