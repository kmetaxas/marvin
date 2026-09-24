package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestConsumerGroupOffsetCommitVerifyTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupOffsetCommitVerifyTask{}
	assert.Equal(t, "kafka.consumer_group.offset_commit.verify", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupOffsetCommitVerifyTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &consumerGroupOffsetCommitVerifyTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: group_id")
}

func TestConsumerGroupOffsetCommitVerifyTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		findGroupCoordinatorsFunc: func(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
			return kadm.FindCoordinatorResponses{
				"order-processor": {Name: "order-processor", NodeID: 1, Host: "kafka-1", Port: 9092},
			}
		},
		fetchOffsetsFunc: func(ctx context.Context, group string) (kadm.OffsetResponses, error) {
			return kadm.OffsetResponses{}, nil
		},
	}
	task := &consumerGroupOffsetCommitVerifyTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "order-processor", data["group_id"])
	assert.True(t, data["coordinator_reachable"].(bool))
	assert.Equal(t, "passed", data["protocol_check"])
	assert.Equal(t, "passed", data["auth_check"])
	assert.True(t, data["would_succeed"].(bool))

	coordinator := data["coordinator"].(BrokerInfo)
	assert.Equal(t, int32(1), coordinator.ID)
}

func TestConsumerGroupOffsetCommitVerifyTaskAuthFailure(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		findGroupCoordinatorsFunc: func(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
			return kadm.FindCoordinatorResponses{
				"order-processor": {Name: "order-processor", NodeID: 1, Host: "kafka-1", Port: 9092},
			}
		},
		fetchOffsetsFunc: func(ctx context.Context, group string) (kadm.OffsetResponses, error) {
			return nil, errors.New("auth failed")
		},
	}
	task := &consumerGroupOffsetCommitVerifyTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "failed", data["auth_check"])
	assert.False(t, data["would_succeed"].(bool))
}

func TestConsumerGroupOffsetCommitVerifyTaskCoordinatorError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		findGroupCoordinatorsFunc: func(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
			return kadm.FindCoordinatorResponses{
				"order-processor": {Name: "order-processor", Err: errors.New("coordinator failed")},
			}
		},
	}
	task := &consumerGroupOffsetCommitVerifyTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "coordinator failed")
}

func TestConsumerGroupOffsetCommitVerifyTaskNoMutation(t *testing.T) {
	t.Parallel()
	fetchCalls := 0
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		findGroupCoordinatorsFunc: func(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
			return kadm.FindCoordinatorResponses{
				"order-processor": {Name: "order-processor", NodeID: 1, Host: "kafka-1", Port: 9092},
			}
		},
		fetchOffsetsFunc: func(ctx context.Context, group string) (kadm.OffsetResponses, error) {
			fetchCalls++
			return kadm.OffsetResponses{
				"orders": {0: {Offset: kadm.Offset{Topic: "orders", Partition: 0, At: 42}}},
			}, nil
		},
	}
	task := &consumerGroupOffsetCommitVerifyTask{client: client}

	before, err := client.FetchOffsets(context.Background(), "order-processor")
	require.NoError(t, err)

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	after, err := client.FetchOffsets(context.Background(), "order-processor")
	require.NoError(t, err)

	assert.Equal(t, before, after, "offsets must be unchanged after verify")
}
