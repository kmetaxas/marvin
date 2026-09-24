package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestBrokerListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &brokerListTask{}
	assert.Equal(t, "kafka.broker.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestBrokerListTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &brokerListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestBrokerListTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listBrokersFunc: func(ctx context.Context) (kadm.BrokerDetails, error) {
			return kadm.BrokerDetails{
				{NodeID: 1, Host: "kafka-1", Port: 9092},
				{NodeID: 2, Host: "kafka-2", Port: 9092},
			}, nil
		},
	}
	task := &brokerListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	brokers := data["brokers"].([]BrokerInfo)
	require.Len(t, brokers, 2)
	assert.Equal(t, int32(1), brokers[0].ID)
}

func TestBrokerListTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listBrokersFunc: func(ctx context.Context) (kadm.BrokerDetails, error) {
			return nil, errors.New("list failed")
		},
	}
	task := &brokerListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "list failed")
}
