package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*brokerDescribeTask)(nil)

type brokerDescribeTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *brokerDescribeTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *brokerDescribeTask) Name() string { return "kafka.broker.describe" }

func (t *brokerDescribeTask) JSONSchema() string { return brokerDescribeSchema }

func (t *brokerDescribeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	brokerID, err := RequireInt32(params, "broker_id")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	meta, err := t.currentClient().Metadata(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	for _, b := range meta.Brokers {
		if b.NodeID == brokerID {
			info := BrokerInfo{
				ID:   b.NodeID,
				Host: b.Host,
				Port: b.Port,
			}
			if b.Rack != nil {
				info.Rack = *b.Rack
			}
			if meta.Controller >= 0 && b.NodeID == meta.Controller {
				info.IsController = true
			}
			return common.SuccessResult(map[string]any{"broker": info}), nil
		}
	}

	return common.TaskFailure(fmt.Errorf("broker %d not found", brokerID))
}

const brokerDescribeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Broker Describe Parameters",
  "description": "Detailed metadata for one broker including endpoints, rack and advertised listeners.",
  "properties": {
    "broker_id": {
      "type": "integer",
      "description": "Broker ID to describe."
    }
  },
  "required": ["broker_id"]
}`
