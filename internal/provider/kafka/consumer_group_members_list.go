package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*consumerGroupMembersListTask)(nil)

type consumerGroupMembersListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *consumerGroupMembersListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *consumerGroupMembersListTask) Name() string { return "kafka.consumer_group.members.list" }

func (t *consumerGroupMembersListTask) JSONSchema() string { return consumerGroupMembersListSchema }

func (t *consumerGroupMembersListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	members := make([]map[string]any, 0, len(group.Members))
	for _, m := range group.Members {
		assignments, _ := memberAssignments(m)
		members = append(members, map[string]any{
			"member_id":   m.MemberID,
			"client_id":   m.ClientID,
			"client_host": m.ClientHost,
			"assignment":  assignments,
		})
	}

	return common.SuccessResult(map[string]any{
		"group_id":     group.Group,
		"members":      members,
		"member_count": len(members),
	}), nil
}

const consumerGroupMembersListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Consumer Group Members List Parameters",
  "description": "Detailed members, client IDs, hosts and assignments.",
  "properties": {
    "group_id": {
      "type": "string"
    }
  },
  "required": ["group_id"]
}`
