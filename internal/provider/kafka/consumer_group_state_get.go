package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*consumerGroupStateGetTask)(nil)

type consumerGroupStateGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *consumerGroupStateGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *consumerGroupStateGetTask) Name() string { return "kafka.consumer_group.state.get" }

func (t *consumerGroupStateGetTask) JSONSchema() string { return consumerGroupStateGetSchema }

func (t *consumerGroupStateGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	groupID, err := common.RequireString(params, "group_id")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	described, err := t.currentClient().DescribeGroups(ctx, groupID)
	if err != nil {
		return common.TaskFailure(err)
	}

	group, err := described.On(groupID, nil)
	if err != nil {
		return common.TaskFailure(err)
	}
	if group.Err != nil {
		return common.TaskFailure(group.Err)
	}

	return common.SuccessResult(map[string]any{
		"group_id":     group.Group,
		"state":        group.State,
		"protocol":     group.Protocol,
		"member_count": len(group.Members),
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}), nil
}

const consumerGroupStateGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Consumer Group State Get Parameters",
  "description": "Lightweight group state lookup, useful for repeated observations of rebalances.",
  "properties": {
    "group_id": {
      "type": "string"
    }
  },
  "required": ["group_id"]
}`
