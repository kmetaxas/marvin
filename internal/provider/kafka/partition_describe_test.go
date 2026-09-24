package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestPartitionDescribeTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionDescribeTask{}
	assert.Equal(t, "kafka.partition.describe", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionDescribeTaskMissingParams(t *testing.T) {
	t.Parallel()
	task := &partitionDescribeTask{client: &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
}

func TestPartitionDescribeTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
					},
				},
			}, nil
		},
	}
	task := &partitionDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "partition": 0})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	info := data["partition"].(PartitionInfo)
	assert.Equal(t, "orders", info.Topic)
	assert.Equal(t, int32(0), info.Partition)
	assert.Equal(t, int32(1), info.Leader)
}

func TestPartitionDescribeTaskNotFound(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {Topic: "orders", Partitions: map[int32]kadm.PartitionDetail{}},
			}, nil
		},
	}
	task := &partitionDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "partition": 5})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not found")
}
