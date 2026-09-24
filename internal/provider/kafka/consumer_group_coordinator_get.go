package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*consumerGroupCoordinatorGetTask)(nil)

type consumerGroupCoordinatorGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *consumerGroupCoordinatorGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *consumerGroupCoordinatorGetTask) Name() string {
	return "kafka.consumer_group.coordinator.get"
}

func (t *consumerGroupCoordinatorGetTask) JSONSchema() string {
	return consumerGroupCoordinatorGetSchema
}

func (t *consumerGroupCoordinatorGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	groupID, err := common.RequireString(params, "group_id")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	responses := t.currentClient().FindGroupCoordinators(ctx, groupID)
	resp, ok := responses[groupID]
	if !ok {
		return common.TaskFailure(fmt.Errorf("no coordinator response for group %s", groupID))
	}
	if resp.Err != nil {
		return common.TaskFailure(resp.Err)
	}

	return common.SuccessResult(map[string]any{
		"group_id":    groupID,
		"coordinator": BrokerInfo{ID: resp.NodeID, Host: resp.Host, Port: resp.Port},
	}), nil
}

const consumerGroupCoordinatorGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Consumer Group Coordinator Get Parameters",
  "description": "Find coordinator broker for a group.",
  "properties": {
    "group_id": {
      "type": "string"
    }
  },
  "required": ["group_id"]
}`
