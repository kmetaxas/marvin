package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionUnderReplicatedListTask)(nil)

type partitionUnderReplicatedListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionUnderReplicatedListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionUnderReplicatedListTask) Name() string {
	return "kafka.partition.under_replicated.list"
}

func (t *partitionUnderReplicatedListTask) JSONSchema() string {
	return partitionUnderReplicatedListSchema
}

func (t *partitionUnderReplicatedListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topics, err := optionalStringSlice(params, "topics")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := NormalizeLimitKafka(params)
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	details, err := t.currentClient().ListTopics(ctx, topics...)
	if err != nil {
		return common.TaskFailure(err)
	}

	underReplicated := make([]PartitionInfo, 0)
	truncated := false
	for _, td := range details.Sorted() {
		for _, pd := range td.Partitions.Sorted() {
			if len(pd.ISR) >= len(pd.Replicas) {
				continue
			}
			if len(underReplicated) >= limit {
				truncated = true
				break
			}
			underReplicated = append(underReplicated, partitionToInfo(pd))
		}
		if truncated {
			break
		}
	}

	return common.SuccessResult(map[string]any{
		"under_replicated_partitions": underReplicated,
		"count":                       len(underReplicated),
		"truncated":                   truncated,
	}), nil
}

const partitionUnderReplicatedListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Under-Replicated Partition List Parameters",
  "description": "Return partitions where ISR is smaller than replica set.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"}
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    }
  }
}`
