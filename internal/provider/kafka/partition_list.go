package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionListTask)(nil)

type partitionListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionListTask) Name() string { return "kafka.partition.list" }

func (t *partitionListTask) JSONSchema() string { return partitionListSchema }

func (t *partitionListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topics, err := optionalStringSlice(params, "topics")
	if err != nil {
		return common.TaskFailure(err)
	}
	leaderID, err := OptionalInt32(params, "leader_id", -1)
	if err != nil {
		return common.TaskFailure(err)
	}
	replicaID, err := OptionalInt32(params, "replica_id", -1)
	if err != nil {
		return common.TaskFailure(err)
	}
	includeInternal, err := common.OptionalBool(params, "include_internal", false)
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

	partitions := make([]PartitionInfo, 0)
	truncated := false
	for _, td := range details.Sorted() {
		if td.IsInternal && !includeInternal {
			continue
		}
		for _, pd := range td.Partitions.Sorted() {
			if leaderID >= 0 && pd.Leader != leaderID {
				continue
			}
			if replicaID >= 0 && !containsInt32(pd.Replicas, replicaID) {
				continue
			}
			if len(partitions) >= limit {
				truncated = true
				break
			}
			partitions = append(partitions, partitionToInfo(pd))
		}
		if truncated {
			break
		}
	}

	return common.SuccessResult(map[string]any{
		"partitions":      partitions,
		"count":           len(partitions),
		"truncated":       truncated,
		"next_page_token": "",
	}), nil
}

func containsInt32(slice []int32, v int32) bool {
	for _, item := range slice {
		if item == v {
			return true
		}
	}
	return false
}

const partitionListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Partition List Parameters",
  "description": "Return partition metadata across selected/all topics with filters.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific topics."
    },
    "leader_id": {
      "type": "integer",
      "description": "Filter to partitions with this leader."
    },
    "replica_id": {
      "type": "integer",
      "description": "Filter to partitions with this broker in replica set."
    },
    "include_internal": {
      "type": "boolean",
      "default": false
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    },
    "page_token": {
      "type": "string"
    }
  }
}`
