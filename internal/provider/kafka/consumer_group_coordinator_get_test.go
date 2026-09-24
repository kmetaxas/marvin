package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestConsumerGroupCoordinatorGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupCoordinatorGetTask{}
	assert.Equal(t, "kafka.consumer_group.coordinator.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupCoordinatorGetTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &consumerGroupCoordinatorGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: group_id")
}

func TestConsumerGroupCoordinatorGetTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		findGroupCoordinatorsFunc: func(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
			return kadm.FindCoordinatorResponses{
				"order-processor": {Name: "order-processor", NodeID: 1, Host: "kafka-1", Port: 9092},
			}
		},
	}
	task := &consumerGroupCoordinatorGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "order-processor", data["group_id"])
	coordinator := data["coordinator"].(BrokerInfo)
	assert.Equal(t, int32(1), coordinator.ID)
	assert.Equal(t, "kafka-1", coordinator.Host)
	assert.Equal(t, int32(9092), coordinator.Port)
}

func TestConsumerGroupCoordinatorGetTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		findGroupCoordinatorsFunc: func(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
			return kadm.FindCoordinatorResponses{
				"order-processor": {Name: "order-processor", Err: errors.New("coordinator failed")},
			}
		},
	}
	task := &consumerGroupCoordinatorGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "coordinator failed")
}

func TestConsumerGroupCoordinatorGetTaskMissing(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		findGroupCoordinatorsFunc: func(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
			return kadm.FindCoordinatorResponses{}
		},
	}
	task := &consumerGroupCoordinatorGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "no coordinator response")
}
