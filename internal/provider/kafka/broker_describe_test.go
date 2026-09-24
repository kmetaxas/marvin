package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestBrokerDescribeTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &brokerDescribeTask{}
	assert.Equal(t, "kafka.broker.describe", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestBrokerDescribeTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &brokerDescribeTask{client: client}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"missing broker_id", map[string]any{}, "missing required parameter: broker_id"},
		{"wrong type", map[string]any{"broker_id": "1"}, "must be an integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := task.Execute(context.Background(), tt.params)
			require.NoError(t, err)
			assert.False(t, res.Success)
			assert.Contains(t, res.Error, tt.want)
		})
	}
}

func TestBrokerDescribeTaskSuccess(t *testing.T) {
	t.Parallel()
	rack := "us-east-1a"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
			return kadm.Metadata{
				Controller: 1,
				Brokers: kadm.BrokerDetails{
					{NodeID: 1, Host: "kafka-1", Port: 9092, Rack: &rack},
				},
			}, nil
		},
	}
	task := &brokerDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_id": 1})
	require.NoError(t, err)
	assert.True(t, res.Success)

	broker := res.Data.(map[string]any)["broker"].(BrokerInfo)
	assert.Equal(t, int32(1), broker.ID)
	assert.Equal(t, "kafka-1", broker.Host)
	assert.True(t, broker.IsController)
}

func TestBrokerDescribeTaskNotFound(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
			return kadm.Metadata{Brokers: kadm.BrokerDetails{{NodeID: 1, Host: "kafka-1", Port: 9092}}}, nil
		},
	}
	task := &brokerDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_id": 99})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not found")
}
