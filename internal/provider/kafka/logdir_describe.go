package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*logdirDescribeTask)(nil)

type logdirDescribeTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *logdirDescribeTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *logdirDescribeTask) Name() string { return "kafka.logdir.describe" }

func (t *logdirDescribeTask) JSONSchema() string { return logdirDescribeSchema }

func (t *logdirDescribeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	brokerIDs, err := OptionalInt32Slice(params, "broker_ids")
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

	all, err := t.currentClient().DescribeAllLogDirs(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	brokers := brokerIDSet(brokerIDs)
	topicsSet := topicSet(topics)

	logdirs := make([]map[string]any, 0)
	for _, d := range sortedLogDirs(all, brokers) {
		replicas := make([]map[string]any, 0)
		for topic, partitions := range d.Topics {
			if !topicMatches(topicsSet, topic) {
				continue
			}
			for partition, p := range partitions {
				replicas = append(replicas, map[string]any{
					"topic":      topic,
					"partition":  partition,
					"size_bytes": p.Size,
					"offset_lag": p.OffsetLag,
					"is_future":  p.IsFuture,
				})
			}
		}

		errStr := ""
		if d.Err != nil {
			errStr = d.Err.Error()
		}

		logdirs = append(logdirs, map[string]any{
			"broker_id": d.Broker,
			"log_dir":   d.Dir,
			"replicas":  replicas,
			"error":     errStr,
		})
	}

	return common.SuccessResult(map[string]any{
		"logdirs": logdirs,
		"count":   len(logdirs),
	}), nil
}

const logdirDescribeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "LogDir Describe Parameters",
  "description": "Describe log directories including replica sizes and errors.",
  "properties": {
    "broker_ids": {
      "type": "array",
      "items": {"type": "integer"}
    },
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific topics."
    }
  }
}`
