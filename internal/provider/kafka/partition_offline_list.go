package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionOfflineListTask)(nil)

type partitionOfflineListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionOfflineListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionOfflineListTask) Name() string { return "kafka.partition.offline.list" }

func (t *partitionOfflineListTask) JSONSchema() string { return partitionOfflineListSchema }

func (t *partitionOfflineListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	offline := make([]PartitionInfo, 0)
	truncated := false
	for _, td := range details.Sorted() {
		for _, pd := range td.Partitions.Sorted() {
			if pd.Leader >= 0 {
				continue
			}
			if len(offline) >= limit {
				truncated = true
				break
			}
			offline = append(offline, partitionToInfo(pd))
		}
		if truncated {
			break
		}
	}

	return common.SuccessResult(map[string]any{
		"offline_partitions": offline,
		"count":              len(offline),
		"truncated":          truncated,
	}), nil
}

const partitionOfflineListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Offline Partition List Parameters",
  "description": "Return partitions with no active leader.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific topics."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    }
  }
}`
