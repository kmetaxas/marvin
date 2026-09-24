package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestConsumerGroupOffsetsGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupOffsetsGetTask{}
	assert.Equal(t, "kafka.consumer_group.offsets.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupOffsetsGetTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &consumerGroupOffsetsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: group_id")
}

func TestConsumerGroupOffsetsGetTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		fetchOffsetsFunc: func(ctx context.Context, group string) (kadm.OffsetResponses, error) {
			return kadm.OffsetResponses{
				"orders": {
					0: {Offset: kadm.Offset{Topic: "orders", Partition: 0, At: 15234000, Metadata: ""}},
					1: {Offset: kadm.Offset{Topic: "orders", Partition: 1, At: 15234001, Metadata: "meta"}},
				},
				"payments": {
					0: {Offset: kadm.Offset{Topic: "payments", Partition: 0, At: 999, Metadata: ""}},
				},
			}, nil
		},
	}
	task := &consumerGroupOffsetsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "order-processor", data["group_id"])
	assert.Equal(t, 3, data["count"])

	offsets := data["offsets"].([]map[string]any)
	require.Len(t, offsets, 3)
	assert.Equal(t, "orders", offsets[0]["topic"])
	assert.Equal(t, int32(0), offsets[0]["partition"])
	assert.Equal(t, int64(15234000), offsets[0]["committed_offset"])
}

func TestConsumerGroupOffsetsGetTaskTopicFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		fetchOffsetsFunc: func(ctx context.Context, group string) (kadm.OffsetResponses, error) {
			return kadm.OffsetResponses{
				"orders":   {0: {Offset: kadm.Offset{Topic: "orders", Partition: 0, At: 1}}},
				"payments": {0: {Offset: kadm.Offset{Topic: "payments", Partition: 0, At: 2}}},
			}, nil
		},
	}
	task := &consumerGroupOffsetsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "g", "topics": []string{"orders"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	offsets := data["offsets"].([]map[string]any)
	assert.Equal(t, "orders", offsets[0]["topic"])
}

func TestConsumerGroupOffsetsGetTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		fetchOffsetsFunc: func(ctx context.Context, group string) (kadm.OffsetResponses, error) {
			return nil, errors.New("fetch failed")
		},
	}
	task := &consumerGroupOffsetsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "g"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "fetch failed")
}
