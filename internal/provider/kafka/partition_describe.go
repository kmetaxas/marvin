package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionDescribeTask)(nil)

type partitionDescribeTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionDescribeTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionDescribeTask) Name() string { return "kafka.partition.describe" }

func (t *partitionDescribeTask) JSONSchema() string { return partitionDescribeSchema }

func (t *partitionDescribeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topic, err := common.RequireString(params, "topic")
	if err != nil {
		return common.TaskFailure(err)
	}
	partition, err := RequireInt32(params, "partition")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	topics, err := t.currentClient().ListTopics(ctx, topic)
	if err != nil {
		return common.TaskFailure(err)
	}

	td, ok := topics[topic]
	if !ok {
		return common.TaskFailure(fmt.Errorf("topic %s not found", topic))
	}
	if td.Err != nil {
		return common.TaskFailure(td.Err)
	}

	pd, ok := td.Partitions[partition]
	if !ok {
		return common.TaskFailure(fmt.Errorf("partition %d not found in topic %s", partition, topic))
	}

	return common.SuccessResult(map[string]any{
		"partition": partitionToInfo(pd),
	}), nil
}

const partitionDescribeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Partition Describe Parameters",
  "description": "Detailed state for one partition.",
  "properties": {
    "topic": {
      "type": "string",
      "description": "Topic name."
    },
    "partition": {
      "type": "integer",
      "description": "Partition number."
    }
  },
  "required": ["topic", "partition"]
}`
