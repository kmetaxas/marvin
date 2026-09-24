package kafka

import (
	"context"
	"fmt"
	"slices"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*logdirReplicaSizesTask)(nil)

type logdirReplicaSizesTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *logdirReplicaSizesTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *logdirReplicaSizesTask) Name() string { return "kafka.logdir.replica_sizes.get" }

func (t *logdirReplicaSizesTask) JSONSchema() string { return logdirReplicaSizesSchema }

func (t *logdirReplicaSizesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	brokerIDs, err := OptionalInt32Slice(params, "broker_ids")
	if err != nil {
		return common.TaskFailure(err)
	}
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

	all, err := t.currentClient().DescribeAllLogDirs(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	brokers := brokerIDSet(brokerIDs)
	topicsSet := topicSet(topics)

	replicas := make([]map[string]any, 0)
	var totalSize int64
	truncated := false

	for _, d := range sortedLogDirs(all, brokers) {
		topicNames := make([]string, 0, len(d.Topics))
		for t := range d.Topics {
			topicNames = append(topicNames, t)
		}
		slices.Sort(topicNames)
		for _, topic := range topicNames {
			partitions := d.Topics[topic]
			if !topicMatches(topicsSet, topic) {
				continue
			}
			// Sort partition IDs for deterministic iteration.
			sortedPartitions := make([]int32, 0, len(partitions))
			for p := range partitions {
				sortedPartitions = append(sortedPartitions, p)
			}
			slices.Sort(sortedPartitions)
			for _, partition := range sortedPartitions {
				p := partitions[partition]
				if len(replicas) >= limit {
					truncated = true
					break
				}
				replicas = append(replicas, map[string]any{
					"broker_id":  d.Broker,
					"log_dir":    d.Dir,
					"topic":      topic,
					"partition":  partition,
					"size_bytes": p.Size,
				})
				totalSize += p.Size
			}
			if truncated {
				break
			}
		}
		if truncated {
			break
		}
	}

	return common.SuccessResult(map[string]any{
		"replicas":         replicas,
		"count":            len(replicas),
		"total_size_bytes": totalSize,
		"truncated":        truncated,
	}), nil
}

const logdirReplicaSizesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "LogDir Replica Sizes Parameters",
  "description": "Retrieve replica sizes grouped by broker/logdir/topic.",
  "properties": {
    "broker_ids": {
      "type": "array",
      "items": {"type": "integer"}
    },
    "topics": {
      "type": "array",
      "items": {"type": "string"}
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10000,
      "default": 1000
    }
  }
}`
