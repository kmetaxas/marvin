package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*logdirListTask)(nil)

type logdirListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *logdirListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *logdirListTask) Name() string { return "kafka.logdir.list" }

func (t *logdirListTask) JSONSchema() string { return logdirListSchema }

func (t *logdirListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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
		logdirs = append(logdirs, map[string]any{
			"broker_id":        d.Broker,
			"log_dir":          d.Dir,
			"total_size_bytes": d.Size(),
			"topic_count":      len(d.Topics),
		})
	}

	return common.SuccessResult(map[string]any{
		"logdirs": logdirs,
		"count":   len(logdirs),
	}), nil
}

const logdirListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "LogDir List Parameters",
  "description": "List Kafka log directories known for brokers.",
  "properties": {
    "broker_ids": {
      "type": "array",
      "items": {"type": "integer"},
      "description": "Filter to specific brokers. Omit for all."
    }
  }
}`
