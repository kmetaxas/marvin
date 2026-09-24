//go:build !linux

package linux

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

type nftablesListTask struct{ provider *Provider }

const nftablesListSchema = `{"type":"object","title":"linux.firewall.nftables.list Parameters"}`

func (t *nftablesListTask) Name() string       { return "linux.firewall.nftables.list" }
func (t *nftablesListTask) JSONSchema() string { return nftablesListSchema }
func (t *nftablesListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	return common.TaskFailure(fmt.Errorf("nftables netlink is only available on linux"))
}

var _ task.Task = (*nftablesListTask)(nil)
