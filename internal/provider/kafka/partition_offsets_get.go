package kafka

import (
	"context"
	"fmt"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionOffsetsGetTask)(nil)

type partitionOffsetsGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionOffsetsGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionOffsetsGetTask) Name() string { return "kafka.partition.offsets.get" }

func (t *partitionOffsetsGetTask) JSONSchema() string { return partitionOffsetsGetSchema }

func (t *partitionOffsetsGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topic, err := common.RequireString(params, "topic")
	if err != nil {
		return common.TaskFailure(err)
	}
	partitions, err := OptionalInt32Slice(params, "partitions")
	if err != nil {
		return common.TaskFailure(err)
	}
	timestamp, err := common.OptionalString(params, "timestamp", "latest")
	if err != nil {
		return common.TaskFailure(err)
	}
	if timestamp != "earliest" && timestamp != "latest" {
		return common.TaskFailure(fmt.Errorf("parameter timestamp must be 'earliest' or 'latest'"))
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	startOffsets, err := t.currentClient().ListStartOffsets(ctx, topic)
	if err != nil {
		return common.TaskFailure(err)
	}
	endOffsets, err := t.currentClient().ListEndOffsets(ctx, topic)
	if err != nil {
		return common.TaskFailure(err)
	}

	partitionSet := make(map[int32]struct{}, len(partitions))
	for _, p := range partitions {
		partitionSet[p] = struct{}{}
	}

	offsets := make([]map[string]any, 0)
	for _, end := range endOffsets[topic] {
		if len(partitionSet) > 0 {
			if _, ok := partitionSet[end.Partition]; !ok {
				continue
			}
		}
		entry := map[string]any{
			"topic":     end.Topic,
			"partition": end.Partition,
			"latest":    end.Offset,
		}
		if start, ok := startOffsets.Lookup(topic, end.Partition); ok {
			entry["earliest"] = start.Offset
		} else {
			entry["earliest"] = int64(0)
		}
		if end.Timestamp >= 0 {
			entry["timestamp_ms"] = end.Timestamp
		}
		offsets = append(offsets, entry)
	}

	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i]["partition"].(int32) < offsets[j]["partition"].(int32)
	})

	return common.SuccessResult(map[string]any{
		"offsets": offsets,
		"count":   len(offsets),
	}), nil
}

const partitionOffsetsGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Partition Offsets Get Parameters",
  "description": "Retrieve earliest/latest offsets for partitions.",
  "properties": {
    "topic": {
      "type": "string",
      "description": "Topic name."
    },
    "partitions": {
      "type": "array",
      "items": {"type": "integer"},
      "description": "Specific partitions. Omit for all."
    },
    "timestamp": {
      "oneOf": [
        {"type": "string", "format": "date-time"},
        {"type": "string", "enum": ["earliest", "latest"]}
      ],
      "default": "latest",
      "description": "Offset to retrieve: earliest, latest, or a specific timestamp."
    }
  },
  "required": ["topic"]
}`
