package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*consumerGroupOffsetsGetTask)(nil)

type consumerGroupOffsetsGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *consumerGroupOffsetsGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *consumerGroupOffsetsGetTask) Name() string { return "kafka.consumer_group.offsets.get" }

func (t *consumerGroupOffsetsGetTask) JSONSchema() string { return consumerGroupOffsetsGetSchema }

func (t *consumerGroupOffsetsGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	groupID, err := common.RequireString(params, "group_id")
	if err != nil {
		return common.TaskFailure(err)
	}
	topics, err := optionalStringSlice(params, "topics")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	offsets, err := t.currentClient().FetchOffsets(ctx, groupID)
	if err != nil {
		return common.TaskFailure(err)
	}

	topicFilter := make(map[string]struct{}, len(topics))
	for _, topic := range topics {
		topicFilter[topic] = struct{}{}
	}

	entries := make([]map[string]any, 0)
	for _, o := range offsets.Sorted() {
		if len(topicFilter) > 0 {
			if _, ok := topicFilter[o.Topic]; !ok {
				continue
			}
		}
		entries = append(entries, map[string]any{
			"topic":            o.Topic,
			"partition":        o.Partition,
			"committed_offset": o.At,
			"metadata":         o.Metadata,
		})
	}

	return common.SuccessResult(map[string]any{
		"group_id": groupID,
		"offsets":  entries,
		"count":    len(entries),
	}), nil
}

const consumerGroupOffsetsGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Consumer Group Offsets Get Parameters",
  "description": "Return committed offsets for a group.",
  "properties": {
    "group_id": {
      "type": "string"
    },
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific topics. Omit for all."
    }
  },
  "required": ["group_id"]
}`
