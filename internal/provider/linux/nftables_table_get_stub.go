//go:build !linux

package linux

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

type nftablesTableGetTask struct{ provider *Provider }

const nftablesTableGetSchema = `{"type":"object","title":"linux.firewall.nftables.table.get Parameters"}`

func (t *nftablesTableGetTask) Name() string       { return "linux.firewall.nftables.table.get" }
func (t *nftablesTableGetTask) JSONSchema() string { return nftablesTableGetSchema }
func (t *nftablesTableGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	return common.TaskFailure(fmt.Errorf("nftables netlink is only available on linux"))
}

var _ task.Task = (*nftablesTableGetTask)(nil)
