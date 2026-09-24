package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestPartitionOffsetsGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionOffsetsGetTask{}
	assert.Equal(t, "kafka.partition.offsets.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionOffsetsGetTaskMissingTopic(t *testing.T) {
	t.Parallel()
	task := &partitionOffsetsGetTask{client: &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "topic")
}

func TestPartitionOffsetsGetTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listStartOffsetsFunc: func(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
			return kadm.ListedOffsets{
				"orders": {
					0: {Topic: "orders", Partition: 0, Offset: 0},
					1: {Topic: "orders", Partition: 1, Offset: 5},
				},
			}, nil
		},
		listEndOffsetsFunc: func(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
			return kadm.ListedOffsets{
				"orders": {
					0: {Topic: "orders", Partition: 0, Offset: 15234567, Timestamp: 1720000000000},
					1: {Topic: "orders", Partition: 1, Offset: 100},
				},
			}, nil
		},
	}
	task := &partitionOffsetsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	offsets := data["offsets"].([]map[string]any)
	require.Len(t, offsets, 2)
	assert.Equal(t, int32(0), offsets[0]["partition"])
	assert.Equal(t, int64(0), offsets[0]["earliest"])
	assert.Equal(t, int64(15234567), offsets[0]["latest"])
	assert.Equal(t, int64(1720000000000), offsets[0]["timestamp_ms"])
}

func TestPartitionOffsetsGetTaskPartitionFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listStartOffsetsFunc: func(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
			return kadm.ListedOffsets{
				"orders": {
					0: {Topic: "orders", Partition: 0, Offset: 0},
					1: {Topic: "orders", Partition: 1, Offset: 5},
				},
			}, nil
		},
		listEndOffsetsFunc: func(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
			return kadm.ListedOffsets{
				"orders": {
					0: {Topic: "orders", Partition: 0, Offset: 100},
					1: {Topic: "orders", Partition: 1, Offset: 200},
				},
			}, nil
		},
	}
	task := &partitionOffsetsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "partitions": []any{1}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	offsets := data["offsets"].([]map[string]any)
	assert.Equal(t, int32(1), offsets[0]["partition"])
}

func TestPartitionOffsetsGetTaskInvalidTimestamp(t *testing.T) {
	t.Parallel()
	task := &partitionOffsetsGetTask{client: &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}}
	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "timestamp": "bogus"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "timestamp")
}
