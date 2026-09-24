package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*topicDescribeTask)(nil)

type topicDescribeTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *topicDescribeTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *topicDescribeTask) Name() string { return "kafka.topic.describe" }

func (t *topicDescribeTask) JSONSchema() string { return topicDescribeSchema }

func (t *topicDescribeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topic, err := common.RequireString(params, "topic")
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

	partitions := make([]PartitionInfo, 0, len(td.Partitions))
	for _, p := range td.Partitions.Sorted() {
		partitions = append(partitions, partitionToInfo(p))
	}

	return common.SuccessResult(map[string]any{
		"topic":              td.Topic,
		"internal":           td.IsInternal,
		"partitions":         partitions,
		"partition_count":    len(partitions),
		"replication_factor": td.Partitions.NumReplicas(),
	}), nil
}

const topicDescribeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Topic Describe Parameters",
  "description": "Describe topic partitions, replicas, ISR, leaders and configuration summary.",
  "properties": {
    "topic": {
      "type": "string",
      "description": "Topic name to describe."
    }
  },
  "required": ["topic"]
}`
