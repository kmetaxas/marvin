package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionLogdirsGetTask)(nil)

type partitionLogdirsGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionLogdirsGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionLogdirsGetTask) Name() string { return "kafka.partition.logdirs.get" }

func (t *partitionLogdirsGetTask) JSONSchema() string { return partitionLogdirsGetSchema }

func (t *partitionLogdirsGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topic, err := common.OptionalString(params, "topic", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	partition, err := OptionalInt32(params, "partition", -1)
	if err != nil {
		return common.TaskFailure(err)
	}
	brokerIDs, err := OptionalInt32Slice(params, "broker_ids")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	all, err := t.currentClient().DescribeAllLogDirs(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	brokers := brokerIDSet(brokerIDs)

	logdirs := make([]map[string]any, 0)
	for _, d := range sortedLogDirs(all, brokers) {
		for t, partitions := range d.Topics {
			if topic != "" && t != topic {
				continue
			}
			for p, info := range partitions {
				if partition >= 0 && p != partition {
					continue
				}
				logdirs = append(logdirs, map[string]any{
					"broker_id":  d.Broker,
					"log_dir":    d.Dir,
					"topic":      t,
					"partition":  p,
					"size_bytes": info.Size,
					"offset_lag": info.OffsetLag,
					"is_future":  info.IsFuture,
				})
			}
		}
	}

	return common.SuccessResult(map[string]any{
		"logdirs": logdirs,
		"count":   len(logdirs),
	}), nil
}

const partitionLogdirsGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Partition LogDirs Get Parameters",
  "description": "Find which broker/log directory hosts each replica.",
  "properties": {
    "topic": {
      "type": "string",
      "description": "Filter to specific topic."
    },
    "partition": {
      "type": "integer",
      "description": "Filter to specific partition (requires topic)."
    },
    "broker_ids": {
      "type": "array",
      "items": {"type": "integer"},
      "description": "Filter to specific brokers."
    }
  }
}`
