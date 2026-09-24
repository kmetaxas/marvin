package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*logdirErrorsListTask)(nil)

type logdirErrorsListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *logdirErrorsListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *logdirErrorsListTask) Name() string { return "kafka.logdir.errors.list" }

func (t *logdirErrorsListTask) JSONSchema() string { return logdirErrorsListSchema }

func (t *logdirErrorsListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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
	errorsList := make([]map[string]any, 0)

	for _, d := range sortedLogDirs(all, brokers) {
		if d.Err == nil {
			continue
		}
		// A directory-level error may not carry per-partition detail; report
		// the directory error with empty topic/partition fields.
		errorsList = append(errorsList, map[string]any{
			"broker_id": d.Broker,
			"log_dir":   d.Dir,
			"error":     d.Err.Error(),
			"topic":     "",
			"partition": int32(-1),
		})
	}

	return common.SuccessResult(map[string]any{
		"errors": errorsList,
		"count":  len(errorsList),
	}), nil
}

const logdirErrorsListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "LogDir Errors List Parameters",
  "description": "Find log directories reporting Kafka storage errors.",
  "properties": {
    "broker_ids": {
      "type": "array",
      "items": {"type": "integer"}
    }
  }
}`
