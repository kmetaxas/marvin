package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionISRDescribeTask)(nil)

type partitionISRDescribeTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionISRDescribeTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionISRDescribeTask) Name() string { return "kafka.partition.isr.describe" }

func (t *partitionISRDescribeTask) JSONSchema() string { return partitionISRDescribeSchema }

func (t *partitionISRDescribeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topic, err := common.RequireString(params, "topic")
	if err != nil {
		return common.TaskFailure(err)
	}
	partitions, err := OptionalInt32Slice(params, "partitions")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	details, err := t.currentClient().ListTopics(ctx, topic)
	if err != nil {
		return common.TaskFailure(err)
	}

	td, ok := details[topic]
	if !ok {
		return common.TaskFailure(fmt.Errorf("topic %s not found", topic))
	}
	if td.Err != nil {
		return common.TaskFailure(td.Err)
	}

	partitionSet := make(map[int32]struct{}, len(partitions))
	for _, p := range partitions {
		partitionSet[p] = struct{}{}
	}

	out := make([]map[string]any, 0)
	for _, pd := range td.Partitions.Sorted() {
		if len(partitionSet) > 0 {
			if _, ok := partitionSet[pd.Partition]; !ok {
				continue
			}
		}
		out = append(out, map[string]any{
			"topic":            pd.Topic,
			"partition":        pd.Partition,
			"replicas":         pd.Replicas,
			"isr":              pd.ISR,
			"offline_replicas": pd.OfflineReplicas,
			"leader":           pd.Leader,
			"leader_epoch":     pd.LeaderEpoch,
		})
	}

	return common.SuccessResult(map[string]any{
		"partitions": out,
		"count":      len(out),
	}), nil
}

const partitionISRDescribeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ISR Describe Parameters",
  "description": "Return detailed replica/ISR state for partitions.",
  "properties": {
    "topic": {
      "type": "string",
      "description": "Topic name."
    },
    "partitions": {
      "type": "array",
      "items": {"type": "integer"},
      "description": "Specific partitions. Omit for all."
    }
  },
  "required": ["topic"]
}`
