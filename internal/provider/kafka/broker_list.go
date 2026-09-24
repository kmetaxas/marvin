package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*brokerListTask)(nil)

type brokerListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *brokerListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *brokerListTask) Name() string { return "kafka.broker.list" }

func (t *brokerListTask) JSONSchema() string { return brokerListSchema }

func (t *brokerListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	brokers, err := t.currentClient().ListBrokers(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	infos := brokersToInfo(brokers, -1)

	return common.SuccessResult(map[string]any{
		"brokers": infos,
		"count":   len(infos),
	}), nil
}

const brokerListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Broker List Parameters",
  "description": "List brokers currently visible in cluster metadata.",
  "properties": {}
}`
